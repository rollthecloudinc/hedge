package search

// IndexStats holds pre-calculated global statistics for IDF scoring.
type IndexStats struct {
    TotalDocuments uint64 // N: Total number of documents in the index.
    
    // DocumentFrequency[token] stores the number of documents (df) 
    // that contain the given analyzed token.
    DocumentFrequency map[string]uint64
}

// GetDocumentFrequency returns the document count for a token, or 1 if unknown.
func (i *IndexStats) GetDocumentFrequency(token string) uint64 {
    if df, ok := i.DocumentFrequency[token]; ok && df > 0 {
        return df
    }
    // Default to 1 (or a small smoothing value) to prevent division by zero/log(1).
    return 1 
}

// CalculateIDF returns the Inverse Document Frequency score for a given token.
// Formula: log(1 + (N - df + 0.5) / (df + 0.5)) (a common variation for better stability)
func (i *IndexStats) CalculateIDF(token string) float64 {
    N := float64(i.TotalDocuments)
    df := float64(i.GetDocumentFrequency(token))
    
    return math.Log(1 + (N - df + 0.5) / (df + 0.5))
}