package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"

	"gocv.io/x/gocv"
	"math"
)

// FeatureVector represents the features for a single pixel or superpixel
type FeatureVector struct {
	X, Y              int     // Pixel or region location
	Red, Green, Blue  float64 // RGB color values
	Cyan, Magenta, Yellow float64 // Secondary color intensities
	EdgeStrength      float64 // Edge strength at the pixel
	LocalContrast     float64 // Local contrast around the pixel
	Saliency          float64 // Saliency score for the pixel
	TextureUniformity float64 // Texture descriptor (e.g., LBP or Gabor filter)
	RegionSize        int     // Size of the superpixel region
	RegionAverageRed  float64 // Average Red intensity of the superpixel
	RegionAverageGreen float64 // Average Green intensity of the superpixel
	RegionAverageBlue  float64 // Average Blue intensity of the superpixel
}

// NormalizeFeature normalizes a slice of feature values using Min-Max scaling
func NormalizeFeature(features []float64) []float64 {
	min := math.MaxFloat64
	max := -math.MaxFloat64

	for _, value := range features {
		if value < min {
			min = value
		}
		if value > max {
			max = value
		}
	}

	normalized := make([]float64, len(features))
	for i, value := range features {
		normalized[i] = (value - min) / (max - min)
	}

	return normalized
}

// ExtractFeatures extracts advanced features for each pixel in the image
func ExtractFeatures(img gocv.Mat) []FeatureVector {
	// Convert to grayscale for edge detection and contrast analysis
	gray := gocv.NewMat()
	gocv.CvtColor(img, &gray, gocv.ColorBGRToGray)

	// Advanced Saliency Detection using Spectral Residual
	saliency := gocv.NewMat()
	gocv.SpectralResidualSaliency(gray, &saliency)

	// Edge Detection using Sobel operator
	edges := gocv.NewMat()
	gocv.Sobel(gray, &edges, gocv.MatTypeCV64F, 1, 1, 3, 1.0, 0.0, gocv.BorderDefault)

	// Local contrast (Laplacian)
	contrast := gocv.NewMat()
	gocv.Laplacian(gray, &contrast, gocv.MatTypeCV64F, 3, 1.0, 0.0, gocv.BorderDefault)

	// Texture Analysis using Gabor Filters
	gaborKernel := gocv.GetGaborKernel(5, 5, math.Pi/4, 1.0, 0.5, 0.0, gocv.BorderDefault)
	texture := gocv.NewMat()
	gocv.Filter2D(gray, &texture, gocv.MatTypeCV64F, gaborKernel)

	// Superpixel Segmentation (placeholder using blurring as a proxy)
	region := gocv.NewMat()
	gocv.Blur(img, &region, image.Pt(10, 10)) // Simulating superpixel segmentation

	// Feature extraction for each pixel
	features := []FeatureVector{}
	rows, cols := img.Rows(), img.Cols()

	for y := 0; y < rows; y++ {

		for x := 0; x < cols; x++ {
			// Get RGB values
			bgr := img.GetVecbAt(y, x)
			r, g, b := float64(bgr[2]), float64(bgr[1]), float64(bgr[0])

			// Compute secondary colors
			cyan := (g + b) / 2.0
			magenta := (r + b) / 2.0
			yellow := (r + g) / 2.0

			// Get Edge Strength and Contrast values
			edgeStrength := edges.GetDoubleAt(y, x)
			localContrast := contrast.GetDoubleAt(y, x)

			// Get Saliency value
			saliencyValue := saliency.GetDoubleAt(y, x)

			// Get Texture Uniformity (using Gabor filter response as proxy)
			textureUniformity := texture.GetDoubleAt(y, x)

			// Get Region-Based Features (simulating superpixels)
			regionBGR := region.GetVecbAt(y, x)
			regionAverageRed := float64(regionBGR[2])
			regionAverageGreen := float64(regionBGR[1])
			regionAverageBlue := float64(regionBGR[0])
			regionSize := 1 // Placeholder for actual superpixel size calculation

			// Create feature vector
			features = append(features, FeatureVector{
				X:                 x,
				Y:                 y,
				Red:               r,
				Green:             g,
				Blue:              b,
				Cyan:              cyan,
				Magenta:           magenta,
				Yellow:            yellow,
				EdgeStrength:      edgeStrength,
				LocalContrast:     localContrast,
				Saliency:          saliencyValue,
				TextureUniformity: textureUniformity,
				RegionSize:        regionSize,
				RegionAverageRed:  regionAverageRed,
				RegionAverageGreen: regionAverageGreen,
				RegionAverageBlue: regionAverageBlue,
			})
		}
	}

	// Cleanup
	gray.Close()
	saliency.Close()
	edges.Close()
	contrast.Close()
	texture.Close()
	region.Close()

	return features
}

// NormalizeFeatures applies Min-Max normalization to all feature vectors
func NormalizeFeatures(features []FeatureVector) []FeatureVector {
	// Normalize individual feature arrays
	normalize := func(values []float64) []float64 {
		return NormalizeFeature(values)
	}

	// Extract feature arrays for normalization
	reds := make([]float64, len(features))
	greens := make([]float64, len(features))
	blues := make([]float64, len(features))
	edges := make([]float64, len(features))
	contrasts := make([]float64, len(features))
	saliencies := make([]float64, len(features))
	textures := make([]float64, len(features))

	for i, feature := range features {
		reds[i] = feature.Red
		greens[i] = feature.Green
		blues[i] = feature.Blue
		edges[i] = feature.EdgeStrength
		contrasts[i] = feature.LocalContrast
		saliencies[i] = feature.Saliency
		textures[i] = feature.TextureUniformity
	}

	// Normalize features
	reds = normalize(reds)
	greens = normalize(greens)
	blues = normalize(blues)
	edges = normalize(edges)
	contrasts = normalize(contrasts)
	saliencies = normalize(saliencies)
	textures = normalize(textures)

	// Update feature vectors with normalized values
	for i := range features {
		features[i].Red = reds

		features[i].Green = greens[i]
		features[i].Blue = blues[i]
		features[i].EdgeStrength = edges[i]
		features[i].LocalContrast = contrasts[i]
		features[i].Saliency = saliencies[i]
		features[i].TextureUniformity = textures[i]
	}

	return features
}

// SaveFeaturesToCSV saves the features to a CSV file
func SaveFeaturesToCSV(filename string, features []FeatureVector) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"X", "Y", "Red", "Green", "Blue", "Cyan", "Magenta", "Yellow",
		"EdgeStrength", "LocalContrast", "Saliency", "TextureUniformity",
		"RegionSize", "RegionAverageRed", "RegionAverageGreen", "RegionAverageBlue",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write feature rows
	for _, feature := range features {
		row := []string{
			strconv.Itoa(feature.X),
			strconv.Itoa(feature.Y),
			fmt.Sprintf("%.4f", feature.Red),
			fmt.Sprintf("%.4f", feature.Green),
			fmt.Sprintf("%.4f", feature.Blue),
			fmt.Sprintf("%.4f", feature.Cyan),
			fmt.Sprintf("%.4f", feature.Magenta),
			fmt.Sprintf("%.4f", feature.Yellow),
			fmt.Sprintf("%.4f", feature.EdgeStrength),
			fmt.Sprintf("%.4f", feature.LocalContrast),
			fmt.Sprintf("%.4f", feature.Saliency),
			fmt.Sprintf("%.4f", feature.TextureUniformity),
			strconv.Itoa(feature.RegionSize),
			fmt.Sprintf("%.4f", feature.RegionAverageRed),
			fmt.Sprintf("%.4f", feature.RegionAverageGreen),
			fmt.Sprintf("%.4f", feature.RegionAverageBlue),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	// Input and output file paths
	inputImagePath := "input.jpg" // Change this to the path of your input image
	outputCSVPath := "features.csv"

	// Read the input image
	img := gocv.IMRead(inputImagePath, gocv.IMReadColor)
	if img.Empty() {
		log.Fatalf("Error reading image from %s", inputImagePath)
	}
	defer img.Close()

	// Extract features from the image
	fmt.Println("Extracting features from the image...")
	features := ExtractFeatures(img)

	// Normalize features
	fmt.Println("Normalizing features...")
	normalizedFeatures := NormalizeFeatures(features)

	// Save features to a CSV file
	fmt.Println("Saving features to CSV...")
	if err := SaveFeaturesToCSV(outputCSVPath, normalizedFeatures); err != nil {
		log.Fatalf("Error saving features to CSV: %v", err)
	}

	fmt.Printf("Feature extraction complete! Features saved to %s\n", outputCSVPath)
}