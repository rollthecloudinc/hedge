package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"encoding/base64"
	"encoding/json"
	"strings" 
	"goclassifieds/lib/repo"
	"goclassifieds/lib/search" 
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/google/go-github/v46/github"
	"golang.org/x/oauth2"
)

// ====================================================================
// === CORE LOGIC STRUCTS =============================================
// ====================================================================

// UnionQueryInput encapsulates all parameters required to execute a union query
// and apply the final result controls.
type UnionQueryInput struct {
	Ctx                   context.Context
	Owner                 string
	RepoName              string
	Branch                string
	GitHubRestClient      *github.Client
	QueriesToExecute      []search.Query
	AggregationMap        map[string]search.Aggregation
	SortRequest           []search.SortField
	Limit                 int
	Offset                int
	SourceFields          []string
	ScoreModifiersRequest *search.FunctionScore
}

// SearchResultPayload is the vendor-agnostic return structure for the search core logic.
type SearchResultPayload struct {
	StatusCode    int
	BodyData      interface{} // Will hold []map[string]interface{} (documents) or search.AggregationResult
	ErrorMessage  string
	IsAggregation bool
}

// ====================================================================
// === CORE EXECUTION LOGIC: executeUnionQueries (Decoupled) ==========
// ====================================================================

// executeUnionQueries performs all data retrieval, filtering, scoring, and post-processing,
// returning a standardized payload and an error.
func executeUnionQueries(input *UnionQueryInput) (*SearchResultPayload, error) {

	allDocuments := make([]map[string]interface{}, 0)
	stage := os.Getenv("STAGE")

	// --- 3. Main Execution Loop for Union Queries (Loading/Filtering/Scoring) ---
	for i, currentQuery := range input.QueriesToExecute {
		log.Printf("--- STARTING QUERY %d/%d (Index: %s) ---", i+1, len(input.QueriesToExecute), currentQuery.Index)
		index := currentQuery.Index

		// [BLOCK A: Index Config and Path Determination] 
		getIndexInput := &search.GetIndexConfigurationInput{
			GithubClient: input.GitHubRestClient,
			Owner:        input.Owner,
			Stage:        stage,
			Repo:         input.Owner + "/" + input.RepoName,
			Branch:       input.Branch,
			Id:           index,
		}

		indexObject, err := search.GetIndexById(getIndexInput)
		if err != nil || indexObject == nil {
			log.Printf("Query %d skipped: Error retrieving index config for ID '%s'.", i+1, index)
			continue
		}

		var contentPath string
		fieldsInterface, fieldsOk := indexObject["fields"].([]interface{})
		if !fieldsOk {
			log.Printf("Query %d skipped: Index configuration missing 'fields'.", i+1)
			continue
		}

		if len(currentQuery.Composite) > 0 {
			compositePath := ""
			for idx, f := range fieldsInterface {
				fStr := f.(string)
				compositeVal, found := currentQuery.Composite[fStr]
				if found {
					compositePath += fmt.Sprintf("%v", compositeVal)
				}
				if idx < (len(fieldsInterface) - 1) {
					compositePath += ":"
				}
			}
			contentPath = compositePath
		} else {
			// Return a payload indicating an error
			return &SearchResultPayload{
				StatusCode: http.StatusInternalServerError,
				ErrorMessage: "Query configuration missing 'Composite'",
			}, nil
		}

		repoToFetch, ok := indexObject["repoName"].(string)
		if !ok || repoToFetch == "" {
			log.Printf("Query %d skipped: Index configuration missing 'repoName'.", i+1)
			continue
		}
		// [END BLOCK A]

		// Fetch Directory Contents
		_, dirContents, _, err := input.GitHubRestClient.Repositories.GetContents(
			input.Ctx, input.Owner, repoToFetch, contentPath,
			&github.RepositoryContentGetOptions{Ref: input.Branch},
		)

		if err != nil || dirContents == nil {
			log.Printf("Query %d: Failed to list contents at path %s: %v. Continuing union.", i+1, contentPath, err)
			continue
		}

		// Filter and Accumulate Results
		for _, content := range dirContents {
			if content.GetType() != "file" || content.GetName() == "" {
				continue
			}

			decodedBytes, err := base64.StdEncoding.DecodeString(content.GetName())
			if err != nil {
				continue
			}
			itemBody := string(decodedBytes)

			var itemData map[string]interface{}
			if err := json.Unmarshal([]byte(itemBody), &itemData); err != nil {
				continue
			}

			// EXECUTE BOOL EVALUATION to capture the score
			matched, score := currentQuery.Bool.Evaluate(itemData, input.Ctx, input.GitHubRestClient, getIndexInput)

			if matched {
				itemData["_score"] = score
				allDocuments = append(allDocuments, itemData)
			}
		}
	}

	// --- 3e. Apply Custom Score Modifiers (Function Score) ---
	if input.ScoreModifiersRequest != nil && len(allDocuments) > 0 {
		search.ApplyScoreModifiers(allDocuments, input.ScoreModifiersRequest)
	}

	// --- 4. Final Processing (Aggregation vs. Standard) ---

	if len(input.AggregationMap) > 0 { // Aggregation Mode
		// 1. Separate Top-Level Aggregations
		bucketAggregations := make(map[string]search.Aggregation)
		pipelineAggregations := make(map[string]search.Aggregation)
		for aggName, agg := range input.AggregationMap {
			aggType := strings.ToLower(agg.Type)
			if aggType == "stats_bucket" || aggType == "bucket_script" { 
				pipelineAggregations[aggName] = agg
			} else {
				bucketAggregations[aggName] = agg
			}
		}

		// 2. Execute Primary Bucket Aggregations
		allAggResults := make(map[string]search.AggregationResult)
		primaryAggName := "" 
		for name, agg := range bucketAggregations {
			result := search.ExecuteAggregation(allDocuments, &agg)
			allAggResults[name] = result
			if primaryAggName == "" {
				primaryAggName = name
			}
		}

		// 3. Execute Top-Level Pipeline Aggregations
		finalPipelineMetrics := make(map[string]interface{})
		for pipeName, pipeAgg := range pipelineAggregations {
			if pipeAgg.Path == "" { continue }
			pathParts := strings.Split(pipeAgg.Path, ">") 
			targetAggName := pathParts[0]

			if targetResult, found := allAggResults[targetAggName]; found {
				pipeResult := search.ExecuteTopLevelPipeline(targetResult.Buckets, pipeAgg, pathParts[1:])
				finalPipelineMetrics[pipeName] = pipeResult
			}
		}
		
		// 4. Construct Final Aggregation Result
		var finalAggResult search.AggregationResult
		if primaryAggName != "" {
			finalAggResult = allAggResults[primaryAggName]
		}
		for k, v := range finalPipelineMetrics {
			finalAggResult.PipelineMetrics[k] = v
		}

		// Return a successful payload with the aggregation result
		return &SearchResultPayload{
			StatusCode: http.StatusOK,
			BodyData: finalAggResult,
			IsAggregation: true,
		}, nil

	} else {
		// Standard Search or Union Query: Apply result controls
		
		// Apply Sorting 
		if len(input.SortRequest) == 0 {
			input.SortRequest = []search.SortField{{Field: "_score", Order: search.SortDesc}}
		}
		search.ApplySort(allDocuments, input.SortRequest)

		// Apply Field Projection (Source)
		if len(input.SourceFields) > 0 {
			allDocuments = search.ProjectFields(allDocuments, input.SourceFields)
		}

		// Apply Paging (Limit/Offset)
		pagedDocuments := search.ApplyPaging(allDocuments, input.Limit, input.Offset)

		// Return a successful payload with the paged documents
		return &SearchResultPayload{
			StatusCode: http.StatusOK,
			BodyData: pagedDocuments,
			IsAggregation: false,
		}, nil
	}
}

// ====================================================================
// === SERVICE LAYER: executeSearchRequest (Handles Setup & Response) =
// ====================================================================

// executeSearchRequest performs all setup (GitHub client/token) and executes the core union/search logic,
// then translates the vendor-agnostic result into the AWS-specific response.
func executeSearchRequest(ctx context.Context, owner, repoName string, requestBody []byte) (events.APIGatewayProxyResponse, error) {
	
	branch := "dev"
	
	// 1. Unmarshal Query and Determine Controls
	var topLevelQuery search.TopLevelQuery
	if err := json.Unmarshal(requestBody, &topLevelQuery); err != nil {
		log.Printf("Error parsing top-level query structure: %s", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Invalid search query format. Must contain 'query' or 'union'.",
		}, nil
	}

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

	// 2. Setup GitHub Client and Token
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

	// 3. Create the single input struct
	input := &UnionQueryInput{
		Ctx:                   ctx,
		Owner:                 owner,
		RepoName:              repoName,
		Branch:                branch,
		GitHubRestClient:      githubRestClient,
		QueriesToExecute:      queriesToExecute,
		AggregationMap:        firstQuery.Aggs,
		SortRequest:           firstQuery.Sort,
		Limit:                 firstQuery.Limit,
		Offset:                firstQuery.Offset,
		SourceFields:          firstQuery.Source,
		ScoreModifiersRequest: firstQuery.ScoreModifiers,
	}

	// 4. Delegate to the core execution logic
	payload, err := executeUnionQueries(input)
	if err != nil {
		log.Printf("Internal error in executeUnionQueries: %v", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "An unexpected error occurred during query execution.",
		}, nil
	}

	// 5. Transform the vendor-agnostic payload into the AWS response
	if payload.StatusCode != http.StatusOK {
		// Handle documented errors (e.g., missing 'Composite' path)
		return events.APIGatewayProxyResponse{
			StatusCode: payload.StatusCode,
			Body:       payload.ErrorMessage,
		}, nil
	}
    
    // Marshal the successful body data (documents or aggregation result)
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
// === HANDLER: Minimal Lambda Entry Point ============================
// ====================================================================

// handler is the entry point for the AWS Lambda function, delegating all logic to the service layer.
func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	// 1. Extract minimal required path/request data
	owner := request.PathParameters["owner"]
	repoName := request.PathParameters["repo"]

	// 2. Validate HTTP Method
	if request.HTTPMethod != "POST" {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusMethodNotAllowed,
			Body:       "Only POST is supported for complex search/union queries.",
		}, nil
	}

	// 3. Delegate to the decoupled execution function
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