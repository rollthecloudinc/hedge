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
	"bytes"
	"unicode"
	"text/template"
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
	Fuzziness *int       `json:"fuzziness,omitempty"` // NEW: Max edit distance for fuzzy matching
	Boost	  *float64   `json:"boost,omitempty"`     // NEW: Relevance boost factor (default 1.0)
}

// NestedDoc defines a filter that must be applied to individual objects within a document array. (NEW)
type NestedDoc struct {
    Path string `json:"path"` // The field path to the array (e.g., "seller.reviews")
    Bool Bool   `json:"bool"` // The Boolean logic applied to each object in the array
}


// Exists defines a filter that matches documents where the specified field is present and non-null. (NEW)
type Exists struct {
    Field string `json:"field"`
}

// Missing defines a filter that matches documents where the specified field is missing or null. (NEW)
type Missing struct {
    Field string `json:"field"`
}

// Template defines a condition where the document is matched if the execution
// of the provided Go template code returns a "true" value. (NEW)
type Template struct {
    Code string `json:"code"` // The Go template string to execute (must resolve to "true")
}

// ScoreFunction defines a single custom score modifier.
type ScoreFunction struct {
    Type    string `json:"type"`          // "factor" or "decay"
    // The Go Template code to execute. Must resolve to a numeric value (float64).
    Code    string `json:"code"`
    // Optional weight to apply to the result of this function. Default 1.0.
    Weight  *float64 `json:"weight,omitempty"` 
    // Field to use for the decay function (e.g., date, geo)
    Field   string `json:"field,omitempty"` 
}

// FunctionScore wraps the custom scoring logic for a query.
type FunctionScore struct {
    // Defines how the score is combined: "multiply", "sum", "replace". Default: "multiply"
    Combine string `json:"combine,omitempty"` 
    // List of functions to execute
    Functions []ScoreFunction `json:"functions"` 
}

// Case wraps one condition type (Term, Bool, Filter, Match, Range, GeoDistance).
type Case struct {
	Term        *Term        `json:"term,omitempty"`
	Bool        *Bool        `json:"bool,omitempty"`
	Filter      *Filter      `json:"filter,omitempty"`
	Match       *Match       `json:"match,omitempty"`
	Range       *Range       `json:"range,omitempty"`      // NEW: Range query
	GeoDistance *GeoDistance `json:"geoDistance,omitempty"` // NEW: Geospatial query
	NestedDoc   *NestedDoc 	 `json:"nested,omitempty"` // NEW: Geospatial query
	Missing     *Missing 	 `json:"missing,omitempty"`
	Exists      *Exists 	 `json:"exists,omitempty"`
	Template    *Template    `json:"template,omitempty"`   // NEW: Template-based condition
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

	// Path for the nested aggregation type
	Path string `json:"path,omitempty"`
	SubAggs map[string]*Aggregation `json:"subAggs,omitempty"` // Used for standard multi-level aggregations
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

	Aggs *Aggregation `json:"aggs,omitempty"`
	// Top-level aggregation definition

    // NEW FIELD
    ScoreModifiers *FunctionScore      `json:"scoreModifiers,omitempty"` 
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
func EvaluateGeoDistance(itemData map[string]interface{}, geo *GeoDistance) (bool, float64) {
    var docLat, docLon float64
    var err error

    // 1. Try to resolve the field path to a specific value
    fieldValue, exists := resolveRawDotNotation(itemData, geo.Field)
    if !exists {
        log.Printf("GeoDistance: Field '%s' not found in document.", geo.Field)
        return false, 0.0
    }

    // --- CASE 1: Nested Object (e.g., {"coordinates": {"lat": N, "lon": N}}) ---
    if geoFieldMap, ok := fieldValue.(map[string]interface{}); ok {
        latVal, latExists := geoFieldMap["lat"]
        lonVal, lonExists := geoFieldMap["lon"]

        if !latExists || !lonExists {
            log.Printf("GeoDistance: Required 'lat' or 'lon' not found in nested object for field '%s'.", geo.Field)
            return false, 0.0
        }

        // Parse Latitude
        docLat, err = parseCoordinate(latVal)
        if err != nil {
            log.Printf("GeoDistance: Failed to parse latitude from object (%v): %v", latVal, err)
            return false, 0.0
        }
        // Parse Longitude
        docLon, err = parseCoordinate(lonVal)
        if err != nil {
            log.Printf("GeoDistance: Failed to parse longitude from object (%v): %v", lonVal, err)
            return false, 0.0
        }

    // --- CASE 2: Comma-separated String (e.g., {"location": "N,N"}) ---
    } else if geoValueStr, ok := fieldValue.(string); ok {
        coords := strings.Split(geoValueStr, ",")
        if len(coords) != 2 {
            log.Printf("GeoDistance: Invalid string coordinate format for field '%s': %s (Expected 'N,N')", geo.Field, geoValueStr)
            return false, 0.0
        }

        docLat, err = strconv.ParseFloat(strings.TrimSpace(coords[0]), 64)
        if err != nil {
            log.Printf("GeoDistance: Failed to parse string latitude: %s", coords[0])
            return false, 0.0
        }
        docLon, err = strconv.ParseFloat(strings.TrimSpace(coords[1]), 64)
        if err != nil {
            log.Printf("GeoDistance: Failed to parse string longitude: %s", coords[1])
            return false, 0.0
        }
    } else {
        log.Printf("GeoDistance: Field '%s' value is neither a nested object nor a string.", geo.Field)
        return false, 0.0
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

    return targetDistance <= geo.Distance, 0.0
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


// EvaluateMatch determines if a document matches the provided Match condition.
// It prioritizes Fuzziness over analyzed matching, and falls back to analyzed 
// token matching for general relevance scoring. The old Modifiers (Contains/StartsWith)
// are removed as the Analyzed Match is a far superior relevance indicator.
func EvaluateMatch(data map[string]interface{}, m *Match) (bool, float64) {
    if m == nil || m.Field == "" || m.Value == "" {
        return false, 0.0
    }

    // 1. Handle Boost
    boost := 1.0
    if m.Boost != nil {
        boost = *m.Boost
    }

    // 2. Resolve the target value from the document
    targetValueRaw, exists := resolveRawDotNotation(data, m.Field)
    if !exists || targetValueRaw == nil {
        return false, 0.0
    }
    documentText := fmt.Sprintf("%v", targetValueRaw)
    conditionValue := m.Value

    // --- A. Fuzzy Match Check (Prioritized, operates on raw tokens) ---
    if m.Fuzziness != nil {
        maxDistance := *m.Fuzziness
        
        // Tokenize the document text based on non-letter/non-number delimiters (basic tokenizer)
        tokens := strings.FieldsFunc(documentText, func(r rune) bool {
            return !unicode.IsLetter(r) && !unicode.IsNumber(r)
        })

        maxScore := 0.0
        matchFound := false
        conditionValueLower := strings.ToLower(conditionValue)

        // Check distance against the lowercased document tokens
        for _, token := range tokens {
            if token == "" {
                continue
            }
            
            distance := LevenshteinDistance(strings.ToLower(token), conditionValueLower)

            if distance <= maxDistance {
                matchFound = true
                
                // Calculate score: 1.0 + (score bonus based on proximity)
                baseScore := 1.0 + ((float64(maxDistance) - float64(distance)) / float64(maxDistance))
                score := baseScore * boost
                if score > maxScore {
                    maxScore = score // Keep the best matching token's score
                }
            }
        }
        
        if matchFound {
            log.Printf("EvaluateMatch: Fuzzy MATCH. Max Score: %.4f", maxScore)
            return true, maxScore
        }
    }
    
    // --- B. Analyzed Token Overlap Check (Relevance Fallback) ---

    // Get the document value as a string for analysis
    rawDocValueStr, _ := resolveDotNotation(data, m.Field) 
    
    // docTokens: Contains the processed, stemmed, and lowercased words from the document field.
    docTokens := Analyze(rawDocValueStr) 

    // queryTokens: Contains the processed, stemmed, and lowercased words from the user's query.
    queryTokens := Analyze(m.Value) 

    if len(queryTokens) == 0 {
        // If query analyzes to nothing, return false (unless a fuzzy match already occurred).
        return false, 0.0
    }
    
    // 3. Compare analyzed tokens to find overlap.
    matchCount := 0
    docTokenSet := make(map[string]struct{}, len(docTokens))
    for _, dToken := range docTokens {
        docTokenSet[dToken] = struct{}{}
    }

    for _, qToken := range queryTokens {
        if _, found := docTokenSet[qToken]; found {
            matchCount++
        }
    }
    
    // 4. Determine Match and Calculate Score.
    
    if matchCount == 0 {
        return false, 0.0
    }

    // Simple Relevance Score Calculation (Term Frequency x Document Length Normalization)
    tf := float64(matchCount) 

    // Document Length Normalization: Penalize matches in very long documents.
    docLengthFactor := 1.0
    if len(docTokens) > 10 {
        docLengthFactor = 1.0 / math.Log(float64(len(docTokens) + 1))
    }
    
    // Final Score: Apply relevance calculation and boost.
    score := (tf * docLengthFactor) * boost

    log.Printf("EvaluateMatch: Analyzed MATCH. Score=%.4f (Matches: %d)", score, matchCount)
    
    return true, score
    
    // NOTE: The legacy Modifiers (Contains, StartsWith, EndsWith, Equal) are omitted here 
    // because they are superseded by the tokenized, stemmed match logic which 
    // provides a scored relevance result, not just a boolean filter.
}

// EvaluateRange performs range checks (numeric or date).
func EvaluateRange(data map[string]interface{}, field string, r *Range) (bool, float64) {
	targetValue, exists := resolveDotNotation(data, field)
	if !exists {
		return false, 0.0
	}

	// 1. Try Date Comparison
	targetTime, errTT := tryParseDate(targetValue)
	isDateOperation := errTT == nil

	if isDateOperation {
		if r.From != nil {
			fromTime, errF := tryParseDate(*r.From)
			if errF != nil || targetTime.Before(fromTime) {
				return false, 0.0
			}
		}
		if r.To != nil {
			toTime, errT := tryParseDate(*r.To)
			// Range is typically exclusive, so targetTime must be strictly Before
			if errT != nil || !targetTime.Before(toTime) {
				return false, 0.0
			}
		}
		log.Printf("EvaluateRange: Date match for field '%s'.", field)
		return true, 0.0
	}

	// 2. Try Numeric Comparison
	targetFloat, errT := strconv.ParseFloat(targetValue, 64)
	if errT != nil {
		log.Printf("EvaluateRange: Field '%s' is neither numeric nor a date.", field)
		return false, 0.0
	}

	if r.From != nil {
		fromFloat, errF := strconv.ParseFloat(*r.From, 64)
		if errF != nil {
			log.Printf("EvaluateRange: Invalid numeric 'from' value: %s", *r.From)
			return false, 0.0
		}
		if targetFloat < fromFloat {
			return false, 0.0
		}
	}

	if r.To != nil {
		toFloat, errT := strconv.ParseFloat(*r.To, 64)
		if errT != nil {
			log.Printf("EvaluateRange: Invalid numeric 'to' value: %s", *r.To)
			return false, 0.0
		}
		// Range is typically exclusive: [From, To)
		if targetFloat >= toFloat {
			return false, 0.0
		}
	}

	log.Printf("EvaluateRange: Numeric match for field '%s'.", field)
	return true, 0.0
}

// EvaluateTemplate executes a Go template against the document data.
// It matches if the executed template output is a "truthy" string (e.g., "true", "1", non-empty).
// For simplicity, we require the template to render the exact string "true" to pass.
// search/package.go (Evaluation Logic)

// ... (Requires import "strconv" at the top of the file)

// search/package.go (Evaluation Logic)

// EvaluateTemplate executes a Go template against the document data.
// It matches if the executed template output is the exact string "true".
func EvaluateTemplate(data map[string]interface{}, tmpl *Template) (bool, float64) {
    
    // The user's input (tmpl.Code) should now contain the full, explicit 
    // Go template string, including all necessary {{ }} wrappers.
    templateCode := tmpl.Code

    // 1. Parse the template (No function map registration)
    t, err := template.New("condition").Parse(templateCode) 
    
    if err != nil {
        log.Printf("EvaluateTemplate: Failed to parse template code: %v", err)
        return false, 0.0
    }

    // 2. Execute the template against the document data
    var buf bytes.Buffer
    // Pass the document map as the root object '.'
    if err := t.Execute(&buf, data); err != nil {
        // If execution fails (e.g., field access error), treat as non-match.
        log.Printf("EvaluateTemplate: Failed to execute template: %v", err)
        return false, 0.0
    }

    // 3. Check the output for a truthy value
    output := strings.TrimSpace(buf.String())

    // --- FIX: Robust Output Cleaning ---
    
    // 1. Attempt to decode the escaped string.
    if unquotedOutput, err := strconv.Unquote(output); err == nil {
        output = unquotedOutput
    }
    
    // 2. Fallback: If unquote failed, or if the string still contains artifacts, 
    //    manually remove common artifacts (backslashes and quotes).
    output = strings.ReplaceAll(output, `\`, "")
    output = strings.ReplaceAll(output, `"`, "")
    
    // 3. Re-trim whitespace just in case
    output = strings.TrimSpace(output)
    // --- END FIX ---
    
    // Define "truthy" output: Must be an exact match for the clean string "true".
    isTrue := strings.EqualFold(output, "true")

    log.Printf("EvaluateTemplate: Template executed. Output: '%s'. Matched: %t", output, isTrue)
    return isTrue, 0.0
}

func (b *Bool) Evaluate(data map[string]interface{}, ctx context.Context, client *github.Client, indexInput *GetIndexConfigurationInput) (bool, float64) {
	totalScore := 0.0

	// 1. ALL (Logical AND)
	if len(b.All) > 0 {
		matchCount := 0
		
		for _, c := range b.All {
			// Changed: Capture score from Case.Evaluate
			matched, score := c.Evaluate(data, ctx, client, indexInput)
			
			if !matched {
				return false, 0.0 // Single failure in ALL fails the whole Bool
			}
			
			// Score aggregation: SUM all scores for ALL/AND conditions
			totalScore += score
			matchCount++
		}
		
		if matchCount == len(b.All) {
			// Changed: Return the sum of scores
			return true, totalScore
		}
	}

	// 2. ONE (Logical OR)
	if len(b.One) > 0 {
		maxScore := 0.0
		matchFound := false
		
		for _, c := range b.One {
			// Changed: Capture score from Case.Evaluate
			matched, score := c.Evaluate(data, ctx, client, indexInput)
			
			if matched {
				matchFound = true
				// Score aggregation: MAX score for the whole OR group
				if score > maxScore {
					maxScore = score
				}
			}
		}
		
		if matchFound {
			// Changed: Return the maximum score found
			return true, maxScore
		}
		return false, 0.0 // Changed: Return 0.0 score
	}

	// 3. NONE (Logical NOT OR)
	if len(b.None) > 0 {
		for _, c := range b.None {
			// Changed: Capture the bool and discard the score
			matched, _ := c.Evaluate(data, ctx, client, indexInput) 
			
			if matched {
				return false, 0.0 // Single match in NONE fails the whole Bool
			}
		}
		// If nothing matched, the NONE condition passes with a neutral score
		return true, 0.0 // Changed: Return 0.0 score
	}

	// 4. NOT (Negation logic)
	if len(b.Not) > 0 {
		if len(b.Not) > 1 {
			log.Print("Bool.Evaluate: Warning, 'not' array has more than one element; only the first is evaluated.")
		}
		
		// Changed: Capture the bool and discard the score
		matched, _ := b.Not[0].Evaluate(data, ctx, client, indexInput)
		
		if !matched {
			// If the inner condition did NOT match, the NOT condition passes with neutral score
			return true, 0.0 // Changed: Return 0.0 score
		}
		return false, 0.0 // Changed: Return 0.0 score
	}

	// Empty Bool matches
	return true, 0.0 // Changed: Return 0.0 score
}

// Evaluate evaluates the condition represented by the Case, returning whether it matches (bool)
// and the associated score (float64). Filter conditions return 0.0 score.
func (c *Case) Evaluate(data map[string]interface{}, ctx context.Context, client *github.Client, indexInput *GetIndexConfigurationInput) (bool, float64) {
	var match bool
	var score float64

	// A) Handle nested Boolean logic
	if c.Bool != nil {
		// Delegates score accumulation/maxing to Bool.Evaluate
		return c.Bool.Evaluate(data, ctx, client, indexInput)
	}

	// B) Handle Range logic (Filter, Score 0.0)
	if c.Range != nil {
		// Changed: EvaluateRange now returns bool and float64 (0.0)
		match, score = EvaluateRange(data, c.Range.Field, c.Range)
		return match, score
	}

	// C) Handle Geospatial logic (Filter, Score 0.0)
	if c.GeoDistance != nil {
		// Changed: EvaluateGeoDistance now returns bool and float64 (0.0)
		match, score = EvaluateGeoDistance(data, c.GeoDistance)
		return match, score
	}

	// D) Handle Nested Document logic
	if c.NestedDoc != nil {
		// Delegates score maxing to EvaluateNestedDoc
		// Changed: EvaluateNestedDoc now returns bool and float64 (max score)
		return EvaluateNestedDoc(data, c.NestedDoc, ctx, client, indexInput) 
	}

	// E) Handle Exists logic (Filter, Score 0.0)
	if c.Exists != nil {
		val, exists := resolveRawDotNotation(data, c.Exists.Field)
		log.Printf("Evaluate: Checking EXISTS for field '%s'. Exists: %t, Value is nil: %t", c.Exists.Field, exists, val == nil)
		
		// Changed: Return score 0.0
		return exists && val != nil, 0.0
	}

	// F) Handle Missing logic (Filter, Score 0.0)
	if c.Missing != nil {
		val, exists := resolveRawDotNotation(data, c.Missing.Field)
		log.Printf("Evaluate: Checking MISSING for field '%s'. Exists: %t, Value is nil: %t", c.Missing.Field, exists, val == nil)
		
		// Changed: Return score 0.0
		return !exists || val == nil, 0.0
	}

	// G) Handle Template logic (Filter, Score 0.0)
	if c.Template != nil {
		// Changed: EvaluateTemplate now returns bool and float64 (0.0)
		match, score = EvaluateTemplate(data, c.Template)
		return match, score
	}

	// H) Handle Match logic (Relevancy, Score > 0.0 possible)
	if c.Match != nil {
		// Delegates score calculation to EvaluateMatch
		// Changed: EvaluateMatch now returns bool and float64 (calculated score)
		return EvaluateMatch(data, c.Match) 
	}

	// I) Extract Condition and default Operation (for Term/Filter)
	var condition Condition
	var defaultOp Operation = Equal

	if c.Term != nil {
		// Term condition: Treated as a filter (Score 0.0)
		condition = *c.Term
	} else if c.Filter != nil {
		// Filter condition: Treated as a filter (Score 0.0)
		condition = *c.Filter
	} else {
		return true, 0.0 // Changed: Empty case matches, score 0.0
	}

	if condition.GetModifiers() != nil {
		defaultOp = condition.GetModifiers().Operation
	}

	// --- 1. Handle SubQuery for IN/NOT IN (Filter, Score 0.0) ---
	if condition.GetSubQuery() != nil && (defaultOp == In || defaultOp == NotIn) {
		subQuery := condition.GetSubQuery()

		localCheckField := condition.GetField()
		resultField := subQuery.ResultField

		if resultField == "" {
			log.Printf("Case.Evaluate: Subquery must specify 'resultField' for IN/NOT IN operation.")
			return false, 0.0 // Changed: Return 0.0 score
		}

		log.Printf("Case.Evaluate: Executing recursive subquery. Target index: %s, Result field: %s", subQuery.Index, resultField)

		subResultData, err := ExecuteSubQuery(ctx, client, indexInput, subQuery, resultField)
		if err != nil {
			log.Printf("Case.Evaluate: Error executing subquery: %v", err)
			return false, 0.0 // Changed: Return 0.0 score
		}

		subResultSet := make(map[string]struct{})
		for _, val := range subResultData {
			subResultSet[val] = struct{}{}
		}

		localValue, exists := resolveDotNotation(data, localCheckField)
		if !exists {
			log.Printf("Case.Evaluate: Local check field '%s' not found in document.", localCheckField)
			return false, 0.0 // Changed: Return 0.0 score
		}

		_, localValueIsInSet := subResultSet[localValue]

		if defaultOp == In {
			return localValueIsInSet, 0.0 // Changed: Return 0.0 score
		}
		if defaultOp == NotIn {
			return !localValueIsInSet, 0.0 // Changed: Return 0.0 score
		}
	}

	// --- 2. Standard Value Evaluation (Dot notation) (Filter, Score 0.0) ---

	targetValue, exists := resolveDotNotation(data, condition.GetField())
	if !exists {
		return false, 0.0 // Changed: Return 0.0 score
	}

	// EvaluateBool is assumed to return a simple bool for Term/Filter/Match conditions
    // when called outside of EvaluateMatch. Since we are here, it's a Term/Filter.
	match = EvaluateBool(condition, targetValue, defaultOp)
    
    // Changed: Term and Filter conditions contribute 0.0 to the score.
    return match, 0.0
}

// EvaluateNestedDoc applies a Boolean query to every object within a target array path.
// The parent document matches if the nested Bool evaluates to true for AT LEAST ONE
// object in the array.
// EvaluateNestedDoc applies a Boolean query to every object within a target array path.
// The parent document matches if the nested Bool evaluates to true for AT LEAST ONE
// object in the array.
// Returns match status (bool) and the MAX score of any matching nested document.
func EvaluateNestedDoc(data map[string]interface{}, nested *NestedDoc, ctx context.Context, client *github.Client, indexInput *GetIndexConfigurationInput) (bool, float64) {
    // 1. Resolve the path to the array using the raw dot notation helper.
    arrayValue, exists := resolveRawDotNotation(data, nested.Path)
    if !exists {
        log.Printf("EvaluateNestedDoc: Array path '%s' not found.", nested.Path)
        return false, 0.0 // Changed: Return 0.0 score
    }

    // 2. Ensure the resolved value is indeed an array (slice of interface{}).
    array, ok := arrayValue.([]interface{})
    if !ok {
        log.Printf("EvaluateNestedDoc: Field '%s' is not an array (found type %T).", nested.Path, arrayValue)
        return false, 0.0 // Changed: Return 0.0 score
    }

    maxScore := 0.0 // Initialize the maximum score found
    matchFound := false

    // 3. Iterate through the array elements and apply the nested Bool logic.
    for i, item := range array {
        // Each item in the array must be treated as a separate map (nested document).
        itemMap, mapOk := item.(map[string]interface{})
        if !mapOk {
            log.Printf("EvaluateNestedDoc: Element %d in array is not a map (found type %T). Skipping.", i, item)
            continue
        }

        // Recursively evaluate the nested Bool logic on the current itemMap.
        // Changed: Capture the score from the recursive call.
        matched, score := nested.Bool.Evaluate(itemMap, ctx, client, indexInput) 
        
        if matched {
            matchFound = true
            // The score for the nested document match is the max score of its children
            if score > maxScore {
                maxScore = score
            }
        }
    }

    // Changed: Return the match status and the accumulated maximum score.
    if matchFound {
        return true, maxScore
    }
    
    // If the loop finishes, no nested document satisfied the condition.
    return false, 0.0
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
// ProjectFields returns a new slice of documents containing only the fields specified in the Source slice,
// ensuring that the special field "_score" is always preserved if it exists.
func ProjectFields(docs []map[string]interface{}, source []string) []map[string]interface{} {
    // If source is empty, return all documents, including existing _score
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
        
        // CRITICAL CHANGE: Preserve the "_score" field regardless of the source request
        if score, ok := doc["_score"]; ok {
            newDoc["_score"] = score
        }
        
        projectedDocs = append(projectedDocs, newDoc)
    }
    log.Printf("ProjectFields: Reduced documents to %d fields (plus _score if present).", len(source))
    return projectedDocs
}

// ApplySort sorts the documents based on the list of SortField definitions.
func ApplySort(docs []map[string]interface{}, sortFields []SortField) {
    if len(sortFields) == 0 || len(docs) < 2 {
        return
    }

    log.Printf("ApplySort: Sorting %d documents.", len(docs))

    sort.Slice(docs, func(i, j int) bool {
        for _, sf := range sortFields {
            var valI interface{}
            var valJ interface{}
            var existsI bool
            var existsJ bool

            // --- SPECIAL CASE: _score ---
            if sf.Field == "_score" {
                // Use assignment (=) for the variables declared above
                valI, existsI = docs[i]["_score"]
                valJ, existsJ = docs[j]["_score"]
                
                // Ensure the score exists and is a float64 for reliable comparison
                if existsI {
                    if _, ok := valI.(float64); !ok { existsI = false }
                }
                if existsJ {
                    if _, ok := valJ.(float64); !ok { existsJ = false }
                }
            } else {
                // --- STANDARD CASE: Other Fields ---
                var valIStr, valJStr string
                
                // IMPORTANT FIX: Use assignment (=), not short declaration (:=)
                valIStr, existsI = resolveDotNotation(docs[i], sf.Field)
                valJStr, existsJ = resolveDotNotation(docs[j], sf.Field)
                
                // Assign the string values to the outer interface{} variables
                valI = valIStr
                valJ = valJStr
            }

            // 1. Missing values logic (UNMODIFIED)
            // Missing values are treated as smaller than existing values (moved to the end in Desc, start in Asc)
            if !existsI && existsJ {
                return sf.Order == SortAsc // i is smaller (missing), return true if sorting ascending
            }
            if existsI && !existsJ {
                return sf.Order == SortDesc // j is smaller (missing), return true if sorting descending
            }
            if !existsI && !existsJ {
                continue // Both missing, check next field
            }

            // 2. Value Comparison (UNMODIFIED)
            var less bool
            
            if sf.Field == "_score" {
                // Numeric comparison for _score (already float64)
                numI := valI.(float64)
                numJ := valJ.(float64)
                
                if numI != numJ {
                    less = numI < numJ
                    return (sf.Order == SortAsc && less) || (sf.Order == SortDesc && !less)
                }
            } else {
                // Standard field comparison (Try Numeric then String)
                valIStr := valI.(string)
                valJStr := valJ.(string)
                
                numI, errI := strconv.ParseFloat(valIStr, 64)
                numJ, errJ := strconv.ParseFloat(valJStr, 64)

                if errI == nil && errJ == nil {
                    // Numeric comparison
                    if numI != numJ {
                        less = numI < numJ
                        return (sf.Order == SortAsc && less) || (sf.Order == SortDesc && !less)
                    }
                } else {
                    // String comparison (includes dates)
                    if valIStr != valJStr {
                        less = valIStr < valJStr
                        return (sf.Order == SortAsc && less) || (sf.Order == SortDesc && !less)
                    }
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

func ApplyScoreModifiers(docs []map[string]interface{}, fs *FunctionScore) {
    if fs == nil || len(fs.Functions) == 0 {
        return
    }

    combineMethod := fs.Combine
    if combineMethod == "" {
        combineMethod = "multiply" // Default to multiply
    }

    for _, doc := range docs {
        currentScore, ok := doc["_score"].(float64)
        if !ok {
            currentScore = 0.0 // Ensure a starting score if _score is missing or wrong type
        }
        
        // Calculate the combined function score for this document
        functionScore := 1.0 // Start at 1.0 for multiplication, 0.0 for summing/replacing

        for _, f := range fs.Functions {
            // Execute the template to get the raw function value
            rawScore, err := executeScoreTemplate(doc, f.Code)
            if err != nil {
                log.Printf("Error executing score function: %v", err)
                continue // Skip this function but continue with others
            }
            
            // Apply weight
            weight := 1.0
            if f.Weight != nil {
                weight = *f.Weight
            }
            weightedScore := rawScore * weight

            // Combine the result of this specific function
            if functionScore == 1.0 && combineMethod == "multiply" {
                functionScore = 0.0 // Reset if we started at 1.0 but found a function
            }
            
            // Simplified combination logic:
            switch combineMethod {
            case "multiply":
                if functionScore == 0.0 { // Initial application
                    functionScore = weightedScore
                } else {
                    functionScore *= weightedScore
                }
            case "sum":
                functionScore += weightedScore
            case "replace":
                functionScore = weightedScore
            default:
                functionScore *= weightedScore // Fallback
            }
        }
        
        // Apply the final functionScore to the currentScore
        switch combineMethod {
        case "replace":
            doc["_score"] = functionScore
        case "sum":
            doc["_score"] = currentScore + functionScore
        default: // "multiply" (default)
            doc["_score"] = currentScore * functionScore
        }
    }
}

// ----------------------------------------------------
// Recursive Subquery Execution Logic
// ----------------------------------------------------

// ExecuteSubQuery fetches a list of values (e.g., IDs) by executing a full, nested search.
// This is the core function for recursive subquery execution.
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
        // Build the path using composite keys provided in the subquery
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
        // Fall back to searchRootPath if no composite keys are provided
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
        return nil, nil // Return empty results rather than an error if path is just missing
    }

    // 4. Filter contents using the subQuery's Bool logic and extract the target field
    results := make([]string, 0)
    for _, content := range dirContents {
        if content.GetType() != "file" { continue }

        decodedBytes, _ := base64.StdEncoding.DecodeString(content.GetName())
        itemBody := string(decodedBytes)
        
        var itemData map[string]interface{}
        if json.Unmarshal([]byte(itemBody), &itemData) == nil {
            
            // Execute the subQuery's BOOL evaluation recursively
            match, _ := subQuery.Bool.Evaluate(itemData, ctx, client, baseInput)
            
            if match {
                // Extract the value of the target field (resultField) from the matching document
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

// goclassifieds/lib/search/aggregation.go

// ExecuteAggregation recursively groups and calculates metrics on a list of documents.
func ExecuteAggregation(docs []map[string]interface{}, agg *Aggregation) []Bucket {
    // Check for valid request and determine if any grouping mechanism is active.
    // NOTE: agg.Path is a new grouping mechanism.
    hasGrouping := len(agg.GroupBy) > 0 || len(agg.RangeBuckets) > 0 || agg.DateHistogram != nil || agg.Path != "" 
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

    // --- NEW: 0. Nested Aggregation (Highest Priority) ---
    if agg.Path != "" {
        log.Printf("ExecuteAggregation: Detected Nested Aggregation on path '%s'.", agg.Path)
        return executeNestedAggregation(docs, agg)
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

// goclassifieds/lib/search/aggregation.go (Modified for Nested Logic)

// goclassifieds/lib/search/aggregation.go (with debugging logs)

// executeNestedAggregation processes documents by flattening a nested array field 
// and recursively running the sub-aggregations on the new set of documents.
func executeNestedAggregation(documents []map[string]interface{}, agg *Aggregation) []Bucket {
    if agg.Path == "" || agg.SubAggs == nil || len(agg.SubAggs) == 0 {
        log.Printf("ERROR: Nested aggregation '%s' failed. Missing 'path' or inner 'subAggs' definition.", agg.Name)
        return []Bucket{}
    }

    log.Printf("DEBUG: Starting nested aggregation for path: '%s'. Total parent documents: %d", agg.Path, len(documents))
    
    // 1. Unnest: Collect all inner objects into a single flat list
    unnestedDocuments := make([]map[string]interface{}, 0)

    for docIndex, doc := range documents {
        // Attempt to retrieve the array field defined by agg.Path
        nestedField, exists := doc[agg.Path]
        
        if !exists {
            log.Printf("DEBUG: Document %d is missing the nested path field '%s'. Skipping.", docIndex, agg.Path)
            continue
        }

        // --- ASSERTION 1: Check if the retrieved field is an array ---
        if array, isArray := nestedField.([]interface{}); isArray {
            log.Printf("DEBUG: Document %d - Path '%s' successfully asserted as an array with %d items.", docIndex, agg.Path, len(array))
            
            for itemIndex, item := range array {
                // --- ASSERTION 2: Check if the array item is a map (JSON object) ---
                if innerDoc, isMap := item.(map[string]interface{}); isMap {
                    // Success: Merge and append
                    mergedDoc := make(map[string]interface{})
                    
                    // Pass down essential fields from the parent document
                    for k, v := range doc {
                        if k == "_score" || k == "_id" { 
                            mergedDoc[k] = v
                        }
                    }
                    // Overwrite with the nested fields
                    for k, v := range innerDoc {
                        mergedDoc[k] = v
                    }
                    unnestedDocuments = append(unnestedDocuments, mergedDoc)
                } else {
                    log.Printf("WARN: Document %d, Item %d in array is NOT map[string]interface{}. Actual type: %T. Skipping item.", docIndex, itemIndex, item)
                }
            }
        } else {
            log.Printf("WARN: Document %d - Field '%s' found, but is NOT a slice/array. Actual type: %T. Skipping document.", docIndex, agg.Path, nestedField)
        }
    }

    // Check if any documents were successfully unnested
    if len(unnestedDocuments) == 0 {
        log.Printf("RESULT: Nested aggregation path '%s' returned 0 unnested documents.", agg.Path)
        return []Bucket{}
    }
    
    log.Printf("DEBUG: Unnesting complete. Total unnested documents: %d", len(unnestedDocuments))

    // 2. Recursive Call: Execute the inner aggregation(s) on the flat list
    finalBuckets := make([]Bucket, 0, len(agg.SubAggs))

    // Run all sub-aggregations defined under the nested path.
    for subAggName, innerAgg := range agg.SubAggs {
        log.Printf("DEBUG: Executing inner aggregation '%s' (Type: %v) on the unnested set.", subAggName)
        
        // RECURSIVE CALL: The innerAgg is processed by the main router function.
        innerBuckets := ExecuteAggregation(unnestedDocuments, innerAgg)

        // The result of a nested aggregation is represented as a single bucket 
        // per inner aggregation, containing the results of that inner agg.
        finalBuckets = append(finalBuckets, Bucket{
            Key:      subAggName,
            Count: len(unnestedDocuments), // Total number of unnseted items
            Buckets:  innerBuckets, // The actual aggregation results
        })
    }
    
    log.Printf("DEBUG: Nested aggregation completed successfully for %d sub-aggregations.", len(finalBuckets))
    return finalBuckets
}

func executeScoreTemplate(data map[string]interface{}, code string) (float64, error) {
    // 1. Setup a custom function map for scoring (math functions are crucial)
    funcMap := template.FuncMap{
        "log":    math.Log,
        "sqrt":   math.Sqrt,
        "pow":    math.Pow,
        "now":    time.Now, 
		"toFloat64": toFloat64,
		"levenshtein": LevenshteinDistance, // Custom function for string distance
		"toTime": toTime, // Convert string to time.Time
		// --- NEW ARITHMETIC HELPERS ---
        "div":       div, // For division (a / b)
        "add":       add, // For addition (a + b)
        // "sub":    sub,
        // "mul":    mul,
		// Add more functions as needed
    }

    t, err := template.New("score_func").Funcs(funcMap).Parse(code)
    if err != nil {
        return 0, fmt.Errorf("failed to parse score template: %w", err)
    }

    var buf bytes.Buffer
    if err := t.Execute(&buf, data); err != nil {
        return 0, fmt.Errorf("failed to execute score template: %w", err)
    }

    // 2. Convert string output to float64
    resultStr := strings.TrimSpace(buf.String())
    result, err := strconv.ParseFloat(resultStr, 64)
    if err != nil {
        return 0, fmt.Errorf("template result '%s' is not a number: %w", resultStr, err)
    }

    return result, nil
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

// search/package.go (Helper Functions)

// LevenshteinDistance calculates the Damerau-Levenshtein distance between two strings.
func LevenshteinDistance(s1, s2 string) int {
    r1 := []rune(s1)
    r2 := []rune(s2)
    n := len(r1)
    m := len(r2)

    if n == 0 { return m }
    if m == 0 { return n }

    // Initialize (n+1) x (m+1) matrix
    d := make([][]int, n+1)
    for i := range d {
        d[i] = make([]int, m+1)
        d[i][0] = i
    }
    for j := 1; j <= m; j++ {
        d[0][j] = j
    }

    // Fill the distance matrix
    for i := 1; i <= n; i++ {
        for j := 1; j <= m; j++ {
            cost := 1
            if r1[i-1] == r2[j-1] {
                cost = 0 // Characters match
            }

            // Standard Levenshtein calculation
            d[i][j] = min3(
                d[i-1][j]+1,      // Deletion
                d[i][j-1]+1,      // Insertion
                d[i-1][j-1]+cost, // Substitution
            )

            // Damerau (Adjacent Transposition) calculation
            if i > 1 && j > 1 && r1[i-1] == r2[j-2] && r1[i-2] == r2[j-1] {
                // Transposition cost
                d[i][j] = min(d[i][j], d[i-2][j-2]+1)
            }
        }
    }
    return d[n][m]
}

// Helper for min3 function (you may already have this)
func min(a, b int) int {
    if a < b { return a }
    return b
}
func min3(a, b, c int) int {
    return min(min(a, b), c)
}
// Add these helper functions (you may need to implement similar ones for 'add', 'sub', 'mul')
func div(a, b interface{}) float64 {
    valA := toFloat64(a)
    valB := toFloat64(b)
    if valB == 0 {
        return 0.0 // Avoid division by zero
    }
    return valA / valB
}

func add(a, b interface{}) float64 {
    return toFloat64(a) + toFloat64(b)
}

func toFloat64(i interface{}) float64 {
    switch v := i.(type) {
    case int:
        return float64(v)
    case int64:
        return float64(v)
    case float64:
        return v
    case string:
        // Attempt to parse string to float
        f, err := strconv.ParseFloat(v, 64)
        if err == nil {
            return f
        }
    }
    // Return 0.0 for nil or unsupported/unparseable types
    return 0.0
}

// toTime safely converts an interface value (usually a string) into a time.Time object.
// It prioritizes standard, unambiguous formats like RFC3339.
func toTime(i interface{}) time.Time {
    var s string
    
    // 1. Convert interface{} to string
    switch v := i.(type) {
    case string:
        s = v
    case time.Time:
        return v // Already a time.Time object, return it directly
    default:
        // Handle nil or non-string/non-time types by returning the zero time value
        return time.Time{}
    }

    // 2. Parse the string using common formats
    // RFC3339 is the standard for your data: "2024-09-01T10:00:00Z"
    t, err := time.Parse(time.RFC3339, s)
    if err == nil {
        return t
    }

    // You can add other common formats here if your data is inconsistent.
    // Example: time.Parse("2006-01-02", s) // YYYY-MM-DD

    // 3. Return zero time if parsing failed
    return time.Time{}
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