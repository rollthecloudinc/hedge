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

// NOTE: This implementation assumes the following structs and helper functions 
// (which were defined or referenced in previous steps) exist within the 'search' package:
// - SearchEngine (with Loader and Metrics fields)
// - UnionQueryInput, SearchResultPayload
// - DocumentLoader, DocumentIterator
// - Query, Aggregation, SortField, FunctionScore, AggregationResult, GetIndexConfigurationInput
// - ApplyScoreModifiers, ApplySort, ProjectFields, ApplyPaging, ExecuteAggregation, ExecuteTopLevelPipeline, GetIndexById
// - MetricsClient (e.g., SimpleMockClient)

// ExecuteUnionQuery is the source-agnostic main entry point for running a union search.
// It orchestrates data loading via the injected Loader and then applies all post-processing.
func (e *SearchEngine) ExecuteUnionQuery(input *UnionQueryInput) (*SearchResultPayload, error) {

    // --- 1. Initial Setup and Metrics Start ---
	// startTime := time.Now()
	totalDocumentsProcessed := 0
	allDocuments := make([]map[string]interface{}, 0)
	
	// --- 2. Main Execution Loop for Union Queries (Loading/Filtering/Scoring) ---
	for i, currentQuery := range input.QueriesToExecute {
		log.Printf("--- STARTING QUERY %d/%d (Index: %s) ---", i+1, len(input.QueriesToExecute), currentQuery.Index)
		
		// 2a. Build the necessary configuration for the current index
		getIndexInput := &GetIndexConfigurationInput{
			GithubClient: input.GitHubRestClient,
			Owner:        input.Owner,
			Stage:        os.Getenv("STAGE"),
			Repo:         input.Owner + "/" + input.RepoName,
			Branch:       input.Branch,
			Id:           currentQuery.Index,
		}
		
		// 2b. Load documents using the injected, dynamic loader.
		iterator, err := e.Loader.Load(
			input.Ctx, 
			input.GitHubRestClient, 
			getIndexInput, 
			currentQuery.Composite, // Composite path data passed to the Loader
		)
		if err != nil {
			log.Printf("ERROR: Failed to load documents for index %s: %v. Continuing union.", currentQuery.Index, err)
			
			// Return a 500 if the error is due to a required input (like missing 'Composite')
			if strings.Contains(err.Error(), "missing 'Composite'") || strings.Contains(err.Error(), "configuration missing") {
				return &SearchResultPayload{
					StatusCode: http.StatusInternalServerError,
					ErrorMessage: err.Error(),
				}, nil
			}

			continue
		}
		defer iterator.Close()

		// 2c. Process documents from the iterator, applying filters and scoring.
		for doc, ok := iterator.Next(); ok; doc, ok = iterator.Next() {
			totalDocumentsProcessed++ 
			
			// EXECUTE BOOL EVALUATION
			matched, score := currentQuery.Bool.Evaluate(doc, input.Ctx, input.GitHubRestClient, getIndexInput)

			if matched {
				doc["_score"] = score
				allDocuments = append(allDocuments, doc)
			}
		}

		if err := iterator.Error(); err != nil {
			log.Printf("WARN: Iterator for index %s finished with non-fatal error: %v.", currentQuery.Index, err)
		}
	}
    
	// --- 3. Final Processing (Score Modifiers, Aggregation, Sort, Paging) ---

	// Apply Custom Score Modifiers
	if input.ScoreModifiersRequest != nil && len(allDocuments) > 0 {
		ApplyScoreModifiers(allDocuments, input.ScoreModifiersRequest)
	}

	// 3a. Aggregation Mode Handling (Restored Pipeline Metrics Logic)
	if len(input.AggregationMap) > 0 { 
        
		// 1. Separate Bucket Aggregations (executed first) from Pipeline Aggregations (executed second)
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

		// 2. Execute Primary Bucket Aggregations (e.g., group_by_category)
		allAggResults := make(map[string]AggregationResult)
		primaryAggName := "" 
        
		for name, agg := range bucketAggregations {
            // ExecuteAggregation handles both basic metrics and bucket_script
			result := ExecuteAggregation(allDocuments, &agg) 
			allAggResults[name] = result
			
			if primaryAggName == "" {
				primaryAggName = name
			}
		}

		// 3. Execute Top-Level Pipeline Aggregations (e.g., category_efficiency_stats)
		finalPipelineMetrics := make(map[string]interface{})
        
		for pipeName, pipeAgg := range pipelineAggregations {
			if pipeAgg.Path == "" { continue }

			// Path: "group_by_category>views_per_dollar_ratio"
			pathParts := strings.Split(pipeAgg.Path, ">") 
			targetAggName := pathParts[0] 

			if targetResult, found := allAggResults[targetAggName]; found {
				// ExecuteTopLevelPipeline runs the stats_bucket logic on the target bucket's metrics.
                // pathParts[1:] is the inner path (e.g., "views_per_dollar_ratio")
				pipeResult := ExecuteTopLevelPipeline(targetResult.Buckets, pipeAgg, pathParts[1:])
				finalPipelineMetrics[pipeName] = pipeResult
			}
		}
		
		// 4. Construct Final Aggregation Result Payload
		var finalAggResult AggregationResult
		if primaryAggName != "" {
			finalAggResult = allAggResults[primaryAggName]
		}
        
        if finalAggResult.PipelineMetrics == nil {
            finalAggResult.PipelineMetrics = make(map[string]interface{})
        }
		for k, v := range finalPipelineMetrics {
			finalAggResult.PipelineMetrics[k] = v
		}
        
        // Record final aggregation metric
        //e.Metrics.MeasureDuration("search.query.total_duration", time.Since(startTime), map[string]string{"type": "aggregation"})

		return &SearchResultPayload{
			StatusCode: http.StatusOK,
			BodyData: finalAggResult,
			IsAggregation: true,
		}, nil

	} else {
		// 3b. Standard Search/Union Query: Apply result controls
		
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
        
        // Record final standard search metrics
        //e.Metrics.MeasureDuration("search.query.total_duration", time.Since(startTime), map[string]string{"type": "standard_search"})
        //e.Metrics.Increment("search.documents.processed.total", map[string]string{"count": fmt.Sprintf("%d", totalDocumentsProcessed)})

		return &SearchResultPayload{
			StatusCode: http.StatusOK,
			BodyData: pagedDocuments,
			IsAggregation: false,
		}, nil
	}
}