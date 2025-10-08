package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os"
    "encoding/base64"
    "encoding/json"
    "strings" // Added for aggregation type comparison
    "goclassifieds/lib/repo"
    "goclassifieds/lib/search" // Contains Query, TopLevelQuery, Bool, ExecuteSubQuery, AggregationResult, etc.
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/google/go-github/v46/github"
    "golang.org/x/oauth2"
)

// NOTE: This handler assumes the 'search.Query' struct has been updated to:
// Aggs map[string]search.Aggregation `json:"aggs,omitempty"`

// handler is the entry point for the AWS Lambda function, managing all search and retrieval logic.
func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

    owner := request.PathParameters["owner"]
    repoName := request.PathParameters["repo"]
    branch := "dev"

    // --- 1. Determine Search Mode & Unmarshal Union/Single Query (UNMODIFIED) ---
    isSearchRequest := request.HTTPMethod == "POST"
    if !isSearchRequest {
        log.Print("Handler received non-POST request; complex queries require POST.")
        return events.APIGatewayProxyResponse{
            StatusCode: http.StatusMethodNotAllowed,
            Body:       "Only POST is supported for complex search/union queries.",
        }, nil
    }

    var topLevelQuery search.TopLevelQuery
    var queriesToExecute []search.Query

    err := json.Unmarshal([]byte(request.Body), &topLevelQuery)
    if err != nil {
        log.Printf("Error parsing top-level query structure: %s", err)
        return events.APIGatewayProxyResponse{
            StatusCode: http.StatusBadRequest,
            Body:       "Invalid search query format. Must contain 'query' or 'union'.",
        }, nil
    }

    // Determine the set of queries to execute (Union or Single)
    if topLevelQuery.Query != nil {
        log.Print("Detected single query request.")
        queriesToExecute = []search.Query{*topLevelQuery.Query}
    } else if topLevelQuery.Union != nil {
        log.Printf("Detected union query request with %d sub-queries.", len(topLevelQuery.Union.Queries))
        queriesToExecute = topLevelQuery.Union.Queries
    } else {
        log.Print("Request body missing 'query' or 'union' fields.")
        return events.APIGatewayProxyResponse{
            StatusCode: http.StatusBadRequest,
            Body:       "Request body must contain 'query' or 'union'.",
        }, nil
    }

    // Determine controls. We use controls from the FIRST query for the final result set processing.
    var aggregationMap map[string]search.Aggregation // <-- CHANGED: Now a map
    var sortRequest []search.SortField
    var limit int
    var offset int
    var sourceFields []string
    var scoreModifiersRequest *search.FunctionScore

    if len(queriesToExecute) > 0 {
        firstQuery := queriesToExecute[0]
        aggregationMap = firstQuery.Aggs // <-- CHANGED: Get the map
        sortRequest = firstQuery.Sort
        limit = firstQuery.Limit
        offset = firstQuery.Offset
        sourceFields = firstQuery.Source
        scoreModifiersRequest = firstQuery.ScoreModifiers

        if len(aggregationMap) > 0 { // Check map length
            log.Printf("Aggregation map detected with %d top-level aggregations.", len(aggregationMap))
        }
        if len(sortRequest) > 0 {
            log.Printf("Sorting requested on %d fields.", len(sortRequest))
        }
        if limit > 0 || offset > 0 {
            log.Printf("Paging requested: Limit=%d, Offset=%d.", limit, offset)
        }
        if scoreModifiersRequest != nil {
            log.Printf("Score modification requested with %d function(s).", len(scoreModifiersRequest.Functions))
        }
    }

    // --- 2. Setup GitHub Client and Token (UNMODIFIED) ---
    githubAppID := os.Getenv("GITHUB_APP_ID")
    if githubAppID == "" {
        log.Print("environment variable GITHUB_APP_ID is missing")
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

    // --- 3. Main Execution Loop for Union Queries ---

    allDocuments := make([]map[string]interface{}, 0)

    for i, currentQuery := range queriesToExecute {
        log.Printf("--- STARTING QUERY %d/%d (Index: %s) ---", i+1, len(queriesToExecute), currentQuery.Index)
        index := currentQuery.Index

        // --- 3a. Retrieve Index Configuration (UNMODIFIED) ---
        // ... (Index configuration retrieval logic remains here) ...
        
        // [BLOCK A: Index Config and Path Determination] 
        getIndexInput := &search.GetIndexConfigurationInput{
            GithubClient: githubRestClient,
            Owner:        owner,
            Stage:        os.Getenv("STAGE"),
            Repo:         owner + "/" + repoName,
            Branch:       branch,
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
            log.Printf("Query %d: Using Composite path: %s", i+1, contentPath)
        } else {
            log.Print("Query configuration missing or invalid 'composite' for full search.")
            return events.APIGatewayProxyResponse{
                StatusCode: http.StatusInternalServerError,
                Body:       "Query configuration missing 'Composite'",
            }, nil
        }

        repoToFetch, ok := indexObject["repoName"].(string)
        if !ok || repoToFetch == "" {
            log.Printf("Query %d skipped: Index configuration missing 'repoName'.", i+1)
            continue
        }
        // [END BLOCK A]

        // --- 3c. Fetch Directory Contents (UNMODIFIED) ---
        log.Printf("Query %d: Fetching contents from repo %s at path %s.", i+1, repoToFetch, contentPath)
        _, dirContents, _, err := githubRestClient.Repositories.GetContents(
            ctx, owner, repoToFetch, contentPath,
            &github.RepositoryContentGetOptions{Ref: branch},
        )

        if err != nil || dirContents == nil {
            log.Printf("Query %d: Failed to list contents at path %s: %v. Continuing union.", i+1, contentPath, err)
            continue
        }

        // --- 3d. Filter and Accumulate Results (MODIFIED FOR SCORING) ---
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

            // MODIFICATION 1: EXECUTE BOOL EVALUATION to capture the score
            matched, score := currentQuery.Bool.Evaluate(itemData, ctx, githubRestClient, getIndexInput)

            if matched {
                // MODIFICATION 2: Store the calculated score in the document map
                itemData["_score"] = score
                allDocuments = append(allDocuments, itemData)
            }
        }

        log.Printf("Query %d finished. Found %d matching results (total documents for processing: %d).", i+1, len(allDocuments))
    }

    // --- NEW STEP: Apply Custom Score Modifiers (Function Score) ---
    if scoreModifiersRequest != nil && len(allDocuments) > 0 {
        log.Printf("--- Applying %d custom score function(s) to %d documents. ---", 
            len(scoreModifiersRequest.Functions), len(allDocuments))
        
        search.ApplyScoreModifiers(allDocuments, scoreModifiersRequest)
        
        log.Print("--- Custom score function application complete. ---")
    }

    // --- 4. Final Response (Aggregation vs. Standard) ---

    if len(aggregationMap) > 0 { // Check map length
        log.Printf("--- AGGREGATION MODE. Processing %d total documents for %d aggs. ---", len(allDocuments), len(aggregationMap))

        // 1. Separate Top-Level Aggregations
        bucketAggregations := make(map[string]search.Aggregation)
        pipelineAggregations := make(map[string]search.Aggregation)
        
        for aggName, agg := range aggregationMap {
            aggType := strings.ToLower(agg.Type)
            // Identify pipeline aggs (stats_bucket, bucket_script, etc.)
            if aggType == "stats_bucket" || aggType == "bucket_script" { 
                pipelineAggregations[aggName] = agg
            } else {
                bucketAggregations[aggName] = agg // Identify primary aggs (terms, range, etc.)
            }
        }

        // 2. Execute Primary Bucket Aggregations
        allAggResults := make(map[string]search.AggregationResult)
        primaryAggName := "" // Used to pick the final response structure

        for name, agg := range bucketAggregations {
            log.Printf("Executing primary bucket aggregation: %s (%s)", name, agg.Type)
            // ExecuteAggregation handles all nested metrics and sub-aggs recursively
            result := search.ExecuteAggregation(allDocuments, &agg)
            allAggResults[name] = result
            primaryAggName = name 
        }

        // 3. Execute Top-Level Pipeline Aggregations
        finalPipelineMetrics := make(map[string]interface{})
        
        for pipeName, pipeAgg := range pipelineAggregations {
            log.Printf("Executing pipeline aggregation: %s (%s)", pipeName, pipeAgg.Type)

            // The 'path' determines the target aggregation and metric
            if pipeAgg.Path == "" {
                log.Printf("Pipeline Aggregation '%s' skipped: Missing 'path'.", pipeName)
                continue
            }
            
            // Expected path format: "target_agg_name>metric_name" or "target_agg_name>sub_agg_name>metric_name"
            pathParts := strings.Split(pipeAgg.Path, ">") 
            targetAggName := pathParts[0]

            if targetResult, found := allAggResults[targetAggName]; found {
                // NOTE: 'executeTopLevelPipeline' must be implemented in lib/search/search.go
                // It takes the target result's buckets and the path to run the pipeline logic (e.g., StatsBucket)
                pipeResult := search.ExecuteTopLevelPipeline(targetResult.Buckets, pipeAgg, pathParts[1:])
                finalPipelineMetrics[pipeName] = pipeResult
            } else {
                log.Printf("Pipeline Aggregation '%s' target '%s' not found. Skipping.", pipeName, targetAggName)
            }
        }
        
        // 4. Construct Final Response: Use the FIRST primary result as the base
        var finalAggResult search.AggregationResult
        if primaryAggName != "" {
            finalAggResult = allAggResults[primaryAggName]
        } else {
            // If only pipeline aggs were requested (e.g., to just run stats on everything)
            // We create an empty result object.
            finalAggResult = search.AggregationResult{} 
        }

        // Merge all calculated top-level pipeline metrics into the final result
        // These metrics are placed at the root level alongside the primary aggregation buckets
        for k, v := range finalPipelineMetrics {
            finalAggResult.PipelineMetrics[k] = v
        }

        // Marshal the final, comprehensive AggregationResult
        responseBody, err := json.Marshal(finalAggResult) 
        if err != nil {
            log.Printf("Error marshaling aggregation result: %v", err)
            return events.APIGatewayProxyResponse{
                StatusCode: http.StatusInternalServerError,
                Body:       "Error generating aggregation response.",
            }, nil
        }

        return events.APIGatewayProxyResponse{
            StatusCode: http.StatusOK,
            Headers:    map[string]string{"Content-Type": "application/json"},
            Body:       string(responseBody),
        }, nil

    } else {
        // Standard Search or Union Query: Apply result controls and return raw list of documents
        log.Printf("--- STANDARD UNION MODE. Applying Result Controls. ---")

        // Apply Sorting (Default to _score descending if no sort requested)
        if len(sortRequest) == 0 {
            log.Print("Handler: No sort specified, defaulting to _score DESC.")
            sortRequest = []search.SortField{{Field: "_score", Order: search.SortDesc}}
        }
        search.ApplySort(allDocuments, sortRequest)

        // Apply Field Projection (Source)
        if len(sourceFields) > 0 {
            allDocuments = search.ProjectFields(allDocuments, sourceFields)
        }

        // Apply Paging (Limit/Offset)
        pagedDocuments := search.ApplyPaging(allDocuments, limit, offset)

        log.Printf("--- UNION COMPLETED. Total documents after paging: %d ---", len(pagedDocuments))

        // Marshal the final, processed documents
        responseBody, err := json.Marshal(pagedDocuments)
        if err != nil {
            log.Printf("Error marshaling final results: %v", err)
            return events.APIGatewayProxyResponse{
                StatusCode: http.StatusInternalServerError,
                Body:       "Error generating search response.",
            }, nil
        }

        return events.APIGatewayProxyResponse{
            StatusCode: http.StatusOK,
            Headers:    map[string]string{"Content-Type": "application/json"},
            Body:       string(responseBody),
        }, nil
    }
}

func main() {
    log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
    lambda.Start(handler)
}