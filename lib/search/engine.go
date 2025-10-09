package search

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"fmt"
)

const MaxFanOutLimit = 8 

// ====================================================================
// === CORE INTERFACES & ENGINE STRUCT (For Dynamic Data Sources) =====
// ====================================================================

// DocumentIterator defines the contract for streaming documents from any source.
type DocumentIterator interface {
	Next() (map[string]interface{}, bool)
	Error() error
	Close()
}

// DocumentLoader defines the contract for initiating the loading process.
type DocumentLoader interface {
	// Load starts the process of fetching documents based on the index configuration.
	// We still pass the client as the loader needs it for the GitHub API calls.
	Load(ctx context.Context, config *GetIndexConfigurationInput, queryComposite map[string]interface{}) (DocumentIterator, error)
}

// UnionQueryInput encapsulates all parameters required to execute a union query.
type UnionQueryInput struct {
	Ctx                   context.Context
	Owner                 string
	RepoName              string
	Branch                string
	QueriesToExecute      []Query
	AggregationMap        map[string]Aggregation
	SortRequest           []SortField
	Limit                 int
	Offset                int
	SourceFields          []string
	ScoreModifiersRequest *FunctionScore
	PostFilter            *Bool
    FacetingAggs          map[string]*Aggregation
}

// SearchResultPayload represents the final, unified response sent back to the client.
type SearchResultPayload struct {
    StatusCode         int                    `json:"statusCode"`
    ErrorMessage       string                 `json:"errorMessage,omitempty"`
    
    // TotalHits reflects the count AFTER applying the PostFilter.
    TotalHits          int                    `json:"totalHits"` 
    IsAggregation      bool                   `json:"isAggregation"`

    // ----------------------------------------------------
    // --- Result Content (Mutually Exclusive Content) ----
    // ----------------------------------------------------

    // BodyData holds the paged documents when IsAggregation is false.
    // Type: []map[string]interface{}
    Hits           interface{}            `json:"hits,omitempty"` 
    
    // AggregationResults holds the complex metrics and bucket data 
    // when IsAggregation is true.
    AggregationResults map[string]AggregationResult `json:"aggregationResults,omitempty"` 

    // ----------------------------------------------------
    // --- Always Included (Metadata/Facets) ---------------
    // ----------------------------------------------------
    
    // FacetingResults holds simple key/count buckets for UI filtering, 
    // calculated using the final filtered document set.
    // Type: map[string][]Bucket
    FacetingResults map[string][]Bucket     `json:"facetingResults,omitempty"` 
}

// QueryResult holds the results from a single parallel query execution.
type QueryResult struct {
	Documents       []map[string]interface{}
	Error           error
	IndexName       string
	DocumentsCount  int
}

// SearchEngine holds the single, swappable loader dependency.
type SearchEngine struct {
	Loader DocumentLoader 
}

// NewSearchEngine requires the concrete loader instance to be injected.
func NewSearchEngine(loader DocumentLoader) *SearchEngine {
	return &SearchEngine{
		Loader: loader,
	}
}

// NOTE: This file assumes the necessary structs (SearchEngine, QueryResult, UnionQueryInput, etc.) 
// and helper functions (ApplySort, ExecuteAggregation, etc.) are defined in 'search/types.go' 
// or other files within the 'search' package.

// ExecuteUnionQuery orchestrates the parallel loading and sequential post-processing.
// It uses a worker pool (MaxFanOutLimit) to prevent resource exhaustion during the I/O phase.
func (e *SearchEngine) ExecuteUnionQuery(input *UnionQueryInput) (*SearchResultPayload, error) {

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
            // NOTE: input.Ctx is passed for cancellation/timeouts
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
                // Bool.Evaluate uses fuzzy MatchPhrase and other logic
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
            // Handle critical configuration errors by returning a 500
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
    
    // --- 3. SEQUENTIAL POST-PROCESSING: FILTERING, AGGREGATION, SORT, PAGING ---
    
    // Apply Custom Score Modifiers (if any)
    if input.ScoreModifiersRequest != nil && len(allDocuments) > 0 {
        ApplyScoreModifiers(allDocuments, input.ScoreModifiersRequest)
    }

    // --- 3A. POST-FILTERING (NEW STAGE) ---
    // Documents are filtered in-memory after initial parallel fetch.
    finalFilteredResults := allDocuments
    if input.PostFilter != nil {
		// The other one is not in scope it is buried in a sub routine.
        getIndexInput := &GetIndexConfigurationInput{
            Owner:        input.Owner,
            Stage:        os.Getenv("STAGE"),
            Repo:         input.Owner + "/" + input.RepoName,
            Branch:       input.Branch,
            //Id:           query.Index, // We shouldn't be doing subqueries anyway here. Noop loader?
        }
        log.Printf("Applying Post-Filter Query with %d initial documents.", len(allDocuments))
        // Reuses existing BoolQuery evaluation logic.
        finalFilteredResults = ApplyPostFilter(allDocuments, input.PostFilter, input.Ctx, e.Loader, getIndexInput) 
        log.Printf("Post-Filter reduced documents to %d.", len(finalFilteredResults))
    }
    
    // Store final computed metrics and facets
    finalAggsResults := make(map[string]AggregationResult)
    finalFacetingResults := make(map[string][]Bucket)
    
    // --- 3B. FACETING AND AGGREGATION ---

    // 1. Faceting Aggregations (Bucket counts for UI Filters)
    if len(input.FacetingAggs) > 0 {
        // ExecuteFaceting uses GroupDocumentsByField and runs on the filtered set
        finalFacetingResults = ExecuteFaceting(finalFilteredResults, input.FacetingAggs)
    }
    
    // 2. Standard and Pipeline Aggregation Handling
    if len(input.AggregationMap) > 0 {
        
        // Split primary and pipeline aggs (logic remains the same)
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

        // Execute Primary Bucket Aggregations
        primaryAggName := "" 
        for name, agg := range bucketAggregations {
            // ExecuteAggregation must now use the finalFilteredResults
            result := ExecuteAggregation(finalFilteredResults, &agg) 
            finalAggsResults[name] = result
            if primaryAggName == "" { primaryAggName = name }
        }

        // Execute Top-Level Pipeline Aggregations (Logic remains the same)
        finalPipelineMetrics := make(map[string]interface{})
        for pipeName, pipeAgg := range pipelineAggregations {
            if pipeAgg.Path == "" { continue }

            pathParts := strings.Split(pipeAgg.Path, ">") 
            targetAggName := pathParts[0] 

            if targetResult, found := finalAggsResults[targetAggName]; found {
                pipeResult := ExecuteTopLevelPipeline(targetResult.Buckets, pipeAgg, pathParts[1:])
                finalPipelineMetrics[pipeName] = pipeResult
            }
        }
        
        // Construct Final Aggregation Result Payload
        var finalAggResult AggregationResult
        if primaryAggName != "" {
            finalAggResult = finalAggsResults[primaryAggName]
        }
        
        if finalAggResult.PipelineMetrics == nil { finalAggResult.PipelineMetrics = make(map[string]interface{}) }
        for k, v := range finalPipelineMetrics { finalAggResult.PipelineMetrics[k] = v }
        
        // Return Aggregation Result Payload
        return &SearchResultPayload{
            StatusCode: http.StatusOK, 
            AggregationResults: finalAggsResults, 
            FacetingResults: finalFacetingResults, // Included Facets
            IsAggregation: true,
        }, nil

    } else {
        // 3C. Standard Search/Union Query: Apply result controls
        
        // Sorting, projection, and paging must use the filtered set
        if len(input.SortRequest) == 0 { input.SortRequest = []SortField{{Field: "_score", Order: SortDesc}} }
        
        ApplySort(finalFilteredResults, input.SortRequest)
        if len(input.SourceFields) > 0 { finalFilteredResults = ProjectFields(finalFilteredResults, input.SourceFields) }
        pagedDocuments := ApplyPaging(finalFilteredResults, input.Limit, input.Offset)
        
        return &SearchResultPayload{
            StatusCode: http.StatusOK, 
            Hits: pagedDocuments, 
            TotalHits: len(finalFilteredResults), // TotalHits is the filtered count
            FacetingResults: finalFacetingResults, // Included Facets
            IsAggregation: false,
        }, nil
    }
}