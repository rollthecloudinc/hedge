package search

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"encoding/base64"
	"encoding/json"
	"strings" 
	
	// External Dependencies
	"github.com/google/go-github/v46/github" 
	
	// Note: We assume 'repo' is defined elsewhere, but its types aren't needed here.
	// GetIndexConfigurationInput is defined within the 'search' package.
)

// ====================================================================
// === CORE LOGIC STRUCTS (Defined in the 'search' package) ===========
// ====================================================================

// UnionQueryInput encapsulates all parameters required to execute a union query
// and apply the final result controls.
type UnionQueryInput struct {
	Ctx                   context.Context
	Owner                 string
	RepoName              string
	Branch                string
	GitHubRestClient      *github.Client
	QueriesToExecute      []Query // Type is now local
	AggregationMap        map[string]Aggregation // Type is now local
	SortRequest           []SortField // Type is now local
	Limit                 int
	Offset                int
	SourceFields          []string
	ScoreModifiersRequest *FunctionScore // Type is now local
}

// SearchResultPayload is the vendor-agnostic return structure for the search core logic.
type SearchResultPayload struct {
	StatusCode    int
	BodyData      interface{} // Will hold []map[string]interface{} (documents) or AggregationResult
	ErrorMessage  string
	IsAggregation bool
}

// ====================================================================
// === CORE EXECUTION FUNCTION (executeUnionQueries) ==================
// ====================================================================

// executeUnionQueries performs all data retrieval, filtering, scoring, and post-processing,
// returning a standardized payload and an error.
func ExecuteUnionQueries(input *UnionQueryInput) (*SearchResultPayload, error) {

	allDocuments := make([]map[string]interface{}, 0)
	stage := os.Getenv("STAGE")

	// --- 3. Main Execution Loop for Union Queries (Loading/Filtering/Scoring) ---
	for i, currentQuery := range input.QueriesToExecute {
		log.Printf("--- STARTING QUERY %d/%d (Index: %s) ---", i+1, len(input.QueriesToExecute), currentQuery.Index)
		index := currentQuery.Index

		// [BLOCK A: Index Config and Path Determination] 
		getIndexInput := &GetIndexConfigurationInput{ // Type is now local
			GithubClient: input.GitHubRestClient,
			Owner:        input.Owner,
			Stage:        stage,
			Repo:         input.Owner + "/" + input.RepoName,
			Branch:       input.Branch,
			Id:           index,
		}

		indexObject, err := GetIndexById(getIndexInput) // Function is now local
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
			// currentQuery.Bool is a struct/interface within the 'search' package
			matched, score := currentQuery.Bool.Evaluate(itemData, input.Ctx, input.GitHubRestClient, getIndexInput)

			if matched {
				itemData["_score"] = score
				allDocuments = append(allDocuments, itemData)
			}
		}
	}

	// --- 3e. Apply Custom Score Modifiers (Function Score) ---
	if input.ScoreModifiersRequest != nil && len(allDocuments) > 0 {
		ApplyScoreModifiers(allDocuments, input.ScoreModifiersRequest) // Function is now local
	}

	// --- 4. Final Processing (Aggregation vs. Standard) ---

	if len(input.AggregationMap) > 0 { // Aggregation Mode
		// 1. Separate Top-Level Aggregations
		bucketAggregations := make(map[string]Aggregation)
		pipelineAggregations := make(map[string]Aggregation)
		for aggName, agg := range input.AggregationMap {
			aggType := strings.ToLower(agg.Type)
			if aggType == "stats_bucket" || aggType == "bucket_script" { 
				pipelineAggregations[aggName] = agg
			} else {
				bucketAggregations[aggName] = agg
			}
		}

		// 2. Execute Primary Bucket Aggregations
		allAggResults := make(map[string]AggregationResult)
		primaryAggName := "" 
		for name, agg := range bucketAggregations {
			result := ExecuteAggregation(allDocuments, &agg) // Function is now local
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
				pipeResult := ExecuteTopLevelPipeline(targetResult.Buckets, pipeAgg, pathParts[1:]) // Function is now local
				finalPipelineMetrics[pipeName] = pipeResult
			}
		}
		
		// 4. Construct Final Aggregation Result
		var finalAggResult AggregationResult // Type is now local
		if primaryAggName != "" {
			finalAggResult = allAggResults[primaryAggName]
		}
		for k, v := range finalPipelineMetrics {
			finalAggResult.PipelineMetrics[k] = v
		}

		return &SearchResultPayload{
			StatusCode: http.StatusOK,
			BodyData: finalAggResult,
			IsAggregation: true,
		}, nil

	} else {
		// Standard Search or Union Query: Apply result controls
		
		// Apply Sorting 
		if len(input.SortRequest) == 0 {
			input.SortRequest = []SortField{{Field: "_score", Order: SortDesc}} // Type and const are local
		}
		ApplySort(allDocuments, input.SortRequest) // Function is now local

		// Apply Field Projection (Source)
		if len(input.SourceFields) > 0 {
			allDocuments = ProjectFields(allDocuments, input.SourceFields) // Function is now local
		}

		// Apply Paging (Limit/Offset)
		pagedDocuments := ApplyPaging(allDocuments, input.Limit, input.Offset) // Function is now local

		return &SearchResultPayload{
			StatusCode: http.StatusOK,
			BodyData: pagedDocuments,
			IsAggregation: false,
		}, nil
	}
}