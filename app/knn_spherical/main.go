package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// DistanceMetric defines the type of distance metric
type DistanceMetric int

const (
	Euclidean DistanceMetric = iota
	Manhattan
	Cosine
)

// Cache for KNN queries
type QueryCache struct {
	mu     sync.Mutex
	cache  map[string][]struct {
		Index    int     `json:"index"`
		Distance float64 `json:"distance"`
	}
}

// Initialize a new cache
func NewQueryCache() *QueryCache {
	return &QueryCache{cache: make(map[string][]struct {
		Index    int     `json:"index"`
		Distance float64 `json:"distance"`
	})}
}

// Add a result to the cache
func (qc *QueryCache) Add(query string, result []struct {
	Index    int     `json:"index"`
	Distance float64 `json:"distance"`
}) {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	qc.cache[query] = result
}

// Get a result from the cache
func (qc *QueryCache) Get(query string) ([]struct {
	Index    int     `json:"index"`
	Distance float64 `json:"distance"`
}, bool) {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	result, found := qc.cache[query]
	return result, found
}

// Read CSV data
func readCSV(filename string) ([][]float64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	data := make([][]float64, len(records))
	for i, record := range records {
		data[i] = make([]float64, len(record))
		for j, value := range record {
			data[i][j], err = strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, err
			}
		}
	}
	return data, nil
}

// Write CSV data
func writeCSV(filename string, data [][]float64) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for _, row := range data {
		record := make([]string, len(row))
		for i, value := range row {
			record[i] = strconv.FormatFloat(value, 'f', 6, 64)
		}
		writer.Write(record)
	}
	return nil
}

// Write JSON data
func writeJSON(filename string, data interface{}) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// Normalize the data to range [0, 1]
func normalize(data [][]float64) [][]float64 {
	minVal := make([]float64, len(data[0]))
	maxVal := make([]float64, len(data[0]))

	copy(minVal, data[0])
	copy(maxVal, data[0])

	for _, row := range data {
		for i, value := range row {
			if value < minVal[i] {
				minVal[i] = value
			}
			if value > maxVal[i] {
				maxVal[i] = value
			}
		}
	}

	normalized := make([][]float64, len(data))
	for i, row := range data {
		normalized[i] = make([]float64, len(row))
		for j, value := range row {
			normalized[i][j] = (value - minVal[j]) / (maxVal[j] - minVal[j])
		}
	}

	return normalized
}

// Map points to spherical space
func mapToSphere(data [][]float64) [][]float64 {
	spherical := make([][]float64, len(data))

	for i, row := range data {
		var radius float64
		for _, value := range row {
			radius += value * value
		}
		radius = math.Sqrt(radius)

		spherical[i] = make([]float64, len(row))
		for j, value := range row {
			if radius > 0 {
				spherical[i][j] = value / radius
			} else {
				spherical[i][j] = value
			}
		}
	}

	return spherical
}

// Compute Euclidean distance
func euclideanDistance(p1, p2 []float64) float64 {
	var sum float64
	for i := range p1 {
		sum += math.Pow(p1[i]-p2[i], 2)
	}
	return math.Sqrt(sum)
}

// Compute Manhattan distance
func manhattanDistance(p1, p2 []float64) float64 {
	var sum float64
	for i := range p1 {
		sum += math.Abs(p1[i] - p2[i])
	}
	return sum
}

// Compute Cosine similarity (distance is 1 - similarity)
func cosineDistance(p1, p2 []float64) float64 {
	var dotProduct, magnitudeP1, magnitudeP2 float64
	for i := range p1 {
		dotProduct += p1[i] * p2[i]
		magnitudeP1 += p1[i] * p1[i]
		magnitudeP2 += p2[i] * p2[i]
	}
	if magnitudeP1 == 0 || magnitudeP2 == 0 {
		return 1 // Treat as completely dissimilar
	}
	return 1 - (dotProduct / (math.Sqrt(magnitudeP1) * math.Sqrt(magnitudeP2)))
}

// KNN with user-selected distance metric
func knn(data [][]float64, queryPoint []float64, k int, metric DistanceMetric) ([]int, []float64) {
	type neighbor struct {
		index    int
		distance float64
	}

	neighbors := make([]neighbor, len(data))
	for i, point := range data {
		var distance float64
		switch metric {
		case Euclidean:
			distance = euclideanDistance(queryPoint, point)
		case Manhattan:
			distance = manhattanDistance(queryPoint, point)
		case Cosine:
			distance = cosineDistance(queryPoint, point)
		}
		neighbors[i] = neighbor{
			index:    i,
			distance: distance,
		}
	}

	sort.Slice(neighbors, func(i, j int) bool {
		return neighbors[i].distance < neighbors[j].distance
	})

	indices := make([]int, k)
	distances := make([]float64, k)
	for i := 0; i < k; i++ {
		indices[i] = neighbors[i].index
		distances[i] = neighbors[i].distance
	}

	return indices, distances
}

// Web server function for processing KNN queries with caching and authentication
func startServer(data [][]float64, sphericalData [][]float64, k int, metric DistanceMetric, apiKey string, cache *QueryCache) {
	http.HandleFunc("/knn", func(w http.ResponseWriter, r *http.Request) {
		// Authenticate using API key
		auth := r.URL.Query().Get("apiKey")
		if auth != apiKey {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Parse the query point(s)
		query := r.URL.Query().Get("query")
		if query == "" {
			http.Error(w, "Query point required", http.StatusBadRequest)
			return
		}

		// Check cache
		cachedResult, found := cache.Get(query)
		if found {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(cachedResult)
			return
		}

		components := strings.Split(query, ",")
		queryPoint := make([]float64, len(components))
		for i, component := range components {
			value, err := strconv.ParseFloat(component, 64)
			if err != nil {
				http.Error(w, "Invalid query point", http.StatusBadRequest)
				return
			}
			queryPoint[i] = value
		}

		// Perform KNN analysis
		indices, distances := knn(sphericalData, queryPoint, k, metric)

		// Prepare response
		result := make([]struct {
			Index    int     `json:"index"`
			Distance float64 `json:"distance"`
		}, len(indices))

		for i := range indices {
			result[i].Index = indices[i]
			result[i].Distance = distances[i]
		}

		// Cache result
		cache.Add(query, result)

		// Return JSON response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	fmt.Println("Starting server on http://localhost:8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func main() {
	// Command-line arguments
	inputFilename := flag.String("input", "input.csv", "Input CSV file containing coordinate data")
	outputSphericalFilename := flag.String("outputSpherical", "spherical_data.csv", "Output CSV file for spherical mapped data")
	outputKNNFilename := flag.String("outputKNN", "knn_results.csv", "Output CSV file for KNN results")
	queryPointFlag := flag.String("queryPoint", "", "Query point as comma-separated values (optional, default is the first point in the dataset)")
	k := flag.Int("k", 3, "Number of nearest neighbors to find")
	metricFlag := flag.String("metric", "euclidean", "Distance metric to use: euclidean, manhattan, or cosine")
	server := flag.Bool("server", false, "Start as a web server (default is false)")
	apiKey := flag.String("apiKey", "secret", "API key for web server authentication")
	flag.Parse()

	// Parse distance metric
	var metric DistanceMetric
	switch strings.ToLower(*metricFlag) {
	case "euclidean":
		metric = Euclidean
	case "manhattan":
		metric = Manhattan
	case "cosine":
		metric = Cosine
	default:
		log.Fatalf("Invalid distance metric: %s", *metricFlag)
	}

	// Step 1: Read input data from CSV
	data, err := readCSV(*inputFilename)
	if err != nil {
		log.Fatalf("Error reading CSV: %v", err)
	}

	// Step 2: Normalize the data
	normalizedData := normalize(data)

	// Step 3: Map points to spherical space
	sphericalData := mapToSphere(normalizedData)

	// Step 4: Determine query point
	var queryPoint []float64
	if *queryPointFlag != "" {
		components := strings.Split(*queryPointFlag, ",")
		queryPoint = make([]float64, len(components))
		for i, component := range components {
			queryPoint[i], err = strconv.ParseFloat(component, 64)
			if err != nil {
				log.Fatalf("Error parsing query point: %v", err)
			}
		}
	} else {
		queryPoint = sphericalData[0] // Default to the first point in the dataset
	}

	// Step 5: Perform KNN analysis or start server
	if !*server {
		indices, distances := knn(sphericalData, queryPoint, *k, metric)

		// Step 6: Save results to CSV files
		err = writeCSV(*outputSphericalFilename, sphericalData)
		if err != nil {
			log.Fatalf("Error writing spherical data to CSV: %v", err)
		}

		knnResults := make([][]float64, *k)
		for i := 0; i < *k; i++ {
			knnResults[i] = []float64{float64(indices[i]), distances[i]}
		}
		err = writeCSV(*outputKNNFilename, knnResults)
		if err != nil {
			log.Fatalf("Error writing KNN results to CSV: %v", err)
		}

		// Print results
		fmt.Println("Query Point:", queryPoint)
		fmt.Println("Nearest Neighbors Indices:", indices)
		fmt.Println("Distances to Neighbors:", distances)
		fmt.Println("\nSpherical Data Points:")
		for i, point := range sphericalData {
			fmt.Printf("Point %d: %v\n", i, point)
		}
		fmt.Println("\nKNN Results saved to:", *outputKNNFilename)
		fmt.Println("Spherical Data saved to:", *outputSphericalFilename)
	} else {
		// Start web server with caching and authentication
		cache := NewQueryCache()
		startServer(data, sphericalData, *k, metric, *apiKey, cache)
	}
}