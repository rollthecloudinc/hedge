package search

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
	"encoding/json"
	"encoding/base64"
	"sort" // REQUIRED for Median and Min/Max calculations
	"github.com/google/go-github/v46/github"
)

// ----------------------------------------------------
// Core Structs and Types
// ----------------------------------------------------

type Operation int32

const (
	Equal Operation = iota
	NotEqual
	GreaterThan
	LessThan
	GreaterThanOrEqual
	LessThanOrEqual
	Contains
	StartsWith
	EndsWith
	In
	NotIn
)

// Modifiers holds the operation for a simple condition.
type Modifiers struct {
	Operation Operation `json:"operation"`
}

// Term, Filter, and Match structs now embed a full Query object for subqueries.
type Term struct {
	Field     string     `json:"field"`
	Value     string     `json:"value,omitempty"`
	SubQuery  *Query     `json:"subquery,omitempty"` // Full recursive Query object
	Modifiers *Modifiers `json:"modifiers,omitempty"`
}

type Filter struct {
	Field     string     `json:"field"`
	Value     string     `json:"value,omitempty"`
	SubQuery  *Query     `json:"subquery,omitempty"` // Full recursive Query object
	Modifiers *Modifiers `json:"modifiers,omitempty"`
}

type Match struct {
	Field     string     `json:"field"`
	Value     string     `json:"value,omitempty"`
	SubQuery  *Query     `json:"subquery,omitempty"` // Full recursive Query object
	Modifiers *Modifiers `json:"modifiers,omitempty"`
}

// Case wraps one condition type (Term, Bool, Filter, Match).
type Case struct {
	Term   *Term   `json:"term,omitempty"`
	Bool   *Bool   `json:"bool,omitempty"`
	Filter *Filter `json:"filter,omitempty"`
	Match  *Match  `json:"match,omitempty"`
}

// Bool implements the recursive AND/OR/NOT logic.
type Bool struct {
	All  []Case `json:"all,omitempty"`  // Logical AND
	None []Case `json:"none,omitempty"` // Logical NOT (OR of the negation)
	One  []Case `json:"one,omitempty"`  // Logical OR
	Not  []Case `json:"not,omitempty"`  // Negation of the first element
}

// MetricRequest defines a single metric calculation type to perform across multiple fields.
type MetricRequest struct {
    Type    string            `json:"type"`  // e.g., "sum", "avg", "median", "mode", "min", "max"
    // A map where Key=Source Field (e.g., "total_price"), Value=Result Name (e.g., "total_sales")
    Fields map[string]string `json:"fields"` 
}

// Aggregation holds metrics and nested grouping logic.
type Aggregation struct {
    Name    string             `json:"name"` // Human-readable name for this group level
    // GroupBy is a slice to support sequential multi-field grouping at this level
    GroupBy []string           `json:"groupBy"` 
    // Metrics is a slice of requests, supporting multiple types and fields
    Metrics []MetricRequest  `json:"metrics,omitempty"`
    
    // Recursive definition for the next level of aggregation
    Aggs    *Aggregation       `json:"aggs,omitempty"` 
}

// Query defines a standard single search, which can now be used recursively.
type Query struct {
	Bool      Bool                   `json:"bool"`
	Index     string                 `json:"index"`
	Composite map[string]interface{} `json:"composite,omitempty"`
	ResultField string                 `json:"resultField,omitempty"` // Field to select/return from this query (used for subqueries)
	
	Aggs      *Aggregation           `json:"aggs,omitempty"` // Top-level aggregation definition
}

// UnionQuery combines the results of multiple standard Queries.
type UnionQuery struct {
	Queries []Query `json:"queries"`
}

// TopLevelQuery wraps either a single Query or a UnionQuery.
type TopLevelQuery struct {
	Query *Query      `json:"query,omitempty"`
	Union *UnionQuery `json:"union,omitempty"`
}

// ----------------------------------------------------
// Condition Interface and Implementations
// ----------------------------------------------------

// Condition is an interface that all simple condition structs must satisfy.
type Condition interface {
	GetField() string
	GetValue() string
	GetSubQuery() *Query
	GetModifiers() *Modifiers
}

func (t Term) GetField() string         { return t.Field }
func (t Term) GetValue() string         { return t.Value }
func (t Term) GetSubQuery() *Query      { return t.SubQuery }
func (t Term) GetModifiers() *Modifiers { return t.Modifiers }

func (f Filter) GetField() string         { return f.Field }
func (f Filter) GetValue() string         { return f.Value }
func (f Filter) GetSubQuery() *Query      { return f.SubQuery }
func (f Filter) GetModifiers() *Modifiers { return f.Modifiers }

func (m Match) GetField() string         { return m.Field }
func (m Match) GetValue() string         { return m.Value }
func (m Match) GetSubQuery() *Query      { return m.SubQuery }
func (m Match) GetModifiers() *Modifiers { return m.Modifiers }

// ----------------------------------------------------
// Aggregation Result Structures
// ----------------------------------------------------

// Bucket represents a single group result.
type Bucket struct {
    Key     string                 `json:"key"`     
    Count   int                    `json:"count"`   
    Metrics map[string]interface{} `json:"metrics,omitempty"` 
    
    // Nested buckets for the next level of grouping
    Buckets []Bucket               `json:"buckets,omitempty"` 
}

// AggregationResult wraps the top-level buckets.
type AggregationResult struct {
    Name    string   `json:"name"`
    Buckets []Bucket `json:"buckets"`
}

// ----------------------------------------------------
// Helper Functions (Dot Notation, Date Parsing)
// ----------------------------------------------------

// resolveDotNotation safely traverses a nested map[string]interface{} using a dot-separated path (e.g., "user.name").
func resolveDotNotation(data map[string]interface{}, path string) (string, bool) {
    if data == nil {
		// Log removed here as it can be very noisy in recursion
        return "", false
    }

	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		val, ok := current[part]
		if !ok {
			return "", false
		}

		if i == len(parts)-1 {
			switch v := val.(type) {
			case string:
				return v, true
			case float64:
				// Convert numbers to string for consistent comparison
				return strconv.FormatFloat(v, 'f', -1, 64), true
			case int:
				return strconv.Itoa(v), true
			case bool:
				return strconv.FormatBool(v), true
			default:
				return "", false
			}
		} else {
			nextMap, ok := val.(map[string]interface{})
			if !ok {
				return "", false
			}
			current = nextMap
		}
	}
	return "", false 
}

var dateFormats = []string{
	time.RFC3339,
	"2006-01-02",
	"1/2/2006",
	"01/02/2006",
	"2006-01-02 15:04:05",
}

// tryParseDate attempts to parse a string value into a time.Time using various formats.
func tryParseDate(value string) (time.Time, error) {
	for _, format := range dateFormats {
		t, err := time.Parse(format, value)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("failed to parse date: %s", value)
}

// ----------------------------------------------------
// Evaluation Logic
// ----------------------------------------------------

// EvaluateBool performs the actual comparison logic (date, string, numeric, set).
func EvaluateBool(c Condition, targetValue string, op Operation) bool {
    // ... (logic remains unchanged) ...
    conditionValue := c.GetValue()

    // 1. Date/Time Comparison Attempt
    targetTime, errTT := tryParseDate(targetValue)
    conditionTime, errCT := tryParseDate(conditionValue)

    isDateOperation := errTT == nil && errCT == nil

    if isDateOperation {
        switch op {
        case Equal:
            return targetTime.Equal(conditionTime)
        case NotEqual:
            return !targetTime.Equal(conditionTime)
        case GreaterThan:
            return targetTime.After(conditionTime)
        case LessThan:
            return targetTime.Before(conditionTime)
        case GreaterThanOrEqual:
            return targetTime.After(conditionTime) || targetTime.Equal(conditionTime)
        case LessThanOrEqual:
            return targetTime.Before(conditionTime) || targetTime.Equal(conditionTime)
        }
    }

    // 2. String and Text Operations
    switch op {
    case Equal:
        return targetValue == conditionValue
    case NotEqual:
        return targetValue != conditionValue
    case Contains:
        return strings.Contains(targetValue, conditionValue)
    case StartsWith:
        return strings.HasPrefix(targetValue, conditionValue)
    case EndsWith:
        return strings.HasSuffix(targetValue, conditionValue)
    }

    // 3. Numeric Comparison Operations
    if op >= GreaterThan && op <= LessThanOrEqual {
        targetFloat, errT := strconv.ParseFloat(targetValue, 64)
        conditionFloat, errC := strconv.ParseFloat(conditionValue, 64)

        if errT == nil && errC == nil {
            switch op {
            case GreaterThan:
                return targetFloat > conditionFloat
            case LessThan:
                return targetFloat < conditionFloat
            case GreaterThanOrEqual:
                return targetFloat >= conditionFloat
            case LessThanOrEqual:
                return targetFloat <= conditionFloat
            }
        } else {
            return false 
        }
    }

    // 4. Set Operations (In/NotIn)
    if op == In || op == NotIn {
        validValues := strings.Split(conditionValue, ",")
        valueSet := make(map[string]struct{})
        for _, v := range validValues {
            valueSet[strings.TrimSpace(v)] = struct{}{}
        }

        _, isInSet := valueSet[targetValue]

        if op == In {
            return isInSet
        }
        if op == NotIn {
            return !isInSet
        }
    }

    return false
}

// Bool.Evaluate and Case.Evaluate logic remains unchanged

func (b *Bool) Evaluate(data map[string]interface{}, ctx context.Context, client *github.Client, indexInput *GetIndexConfigurationInput) bool {
	// 1. ALL (AND logic)
	if len(b.All) > 0 {
		for _, c := range b.All {
			if !c.Evaluate(data, ctx, client, indexInput) {
				return false
			}
		}
		return true
	}

	// 2. ONE (OR logic)
	if len(b.One) > 0 {
		for _, c := range b.One {
			if c.Evaluate(data, ctx, client, indexInput) {
				return true
			}
		}
		return false
	}

	// 3. NONE (NOT OR logic)
	if len(b.None) > 0 {
		for _, c := range b.None {
			if c.Evaluate(data, ctx, client, indexInput) {
				return false
			}
		}
		return true
	}

	// 4. NOT (Negation logic)
	if len(b.Not) > 0 {
		if len(b.Not) > 1 {
            log.Print("Bool.Evaluate: Warning, 'not' array has more than one element; only the first is evaluated.")
        }
		return !b.Not[0].Evaluate(data, ctx, client, indexInput)
	}

	return true // Empty Bool matches
}

func (c *Case) Evaluate(data map[string]interface{}, ctx context.Context, client *github.Client, indexInput *GetIndexConfigurationInput) bool {
	// A) Handle nested Boolean logic
	if c.Bool != nil {
		return c.Bool.Evaluate(data, ctx, client, indexInput)
	}

	// B) Extract Condition and default Operation
	var condition Condition
	var defaultOp Operation = Equal

	if c.Term != nil {
		condition = *c.Term
	} else if c.Filter != nil {
		condition = *c.Filter
	} else if c.Match != nil {
		condition = *c.Match
	} else {
		return true // Empty case matches
	}

	if condition.GetModifiers() != nil {
		defaultOp = condition.GetModifiers().Operation
	}
    
    // --- 1. Handle SubQuery for IN/NOT IN ---
	if condition.GetSubQuery() != nil && (defaultOp == In || defaultOp == NotIn) {
		subQuery := condition.GetSubQuery()
        
        localCheckField := condition.GetField() 
        resultField := subQuery.ResultField
        
        if resultField == "" {
            log.Printf("Case.Evaluate: Subquery must specify 'resultField' for IN/NOT IN operation.")
            return false
        }
		
        log.Printf("Case.Evaluate: Executing recursive subquery. Target index: %s, Result field: %s", subQuery.Index, resultField)
        
		subResultData, err := ExecuteSubQuery(ctx, client, indexInput, subQuery, resultField)
		if err != nil {
			log.Printf("Case.Evaluate: Error executing subquery: %v", err)
			return false 
		}
		
		subResultSet := make(map[string]struct{})
		for _, val := range subResultData {
			subResultSet[val] = struct{}{}
		}

		localValue, exists := resolveDotNotation(data, localCheckField)
		if !exists { 
            log.Printf("Case.Evaluate: Local check field '%s' not found in document.", localCheckField)
            return false 
        }
		
		_, localValueIsInSet := subResultSet[localValue]

		if defaultOp == In {
			return localValueIsInSet
		}
		if defaultOp == NotIn {
			return !localValueIsInSet
		}
	}
    
	// --- 2. Standard Value Evaluation (Dot notation) ---
    
	targetValue, exists := resolveDotNotation(data, condition.GetField())
	if !exists {
		return false
	}

	return EvaluateBool(condition, targetValue, defaultOp)
}

// ----------------------------------------------------
// Metric Calculation Functions (New/Updated)
// ----------------------------------------------------

// extractNumericValues attempts to extract and convert a list of field values to float64.
func extractNumericValues(docs []map[string]interface{}, field string) []float64 {
    values := make([]float64, 0, len(docs))
    for _, doc := range docs {
        valStr, exists := resolveDotNotation(doc, field)
        if !exists {
            continue
        }
        if floatVal, err := strconv.ParseFloat(valStr, 64); err == nil {
            values = append(values, floatVal)
        } else {
            // Log warning about unparseable values
            log.Printf("CalculateMetrics: Could not parse value '%s' in field '%s' as float.", valStr, field)
        }
    }
    return values
}

// calculateSum calculates the sum of all numeric values for a field in the group.
func calculateSum(docs []map[string]interface{}, field string) float64 {
    values := extractNumericValues(docs, field)
    var sum float64
    for _, v := range values {
        sum += v
    }
    return sum
}

// calculateAvg calculates the average (mean) of all numeric values for a field.
func calculateAvg(docs []map[string]interface{}, field string) float64 {
    values := extractNumericValues(docs, field)
    if len(values) == 0 {
        return 0.0
    }
    return calculateSum(docs, field) / float64(len(values))
}

// calculateMedian calculates the median of all numeric values for a field.
func calculateMedian(docs []map[string]interface{}, field string) float64 {
    values := extractNumericValues(docs, field)
    if len(values) == 0 {
        return 0.0
    }
    
    // Sort the slice 
    sort.Float64s(values) 
    n := len(values)
    
    if n%2 == 1 {
        // Odd number of elements
        return values[n/2]
    }
    // Even number of elements
    return (values[n/2-1] + values[n/2]) / 2.0
}

// calculateMode finds the most frequently occurring string value (Mode).
func calculateMode(docs []map[string]interface{}, field string) string {
    counts := make(map[string]int)
    for _, doc := range docs {
        valStr, exists := resolveDotNotation(doc, field)
        if exists {
            counts[valStr]++
        }
    }
    
    var mode string
    maxCount := -1
    
    for val, count := range counts {
        if count > maxCount {
            maxCount = count
            mode = val
        }
    }
    return mode 
}

// calculateMin finds the minimum numeric value for a field.
func calculateMin(docs []map[string]interface{}, field string) float64 {
    values := extractNumericValues(docs, field)
    if len(values) == 0 {
        return 0.0 // Return 0.0 or a very large number if appropriate
    }
    
    minVal := values[0]
    for _, v := range values {
        if v < minVal {
            minVal = v
        }
    }
    return minVal
}

// calculateMax finds the maximum numeric value for a field.
func calculateMax(docs []map[string]interface{}, field string) float64 {
    values := extractNumericValues(docs, field)
    if len(values) == 0 {
        return 0.0 // Return 0.0 or a very small number if appropriate
    }
    
    maxVal := values[0]
    for _, v := range values {
        if v > maxVal {
            maxVal = v
        }
    }
    return maxVal
}

// CalculateMetrics iterates through the Aggregation's requested metrics and computes them.
func CalculateMetrics(groupDocs []map[string]interface{}, metrics []MetricRequest) map[string]interface{} {
    results := make(map[string]interface{})

    for _, req := range metrics {
        calcType := strings.ToLower(req.Type)
        
        if len(req.Fields) == 0 {
            continue
        }

        for sourceField, resultName := range req.Fields {
            if resultName == "" {
                continue
            }
            
            var calculatedValue interface{}
            
            switch calcType {
            case "sum":
                calculatedValue = calculateSum(groupDocs, sourceField)
            case "avg", "mean": // Added "mean" as an alias for "avg"
                calculatedValue = calculateAvg(groupDocs, sourceField)
            case "median":
                calculatedValue = calculateMedian(groupDocs, sourceField)
            case "mode":
                calculatedValue = calculateMode(groupDocs, sourceField)
            case "min":
                calculatedValue = calculateMin(groupDocs, sourceField)
            case "max":
                calculatedValue = calculateMax(groupDocs, sourceField)
            default:
                log.Printf("CalculateMetrics: Unknown metric type '%s'. Skipping field '%s'.", req.Type, sourceField)
                continue
            }
            results[resultName] = calculatedValue
        }
    }
    return results
}

// ----------------------------------------------------
// Recursive Subquery Execution Logic
// ----------------------------------------------------

// ExecuteSubQuery fetches a list of values (e.g., IDs) by executing a full, nested search.
// ... (logic remains unchanged) ...
func ExecuteSubQuery(ctx context.Context, client *github.Client, baseInput *GetIndexConfigurationInput, subQuery *Query, resultField string) ([]string, error) {
    
    log.Printf("ExecuteSubQuery: Starting recursive search for index '%s' and composite keys: %+v", subQuery.Index, subQuery.Composite)
    
    // 1. Get the Index Config for the subQuery's index
    subInput := *baseInput 
    subInput.Id = subQuery.Index
    subInput.GithubClient = client 
    
    subIndexObject, err := GetIndexById(&subInput)
    if err != nil || subIndexObject == nil {
        return nil, fmt.Errorf("failed to load configuration for subquery index '%s': %w", subQuery.Index, err)
    }

    // 2. Build the content path using the subQuery's Composite map (Scoped Search)
    fields, ok := subIndexObject["fields"].([]interface{})
    if !ok {
        return nil, errors.New("subquery index configuration missing 'fields'")
    }
    
    contentPath := ""
    if len(subQuery.Composite) > 0 {
        for idx, f := range fields {
            fStr := f.(string)
            compositeVal, found := subQuery.Composite[fStr]
            if found {
                contentPath += fmt.Sprintf("%v", compositeVal)
            }
            if idx < (len(fields) - 1) {
                contentPath += ":"
            }
        }
        log.Printf("ExecuteSubQuery: Using composite path: %s", contentPath)
    } else {
        searchRootPath, ok := subIndexObject["searchRootPath"].(string)
        if ok {
            contentPath = searchRootPath
            log.Printf("ExecuteSubQuery: Using searchRootPath: %s", contentPath)
        } else {
             return nil, errors.New("subquery must specify Composite keys or index must have searchRootPath")
        }
    }

    // 3. Fetch directory contents 
    repoToFetch := subIndexObject["repoName"].(string)
    _, dirContents, _, err := client.Repositories.GetContents(
        ctx, subInput.Owner, repoToFetch, contentPath, 
        &github.RepositoryContentGetOptions{Ref: subInput.Branch},
    )
    if err != nil || dirContents == nil {
        log.Printf("ExecuteSubQuery: Failed to fetch contents from path '%s': %v", contentPath, err)
        return nil, nil
    }

    // 4. Filter contents using the subQuery's Bool logic and extract the target field
    results := make([]string, 0)
    for _, content := range dirContents {
        if content.GetType() != "file" { continue }

        decodedBytes, _ := base64.StdEncoding.DecodeString(content.GetName())
        itemBody := string(decodedBytes)
        
        var itemData map[string]interface{}
        if json.Unmarshal([]byte(itemBody), &itemData) == nil {
            
            match := subQuery.Bool.Evaluate(itemData, ctx, client, baseInput)
            
            if match {
                if val, exists := resolveDotNotation(itemData, resultField); exists {
                    results = append(results, val)
                }
            }
        }
    }
    log.Printf("ExecuteSubQuery: Completed. Found %d matching results to return.", len(results))
    return results, nil
}


// ----------------------------------------------------
// Recursive Aggregation Logic
// ----------------------------------------------------

// ExecuteAggregation recursively groups and calculates metrics on a list of documents.
func ExecuteAggregation(docs []map[string]interface{}, agg *Aggregation) []Bucket {
    if agg == nil || len(agg.GroupBy) == 0 {
        log.Print("ExecuteAggregation: Aggregation is nil or GroupBy field list is empty, terminating recursion.")
        return nil
    }

    // This map holds the aggregated groups at the *current* level.
    groups := make(map[string][]map[string]interface{})
    groups[""] = docs // Start with all documents in a single initial group
    
    finalGroupedBuckets := make(map[string][]map[string]interface{})

    // 1. Process Sequential Grouping (Field1, then Field2, etc.)
    for fieldIndex, field := range agg.GroupBy {
        newGroups := make(map[string][]map[string]interface{})
        log.Printf("ExecuteAggregation: Grouping field %d/%d by '%s'.", fieldIndex+1, len(agg.GroupBy), field)

        for compositeKey, groupDocs := range groups {
            for _, doc := range groupDocs {
                
                keyVal, exists := resolveDotNotation(doc, field)
                if !exists {
                    keyVal = "_missing_" 
                }
                
                newKey := keyVal
                if compositeKey != "" {
                    newKey = compositeKey + ":" + keyVal
                }
                
                newGroups[newKey] = append(newGroups[newKey], doc)
            }
        }
        groups = newGroups 
        finalGroupedBuckets = newGroups 
    }

    // 2. Build Buckets from the final grouping map
    buckets := make([]Bucket, 0, len(finalGroupedBuckets))
    for compositeKey, groupDocs := range finalGroupedBuckets {
        
        // --- 3. Metric Calculation ---
        metricResults := CalculateMetrics(groupDocs, agg.Metrics)

        bucket := Bucket{
            Key:     compositeKey, 
            Count:   len(groupDocs),
            Metrics: metricResults, 
        }
        
        // 4. Recurse for nested aggregations
        if agg.Aggs != nil {
            log.Printf("ExecuteAggregation: Found nested aggregation '%s' under composite key '%s'. Recursing...", agg.Aggs.Name, compositeKey)
            bucket.Buckets = ExecuteAggregation(groupDocs, agg.Aggs)
        }

        buckets = append(buckets, bucket)
    }

    return buckets
}


// ----------------------------------------------------
// GitHub Index Configuration Retrieval
// ----------------------------------------------------

// GetIndexConfigurationInput holds parameters needed to fetch an index config from GitHub.
type GetIndexConfigurationInput struct {
    GithubClient       *github.Client // Client for API calls
    Owner              string         // Owner of the repository
    Stage              string         // Environment stage
    Repo               string         // Repository name (e.g., "owner/repo-name")
    Branch             string         // Branch to check
    Id                 string         // Index ID (name of the .json file)
}

// GetIndexById retrieves the index configuration JSON file from the GitHub repository.
func GetIndexById(c *GetIndexConfigurationInput) (map[string]interface{}, error) {
    log.Printf("GetIndexById: Attempting to retrieve config for ID: %s", c.Id)
    var contract map[string]interface{}

    pieces := strings.Split(c.Repo, "/")
    opts := &github.RepositoryContentGetOptions{
        Ref: c.Branch,
    }
    // File path is assumed to be index/{ID}.json
    file, _, res, err := c.GithubClient.Repositories.GetContents(context.Background(), pieces[0], pieces[1], "index/" + c.Id + ".json", opts)
    if err != nil || res.StatusCode != 200 {
        log.Printf("GetIndexById: Failed to retrieve config for %s: Status %d, Error: %v", c.Id, res.StatusCode, err)
        return contract, nil
    }
    if file != nil && file.Content != nil {
        content, err := base64.StdEncoding.DecodeString(*file.Content)
        if err == nil {
            json.Unmarshal(content, &contract)
            log.Printf("GetIndexById: Successfully retrieved config for %s.", c.Id)
        } else {
            log.Printf("GetIndexById: Invalid index unable to parse content for %s: %v", c.Id, err)
            return contract, errors.New("Invalid index unable to parse.")
        }
    }
    return contract, nil
}