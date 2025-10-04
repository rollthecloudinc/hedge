package search

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math" // REQUIRED for Standard Deviation, Percentile, and Haversine
	"strconv"
	"strings"
	"time"
	"encoding/json"
	"encoding/base64"
	"sort" // REQUIRED for Median, Percentile, Min/Max, and general result sorting
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

type SortOrder string

const (
	SortAsc  SortOrder = "asc"
	SortDesc SortOrder = "desc"
)

// Modifiers holds the operation for a simple condition.
type Modifiers struct {
	Operation Operation `json:"operation"`
}

// Range defines a single range query on a numeric or date field.
type Range struct {
	Field string  `json:"field"`        // NEW: Explicit field for range query
	From  *string `json:"from,omitempty"` // Start value (inclusive)
	To    *string `json:"to,omitempty"`   // End value (exclusive by default)
}

// GeoDistance defines the criteria for a radial (circle) search around a point. (NEW)
type GeoDistance struct {
	Field     string  `json:"field"`        // Field containing the coordinates (e.g., "lat,lon")
	Latitude  float64 `json:"lat"`     // Center point latitude
	Longitude float64 `json:"lon"`    // Center point longitude
	Distance  float64 `json:"distance"`     // Radius value
	Unit      string  `json:"unit"`         // Unit of distance (e.g., "km", "mi")
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

// Case wraps one condition type (Term, Bool, Filter, Match, Range, GeoDistance).
type Case struct {
	Term        *Term        `json:"term,omitempty"`
	Bool        *Bool        `json:"bool,omitempty"`
	Filter      *Filter      `json:"filter,omitempty"`
	Match       *Match       `json:"match,omitempty"`
	Range       *Range       `json:"range,omitempty"`      // NEW: Range query
	GeoDistance *GeoDistance `json:"geoDistance,omitempty"` // NEW: Geospatial query
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
	Type        string            `json:"type"`          // e.g., "sum", "avg", "median", "percentile", "std_dev"
	Percentile  float64           `json:"percentile,omitempty"` // NEW: Specific percentile value (e.g., 95.0)
	// A map where Key=Source Field (e.g., "total_price"), Value=Result Name (e.g., "total_sales")
	Fields map[string]string `json:"fields"`
}

// RangeBucket defines a single numeric bucket for histogram aggregation
type RangeBucket struct {
	Key  string  `json:"key"`  // Name for the bucket
	From float64 `json:"from"` // Start (inclusive)
	To   float64 `json:"to"`   // End (exclusive)
}

// Aggregation holds metrics and nested grouping logic.
type Aggregation struct {
	Name string `json:"name"` // Human-readable name for this group level
	// GroupBy is a slice to support sequential multi-field grouping at this level
	GroupBy []string `json:"groupBy"`

    // NEW FIELD FOR DATE HISTOGRAM
    DateHistogram *DateHistogram `json:"dateHistogram,omitempty"`

    // NEW FIELD FOR TOP HITS
    TopHits *TopHits `json:"topHits,omitempty"`

	// NEW: For histogram/range aggregation (only one field per map is usually used)
	RangeBuckets map[string][]RangeBucket `json:"rangeBuckets,omitempty"` // Key=Field Name, Value=Buckets

	// Metrics is a slice of requests, supporting multiple types and fields
	Metrics []MetricRequest `json:"metrics,omitempty"`

	// Recursive definition for the next level of aggregation
	Aggs *Aggregation `json:"aggs,omitempty"`
}

// NEW STRUCT
type DateHistogram struct {
    Field string `json:"field"` // Field containing the date (e.g., "created_at")
    Interval string `json:"interval"` // Bucket size: "minute", "hour", "day", "month", "year"
}

// NEW STRUCT
type TopHits struct {
    Size int `json:"size"` // Number of top documents to return
    Sort []SortField `json:"sort,omitempty"` // How to sort them (reuses existing SortField)
    Source []string `json:"source,omitempty"` // Which fields to project (reuses existing source projection)
}

// SortField defines how to sort the final result set
type SortField struct {
	Field string    `json:"field"`
	Order SortOrder `json:"order"`
}

// Query defines a standard single search, which can now be used recursively.
type Query struct {
	Bool      Bool                   `json:"bool"`
	Index     string                 `json:"index"`
	Composite map[string]interface{} `json:"composite,omitempty"`
	ResultField string                 `json:"resultField,omitempty"` // Field to select/return from this query (used for subqueries)

	// NEW: Result Control Fields
	Sort      []SortField            `json:"sort,omitempty"`      // For sorting final documents
	Limit     int                    `json:"limit,omitempty"`     // For paging: max documents to return
	Offset    int                    `json:"offset,omitempty"`    // For paging: starting index
	Source    []string               `json:"source,omitempty"`    // For field projection (subset of fields to return)

	Aggs *Aggregation `json:"aggs,omitempty"` // Top-level aggregation definition
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

// GetIndexConfigurationInput holds the necessary parameters for fetching the index config.
// (Needed here for the recursive subquery call signature)
type GetIndexConfigurationInput struct {
	GithubClient *github.Client
	Owner string
	Stage string
	Repo string
	Branch string
	Id string // Index ID (e.g., "ads", "users")
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

    // NEW FIELD TO STORE TOP HITS
    TopHits []map[string]interface{} `json:"topHits,omitempty"` 

	// Nested buckets for the next level of grouping
	Buckets []Bucket `json:"buckets,omitempty"`
}

// AggregationResult wraps the top-level buckets.
type AggregationResult struct {
	Name    string   `json:"name"`
	Buckets []Bucket `json:"buckets"`
}

// ----------------------------------------------------
// Helper Functions (Dot Notation, Date Parsing, Geo)
// ----------------------------------------------------

// resolveDotNotation safely traverses a nested map[string]interface{} using a dot-separated path (e.g., "user.name").
func resolveDotNotation(data map[string]interface{}, path string) (string, bool) {
	if data == nil {
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

// resolveRawDotNotation is a utility to resolve a dot notation path and return the raw interface{} value.
func resolveRawDotNotation(data map[string]interface{}, path string) (interface{}, bool) {
    if data == nil {
        return nil, false
    }

    parts := strings.Split(path, ".")
    current := data
    var val interface{}
    var ok bool

    for i, part := range parts {
        val, ok = current[part]
        if !ok {
            // Key not found in the current map level
            return nil, false
        }

        if i < len(parts)-1 {
            // If it's not the final part of the path, we must continue traversing
            current, ok = val.(map[string]interface{})
            if !ok {
                // The intermediate key does not lead to another map
                return nil, false
            }
        }
    }
    // Return the value found at the end of the path
    return val, true
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

// --- Geospatial Helpers (NEW) ---

// EARTH_RADIUS_KM is the radius of the Earth in kilometers.
const EARTH_RADIUS_KM = 6371.0
// EARTH_RADIUS_MI is the radius of the Earth in miles.
const EARTH_RADIUS_MI = 3958.8

// degreesToRadians converts degrees to radians.
func degreesToRadians(degrees float64) float64 {
	return degrees * (math.Pi / 180)
}

// Haversine calculates the great-circle distance between two points on a sphere.
// It returns the distance in kilometers.
func Haversine(lat1, lon1, lat2, lon2 float64) float64 {
	// Convert degrees to radians
	rLat1 := degreesToRadians(lat1)
	rLat2 := degreesToRadians(lat2)
	rLon1 := degreesToRadians(lon1)
	rLon2 := degreesToRadians(lon2)

	// Haversine formula
	dLon := rLon2 - rLon1
	dLat := rLat2 - rLat1

	a := math.Pow(math.Sin(dLat/2), 2) + math.Cos(rLat1)*math.Cos(rLat2)*math.Pow(math.Sin(dLon/2), 2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	// Result is in kilometers (default radius)
	return EARTH_RADIUS_KM * c
}

// EvaluateGeoDistance checks if a document's geographic field is within the specified radius.
// It supports two formats for document coordinates:
// 1. Nested Object (e.g., {"coordinates": {"lat": 42.0, "lon": -83.0}})
// 2. Comma-separated String (e.g., {"location": "42.0,-83.0"})
func EvaluateGeoDistance(itemData map[string]interface{}, geo *GeoDistance) bool {
    var docLat, docLon float64
    var err error

    // 1. Try to resolve the field path to a specific value
    fieldValue, exists := resolveRawDotNotation(itemData, geo.Field)
    if !exists {
        log.Printf("GeoDistance: Field '%s' not found in document.", geo.Field)
        return false
    }

    // --- CASE 1: Nested Object (e.g., {"coordinates": {"lat": N, "lon": N}}) ---
    if geoFieldMap, ok := fieldValue.(map[string]interface{}); ok {
        latVal, latExists := geoFieldMap["lat"]
        lonVal, lonExists := geoFieldMap["lon"]

        if !latExists || !lonExists {
            log.Printf("GeoDistance: Required 'lat' or 'lon' not found in nested object for field '%s'.", geo.Field)
            return false
        }

        // Parse Latitude
        docLat, err = parseCoordinate(latVal)
        if err != nil {
            log.Printf("GeoDistance: Failed to parse latitude from object (%v): %v", latVal, err)
            return false
        }
        // Parse Longitude
        docLon, err = parseCoordinate(lonVal)
        if err != nil {
            log.Printf("GeoDistance: Failed to parse longitude from object (%v): %v", lonVal, err)
            return false
        }

    // --- CASE 2: Comma-separated String (e.g., {"location": "N,N"}) ---
    } else if geoValueStr, ok := fieldValue.(string); ok {
        coords := strings.Split(geoValueStr, ",")
        if len(coords) != 2 {
            log.Printf("GeoDistance: Invalid string coordinate format for field '%s': %s (Expected 'N,N')", geo.Field, geoValueStr)
            return false
        }

        docLat, err = strconv.ParseFloat(strings.TrimSpace(coords[0]), 64)
        if err != nil {
            log.Printf("GeoDistance: Failed to parse string latitude: %s", coords[0])
            return false
        }
        docLon, err = strconv.ParseFloat(strings.TrimSpace(coords[1]), 64)
        if err != nil {
            log.Printf("GeoDistance: Failed to parse string longitude: %s", coords[1])
            return false
        }
    } else {
        log.Printf("GeoDistance: Field '%s' value is neither a nested object nor a string.", geo.Field)
        return false
    }

    // Log inputs
    log.Printf("Haversine Input: Center (%.4f, %.4f), Doc (%.4f, %.4f)", geo.Latitude, geo.Longitude, docLat, docLon)

    // Calculate distance
    distanceKM := Haversine(geo.Latitude, geo.Longitude, docLat, docLon)

    // Log output
    log.Printf("Haversine Output: distanceKM = %f", distanceKM)
    
    // 3. Convert calculated distance to the search unit and compare
    var targetDistance float64
    searchUnit := strings.ToLower(geo.Unit)
    
    if searchUnit == "mi" || searchUnit == "miles" {
        targetDistance = distanceKM / (EARTH_RADIUS_KM / EARTH_RADIUS_MI)
    } else { 
        targetDistance = distanceKM
    }

    log.Printf("GeoDistance: Doc coords (%.4f, %.4f). Calculated distance: %.2f %s. Required: %.2f %s", 
        docLat, docLon, targetDistance, searchUnit, geo.Distance, searchUnit)

    return targetDistance <= geo.Distance
}

// parseCoordinate is a helper to convert various types to float64.
func parseCoordinate(v interface{}) (float64, error) {
    switch val := v.(type) {
    case float64:
        return val, nil
    case string:
        return strconv.ParseFloat(val, 64)
    case int:
        return float64(val), nil
    default:
        return 0, fmt.Errorf("unsupported coordinate type: %T", val)
    }
}


// ----------------------------------------------------
// Evaluation Logic
// ----------------------------------------------------

// EvaluateBool performs the actual comparison logic (date, string, numeric, set).
func EvaluateBool(c Condition, targetValue string, op Operation) bool {
	// ... (Existing logic for date, string, numeric, and set operations) ...
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
			// If numerical parsing failed, comparison operations cannot proceed
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

// EvaluateRange performs range checks (numeric or date).
func EvaluateRange(data map[string]interface{}, field string, r *Range) bool {
	targetValue, exists := resolveDotNotation(data, field)
	if !exists {
		return false
	}

	// 1. Try Date Comparison
	targetTime, errTT := tryParseDate(targetValue)
	isDateOperation := errTT == nil

	if isDateOperation {
		if r.From != nil {
			fromTime, errF := tryParseDate(*r.From)
			if errF != nil || targetTime.Before(fromTime) {
				return false
			}
		}
		if r.To != nil {
			toTime, errT := tryParseDate(*r.To)
			// Range is typically exclusive, so targetTime must be strictly Before
			if errT != nil || !targetTime.Before(toTime) {
				return false
			}
		}
		log.Printf("EvaluateRange: Date match for field '%s'.", field)
		return true
	}

	// 2. Try Numeric Comparison
	targetFloat, errT := strconv.ParseFloat(targetValue, 64)
	if errT != nil {
		log.Printf("EvaluateRange: Field '%s' is neither numeric nor a date.", field)
		return false
	}

	if r.From != nil {
		fromFloat, errF := strconv.ParseFloat(*r.From, 64)
		if errF != nil {
			log.Printf("EvaluateRange: Invalid numeric 'from' value: %s", *r.From)
			return false
		}
		if targetFloat < fromFloat {
			return false
		}
	}

	if r.To != nil {
		toFloat, errT := strconv.ParseFloat(*r.To, 64)
		if errT != nil {
			log.Printf("EvaluateRange: Invalid numeric 'to' value: %s", *r.To)
			return false
		}
		// Range is typically exclusive: [From, To)
		if targetFloat >= toFloat {
			return false
		}
	}

	log.Printf("EvaluateRange: Numeric match for field '%s'.", field)
	return true
}

func (b *Bool) Evaluate(data map[string]interface{}, ctx context.Context, client *github.Client, indexInput *GetIndexConfigurationInput) bool {
	// ... (Existing Bool logic remains unchanged) ...
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

	// B) Handle Range logic
	if c.Range != nil {
		return EvaluateRange(data, c.Range.Field, c.Range)
	}

	// C) Handle Geospatial logic (NEW)
	if c.GeoDistance != nil {
		return EvaluateGeoDistance(data, c.GeoDistance)
	}

	// D) Extract Condition and default Operation (for Term/Filter/Match)
	var condition Condition
	var defaultOp Operation = Equal

	if c.Term != nil {
		condition = *c.Term
	} else if c.Filter != nil {
		condition = *c.Filter
	} else if c.Match != nil {
		condition = *c.Match
	} else {
		return true // Empty case matches (excluding Range/Geo, which are handled above)
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
// Metric Calculation Functions (Updated)
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
		return 0.0
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
		return 0.0
	}

	maxVal := values[0]
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}

// calculateStdDev calculates the sample standard deviation of a set of values. (NEW)
func calculateStdDev(docs []map[string]interface{}, field string) float64 {
	values := extractNumericValues(docs, field)
	n := len(values)
	if n < 2 {
		return 0.0 // Need at least two points for meaningful sample std dev
	}

	mean := calculateSum(docs, field) / float64(n)

	var sumOfSquares float64
	for _, v := range values {
		diff := v - mean
		sumOfSquares += diff * diff
	}

	// Sample Standard Deviation: sqrt(Sum((x_i - mean)^2) / (n - 1))
	return math.Sqrt(sumOfSquares / float64(n-1))
}

// calculatePercentile calculates the value at a specific percentile (0-100). (NEW)
func calculatePercentile(docs []map[string]interface{}, field string, percentile float64) float64 {
	values := extractNumericValues(docs, field)
	if len(values) == 0 {
		return 0.0
	}

	sort.Float64s(values)
	n := float64(len(values))

	// Formula: R = P/100 * (N-1) + 1. Index i = R-1
	// Using a simple index calculation for percentile: (N-1) * P / 100
	index := (n - 1.0) * (percentile / 100.0)

	i := int(index)
	fraction := index - float64(i)

	if i >= int(n-1) {
		return values[int(n)-1]
	}
	if i < 0 {
		return values[0]
	}

	// Linear interpolation
	return values[i] + fraction*(values[i+1]-values[i])
}

// In search/package.go

// calculateCardinality counts the number of unique string values for a field.
func calculateCardinality(docs []map[string]interface{}, field string) int {
    uniqueValues := make(map[string]struct{})
    
    // Log the start of the calculation
    log.Printf("CalculateCardinality: Starting calculation for field '%s' on %d documents.", field, len(docs))
    
    for i, doc := range docs {
        // Use resolveDotNotation to get the string value for comparison
        valStr, exists := resolveDotNotation(doc, field)
        
        if exists {
            // Log successful retrieval and the value found
            log.Printf("CalculateCardinality DEBUG: Doc %d: Found value '%s'", i, valStr)
            
            // Add the value to the set of unique strings
            uniqueValues[valStr] = struct{}{}
        } else {
            // Log when a field is missing or the traversal failed
            log.Printf("CalculateCardinality DEBUG: Doc %d: Field '%s' not found or failed traversal.", i, field)
        }
    }
    
    // Log the final count
    finalCount := len(uniqueValues)
    log.Printf("CalculateCardinality: Completed. Found %d unique values for field '%s'.", finalCount, field)
    
    return finalCount
}

// CalculateMetrics iterates through the Aggregation's requested metrics and computes them. (UPDATED)
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
			case "avg", "mean":
				calculatedValue = calculateAvg(groupDocs, sourceField)
			case "median":
				calculatedValue = calculateMedian(groupDocs, sourceField)
			case "mode":
				calculatedValue = calculateMode(groupDocs, sourceField)
			case "min":
				calculatedValue = calculateMin(groupDocs, sourceField)
			case "max":
				calculatedValue = calculateMax(groupDocs, sourceField)
			case "std_dev": // NEW
				calculatedValue = calculateStdDev(groupDocs, sourceField)
			case "percentile": // NEW
				p := req.Percentile // Use the percentile specified in the request
				if p <= 0 || p > 100 {
					p = 50.0 // Default to median if invalid
					log.Printf("CalculateMetrics: Invalid percentile value in request, defaulting to 50th.")
				}
				calculatedValue = calculatePercentile(groupDocs, sourceField, p)
            case "cardinality", "unique_count": // NEW
				// Ensure the value is calculated and cast it to float64 for result consistency
				count := calculateCardinality(groupDocs, sourceField)
				calculatedValue = float64(count) // Cast int to float64
			case "count":
				// Count is implicit in the aggregation logic, but supported here for field-level count
				values := extractNumericValues(groupDocs, sourceField)
				calculatedValue = len(values)
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
// Result Control Helpers
// ----------------------------------------------------

// ProjectFields returns a new slice of documents containing only the fields specified in the Source slice.
func ProjectFields(docs []map[string]interface{}, source []string) []map[string]interface{} {
	if len(source) == 0 {
		return docs
	}

	projectedDocs := make([]map[string]interface{}, 0, len(docs))
	for _, doc := range docs {
		newDoc := make(map[string]interface{})
		for _, field := range source {
			// Simplified Projection: Assumes users only project top-level or fully nested keys.
			// A truly robust solution would recursively build the structure.
			val, exists := resolveDotNotation(doc, field) 
			if exists {
				// If the key is top-level (no dots), add the actual value type back
				if !strings.Contains(field, ".") {
					if originalVal, ok := doc[field]; ok {
						newDoc[field] = originalVal
						continue
					}
				}
				// Fallback for nested/missing: use the string value from resolveDotNotation
				newDoc[field] = val 
			}
		}
		projectedDocs = append(projectedDocs, newDoc)
	}
	log.Printf("ProjectFields: Reduced documents to %d fields.", len(source))
	return projectedDocs
}

// ApplySort sorts the documents based on the list of SortField definitions.
func ApplySort(docs []map[string]interface{}, sortFields []SortField) {
	if len(sortFields) == 0 || len(docs) < 2 {
		return
	}

	log.Printf("ApplySort: Sorting %d documents.", len(docs))

	// Sort uses a closure to implement the less function based on multiple fields
	sort.Slice(docs, func(i, j int) bool {
		for _, sf := range sortFields {
			valI, existsI := resolveDotNotation(docs[i], sf.Field)
			valJ, existsJ := resolveDotNotation(docs[j], sf.Field)

			// Missing values are treated as smaller than existing values
			if !existsI && existsJ { return sf.Order == SortAsc }
			if existsI && !existsJ { return sf.Order == SortDesc }
			if !existsI && !existsJ { continue } // Both missing, check next field

			// Try Numeric Comparison
			numI, errI := strconv.ParseFloat(valI, 64)
			numJ, errJ := strconv.ParseFloat(valJ, 64)

			var less bool
			if errI == nil && errJ == nil {
				// Numeric comparison
				if numI != numJ {
					less = numI < numJ
					return (sf.Order == SortAsc && less) || (sf.Order == SortDesc && !less)
				}
			} else {
				// String comparison (includes dates)
				if valI != valJ {
					less = valI < valJ
					return (sf.Order == SortAsc && less) || (sf.Order == SortDesc && !less)
				}
			}
			// Values are equal, move to the next sort field
		}
		return false // Considered equal based on all sort fields
	})
}

// ApplyPaging applies the Limit and Offset constraints to the document list.
func ApplyPaging(docs []map[string]interface{}, limit, offset int) []map[string]interface{} {
	if offset < 0 { offset = 0 }
	if limit <= 0 { limit = len(docs) } // Limit 0 or negative means no limit

	if offset >= len(docs) {
		log.Printf("ApplyPaging: Offset %d is beyond the total count %d. Returning empty slice.", offset, len(docs))
		return []map[string]interface{}{}
	}

	start := offset
	end := offset + limit
	if end > len(docs) {
		end = len(docs)
	}

	log.Printf("ApplyPaging: Returning documents from index %d to %d (Limit: %d).", start, end-1, limit)
	return docs[start:end]
}


// ----------------------------------------------------
// Recursive Subquery Execution Logic (UNCHANGED)
// ----------------------------------------------------

func ExecuteSubQuery(ctx context.Context, client *github.Client, baseInput *GetIndexConfigurationInput, subQuery *Query, resultField string) ([]string, error) {
	// ... (Existing logic for fetching index config, building path, fetching contents, filtering, and extracting results) ...
	log.Printf("ExecuteSubQuery: Starting recursive search for index '%s' and composite keys: %+v", subQuery.Index, subQuery.Composite)

	// 1. Get the Index Config for the subQuery's index
	subInput := *baseInput
	subInput.Id = subQuery.Index
	subInput.GithubClient = client

	// NOTE: GetIndexById is assumed to exist in another file (e.g., repo/index.go)
	// We'll simulate fetching a map here for completeness.
	// You must replace this with your actual Index fetching logic.
	subIndexObject, err := func(input *GetIndexConfigurationInput) (map[string]interface{}, error) {
		// DUMMY IMPLEMENTATION: REPLACE WITH ACTUAL GITHUB FETCH LOGIC
		return map[string]interface{}{
			"fields": []interface{}{"region", "category"},
			"repoName": "dummy/repo",
			"searchRootPath": "data/search",
		}, nil
	}(&subInput)
	// End DUMMY IMPLEMENTATION

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
	// DUMMY: Replace with actual client.Repositories.GetContents
	var dirContents []*github.RepositoryContent
	if repoToFetch == "dummy/repo" {
		log.Print("ExecuteSubQuery: [DUMMY] Simulating content fetch.")
		// We can't actually fetch content in this dummy function, so this will likely be empty
	}
	// End DUMMY

	if err != nil || dirContents == nil {
		log.Printf("ExecuteSubQuery: Failed to fetch contents from path '%s': %v", contentPath, err)
		// For the sake of not crashing the dummy code, we'll return an empty list on failure
		return nil, nil
	}

	// 4. Filter contents using the subQuery's Bool logic and extract the target field
	results := make([]string, 0)
	for _, content := range dirContents {
		if content.GetType() != "file" {
			continue
		}

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
// Recursive Aggregation Logic (UPDATED for Range Buckets)
// ----------------------------------------------------

// In search/package.go

// ExecuteAggregation recursively groups and calculates metrics on a list of documents.
func ExecuteAggregation(docs []map[string]interface{}, agg *Aggregation) []Bucket {
    // Check for valid request and determine if any grouping mechanism is active.
    hasGrouping := len(agg.GroupBy) > 0 || len(agg.RangeBuckets) > 0 || agg.DateHistogram != nil
    hasMetrics := agg != nil && len(agg.Metrics) > 0

    if !hasGrouping {
        if hasMetrics {
            // SPECIAL CASE: Only metrics requested (top-level aggregation).
            log.Print("ExecuteAggregation: No grouping defined. Calculating top-level metrics.")
            
            // Create a single, anonymous bucket for the entire document set.
            topBucket := processGroup(agg, "", docs) 
            return []Bucket{topBucket}
        }
        
        // Final exit if neither grouping nor metrics are defined.
        log.Print("ExecuteAggregation: No grouping or metrics defined. Returning empty buckets.")
        return []Bucket{}
    }

    // --- 1. Standard GroupBy (Field Value Grouping) ---
    if len(agg.GroupBy) > 0 {
        groupByField := agg.GroupBy[0] // Only use the first field for this level
        log.Printf("ExecuteAggregation: Grouping by field '%s'", groupByField)

        groups := make(map[string][]map[string]interface{})
        for _, doc := range docs {
            key, exists := resolveDotNotation(doc, groupByField)
            if exists {
                groups[key] = append(groups[key], doc)
            }
        }
        
        buckets := make([]Bucket, 0, len(groups))
        for key, groupDocs := range groups {
            newBucket := processGroup(agg, key, groupDocs)
            buckets = append(buckets, newBucket)
        }
        return buckets

    // --- 2. Grouping by numeric ranges (Histogram) ---
    } else if len(agg.RangeBuckets) > 0 {
        // RangeBuckets is a map, we process the first (and typically only) entry
        var field string
        var ranges []RangeBucket
        for f, r := range agg.RangeBuckets {
            field = f
            ranges = r
            break
        }
        log.Printf("ExecuteAggregation: Grouping by range on field '%s'", field)

        rangeGroups := make(map[string][]map[string]interface{})
        for _, doc := range docs {
            valStr, exists := resolveDotNotation(doc, field)
            if !exists {
                continue
            }
            val, err := strconv.ParseFloat(valStr, 64)
            if err != nil {
                continue // Skip non-numeric/unparseable values for range bucketing
            }
            
            // Find which range bucket this value belongs to
            for _, r := range ranges {
                // To=0.0 implies no upper bound
                if val >= r.From && (r.To == 0.0 || val < r.To) { 
                    rangeGroups[r.Key] = append(rangeGroups[r.Key], doc)
                    break 
                }
            }
        }
        
        buckets := make([]Bucket, 0, len(ranges))
        for _, r := range ranges { // Iterate over the defined ranges to ensure all keys are present
            groupDocs := rangeGroups[r.Key]
            newBucket := processGroup(agg, r.Key, groupDocs)
            buckets = append(buckets, newBucket)
        }
        return buckets
    
    // --- 3. Grouping by Date Histogram (Time-based Grouping) ---
    } else if agg.DateHistogram != nil {
        dh := agg.DateHistogram
        log.Printf("ExecuteAggregation: Grouping by Date Histogram on field '%s' with interval '%s'", dh.Field, dh.Interval)

        groups := make(map[string][]map[string]interface{})
        
        // Define the Go time format based on the requested interval
        var format string
        switch strings.ToLower(dh.Interval) {
        case "minute":
            format = "2006-01-02T15:04" // Year-Month-DayTHour:Minute
        case "hour":
            format = "2006-01-02T15"    // Year-Month-DayTHour
        case "day":
            format = "2006-01-02"       // Year-Month-Day
        case "month":
            format = "2006-01"          // Year-Month
        case "year":
            format = "2006"             // Year
        default:
            log.Printf("ExecuteAggregation: Invalid date histogram interval '%s'.", dh.Interval)
            return []Bucket{}
        }

        for _, doc := range docs {
            valStr, exists := resolveDotNotation(doc, dh.Field)
            if !exists {
                continue
            }
            
            t, err := tryParseDate(valStr) 
            if err != nil {
                continue 
            }
            
            // Format the time to the chosen precision to create the bucket key
            key := t.Format(format)
            groups[key] = append(groups[key], doc)
        }
        
        // Convert map to buckets
        buckets := make([]Bucket, 0, len(groups))
        for key, groupDocs := range groups {
            newBucket := processGroup(agg, key, groupDocs)
            buckets = append(buckets, newBucket)
        }
        
        // Sort buckets chronologically by key (the formatted date string)
        sort.Slice(buckets, func(i, j int) bool {
            return buckets[i].Key < buckets[j].Key
        })
        
        return buckets
    }

    return []Bucket{} 
}

// processGroup handles metric calculation and recursion for a single group of documents.
func processGroup(agg *Aggregation, key string, groupDocs []map[string]interface{}) Bucket {
    newBucket := Bucket{
        Key:   key,
        Count: len(groupDocs),
    }

    // 2. Calculate Metrics for this group
    if len(agg.Metrics) > 0 {
        newBucket.Metrics = CalculateMetrics(groupDocs, agg.Metrics)
    }

    // NEW: Handle Top Hits
    if agg.TopHits != nil && agg.TopHits.Size > 0 {
        hitsDocs := groupDocs // Start with all documents in the group
        
        // 1. Apply Top Hits Sorting
        if len(agg.TopHits.Sort) > 0 {
            // NOTE: We must clone hitsDocs before sorting if we need the original order for other ops,
            // but for aggregation context, mutating the slice is acceptable since we only use it here.
            ApplySort(hitsDocs, agg.TopHits.Sort)
        }
        
        // 2. Apply Top Hits Paging (Limit/Size)
        // Use ApplyPaging logic: offset=0, limit=TopHits.Size
        hitsDocs = ApplyPaging(hitsDocs, agg.TopHits.Size, 0) 
        
        // 3. Apply Top Hits Projection
        if len(agg.TopHits.Source) > 0 {
            hitsDocs = ProjectFields(hitsDocs, agg.TopHits.Source)
        }
        
        newBucket.TopHits = hitsDocs
    }

    // 3. Handle Nested Aggregations
    if agg.Aggs != nil {
        newBucket.Buckets = ExecuteAggregation(groupDocs, agg.Aggs)
    }
    return newBucket
}

// ----------------------------------------------------
// GitHub Index Configuration Retrieval (UNCHANGED)
// ----------------------------------------------------

// GetIndexById retrieves the index configuration JSON file from the GitHub repository.
func GetIndexById(c *GetIndexConfigurationInput) (map[string]interface{}, error) {
	log.Printf("GetIndexById: Attempting to retrieve config for ID: %s", c.Id)
	var contract map[string]interface{}

	pieces := strings.Split(c.Repo, "/")
	opts := &github.RepositoryContentGetOptions{
		Ref: c.Branch,
	}
	// File path is assumed to be index/{ID}.json
	file, _, res, err := c.GithubClient.Repositories.GetContents(context.Background(), pieces[0], pieces[1], "index/"+c.Id+".json", opts)
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