package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"encoding/base64"
	"encoding/json"
	"goclassifieds/lib/repo"
	"goclassifieds/lib/search"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/google/go-github/v46/github"
	"golang.org/x/oauth2"
)

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	var searchQuery search.Query 

	owner := request.PathParameters["owner"]
	repoName := request.PathParameters["repo"]
	
	// Initial index comes from the path, but will be overridden for POST
	index := request.PathParameters["index"] 
	branch := "dev"
    
    // --- 1. Determine Search Mode vs. Retrieval Mode & Unmarshal Query ---
	isSearchRequest := request.HTTPMethod == "POST"
	if isSearchRequest {
		err := json.Unmarshal([]byte(request.Body), &searchQuery)
		if err != nil {
			log.Printf("Error parsing search query: %s", err)
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusBadRequest,
				Body:       "Invalid search query",
			}, nil
		}
		log.Printf("Search query (filtering): %+v", searchQuery)
        
        // Use the index from the JSON body if it's a POST request and present
        if searchQuery.Index != "" {
            log.Printf("Overriding index from path ('%s') with index from query body ('%s').", index, searchQuery.Index)
            index = searchQuery.Index
        } else {
            log.Printf("Warning: POST query body is missing 'index'. Falling back to path parameter: '%s'.", index)
        }
	}

    // --- 2. Setup GitHub Client and Token ---
	githubAppID := os.Getenv("GITHUB_APP_ID")
    if githubAppID == "" {
        err := fmt.Errorf("environment variable GITHUB_APP_ID is missing")
        log.Print(err)
        return events.APIGatewayProxyResponse{
            StatusCode: http.StatusInternalServerError,
            Body:       "environment variable GITHUB_APP_ID is missing",
        }, nil
    }

	pemFilePath := fmt.Sprintf("rtc-vertigo-%s.private-key.pem", os.Getenv("STAGE"))
	pem, err := os.ReadFile(pemFilePath)
	if err != nil {
		log.Printf("Failed to read PEM file '%s': %v", pemFilePath, err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "failed to load GitHub app PEM file",
		}, nil
	}

	getTokenInput := &repo.GetInstallationTokenInput{
		GithubAppPem: pem,
		Owner:        owner,
		GithubAppId:  githubAppID,
	}
	installationToken, err := repo.GetInstallationToken(getTokenInput)
	if err != nil {
		log.Printf("Error generating GitHub installation token for owner '%s': %v", owner, err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "Error generating GitHub installation token for owner",
		}, nil
	}

	srcToken := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: *installationToken.Token})
	httpClient := oauth2.NewClient(ctx, srcToken)
	githubRestClient := github.NewClient(httpClient)

    // --- 3. Retrieve Index Configuration ---
	getIndexInput := &search.GetIndexConfigurationInput{
		GithubClient: githubRestClient,
		Stage: os.Getenv("STAGE"),
		Repo: owner + "/" + repoName,
		Branch: branch,
		Id: index, 
	}

	log.Printf("Retrieving index configuration for ID: %s", index)

	indexObject, err := search.GetIndexById(getIndexInput)
	if err != nil {
		log.Printf("Error retrieving index configuration: %v", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "Error retrieving index configuration",
		}, nil
	}
    if indexObject == nil {
         log.Printf("Index configuration not found for ID: %s", index)
         return events.APIGatewayProxyResponse{
            StatusCode: http.StatusNotFound,
            Body:       "Index configuration not found",
        }, nil
    }

	// ------------------------------------------------------------------
	// 4. RETRIEVAL LOGIC: Determine Path to Fetch (GET vs. POST/Composite)
	// ------------------------------------------------------------------
    
	var contentPath string 
    
    // Get fields defining the composite key from the index config
    fieldsInterface, fieldsOk := indexObject["fields"].([]interface{})
    if !fieldsOk {
         log.Print("Index configuration missing or invalid 'fields' in index object.")
         return events.APIGatewayProxyResponse{
            StatusCode: http.StatusInternalServerError,
            Body:       "Index configuration missing 'fields'",
        }, nil
    }
    
	if isSearchRequest {
		// Search Mode (POST)
        
        compositePath := ""
        
        // Build the path using values from the Composite map in the query body
        if len(searchQuery.Composite) > 0 {
            log.Printf("Using Composite map from query body to scope search.")
            
            for idx, f := range fieldsInterface {
                fStr := f.(string)
                
                compositeVal, found := searchQuery.Composite[fStr]
                if !found {
                    // If a composite key is missing, it implies a path part is empty
                    compositePath += ""
                } else {
                    // Convert the composite value to string (JSON unmarshals primitives to interface{})
                    compositePath += fmt.Sprintf("%v", compositeVal)
                }

                if idx < (len(fieldsInterface) - 1) {
                    compositePath += ":"
                }
            }
            contentPath = compositePath
            
        } else {

            log.Print("Query configuration missing or invalid 'composite' for full search.")
            return events.APIGatewayProxyResponse{
                StatusCode: http.StatusInternalServerError,
                Body:       "Query configuration missing 'Composite'",
            }, nil

        }

	} else {
		// Retrieval Mode (GET) - Uses QueryStringParameters
		uniqueKeyValues := ""
        log.Print("Extracting field values for key retrieval from query string.")

		for idx, f := range fieldsInterface {
			fStr := f.(string)
			uniqueKeyValues += request.QueryStringParameters[fStr]
			if idx < (len(fieldsInterface) - 1) {
				uniqueKeyValues += ":"
			}
		}
		contentPath = uniqueKeyValues
	}
    
    // Ensure we have a repoName
    repoToFetch, ok := indexObject["repoName"].(string)
    if !ok || repoToFetch == "" {
        log.Print("Index configuration missing 'repoName'")
        return events.APIGatewayProxyResponse{
            StatusCode: http.StatusInternalServerError,
            Body:       "Index configuration missing 'repoName'",
        }, nil
    }

	log.Printf("Fetching contents from path: %s in repo %s.", contentPath, repoToFetch)

    // --- 5. Fetch Directory Contents ---
	_, dirContents, _, err := githubRestClient.Repositories.GetContents(
		ctx, owner, repoToFetch, contentPath, 
		&github.RepositoryContentGetOptions{Ref: branch},
	)

	if err != nil {
		log.Printf("failed to list contents of the index at path %s: %v", contentPath, err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusNotFound,
			Body:       "Failed to list contents of the index or path not found.",
		}, nil
	}

	if dirContents == nil {
		log.Printf("index empty or not accessible at path %s", contentPath)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusNotFound,
			Body:       "Index empty or not accessible.",
		}, nil
	}

	// ------------------------------------------------------------------
	// 6. Data Processing and Filtering Logic
	// ------------------------------------------------------------------

	results := make([]string, 0)
    
	for _, content := range dirContents {
		if content.GetType() != "file" || content.GetName() == "" {
			continue
		}

		// Assume file name is the Base64-encoded JSON data
		decodedBytes, err := base64.StdEncoding.DecodeString(content.GetName())
		if err != nil {
			log.Printf("Error decoding Base64 name '%s': %v", content.GetName(), err)
			continue
		}
		itemBody := string(decodedBytes)
		
		var match bool
		
		if isSearchRequest {
			var itemData map[string]interface{}
			
			// Unmarshal for dot-notation support
			if err := json.Unmarshal([]byte(itemBody), &itemData); err != nil {
				log.Printf("Error unmarshaling item '%s' for filtering: %v", content.GetName(), err)
				continue
			}
			
			// Execute the recursive Bool evaluation
			match = searchQuery.Bool.Evaluate(itemData)
		} else {
			// Retrieval Mode: Always matches
			match = true
		}

		if match {
			results = append(results, itemBody)
		}
	}

	log.Printf("Index Request completed. Found %d results.", len(results))
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       fmt.Sprintf("[%s]", strings.Join(results, ",")),
	}, nil
}

func main() {
	log.SetFlags(0)
	lambda.Start(handler)
}