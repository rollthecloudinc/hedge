package search

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"fmt"
	
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

// QueryResult holds the results from a single parallel query execution.
type QueryResult struct {
	Documents       []map[string]interface{}
	Error           error
	IndexName       string
	DocumentsCount  int
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
// Define the maximum number of Go routines that can run concurrently.
// This acts as the worker pool limit (Max Fan-Out).
const MaxFanOutLimit = 8 

// NOTE: This file assumes the necessary structs (SearchEngine, QueryResult, UnionQueryInput, etc.) 
// and helper functions (ApplySort, ExecuteAggregation, etc.) are defined in 'search/types.go' 
// or other files within the 'search' package.

// ExecuteUnionQuery orchestrates the parallel loading and sequential post-processing.
// It uses a worker pool (MaxFanOutLimit) to prevent resource exhaustion during the I/O phase.
func (e *SearchEngine) ExecuteUnionQuery(input *UnionQueryInput) (*SearchResultPayload, error) {

	//startTime := time.Now()
    
    // Set up concurrency controls
	var wg sync.WaitGroup
	
    // Worker Pool Semaphore: controls the number of simultaneous Load/Filter routines.
	workerPool := make(chan struct{}, MaxFanOutLimit) 
    
    // Channel to collect results from all Go routines
	resultsChan := make(chan QueryResult, len(input.QueriesToExecute))
    
    // --- 1. Fan-Out: Launch a Worker Pool for Queries ---
	for i, currentQuery := range input.QueriesToExecute {
        
        // Use local variable copies for the goroutine to prevent capture issues
        query := currentQuery 
        queryIndex := i
        
		wg.Add(1)

		go func() {
            // ACQUIRE TOKEN: Blocks if MaxFanOutLimit routines are already running.
            workerPool <- struct{}{} 
            
            // RELEASE TOKEN & WaitGroup on exit.
            defer func() {
                <-workerPool 
                wg.Done()
            }()
			
			res := QueryResult{
                IndexName: query.Index,
                Documents: make([]map[string]interface{}, 0),
            }

			log.Printf("--- STARTING PARALLEL QUERY %d/%d (Index: %s) ---", queryIndex+1, len(input.QueriesToExecute), query.Index)
			
			// Build configuration
			getIndexInput := &GetIndexConfigurationInput{
				Owner:        input.Owner,
				Stage:        os.Getenv("STAGE"),
				Repo:         input.Owner + "/" + input.RepoName,
				Branch:       input.Branch,
				Id:           query.Index,
			}
			
			// Load documents using the injected, dynamic loader
			iterator, err := e.Loader.Load(input.Ctx, getIndexInput, query.Composite)
			if err != nil {
				res.Error = fmt.Errorf("failed to load documents for index %s: %v", query.Index, err)
				resultsChan <- res
                return 
			}
			defer iterator.Close()

			// Process documents (filtering/scoring)
			count := 0
			for doc, ok := iterator.Next(); ok; doc, ok = iterator.Next() {
				matched, score := query.Bool.Evaluate(doc, input.Ctx, e.Loader, getIndexInput)
				if matched {
					doc["_score"] = score
					res.Documents = append(res.Documents, doc)
                    count++
				}
			}
            res.DocumentsCount = count
			
			if err := iterator.Error(); err != nil {
				log.Printf("WARN: Iterator for index %s finished with non-fatal error: %v.", query.Index, err)
			}
			resultsChan <- res
		}()
	}
    
    // --- 2. Fan-In: Collect and Merge Results ---
	// Wait for all routines to complete, then close channels.
	wg.Wait() 
	close(resultsChan)
    close(workerPool)

    totalDocumentsProcessed := 0
	allDocuments := make([]map[string]interface{}, 0)
    
	for res := range resultsChan {
		if res.Error != nil {
			log.Printf("FATAL ERROR in parallel query for index %s: %v. Skipping results.", res.IndexName, res.Error)
			// Return a 500 if the error is due to a required input (e.g., config error)
			if strings.Contains(res.Error.Error(), "configuration missing") {
				return &SearchResultPayload{
					StatusCode: http.StatusInternalServerError,
					ErrorMessage: res.Error.Error(),
				}, nil
			}
			continue 
		}
		allDocuments = append(allDocuments, res.Documents...)
        totalDocumentsProcessed += res.DocumentsCount
	}
    
	// --- 3. Sequential Post-Processing (Aggregation, Sort, Paging) ---
	
	// Apply Custom Score Modifiers
	if input.ScoreModifiersRequest != nil && len(allDocuments) > 0 {
		ApplyScoreModifiers(allDocuments, input.ScoreModifiersRequest)
	}

	// 3a. Aggregation Mode Handling (Restored Pipeline Metrics Logic)
	if len(input.AggregationMap) > 0 { 
        
		// 1. Separate Bucket Aggregations from Pipeline Aggregations
		bucketAggregations := make(map[string]Aggregation)
		pipelineAggregations := make(map[string]Aggregation)
        
		for aggName, agg := range input.AggregationMap {
			aggType := strings.ToLower(agg.Type)
            // pipeline aggs (like stats_bucket) operate on the results of other aggs
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
            // ExecuteAggregation must handle nested bucket_script calculation
			result := ExecuteAggregation(allDocuments, &agg) 
			allAggResults[name] = result
			if primaryAggName == "" { primaryAggName = name }
		}

		// 3. Execute Top-Level Pipeline Aggregations (e.g., category_efficiency_stats)
		finalPipelineMetrics := make(map[string]interface{})
		for pipeName, pipeAgg := range pipelineAggregations {
			if pipeAgg.Path == "" { continue }

			// Parse path: "group_by_category>views_per_dollar_ratio"
			pathParts := strings.Split(pipeAgg.Path, ">") 
			targetAggName := pathParts[0] 

			if targetResult, found := allAggResults[targetAggName]; found {
				// ExecuteTopLevelPipeline runs the stats_bucket logic
				pipeResult := ExecuteTopLevelPipeline(targetResult.Buckets, pipeAgg, pathParts[1:])
				finalPipelineMetrics[pipeName] = pipeResult
			}
		}
		
		// 4. Construct Final Aggregation Result Payload
		var finalAggResult AggregationResult
		if primaryAggName != "" {
			finalAggResult = allAggResults[primaryAggName]
		}
        
        // Merge top-level pipeline metrics into the final result
        if finalAggResult.PipelineMetrics == nil { finalAggResult.PipelineMetrics = make(map[string]interface{}) }
		for k, v := range finalPipelineMetrics { finalAggResult.PipelineMetrics[k] = v }
        
        //e.Metrics.MeasureDuration("search.query.total_duration", time.Since(startTime), map[string]string{"type": "aggregation"})

		return &SearchResultPayload{
			StatusCode: http.StatusOK, BodyData: finalAggResult, IsAggregation: true,
		}, nil

	} else {
		// 3b. Standard Search/Union Query: Apply result controls
		
		if len(input.SortRequest) == 0 { input.SortRequest = []SortField{{Field: "_score", Order: SortDesc}} }
		ApplySort(allDocuments, input.SortRequest)
		if len(input.SourceFields) > 0 { allDocuments = ProjectFields(allDocuments, input.SourceFields) }
		pagedDocuments := ApplyPaging(allDocuments, input.Limit, input.Offset)
        
        //e.Metrics.MeasureDuration("search.query.total_duration", time.Since(startTime), map[string]string{"type": "standard_search"})
        //e.Metrics.Increment("search.documents.processed.total", map[string]string{"count": fmt.Sprintf("%d", totalDocumentsProcessed)})

		return &SearchResultPayload{
			StatusCode: http.StatusOK, BodyData: pagedDocuments, IsAggregation: false,
		}, nil
	}
}