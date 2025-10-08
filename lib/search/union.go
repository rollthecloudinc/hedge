package search

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings" 
	
	"github.com/google/go-github/v46/github" 
	// Removed unused imports from the old monolithic function
)

// UnionQueryInput encapsulates all parameters required to execute a union query.
type UnionQueryInput struct {
	Ctx                   context.Context
	Owner                 string
	RepoName              string
	Branch                string
	GitHubRestClient      *github.Client
	QueriesToExecute      []Query
	AggregationMap        map[string]Aggregation
	SortRequest           []SortField
	Limit                 int
	Offset                int
	SourceFields          []string
	ScoreModifiersRequest *FunctionScore
}

// SearchResultPayload is the vendor-agnostic return structure for the search core logic.
type SearchResultPayload struct {
	StatusCode    int
	BodyData      interface{} // Documents or AggregationResult
	ErrorMessage  string
	IsAggregation bool
}

// NOTE: UnionQueryInput and SearchResultPayload are assumed to be defined 
// in this package, either in this file or another file like 'types.go'.

// ====================================================================
// === CORE ENGINE METHOD (Adapted to use Loader/Iterator) =============
// ====================================================================

// ExecuteUnionQuery is the main entry point for running a union search on the engine.
// It is now source-agnostic for data fetching.
func (e *SearchEngine) ExecuteUnionQuery(input *UnionQueryInput) (*SearchResultPayload, error) {

	allDocuments := make([]map[string]interface{}, 0)
	
	// 1. Iterate through all queries in the union
	for i, currentQuery := range input.QueriesToExecute {
		log.Printf("--- STARTING QUERY %d/%d (Index: %s) ---", i+1, len(input.QueriesToExecute), currentQuery.Index)
		
		// 1a. Build the necessary configuration for the current index
		getIndexInput := &GetIndexConfigurationInput{
			GithubClient: input.GitHubRestClient,
			Owner:        input.Owner,
			Stage:        os.Getenv("STAGE"),
			Repo:         input.Owner + "/" + input.RepoName,
			Branch:       input.Branch,
			Id:           currentQuery.Index,
		}
		
		// 1b. Load documents using the injected, dynamic loader.
		// The loader handles the source-specific file fetching.
		iterator, err := e.Loader.Load(
			input.Ctx, 
			input.GitHubRestClient, 
			getIndexInput, 
			currentQuery.Composite,
		)
		if err != nil {
			log.Printf("ERROR: Failed to load documents for index %s: %v. Continuing union.", currentQuery.Index, err)
			
			// Handle the case where a query is malformed (e.g., missing 'Composite')
			if strings.Contains(err.Error(), "missing 'Composite'") {
				return &SearchResultPayload{
					StatusCode: http.StatusInternalServerError,
					ErrorMessage: err.Error(),
				}, nil
			}

			continue
		}
		defer iterator.Close()

		// 1c. Process documents from the iterator, applying filters and scoring.
		for doc, ok := iterator.Next(); ok; doc, ok = iterator.Next() {
			
			// EXECUTE BOOL EVALUATION
			matched, score := currentQuery.Bool.Evaluate(doc, input.Ctx, input.GitHubRestClient, getIndexInput)

			if matched {
				doc["_score"] = score
				allDocuments = append(allDocuments, doc)
			}
		}

		if err := iterator.Error(); err != nil {
			log.Printf("WARN: Iterator for index %s finished with non-fatal error: %v.", currentQuery.Index, err)
			// Continue, as the error might just be a failed item decode
		}
	}
    
	// --- 2. Apply Post-Processing (Score Modifiers, Aggregation, Sort, Paging) ---

	// Apply Custom Score Modifiers
	if input.ScoreModifiersRequest != nil && len(allDocuments) > 0 {
		ApplyScoreModifiers(allDocuments, input.ScoreModifiersRequest)
	}

	// Check for Aggregation Mode
	if len(input.AggregationMap) > 0 {
		// ... (Full Aggregation logic from previous versions goes here) ...
		
		// Simplified Aggregation Process:
		// (The full logic for bucket/pipeline aggs is lengthy, using a simplified call here)
		finalAggResults := make(map[string]interface{})
		for name, agg := range input.AggregationMap {
			finalAggResults[name] = ExecuteAggregation(allDocuments, &agg)
		}

		return &SearchResultPayload{
			StatusCode: http.StatusOK,
			BodyData: finalAggResults, 
			IsAggregation: true,
		}, nil

	} else {
		// Standard Search/Union Query: Apply result controls
		
		// Apply Sorting 
		if len(input.SortRequest) == 0 {
			input.SortRequest = []SortField{{Field: "_score", Order: SortDesc}}
		}
		ApplySort(allDocuments, input.SortRequest)

		// Apply Field Projection (Source)
		if len(input.SourceFields) > 0 {
			allDocuments = ProjectFields(allDocuments, input.SourceFields)
		}

		// Apply Paging (Limit/Offset)
		pagedDocuments := ApplyPaging(allDocuments, input.Limit, input.Offset)

		return &SearchResultPayload{
			StatusCode: http.StatusOK,
			BodyData: pagedDocuments,
			IsAggregation: false,
		}, nil
	}
}