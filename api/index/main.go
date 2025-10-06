package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os"
    "encoding/base64"
    "encoding/json"
    "goclassifieds/lib/repo"
    "goclassifieds/lib/search" // Contains Query, TopLevelQuery, Bool, ExecuteSubQuery, AggregationResult, etc.
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/google/go-github/v46/github"
    "golang.org/x/oauth2"
)

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

    // Determine if aggregation, sorting, or projection is requested.
    // We use the control structures from the FIRST query for the final result set processing.
    var aggregationRequest *search.Aggregation
    var sortRequest []search.SortField
    var limit int
    var offset int
    var sourceFields []string
    var scoreModifiersRequest *search.FunctionScore // NEW: Score modifiers variable

    if len(queriesToExecute) > 0 {
        firstQuery := queriesToExecute[0]
        aggregationRequest = firstQuery.Aggs
        sortRequest = firstQuery.Sort
        limit = firstQuery.Limit
        offset = firstQuery.Offset
        sourceFields = firstQuery.Source
        scoreModifiersRequest = firstQuery.ScoreModifiers // NEW: Extract score modifiers (assuming the field is named ScoreModifiers)

        if aggregationRequest != nil {
            log.Printf("Aggregation requested: %s", aggregationRequest.Name)
        }
        if len(sortRequest) > 0 {
            log.Printf("Sorting requested on %d fields.", len(sortRequest))
        }
        if limit > 0 || offset > 0 {
            log.Printf("Paging requested: Limit=%d, Offset=%d.", limit, offset)
        }
        if scoreModifiersRequest != nil { // NEW: Log if score modification is requested
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

    // NOTE: Reading PEM file is sensitive and assumes a secure execution environment.
    pemFilePath := fmt.Sprintf("rtc-vertigo-%s.private-key.pem", os.Getenv("STAGE"))
    pem, err := os.ReadFile(pemFilePath)
    if err != nil {
        log.Printf("Failed to read PEM file '%s': %v", pemFilePath, err)
        return events.APIGatewayProxyResponse{
            StatusCode: http.StatusInternalServerError,
            Body:       "failed to load GitHub app PEM file",
        }, nil
    }

    // Get installation token
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

    // Create authenticated GitHub client
    srcToken := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: *installationToken.Token})
    httpClient := oauth2.NewClient(ctx, srcToken)
    githubRestClient := github.NewClient(httpClient)

    // --- 3. Main Execution Loop for Union Queries ---

    // Slice to hold all matching documents (map[string]interface{}) for aggregation, sorting, and projection.
    allDocuments := make([]map[string]interface{}, 0)

    // Loop through each query defined in the Union
    for i, currentQuery := range queriesToExecute {

        log.Printf("--- STARTING QUERY %d/%d (Index: %s) ---", i+1, len(queriesToExecute), currentQuery.Index)

        index := currentQuery.Index

        // --- 3a. Retrieve Index Configuration for the current query (UNMODIFIED) ---
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
            continue // Skip this query, but continue with the Union
        }

        // --- 3b. Determine Content Path (Scoped Search using Composite or Root) (UNMODIFIED) ---

        var contentPath string
        fieldsInterface, fieldsOk := indexObject["fields"].([]interface{})
        if !fieldsOk {
            log.Printf("Query %d skipped: Index configuration missing 'fields'.", i+1)
            continue
        }

        if len(currentQuery.Composite) > 0 {
            // Scoped Search: Build the path using values from the Composite map
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

            // Decode the file name (which holds the Base64 JSON data)
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
                // Store the document map for aggregation OR final processing
                allDocuments = append(allDocuments, itemData)
            }
        }

        log.Printf("Query %d finished. Found %d matching results (total documents for processing: %d).", i+1, len(allDocuments))
    }

    // --- NEW STEP: Apply Custom Score Modifiers (Function Score) ---
    // This must run after all base scores are calculated, but before sorting or aggregation.
    if scoreModifiersRequest != nil && len(allDocuments) > 0 {
        log.Printf("--- Applying %d custom score function(s) to %d documents. ---", 
            len(scoreModifiersRequest.Functions), len(allDocuments))
        
        // This function must be implemented in lib/search/search.go
        search.ApplyScoreModifiers(allDocuments, scoreModifiersRequest)
        
        log.Print("--- Custom score function application complete. ---")
    }

    // --- 4. Final Response (Aggregation vs. Standard) ---

    if aggregationRequest != nil {
        log.Printf("--- AGGREGATION MODE. Processing %d total documents. ---", len(allDocuments))

        // Execute the recursive aggregation function on the combined document set
        // The scores are now included in 'allDocuments' and available for 'top_hits' aggregation if requested.
        resultsBuckets := search.ExecuteAggregation(allDocuments, aggregationRequest)

        aggResult := search.AggregationResult{
            Name:    aggregationRequest.Name,
            Buckets: resultsBuckets,
        }

        responseBody, err := json.Marshal(aggResult)
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

        // MODIFICATION 3: Apply Sorting (Default to _score descending if no sort requested)
        if len(sortRequest) == 0 {
            log.Print("Handler: No sort specified, defaulting to _score DESC.")
            sortRequest = []search.SortField{{Field: "_score", Order: search.SortDesc}}
        }
        search.ApplySort(allDocuments, sortRequest)

        // 4b. Apply Field Projection (Source)
        if len(sourceFields) > 0 {
            // search.ProjectFields now implicitly preserves _score
            allDocuments = search.ProjectFields(allDocuments, sourceFields)
        }

        // 4c. Apply Paging (Limit/Offset)
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
    log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile) // Maintain useful logging flags
    lambda.Start(handler)
}