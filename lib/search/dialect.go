package search

import (
	"context"
	"fmt"
	"log"
	"math" // REQUIRED for Standard Deviation, Percentile, and Haversine
	"strconv"
	"strings"
	"time"
	"bytes"
	"unicode"
	"text/template"
	"sort" // REQUIRED for Median, Percentile, Min/Max, and general result sorting
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

// GeoPolygon defines a condition where the document's location field 
// must fall within the boundaries of the specified polygon.
type GeoPolygon struct {
    Field  string    `json:"field"`  // The name of the geographic field (e.g., "location")
    Points []Point   `json:"points"` // The vertices of the polygon
}

type GeoMultiPolygon struct {
    Field   string      `json:"field"` // Field containing coordinates
    Polygons [][]Point  `json:"polygons"` // Array of polygons
}

type GeoLine struct {
    Field    string    `json:"field"`   // Field with coordinates in the document
    Line     []Point   `json:"line"`    // Array of points defining the line
    Distance float64   `json:"distance"`// Matching threshold (max distance from line)
    Unit     string    `json:"unit"`    // Units (e.g., km, mi)
}

// Point represents a single coordinate pair.
type Point struct {
    Lat float64 `json:"lat"` // Latitude
    Lon float64 `json:"lon"` // Longitude
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

// MatchPhrase represents a phrase match condition, optionally allowing
// a number of intervening tokens (slop).
type MatchPhrase struct {
    Field string  `json:"field"`
    Value string  `json:"value"`
    Slop  *int    `json:"slop"`   // NEW: Max intervening tokens allowed
    Boost *float64 `json:"boost"`
    Fuzziness *int       `json:"fuzziness,omitempty"` // NEW: Max edit distance for fuzzy matching
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
    MatchPhrase *MatchPhrase `json:"matchPhrase,omitempty"`
	Range       *Range       `json:"range,omitempty"`      // NEW: Range query
	GeoDistance *GeoDistance `json:"geoDistance,omitempty"` // NEW: Geospatial query
    GeoPolygon  *GeoPolygon  `json:"geo_polygon,omitempty"` 
	GeoLine 	*GeoLine 	 `json:"geoLine,omitempty"`
	GeoMultiPolygon *GeoMultiPolygon `json:"geoMultiPolygon,omitempty"`
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
    Path string `json:"path,omitempty"` // Path to the nested array (e.g., "reviews")
	Fields map[string]string `json:"fields"`
        // NEW FIELD: Used only for scripted metrics
    Scripted *ScriptedMetricDefinition `json:"scripted,omitempty"` 
    Script          string            `json:"script,omitempty"`
    BucketsPath     map[string]string `json:"bucketsPath,omitempty"`
    ResultName string `json:"resultName,omitempty"`
}

// NEW STRUCTURE: Defines the actual script and reduction logic
type ScriptedMetricDefinition struct {
    Name        string `json:"name"`
    // The script/template code to run on EACH document in the bucket.
    Script      string `json:"script"` 
    // The final aggregation type to perform on the results of the script:
    // "sum", "avg", "min", "max", or "count"
    ReduceType  string `json:"reduceType"` 
    // Initial value for the reduction accumulator
    InitialValue float64 `json:"initialValue"` 
}

// RangeBucket defines a single numeric bucket for histogram aggregation
type RangeBucket struct {
	Key  string  `json:"key"`  // Name for the bucket
	From float64 `json:"from"` // Start (inclusive)
	To   float64 `json:"to"`   // End (exclusive)
}

// NEW: Pipeline Aggregation Request (for StatsBucket, AvgBucket, etc.)
type PipelineRequest struct {
    Type string `json:"type"`   // Must be "stats_bucket" for this feature
    Path string `json:"path"`   // The path to the metric to be analyzed (e.g., "group_by_state>total_sales")
}

// Aggregation holds metrics and nested grouping logic.
type Aggregation struct {
    Type    string                   `json:"type"` // e.g., "terms"
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
	Aggs map[string]*Aggregation `json:"aggs,omitempty"`

	// Path for the nested aggregation type
	Path string `json:"path,omitempty"`

    // NEW: Pipeline Aggregations that operate on the results of the primary buckets
    PipelineAggs map[string]PipelineRequest `json:"pipelineAggs,omitempty"` 
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

	Aggs      map[string]Aggregation `json:"aggs,omitempty"`
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

    Aggs            map[string]AggregationResult `json:"aggs,omitempty"` // Nested aggs results 
}

// AggregationResult wraps the top-level buckets.
type AggregationResult struct {
	Name    string   `json:"name"`
	Buckets []Bucket `json:"buckets"`
    PipelineMetrics map[string]interface{} // Top-level metrics (e.g., StatsBucket results)
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

// EvaluateGeoLine determines if a document's geo field is within a specified distance from a polyline.
// It takes the item's data, the GeoLine condition, and computes the proximity.
func EvaluateGeoLine(data map[string]interface{}, geoLine *GeoLine) (bool, float64) {
    if geoLine == nil || len(geoLine.Line) < 2 {
        log.Printf("EvaluateGeoLine: Invalid GeoLine definition. Line requires at least 2 points.")
        return false, 0.0
    }

    // 1. Resolve the document's geographic field value
    docLat, docLon, exists := resolvePointFromDoc(data, geoLine.Field)
    if !exists {
        log.Printf("EvaluateGeoLine: Field '%s' not found in document.", geoLine.Field)
        return false, 0.0
    }

    // 2. Calculate the shortest distance from the document location to the polyline segments
    minDistance := math.MaxFloat64
    for i := 0; i < len(geoLine.Line)-1; i++ {
        // Extract start and end points of the current line segment
        start := geoLine.Line[i]
        end := geoLine.Line[i+1]

        // Compute the distance from the document's location to the line segment
        segmentDistance := distanceToLineSegment(docLat, docLon, start.Lat, start.Lon, end.Lat, end.Lon)
        if segmentDistance < minDistance {
            minDistance = segmentDistance
        }
    }

    // Log distance calculation for understanding
    log.Printf("EvaluateGeoLine: Document location (%.4f, %.4f). Closest distance to line: %.2f %s.",
        docLat, docLon, minDistance, geoLine.Unit)

    // 3. Convert distance to the specified unit
    var convertedDistance float64
    searchUnit := strings.ToLower(geoLine.Unit)
    if searchUnit == "mi" || searchUnit == "miles" {
        convertedDistance = minDistance / (EARTH_RADIUS_KM / EARTH_RADIUS_MI)
    } else { // Default to kilometers
        convertedDistance = minDistance
    }

    // 4. Return whether the distance is within the specified threshold and the computed score
    isWithinThreshold := convertedDistance <= geoLine.Distance
    log.Printf("EvaluateGeoLine: Required distance: %.2f %s. Result: %t", geoLine.Distance, geoLine.Unit, isWithinThreshold)
    return isWithinThreshold, 0.0 // Score remains neutral (0.0) for filter-based conditions
}

// EvaluateGeoMultiPolygon determines if a document's geo field value falls within any of the defined polygons.
func EvaluateGeoMultiPolygon(data map[string]interface{}, geoMultiPolygon *GeoMultiPolygon) (bool, float64) {
    // Ensure that the GeoMultiPolygon has valid polygons defined
    if geoMultiPolygon == nil || len(geoMultiPolygon.Polygons) == 0 {
        log.Printf("EvaluateGeoMultiPolygon: Invalid GeoMultiPolygon definition or no polygons provided.")
        return false, 0.0
    }

    // 1. Resolve the document's geographic field value
    docLat, docLon, exists := resolvePointFromDoc(data, geoMultiPolygon.Field)
    if !exists {
        log.Printf("EvaluateGeoMultiPolygon: Field '%s' not found in document.", geoMultiPolygon.Field)
        return false, 0.0
    }

    // 2. Iterate through the polygons and check if the point lies within any of them
    for i, polygon := range geoMultiPolygon.Polygons {
        if len(polygon) < 3 {
            log.Printf("EvaluateGeoMultiPolygon: Skipping invalid polygon at index %d (less than 3 points).", i)
            continue
        }

        if isPointInPolygon(docLat, docLon, polygon) {
            log.Printf("EvaluateGeoMultiPolygon: Match found. Document point (%.4f, %.4f) is inside polygon %d.", docLat, docLon, i)
            return true, 0.0 // Return true if the point is inside any polygon
        }
    }

    // 3. Return false if the point is not in any of the polygons
    log.Printf("EvaluateGeoMultiPolygon: No match found. Document point (%.4f, %.4f) does not fall inside any polygons.", docLat, docLon)
    return false, 0.0
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

// distanceToLineSegment calculates the shortest distance between a point (lat/lon) and a line segment defined by two points.
func distanceToLineSegment(lat, lon, startLat, startLon, endLat, endLon float64) float64 {
    // Convert coordinates to radians
    latRad := degreesToRadians(lat)
    lonRad := degreesToRadians(lon)
    startLatRad := degreesToRadians(startLat)
    startLonRad := degreesToRadians(startLon)
    endLatRad := degreesToRadians(endLat)
    endLonRad := degreesToRadians(endLon)

    // Compute the distance between start and end points of the segment
    segmentLength := Haversine(startLat, startLon, endLat, endLon)

    if segmentLength == 0.0 {
        // Degenerate case: The line segment is a single point
        return Haversine(lat, lon, startLat, startLon)
    }

    // Projection formula: Find where the perpendicular intersects the line segment
    u := ((latRad - startLatRad) * (endLatRad - startLatRad) + (lonRad - startLonRad) * (endLonRad - startLonRad)) /
         math.Pow(segmentLength, 2)

    // Clamp to bounds [0, 1] (projection falls within segment)
    u = math.Max(0, math.Min(1, u))

    // Compute the closest point along the segment
    closestLat := startLatRad + u*(endLatRad - startLatRad)
    closestLon := startLonRad + u*(endLonRad - startLonRad)

    // Return the distance between the original point and the closest point on the line segment
    return Haversine(lat, lon, radiansToDegrees(closestLat), radiansToDegrees(closestLon))
}

// radiansToDegrees converts radians to degrees.
func radiansToDegrees(radians float64) float64 {
    return radians * (180 / math.Pi)
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

// EvaluateMatchPhrase determines if the query tokens appear in the document
// in the same sequential order, allowing for a defined number of intervening tokens (slop).
// It returns the score based on the tightest (minimum slop) match found.
func EvaluateMatchPhrase(data map[string]interface{}, mp *MatchPhrase) (bool, float64) {
    if mp == nil || mp.Field == "" || mp.Value == "" {
        return false, 0.0
    }

    // 1. Setup Slop, Boost, and Fuzziness
    boost := 1.0
    if mp.Boost != nil {
        boost = *mp.Boost
    }
    slop := 0
    if mp.Slop != nil && *mp.Slop > 0 {
        slop = *mp.Slop
    }
    
    fuzzinessLimit := 0
    if mp.Fuzziness != nil && *mp.Fuzziness > 0 {
        fuzzinessLimit = *mp.Fuzziness
    }
    
    // 2. Resolve Tokens
    rawDocValueStr, _ := resolveDotNotation(data, mp.Field) 
    docTokens := AnalyzeForPhrase(rawDocValueStr)
    queryTokens := AnalyzeForPhrase(mp.Value)

    queryLen := len(queryTokens)
    docLen := len(docTokens)

    if queryLen == 0 || docLen == 0 || queryLen > docLen {
        return false, 0.0
    }

    // --- 3. Sequence Matching to Find Minimum Slop and Fuzziness ---
    matchFound := false
    minSlopUsed := slop + 1 
    // NEW: Track the total fuzziness for the sequence that achieved minSlopUsed
    bestFuzziness := 0 
    
    // Outer loop: Iterate through the document to find all possible start positions
    for docStartPos := 0; docStartPos <= docLen - queryLen; docStartPos++ {
        
        // CHECK 1: Fuzzy match for the FIRST query token
        initialDistance := LevenshteinDistance(docTokens[docStartPos], queryTokens[0])
        if initialDistance > fuzzinessLimit {
            continue 
        }
        
        // Initialize sequence-specific tracking variables
        currentSlop := 0
        currentFuzziness := initialDistance // Initialize with the fuzziness of the first token
        currentDocPos := docStartPos
        
        // Inner loop: Check the remaining query tokens
        for queryTokenIndex := 1; queryTokenIndex < queryLen; queryTokenIndex++ {
            
            targetToken := queryTokens[queryTokenIndex]
            foundNextToken := false
            
            // Look ahead for the next matching token
            for nextDocPos := currentDocPos + 1; nextDocPos < docLen; nextDocPos++ {
                
                newSlopNeeded := (nextDocPos - currentDocPos - 1)
                
                // CHECK 2: Fuzzy match for subsequent query tokens
                distance := LevenshteinDistance(docTokens[nextDocPos], targetToken)
                
                if distance <= fuzzinessLimit {
                    // Check if the total slop is still acceptable
                    if currentSlop + newSlopNeeded <= slop {
                        currentSlop += newSlopNeeded
                        currentFuzziness += distance // Accumulate fuzziness
                        currentDocPos = nextDocPos
                        foundNextToken = true
                        break 
                    } else {
                        // Slop exceeded for this path, abandon it
                        break
                    }
                }
            }

            if !foundNextToken {
                // Sequence broken or exceeded slop, abandon this start position
                goto nextStartPos 
            }
        }
        
        // If we reach here, the full phrase was matched successfully.
        matchFound = true
        
        // Update the minimum slop found across all sequences
        if currentSlop < minSlopUsed {
            minSlopUsed = currentSlop
            // Track the total fuzziness associated with this best sequence
            bestFuzziness = currentFuzziness // <-- FIXED: Capture the fuzziness of the best match
        }

    nextStartPos:
        continue
    }
    
    // --- 4. Final Score Calculation based on Minimum Slop and Fuzziness ---
    if matchFound {
        
        // Proximity Factor: Rewards tighter matches (less slop).
        proximityFactor := 1.0
        if slop > 0 {
            proximityFactor = 1.0 - (float64(minSlopUsed) / float64(slop + 1)) 
        }
        
        // Fuzziness Penalty: Penalize the score based on how much slop and fuzziness was used.
        // NOTE: bestFuzziness is now defined and holds the correct accumulated value.
        effectiveTF := float64(queryLen) - float64(minSlopUsed) - float64(bestFuzziness) 
        if effectiveTF < 1.0 { 
            effectiveTF = 1.0 
        }

        // Standard Document Length Normalization
        docLengthFactor := 1.0
        if docLen > 10 {
            docLengthFactor = 1.0 / math.Log(float64(docLen + 1))
        }
        
        // Final Score calculation
        bestScore := (effectiveTF * docLengthFactor) * 2.5 * proximityFactor * boost
        
        log.Printf("EvaluateMatchPhrase: Phrase MATCH (Min Slop: %d, Fuzziness: %d). Best Score=%.4f (Phrase: %s)", minSlopUsed, bestFuzziness, bestScore, mp.Value)
        return true, bestScore
    }
    
    return false, 0.0
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

// evaluateBucketScript executes the pipeline script using the existing template
// execution framework (executeScoreTemplate).
func evaluateBucketScript(script string, paths map[string]string, currentMetrics map[string]interface{}) (float64, error) {
    // 1. Prepare data (metrics) for the template context
    // The context map must contain variables named according to the BucketsPath.
    templateContext := make(map[string]interface{})
    
    for varName, metricName := range paths {
        val, exists := currentMetrics[metricName]
        if !exists {
            // Treat missing prerequisite metrics as 0.0 for calculation safety
            templateContext[varName] = 0.0 
            continue
        }
        
        // Ensure values are transformed to float64, which is required by the
        // math helpers (add, div, mul) in the executeScoreTemplate's FuncMap.
        var fVal float64
        var ok bool
        
        if fVal, ok = val.(float64); !ok {
            if iVal, isInt := val.(int); isInt {
                fVal = float64(iVal)
            } else {
                // For non-numeric or nil, treat as 0.0
                fVal = 0.0 
            }
        }
        
        templateContext[varName] = fVal
    }
    
    // 2. Execute the script using the reusable template engine
    // NOTE: This assumes executeScoreTemplate is accessible here.
    result, err := executeScoreTemplate(templateContext, script)
    
    if err != nil {
        log.Printf("ERROR: Bucket script execution failed for script '%s': %v", script, err)
        // Return 0.0 and the error on failure
        return 0.0, err
    }

    return result, nil
}

func (b *Bool) Evaluate(data map[string]interface{}, ctx context.Context, loader DocumentLoader, indexInput *GetIndexConfigurationInput) (bool, float64) {
	totalScore := 0.0

	// 1. ALL (Logical AND)
	if len(b.All) > 0 {
		matchCount := 0
		
		for _, c := range b.All {
			// Changed: Capture score from Case.Evaluate
			matched, score := c.Evaluate(data, ctx, loader, indexInput)
			
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
			matched, score := c.Evaluate(data, ctx, loader, indexInput)
			
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
			matched, _ := c.Evaluate(data, ctx, loader, indexInput) 
			
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
		matched, _ := b.Not[0].Evaluate(data, ctx, loader, indexInput)
		
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
func (c *Case) Evaluate(data map[string]interface{}, ctx context.Context, loader DocumentLoader, indexInput *GetIndexConfigurationInput) (bool, float64) {
	var match bool
	var score float64

	// A) Handle nested Boolean logic
	if c.Bool != nil {
		// Delegates score accumulation/maxing to Bool.Evaluate
		return c.Bool.Evaluate(data, ctx, loader, indexInput)
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

    // --- NEW: GeoPolygon Check ---
    if c.GeoPolygon != nil {
        poly := c.GeoPolygon
        
        // 1. Resolve the target location from the document
        targetLat, targetLon, found := resolvePointFromDoc(data, poly.Field)
        if !found {
            return false, 0.0 // Document missing location field
        }

        // 2. Perform the geometric check
        isInside := isPointInPolygon(targetLat, targetLon, poly.Points)

        // Geo filters are typically non-scoring (like Term or Range filters)
        if isInside {
            // Returns a match with a non-zero score for inclusion in the result set
            // The score is usually 1.0 or the base scoreMultiplier for a filter.
            return true, 0.0
        } else {
            return false, 0.0
        }
    }

	// Case.Evaluate updates:
	if c.GeoLine != nil {
    	return EvaluateGeoLine(data, c.GeoLine)
	}

	if c.GeoMultiPolygon != nil {
    	return EvaluateGeoMultiPolygon(data, c.GeoMultiPolygon)
	}

	// D) Handle Nested Document logic
	if c.NestedDoc != nil {
		// Delegates score maxing to EvaluateNestedDoc
		// Changed: EvaluateNestedDoc now returns bool and float64 (max score)
		return EvaluateNestedDoc(data, c.NestedDoc, ctx, loader, indexInput) 
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

    // --- NEW: I) Handle MatchPhrase logic (Highest Relevancy, Score > 0.0) ---
    if c.MatchPhrase != nil {
        // Delegates score calculation to EvaluateMatchPhrase
        return EvaluateMatchPhrase(data, c.MatchPhrase) 
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

		subResultData, err := ExecuteSubQuery(ctx, loader, indexInput, subQuery, resultField)
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
func EvaluateNestedDoc(data map[string]interface{}, nested *NestedDoc, ctx context.Context, loader DocumentLoader, indexInput *GetIndexConfigurationInput) (bool, float64) {
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
        matched, score := nested.Bool.Evaluate(itemMap, ctx, loader, indexInput) 
        
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

// calculateCardinality counts the number of unique values for a field, 
// supporting both single values and arrays of values.
func calculateCardinality(docs []map[string]interface{}, field string) int {
	// Use a map to store unique values. struct{} is used as the value for minimal memory usage.
	uniqueValues := make(map[string]struct{})

	log.Printf("CalculateCardinality: Starting calculation for field '%s' on %d documents.", field, len(docs))

	for i, doc := range docs {
		// Use a raw resolver to get the value, which could be any type (string, float, []interface{}, etc.)
		val, exists := resolveRawDotNotation(doc, field)

		if !exists {
			log.Printf("CalculateCardinality DEBUG: Doc %d: Field '%s' not found.", i, field)
			continue
		}

		// Helper closure to process and add a single value to the unique set
		processAndAdd := func(item interface{}) {
			var valStr string
			var isString bool

			// 1. Check for string
			if valStr, isString = item.(string); isString {
				// Use the string directly
			} else if fVal, isFloat := item.(float64); isFloat {
				// 2. Check for float (common for JSON numbers) and convert to string
				valStr = strconv.FormatFloat(fVal, 'f', -1, 64)
				isString = true
			} else if iVal, isInt := item.(int); isInt {
				// 3. Check for integer and convert to string
				valStr = strconv.Itoa(iVal)
				isString = true
			} else {
				// 4. Handle other types (bool, complex objects, etc.) by using fmt.Sprintf
				// NOTE: This can lead to unexpected cardinality for complex objects.
				valStr = fmt.Sprintf("%v", item)
				isString = true 
			}

			if isString {
				uniqueValues[valStr] = struct{}{}
				log.Printf("CalculateCardinality DEBUG: Doc %d: Found value '%s'", i, valStr)
			}
		}

		// --- Check if the resolved value is an array (multi-value field) ---
		if valArray, isArray := val.([]interface{}); isArray {
			// If it's an array, iterate through all elements
			for _, item := range valArray {
				processAndAdd(item)
			}
		} else {
			// --- Handle single value field ---
			processAndAdd(val)
		}
	}

	finalCount := len(uniqueValues)
	log.Printf("CalculateCardinality: Completed. Found %d unique values for field '%s'.", finalCount, field)

	return finalCount
}

// CalculateMetrics iterates through the Aggregation's requested metrics and computes them. (UPDATED)
func CalculateMetrics(groupDocs []map[string]interface{}, metrics []MetricRequest) map[string]interface{} {
	results := make(map[string]interface{})

    // Check if the documents are present
    if len(groupDocs) == 0 {
        return results
    }

	for _, req := range metrics {
        if req.Scripted != nil {

            // --- NEW: Handle Scripted Metric ---
            sm := req.Scripted
            if sm.Name == "" {
                sm.Name = "scriptMetric" // Default name if missing
            }
            
            // Call the new function
            results[sm.Name] = CalculateScriptedMetric(groupDocs, sm) 

        } else {

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
	}
	return results
}

// CalculateScriptedMetric executes a script on every document in the group and then reduces the results.
func CalculateScriptedMetric(docs []map[string]interface{}, sm *ScriptedMetricDefinition) float64 {
    scriptResults := make([]float64, 0, len(docs))

    // Phase 1: Map (Execute Script on Each Document)
    for _, doc := range docs {
        // Reuse the existing template execution function (assumed to be in scope)
        result, err := executeScoreTemplate(doc, sm.Script) 
        if err != nil {
            // Log the error but treat the result for this document as 0.0 or skip
            log.Printf("WARN: Scripted metric '%s' failed for a document: %v", sm.Name, err)
            // For robustness, skip the document's contribution or use 0.0
            continue 
        }
        scriptResults = append(scriptResults, result)
    }
    
    // Phase 2: Reduce (Aggregate the Script Results)
    if len(scriptResults) == 0 {
        return sm.InitialValue // Return the default if no documents processed
    }

    // Accumulator starts with the initial value
    accumulator := sm.InitialValue 
    count := len(scriptResults)

    // Handle reduction logic
    switch strings.ToLower(sm.ReduceType) {
    case "sum":
        for _, val := range scriptResults {
            accumulator += val
        }
        return accumulator
        
    case "avg":
        for _, val := range scriptResults {
            accumulator += val
        }
        // If InitialValue was 0, this is a standard average
        return accumulator / float64(count) 

    case "min":
        // Start min/max with the first element, ignoring initialValue
        if count > 0 {
            minVal := scriptResults[0]
            for _, val := range scriptResults[1:] {
                if val < minVal {
                    minVal = val
                }
            }
            return minVal
        }
        return sm.InitialValue

    case "max":
        // Start min/max with the first element
        if count > 0 {
            maxVal := scriptResults[0]
            for _, val := range scriptResults[1:] {
                if val > maxVal {
                    maxVal = val
                }
            }
            return maxVal
        }
        return sm.InitialValue

    case "count":
        // Count the number of successful script executions (already done implicitly by len(scriptResults))
        return float64(count)
        
    default:
        log.Printf("ERROR: Scripted metric '%s' has unknown reduce type '%s'.", sm.Name, sm.ReduceType)
        return sm.InitialValue
    }
}

// calculateNestedMetric extracts the array at 'nestedPath' from parent documents, 
// flattens the relevant field, and computes the metric across all nested items.
func calculateNestedMetric(docs []map[string]interface{}, metricType, nestedPath, sourceField string) interface{} {
    unnestedValues := make([]float64, 0)

    for _, doc := range docs {
        // Resolve the raw array (slice of interfaces)
        rawVal, exists := resolveRawDotNotation(doc, nestedPath)
        if !exists {
            continue
        }

        if array, isArray := rawVal.([]interface{}); isArray {
            for _, item := range array {
                if innerDoc, isMap := item.(map[string]interface{}); isMap {
                    // Resolve the specific source field (e.g., "rating") within the inner document
                    // NOTE: Assumes resolveDotNotation returns a string
                    valStr, exists := resolveDotNotation(innerDoc, sourceField)
                    if exists {
                        if val, err := strconv.ParseFloat(valStr, 64); err == nil {
                            unnestedValues = append(unnestedValues, val)
                        }
                    }
                }
            }
        }
    }

    if len(unnestedValues) == 0 {
        return 0.0
    }

    // Compute the final metric (e.g., avg)
    switch strings.ToLower(metricType) {
    case "sum":
        var sum float64
        for _, v := range unnestedValues { sum += v }
        return sum
    case "avg", "mean":
        var sum float64
        for _, v := range unnestedValues { sum += v }
        return sum / float64(len(unnestedValues))
    // ... [Include min/max/etc. logic here if needed] ...
    default:
        log.Printf("WARN: Unsupported nested metric type '%s'.", metricType)
        return nil
    }
}

func calculateStatsBucket(buckets []Bucket, path string) map[string]float64 {
    // Path format expected: AGG_NAME>METRIC_NAME 
    parts := strings.Split(path, ">")
    if len(parts) != 2 {
        log.Printf("ERROR: Invalid stats_bucket path format: %s. Must be AGG_NAME>METRIC_NAME", path)
        return nil
    }
    // For StatsBucket, we are analyzing the final metric results from the SIBLING buckets.
    metricName := parts[1] 

    values := make([]float64, 0, len(buckets))

    for _, b := range buckets {
        if val, exists := b.Metrics[metricName]; exists {
            // Ensure the extracted metric is a float64 for calculation
            if fVal, ok := val.(float64); ok {
                values = append(values, fVal)
            } else if iVal, ok := val.(int); ok {
                values = append(values, float64(iVal))
            }
        }
    }

    if len(values) == 0 {
        return nil
    }

    // --- Calculate the required statistics (min, max, sum, avg, count, std_dev) ---
    count := float64(len(values))
    sum := 0.0
    min := values[0]
    max := values[0]
    
    for _, v := range values {
        sum += v
        if v < min { min = v }
        if v > max { max = v }
    }
    
    avg := sum / count
    
    // Calculate Standard Deviation (StdDev)
    varianceSum := 0.0
    for _, v := range values {
        varianceSum += math.Pow(v - avg, 2)
    }
    stdDev := math.Sqrt(varianceSum / count) // Use N (population) for sample size

    return map[string]float64{
        "count": count,
        "min": min,
        "max": max,
        "sum": sum,
        "avg": avg,
        "std_dev": stdDev,
    }
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
func ApplySort(docs []map[string]interface{}, sortFields []SortField) []map[string]interface{} {
    if len(sortFields) == 0 || len(docs) < 2 {
        return docs
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
    return docs
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
// This function is fully decoupled from the *github.Client by using the DocumentLoader interface
// and includes the fix for the scope/type conflict error.
func ExecuteSubQuery(
    ctx context.Context, 
    loader DocumentLoader, // The abstract mechanism to fetch data
    baseInput *GetIndexConfigurationInput, 
    subQuery *Query, 
    resultField string,
) ([]string, error) {
    
    log.Printf("ExecuteSubQuery: Starting recursive search for index '%s' and composite keys: %+v", subQuery.Index, subQuery.Composite)
    
    subInput := *baseInput 
    subInput.Id = subQuery.Index

    // 1. Load documents using the generic DocumentLoader
    iterator, err := loader.Load(
        ctx, 
        &subInput,            // Pass the subquery index config
        subQuery.Composite,   // Pass the composite keys for scoped search
    )
    if err != nil {
        return nil, fmt.Errorf("failed to load data for subquery index '%s': %w", subQuery.Index, err)
    }
    defer iterator.Close()

    // 2. Filter contents and extract the target field
    results := make([]string, 0)
    
    // START OF FIX: Explicitly declare the variables to receive the result from resolveDotNotation.
    // This forces 'extractedVal' to be of type interface{} and prevents shadowing conflicts.
    var extractedVal interface{}
    var exists bool
    
    for itemData, ok := iterator.Next(); ok; itemData, ok = iterator.Next() {
        
        if iterator.Error() != nil {
            log.Printf("ExecuteSubQuery: Non-fatal error during iteration: %v", iterator.Error())
            continue
        }
        
        // Execute the subQuery's BOOL evaluation recursively
        // Pass the loader down for potential deeper subqueries.
        match, _ := subQuery.Bool.Evaluate(itemData, ctx, loader, &subInput)
        
        if match {
            // Use the assignment operator (=) to assign the result to the pre-declared interface{} variables.
            extractedVal, exists = resolveDotNotation(itemData, resultField)

            if exists {
                // Now safely assert the interface{} value extractedVal to a string
                if valStr, isStr := extractedVal.(string); isStr { 
                    results = append(results, valStr)
                } 
            }
        }
    }
    
    // Check for a fatal error that may have occurred at the end of iteration
    if err := iterator.Error(); err != nil {
        return nil, fmt.Errorf("subquery iteration finished with fatal error: %w", err)
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
// ExecuteAggregation recursively groups and calculates metrics on a list of documents.
// It is the entry point for all aggregation requests.
func ExecuteAggregation(docs []map[string]interface{}, agg *Aggregation) AggregationResult {
    
    // Default empty result for error/no-op paths
    emptyResult := AggregationResult{Buckets: []Bucket{}, PipelineMetrics: make(map[string]interface{})}

    // Determine if we need to group the documents (primary grouping/bucketization)
    hasGrouping := len(agg.GroupBy) > 0 || len(agg.RangeBuckets) > 0 || agg.DateHistogram != nil || agg.Path != "" 
    hasMetrics := agg != nil && (len(agg.Metrics) > 0 || len(agg.Aggs) > 0 || len(agg.PipelineAggs) > 0)

    // --- Placeholder for the generated buckets list ---
    var buckets []Bucket
    
    // ----------------------------------------------------------------------
    // --- SPECIAL CASE: Only Metrics/Aggs Requested (No Primary Grouping) ---
    // ----------------------------------------------------------------------
    if !hasGrouping {
        if hasMetrics {
            log.Print("ExecuteAggregation: No primary grouping defined. Calculating top-level metrics/aggregations.")
            // Create a single, anonymous bucket for the entire document set and process it.
            topBucket := processGroup(agg, "", docs) 
            buckets = []Bucket{topBucket} // Store in the buckets slice
        } else {
            // Final exit if neither grouping nor metrics/aggs are defined.
            log.Print("ExecuteAggregation: No grouping or metrics/aggs defined. Returning empty results.")
            return emptyResult
        }
    } else {
    
        // ----------------------------------------------------------------------
        // --- STEP 1: Execute Primary Grouping (Populates 'buckets' slice) ---
        // ----------------------------------------------------------------------
        
        if agg.Path != "" {
            log.Printf("ExecuteAggregation: Detected Nested Aggregation on path '%s'.", agg.Path)
            // Nested aggregation handles both unnesting and recursive processing via its inner Aggs map.
            buckets = executeNestedAggregation(docs, agg)

        } else if len(agg.GroupBy) > 0 {
            groupByField := agg.GroupBy[0] 
            log.Printf("ExecuteAggregation: Grouping by field '%s'", groupByField)

            groups := make(map[string][]map[string]interface{})
            for _, doc := range docs {
                key, exists := resolveDotNotation(doc, groupByField)
                if exists {
                    groups[key] = append(groups[key], doc)
                }
            }
            
            buckets = make([]Bucket, 0, len(groups))
            for key, groupDocs := range groups {
                // Process the group to calculate metrics and run sub-aggs
                newBucket := processGroup(agg, key, groupDocs)
                buckets = append(buckets, newBucket)
            }
            
        } else if len(agg.RangeBuckets) > 0 {
            // RangeBuckets logic (Field, ranges)
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
                // Find the correct range for the document
                valStr, exists := resolveDotNotation(doc, field)
                if !exists { continue }
                val, err := strconv.ParseFloat(valStr, 64)
                if err != nil { continue }
                
                for _, r := range ranges {
                    // Range is inclusive of From, exclusive of To (if To is set)
                    if val >= r.From && (r.To == 0.0 || val < r.To) { 
                        rangeGroups[r.Key] = append(rangeGroups[r.Key], doc)
                        break 
                    }
                }
            }
            
            // Generate and process a bucket for every *defined* range
            buckets = make([]Bucket, 0, len(ranges))
            for _, r := range ranges { 
                groupDocs := rangeGroups[r.Key]
                newBucket := processGroup(agg, r.Key, groupDocs)
                buckets = append(buckets, newBucket)
            }

        } else if agg.DateHistogram != nil {
            dh := agg.DateHistogram
            log.Printf("ExecuteAggregation: Grouping by Date Histogram on field '%s' with interval '%s'", dh.Field, dh.Interval)

            groups := make(map[string][]map[string]interface{})
            var format string
            switch strings.ToLower(dh.Interval) {
                case "minute": format = "2006-01-02T15:04" 
                case "hour": format = "2006-01-02T15"    
                case "day": format = "2006-01-02"       
                case "month": format = "2006-01"          
                case "year": format = "2006"             
                default:
                    log.Printf("ExecuteAggregation: Invalid date histogram interval '%s'.", dh.Interval)
                    return emptyResult
            }

            for _, doc := range docs {
                valStr, exists := resolveDotNotation(doc, dh.Field)
                if !exists { continue }
                t, err := tryParseDate(valStr) // Assume tryParseDate exists
                if err != nil { continue }
                key := t.Format(format)
                groups[key] = append(groups[key], doc)
            }
            
            // Convert map to buckets and process each group
            buckets = make([]Bucket, 0, len(groups))
            for key, groupDocs := range groups {
                newBucket := processGroup(agg, key, groupDocs)
                buckets = append(buckets, newBucket)
            }
            
            // Sort buckets chronologically by key
            sort.Slice(buckets, func(i, j int) bool {
                return buckets[i].Key < buckets[j].Key
            })
        }
    }

    // ----------------------------------------------------------------------
    // --- STEP 2: Execute Pipeline Aggregations (e.g., StatsBucket) ---
    // This runs AFTER all primary buckets and their intra-bucket metrics (Pass 1 & 2) are complete.
    // ----------------------------------------------------------------------
    pipelineMetrics := make(map[string]interface{})
    
    if len(agg.PipelineAggs) > 0 {
        pipelineMetrics = executePipelineAggregations(buckets, agg.PipelineAggs)
    }

    // ----------------------------------------------------------------------
    // --- STEP 3: Return Final Result ---
    // ----------------------------------------------------------------------
    return AggregationResult{
        Buckets: buckets,
        PipelineMetrics: pipelineMetrics,
    }
}

// goclassifieds/lib/search/aggregation.go (Modified for Nested Logic)

// goclassifieds/lib/search/aggregation.go (with debugging logs)

// executeNestedAggregation processes documents by flattening a nested array field 
// and recursively running the sub-aggregations on the new set of documents.
// executeNestedAggregation processes documents by flattening a nested array field 
// and recursively running the sub-aggregations on the new set of documents.
func executeNestedAggregation(documents []map[string]interface{}, agg *Aggregation) []Bucket {
    // Corrected check: look for 'Aggs' instead of 'SubAggs'
    if agg.Path == "" || agg.Aggs == nil || len(agg.Aggs) == 0 { 
        log.Printf("ERROR: Nested aggregation '%s' failed. Missing 'path' or inner 'Aggs' definition.", agg.Name)
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
    finalBuckets := make([]Bucket, 0, len(agg.Aggs)) // Use length of Aggs map

    // Run all sub-aggregations defined under the nested path.
    for subAggName, innerAgg := range agg.Aggs { // Iterate over the Aggs map
        log.Printf("DEBUG: Executing inner aggregation '%s' (Type: %v) on the unnested set.", subAggName)
        
        // ExecuteAggregation now returns AggregationResult, so we must access the .Buckets field.
        innerAggResult := ExecuteAggregation(unnestedDocuments, innerAgg)

        // The result of a nested aggregation is represented as a single bucket 
        // per inner aggregation, containing the results of that inner agg.
        finalBuckets = append(finalBuckets, Bucket{
            Key:      subAggName,
            Count: len(unnestedDocuments), // Total number of unnseted items
            // CORRECTED: Assign ONLY the buckets slice from the result.
            Buckets:  innerAggResult.Buckets, // The actual aggregation results
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
        "mul":    mul,
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

// executePipelineAggregations runs pipeline aggregations on a slice of completed buckets.
// It is called AFTER all primary buckets are grouped and have their simple metrics calculated.
func executePipelineAggregations(buckets []Bucket, pipelineAggs map[string]PipelineRequest) map[string]interface{} {
    pipelineResults := make(map[string]interface{})

    for aggName, req := range pipelineAggs {
        if strings.ToLower(req.Type) == "stats_bucket" {
            // StatsBucket logic
            if result := calculateStatsBucket(buckets, req.Path); result != nil {
                pipelineResults[aggName] = result
            }
        }
        // Future: Add other pipeline types here (e.g., "avg_bucket", "sum_bucket")
    }

    return pipelineResults
}

// Inside lib/search/search.go

// ExecuteTopLevelPipeline runs pipeline aggregations (like stats_bucket) against a set of
// sibling buckets generated by a primary aggregation.
// targetBuckets: The list of Buckets from the primary aggregation (e.g., all "group_by_category" buckets).
// pipelineAgg:   The PipelineAggregation request (e.g., the stats_bucket definition).
// pathFromTarget: The remaining parts of the path, e.g., ["views_per_dollar_ratio"] or ["sub_agg_name", "metric_name"].
func ExecuteTopLevelPipeline(
    targetBuckets []Bucket,
    pipelineAgg Aggregation,
    pathFromTarget []string,
) interface{} {
    
    pipelineType := strings.ToLower(pipelineAgg.Type)

    // Ensure the path is valid for a StatsBucket, which requires a target metric name.
    if len(pathFromTarget) < 1 {
        log.Printf("ERROR: Pipeline aggregation '%s' requires a metric path.", pipelineAgg.Name)
        return nil
    }

    switch pipelineType {
    case "stats_bucket":
        // 1. Collect all numerical values for the target metric across all sibling buckets.
        values := make([]float64, 0)
        
        // The last element in the path is the target metric name.
        metricName := pathFromTarget[len(pathFromTarget)-1]
        
        // The preceding elements define the path through nested sub-aggregations.
        subAggPath := pathFromTarget[:len(pathFromTarget)-1] 

        for _, bucket := range targetBuckets {
            // Find the numerical value by traversing the path within the bucket
            val, found := findMetricValueInBucket(bucket, subAggPath, metricName)

            if found {
                if fVal, ok := val.(float64); ok {
                    values = append(values, fVal)
                } else if iVal, ok := val.(int); ok {
                    values = append(values, float64(iVal))
                }
            }
        }
        
        if len(values) == 0 {
            return map[string]interface{}{
                "count": 0,
                "avg":   0.0,
                "min":   nil,
                "max":   nil,
                "sum":   0.0,
            }
        }

        // 2. Calculate statistics on the collected values.
        count := len(values)
        min := values[0]
        max := values[0]
        sum := 0.0

        for _, v := range values {
            sum += v
            if v < min {
                min = v
            }
            if v > max {
                max = v
            }
        }
        avg := sum / float64(count)

        // NOTE: Standard deviation calculation is complex and often skipped for simpler engines.
        // For now, we omit std_dev but include the standard stats.
        
        return map[string]interface{}{
            "count": count,
            "min":   min,
            "max":   max,
            "avg":   avg,
            "sum":   sum,
        }
    
    // Add other pipeline aggregations here (e.g., "avg_bucket", "max_bucket")
    
    default:
        log.Printf("ERROR: Unsupported top-level pipeline aggregation type: %s", pipelineType)
        return nil
    }
}

// Helper function to recursively traverse a bucket to find a specific metric value.
func findMetricValueInBucket(b Bucket, aggPath []string, metricName string) (interface{}, bool) {
    
    currentBucket := b
    
    // 1. Traverse through sub-aggregations (if path is deep)
    for _, pathName := range aggPath {
        found := false
        // Search through the sub-buckets for a key matching the pathName
        for _, subBucket := range currentBucket.Buckets {
            if subBucket.Key == pathName { 
                currentBucket = subBucket
                found = true
                break
            }
            // If the sub-aggregation isn't a terms agg, the key might be the sub-agg name itself
            // NOTE: This assumes subAggPath is only used for traversing terms aggregations.
        }
        if !found {
            // If we can't find the required nested sub-aggregation bucket, we can't continue.
            return nil, false
        }
    }
    
    // 2. Look for the metric name in the final bucket's Metrics map
    if currentBucket.Metrics != nil {
        if val, found := currentBucket.Metrics[metricName]; found {
            return val, true
        }
    }
    
    return nil, false
}

// processGroup handles metric calculation and recursion for a single group of documents.
// This is where the FUSED LOGIC is implemented.
// processGroup handles metric calculation and recursion for a single group of documents.
// processGroup is the worker function that takes a subset of documents (a group) 
// and calculates all requested metrics and sub-aggregations on them.
// processGroup is the worker function that takes a subset of documents (a group) 
// and calculates all requested metrics and sub-aggregations on them.
func processGroup(agg *Aggregation, key string, groupDocs []map[string]interface{}) Bucket {
    
    bucket := Bucket{
        // FIX 1: The correct variable name is 'bucket'
        Key:   normalizeBucketKey(key),
        Count: len(groupDocs),
        Metrics: make(map[string]interface{}),
        Aggs: make(map[string]AggregationResult), 
    }

    if bucket.Count == 0 {
        return bucket
    }
    
    log.Printf("processGroup: Processing bucket '%v' with %d documents.", bucket.Key, bucket.Count)

    // --- Metric Filtering (Separation of Concerns for Two-Pass System) ---
    simpleMetrics := make([]MetricRequest, 0)
    pipelineMetrics := make([]MetricRequest, 0)
    
    for _, req := range agg.Metrics {
        if strings.ToLower(req.Type) == "bucket_script" {
            pipelineMetrics = append(pipelineMetrics, req)
        } else {
            simpleMetrics = append(simpleMetrics, req)
        }
    }

    // --------------------------------------------------------
    // --- PASS 1: Calculate Simple and Nested Metrics ---
    // --------------------------------------------------------
    
    // Separate simpleMetrics into NESTED and STANDARD requests
    standardMetricsToQueue := make([]MetricRequest, 0)

    for _, req := range simpleMetrics {
        if req.Path != "" { // <-- 1. PROCESS NESTED METRICS ROLLUP
            calcType := strings.ToLower(req.Type)
            
            // req.Fields is a map of {sourceField: resultName}
            for sourceField, resultName := range req.Fields {
                if resultName == "" {
                    continue
                }
                
                // CALLING CUSTOM NESTED ROLLUP FUNCTION
                calculatedValue := calculateNestedMetric(groupDocs, calcType, req.Path, sourceField)
                if calculatedValue != nil {
                    // Place the nested metric result directly into the parent bucket's Metrics map
                    bucket.Metrics[resultName] = calculatedValue // FIX 2: Corrected to 'bucket.Metrics'
                }
            }
        } else {
            // 2. Queue non-nested metrics for the generic CalculateMetrics function
            standardMetricsToQueue = append(standardMetricsToQueue, req)
        }
    }
    
    // 3. Calculate STANDARD Metrics
    if len(standardMetricsToQueue) > 0 {
        // CALLING GENERIC STANDARD METRICS FUNCTION
        standardResults := CalculateMetrics(groupDocs, standardMetricsToQueue) 
        
        // Merge results into the bucket's Metrics map (already contains nested rollups)
        for k, v := range standardResults {
            bucket.Metrics[k] = v 
        }
    }
    
    // At this point, bucket.Metrics contains all document-level metrics (nested rollups + standard).
    
    // --------------------------------------------------------
    // --- PASS 2: Calculate Pipeline Metrics (Bucket Script) ---
    // --------------------------------------------------------
    if len(pipelineMetrics) > 0 {
        for _, req := range pipelineMetrics {
            if req.Script == "" || req.BucketsPath == nil || req.ResultName == "" {
                log.Printf("ERROR: Bucket script request is incomplete (missing script, path, or resultName).")
                continue 
            }
            
            // Bucket Script can now access both Nested and Standard metrics in bucket.Metrics
            calculatedValue, err := evaluateBucketScript(req.Script, req.BucketsPath, bucket.Metrics)
            
            if err == nil {
                bucket.Metrics[req.ResultName] = calculatedValue 
            } else {
                log.Printf("ERROR: Bucket script failed for '%s': %v", req.ResultName, err)
                bucket.Metrics[req.ResultName] = 0.0
            }
        }
    }

    // --------------------------------------------------------
    // --- Top Hits (Restored Functionality) ---
    // --------------------------------------------------------
    if agg.TopHits != nil && agg.TopHits.Size > 0 {
        hitsDocs := groupDocs 
        
        if len(agg.TopHits.Sort) > 0 {
            hitsDocs = ApplySort(hitsDocs, agg.TopHits.Sort) 
        }
        
        from := 0
        hitsDocs = ApplyPaging(hitsDocs, agg.TopHits.Size, from) 
        
        if len(agg.TopHits.Source) > 0 {
            hitsDocs = ProjectFields(hitsDocs, agg.TopHits.Source)
        }
        
        bucket.TopHits = hitsDocs
    }

    // --------------------------------------------------------
    // --- Sub-Aggregations (Recursive Grouping using Aggs map) ---
    // --------------------------------------------------------
    if len(agg.Aggs) > 0 {
        log.Printf("processGroup: Executing %d recursive sub-aggregations.", len(agg.Aggs))

        // Loop through all named sub-aggregations
        for subAggName, subAgg := range agg.Aggs {
            // Recursively call ExecuteAggregation on the documents in this group/bucket.
            subResult := ExecuteAggregation(groupDocs, subAgg)
            
            // Store the result in the bucket's Aggs map using the aggregation name as the key
            bucket.Aggs[subAggName] = subResult
        }
    } 
    
    return bucket
}

// In goclassifieds/lib/search/geo.go (New File or within search/search.go helpers)

// isPointInPolygon checks if a target point is inside the given polygon using the 
// Ray Casting Algorithm. The polygon points must be ordered (clockwise or counter-clockwise).
// It returns true if the point is strictly inside, and false otherwise.
func isPointInPolygon(targetLat, targetLon float64, polygon []Point) bool {
    n := len(polygon)
    if n < 3 {
        return false // Not a valid polygon
    }

    inside := false
    
    // Iterate over each edge of the polygon
    for i, j := 0, n-1; i < n; j, i = i, i+1 {
        // p1 and p2 are the endpoints of the current edge
        p1 := polygon[i]
        p2 := polygon[j]

        // 1. Check if the ray from targetLat crosses the edge vertically
        // The condition (p1.Lat > targetLat) != (p2.Lat > targetLat) checks if the
        // edge crosses the horizontal line y = targetLat.
        if (p1.Lat > targetLat) != (p2.Lat > targetLat) {
            
            // 2. Calculate the intersection point's longitude (x-coordinate)
            // lon_intersection = (targetLat - p1.Lat) * (p2.Lon - p1.Lon) / (p2.Lat - p1.Lat) + p1.Lon
            // This is the x-coordinate (longitude) of the intersection point.
            intersectLon := (p2.Lon-p1.Lon)*(targetLat-p1.Lat)/(p2.Lat-p1.Lat) + p1.Lon

            // 3. If the ray crosses the edge to the right of the target point, 
            // the inside status toggles.
            if targetLon < intersectLon {
                inside = !inside
            }
        }
    }

    return inside
}

// In goclassifieds/lib/search/geo.go (Corrected resolvePointFromDoc)

// resolvePointFromDoc attempts to extract a Point{Lat, Lon} from a document field.
func resolvePointFromDoc(doc map[string]interface{}, field string) (float64, float64, bool) {
    // FIX: Use resolveRawDotNotation to get the raw map or slice value.
    rawVal, exists := resolveRawDotNotation(doc, field) 
    if !exists {
        return 0, 0, false
    }

    // Case 1: Value is a map {"lat": 40.0, "lon": -70.0}
    if pointMap, ok := rawVal.(map[string]interface{}); ok { 
        lat := toFloat64(pointMap["lat"])
        lon := toFloat64(pointMap["lon"])
        return lat, lon, true
    }
    
    // Case 2: Value is a slice/array [lon, lat] (common GeoJSON format)
    if pointArr, ok := rawVal.([]interface{}); ok && len(pointArr) >= 2 {
        lon := toFloat64(pointArr[0])
        lat := toFloat64(pointArr[1])
        return lat, lon, true
    }

    // The field was found but was not a recognizable map or array format for geo points.
    return 0, 0, false
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

// Add these helper functions (you may need to implement similar ones for 'add', 'sub', 'mul')
func mul(a, b interface{}) float64 {
    return toFloat64(a) * toFloat64(b)
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

// normalizeBucketKey is a helper function to ensure bucket keys are consistent.
// It is called by processGroup to sanitize or standardize the group key.
func normalizeBucketKey(key string) string {
    // Basic implementation: trim whitespace.
    // Can be extended to handle character escaping or specific format rules if needed.
    return strings.TrimSpace(key)
}