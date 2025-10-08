package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"encoding/json"
	
	"goclassifieds/lib/repo"
	"goclassifieds/lib/search" 
	
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/google/go-github/v46/github"
	"golang.org/x/oauth2"
)

// ====================================================================
// === SERVICE LAYER: executeSearchRequest (Engine Instantiation) =====
// ====================================================================

func executeSearchRequest(ctx context.Context, owner, repoName string, requestBody []byte) (events.APIGatewayProxyResponse, error) {
	
	branch := "dev"
	
	// 1. Unmarshal Query and Determine Controls (UNCHANGED)
	var topLevelQuery search.TopLevelQuery
	if err := json.Unmarshal(requestBody, &topLevelQuery); err != nil {
		log.Printf("Error parsing top-level query structure: %s", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Invalid search query format. Must contain 'query' or 'union'.",
		}, nil
	}
	
	// ... (Query extraction logic) ...

	var queriesToExecute []search.Query
	if topLevelQuery.Query != nil {
		queriesToExecute = []search.Query{*topLevelQuery.Query}
	} else if topLevelQuery.Union != nil {
		queriesToExecute = topLevelQuery.Union.Queries
	} else {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Request body must contain 'query' or 'union'.",
		}, nil
	}
    
    if len(queriesToExecute) == 0 {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "No queries found to execute.",
		}, nil
	}

	firstQuery := queriesToExecute[0]

	// 2. Setup GitHub Client and Token (UNCHANGED)
	githubAppID := os.Getenv("GITHUB_APP_ID")
	if githubAppID == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "environment variable GITHUB_APP_ID is missing"}, nil
	}
	pemFilePath := fmt.Sprintf("rtc-vertigo-%s.private-key.pem", os.Getenv("STAGE"))
	pem, err := os.ReadFile(pemFilePath)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "failed to load GitHub app PEM file"}, nil
	}

	getTokenInput := &repo.GetInstallationTokenInput{
		GithubAppPem: pem,
		Owner:        owner,
		GithubAppId:  githubAppID,
	}
	installationToken, err := repo.GetInstallationToken(getTokenInput)
	if err != nil {
		log.Printf("Error generating GitHub installation token for owner '%s': %v", owner, err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Error generating GitHub installation token for owner"}, nil
	}

	srcToken := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: *installationToken.Token})
	httpClient := oauth2.NewClient(ctx, srcToken)
	githubRestClient := github.NewClient(httpClient)

	// ===============================================================
	// === 3. Dependency Injection and Engine Instantiation (NEW) ======
	// ===============================================================
	
	// Create the concrete loader instance (the dependency)
	githubLoader := search.NewGitHubLoader(githubRestClient)
	
	// Inject the loader into the search engine
	engine := search.NewSearchEngine(githubLoader) 
	
	// ===============================================================

	// 4. Create the single input struct (UNCHANGED)
	input := &search.UnionQueryInput{
		Ctx:                   ctx,
		Owner:                 owner,
		RepoName:              repoName,
		Branch:                branch,
		QueriesToExecute:      queriesToExecute,
		AggregationMap:        firstQuery.Aggs,
		SortRequest:           firstQuery.Sort,
		Limit:                 firstQuery.Limit,
		Offset:                firstQuery.Offset,
		SourceFields:          firstQuery.Source,
		ScoreModifiersRequest: firstQuery.ScoreModifiers,
	}

	// 5. Delegate to the core engine method
	payload, err := engine.ExecuteUnionQuery(input)
	if err != nil {
		log.Printf("Internal error in ExecuteUnionQuery: %v", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "An unexpected error occurred during query execution.",
		}, nil
	}

	// 6. Transform the payload into the AWS response (UNCHANGED)
	if payload.StatusCode != http.StatusOK {
		return events.APIGatewayProxyResponse{
			StatusCode: payload.StatusCode,
			Body:       payload.ErrorMessage,
		}, nil
	}
    
    responseBody, marshalErr := json.Marshal(payload.BodyData)
    if marshalErr != nil {
        log.Printf("Error marshaling final payload: %v", marshalErr)
        return events.APIGatewayProxyResponse{
            StatusCode: http.StatusInternalServerError,
            Body:       "Error generating final response body.",
        }, nil
    }

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(responseBody),
	}, nil
}

// ====================================================================
// === HANDLER: Minimal Lambda Entry Point (UNCHANGED) ================
// ====================================================================

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	owner := request.PathParameters["owner"]
	repoName := request.PathParameters["repo"]

	if request.HTTPMethod != "POST" {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusMethodNotAllowed,
			Body:       "Only POST is supported for complex search/union queries.",
		}, nil
	}

	return executeSearchRequest(
		ctx,
		owner,
		repoName,
		[]byte(request.Body),
	)
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	lambda.Start(handler)
}