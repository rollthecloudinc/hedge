package search

import (
    "regexp"
    "strings"
    "strconv"
)

// Define a slightly expanded set of common English stopwords.
var englishStopwords = map[string]struct{}{
    "the": {}, "a": {}, "an": {}, "and": {}, "or": {}, "but": {}, "is": {},
    "are": {}, "was": {}, "were": {}, "of": {}, "to": {}, "in": {}, "for": {},
    "on": {}, "with": {}, "as": {}, "at": {}, "by": {}, "it": {}, "its": {},
    "he": {}, "she": {}, "they": {}, "we": {}, "you": {}, "your": {}, "my": {},
    "me": {}, "him": {}, "her": {}, "them": {}, "this": {}, "that": {}, "those": {},
    "then": {}, "not": {}, "do": {}, "will": {}, "can": {}, "could": {}, "would": {},
    "should": {}, "have": {}, "has": {}, "had": {}, "be": {},
}

// --------------------------------------------------------------------------
// 1. ANALYSIS PIPELINE (Used to process both query terms and document content)
// --------------------------------------------------------------------------

// Analyze takes raw text and applies the full processing pipeline:
// Normalization -> Tokenization -> Lowercase -> Stopword Removal -> Stemming -> Cleanup.
func Analyze(text string) []string {
    // 1. Pre-Tokenization Normalization (Possessives, Contractions)
    text = normalizeText(text)
    
    // 2. Tokenization: Split the text into individual words
    tokens := simpleTokenizer(text) 

    // 3. Token Filtering: Apply initial transformations
    tokens = lowercaseFilter(tokens)
    tokens = removeStopwords(tokens)
    
    // 4. Stemming: Reduce words to their root form
    tokens = stemTokens(tokens) 

    // 5. Post-Stemming Cleanup (Short/Numeric tokens)
    tokens = filterShortAndNumericTokens(tokens)

    // --- NEW STEP 6: N-GRAM GENERATION ---
    // Generate bi-grams (2-grams) and tri-grams (3-grams) from the cleaned, stemmed tokens.
    // We use minN=1 to keep the original tokens (unigrams) in the final list.
    tokens = tokenNGramFilter(tokens, 1, 3) 

    return tokens
}

// AnalyzeForPhrase takes raw text and applies all filtering and stemming 
// but DOES NOT generate N-Grams, as the result is intended for sequential
// word-by-word comparison (phrase matching).
func AnalyzeForPhrase(text string) []string {
    // 1. Pre-Tokenization Normalization
    text = normalizeText(text)
    
    // 2. Tokenization: Split the text into individual words
    tokens := simpleTokenizer(text) 

    // 3. Token Filtering: Apply initial transformations
    tokens = lowercaseFilter(tokens)
    tokens = removeStopwords(tokens)
    
    // 4. Stemming: Reduce words to their root form
    tokens = stemTokens(tokens) 

    // 5. Post-Stemming Cleanup
    // We intentionally keep tokens here even if they are short (like "t" from "lt")
    // but the filterShortAndNumericTokens function seems generally safe to keep
    tokens = filterShortAndNumericTokens(tokens)

    // *** IMPORTANT: NO tokenNGramFilter CALL HERE ***

    return tokens
}

// --------------------------------------------------------------------------
// 2. NORMALIZATION, TOKENIZATION AND FILTERS
// --------------------------------------------------------------------------

// normalizeText handles possessives and common punctuation issues before tokenization.
func normalizeText(text string) string {
    // Remove possessive 's (e.g., John's -> John)
    rePossessive := regexp.MustCompile(`'s\b`)
    text = rePossessive.ReplaceAllString(text, "")
    
    // Remove trailing ' (e.g., the cars' -> the cars)
    reApostrophe := regexp.MustCompile(`'\b`)
    text = reApostrophe.ReplaceAllString(text, "")

    // Replace hyphens with spaces to split compound words (e.g., "high-quality" -> "high quality")
    text = strings.ReplaceAll(text, "-", " ")
    
    return text
}

// simpleTokenizer splits text by common non-alphanumeric characters.
func simpleTokenizer(text string) []string {
    // Replace sequences of non-word characters (including punctuation) with a single space.
    re := regexp.MustCompile(`[^\w]+`)
    text = re.ReplaceAllString(text, " ")
    return strings.Fields(text)
}

// lowercaseFilter converts all tokens to lowercase.
func lowercaseFilter(tokens []string) []string {
    for i, token := range tokens {
        tokens[i] = strings.ToLower(token)
    }
    return tokens
}

// removeStopwords removes common, less informative words.
func removeStopwords(tokens []string) []string {
    filtered := make([]string, 0, len(tokens))
    for _, token := range tokens {
        if token != "" {
            if _, found := englishStopwords[token]; !found {
                filtered = append(filtered, token)
            }
        }
    }
    return filtered
}

// filterShortAndNumericTokens removes tokens that are too short or are purely numeric.
func filterShortAndNumericTokens(tokens []string) []string {
    filtered := make([]string, 0, len(tokens))
    for _, token := range tokens {
        // Discard tokens that are one character long (common result of over-stemming)
        if len(token) <= 1 {
            continue
        }
        
        // Discard tokens that are purely numeric (we don't want "123" in our text index)
        if _, err := strconv.ParseFloat(token, 64); err == nil {
            continue
        }
        
        filtered = append(filtered, token)
    }
    return filtered
}


// --------------------------------------------------------------------------
// 3. ADVANCED STEMMING IMPLEMENTATION (Rule-based with V/C helpers)
// --------------------------------------------------------------------------

// stemTokens applies the advanced rule-based stemmer to each token.
func stemTokens(tokens []string) []string {
    stemmed := make([]string, 0, len(tokens))
    for _, token := range tokens {
        stemmed = append(stemmed, advancedRuleStem(token))
    }
    return stemmed
}

// isVowel returns true if the rune is a standard English vowel (a, e, i, o, u) 
// or 'y' when it follows a consonant.
func isVowel(r rune, i int, word []rune) bool {
    switch r {
    case 'a', 'e', 'i', 'o', 'u':
        return true
    case 'y':
        // 'y' is a vowel if it's not the first letter and the preceding letter is a consonant.
        return i > 0 && !isVowel(word[i-1], i-1, word)
    default:
        return false
    }
}

// hasVowel returns true if the word contains at least one vowel in its stem.
func hasVowel(word string) bool {
    r := []rune(word)
    for i, char := range r {
        if isVowel(char, i, r) {
            return true
        }
    }
    return false
}

// advancedRuleStem implements more sophisticated suffix removal rules by checking
// for the presence of a vowel in the stem.
func advancedRuleStem(word string) string {
    r := []rune(word)
    if len(r) < 3 {
        return word
    }

    // Step 1: Plurals and Past Tense/Present Participle
    
    // 1a. Plurals: -s, -ies, -sses
    if strings.HasSuffix(word, "sses") {
        word = strings.TrimSuffix(word, "sses") + "ss"
    } else if strings.HasSuffix(word, "ies") {
        word = strings.TrimSuffix(word, "ies") + "i"
    } else if strings.HasSuffix(word, "s") && !strings.HasSuffix(word, "ss") && len(word) > 3 {
        word = strings.TrimSuffix(word, "s")
    }

    // 1b. -ed and -ing (requires a vowel in the remaining stem)
    stem := word
    if strings.HasSuffix(stem, "eed") {
        // Only remove if the stem (without 'eed') has a vowel
        if hasVowel(strings.TrimSuffix(stem, "eed")) {
            stem = strings.TrimSuffix(stem, "d")
        }
    } else if strings.HasSuffix(stem, "ed") {
        newStem := strings.TrimSuffix(stem, "ed")
        if hasVowel(newStem) {
            stem = newStem
        }
    } else if strings.HasSuffix(stem, "ing") {
        newStem := strings.TrimSuffix(stem, "ing")
        if hasVowel(newStem) {
            stem = newStem
        }
    }

    // After removing -ed or -ing, handle simple double consonants (e.g., hopping -> hop)
    if stem != word {
        word = stem
        // Remove simple double consonants: tt -> t, pp -> p
        if len(word) >= 2 && word[len(word)-1] == word[len(word)-2] && 
           word[len(word)-1] != 'l' && word[len(word)-1] != 's' && word[len(word)-1] != 'z' {
            word = word[:len(word)-1]
        }
    }

    // Step 2: Common Suffixes (e.g., -ational, -tional, -iveness)
    if len(word) > 5 && hasVowel(word[:len(word)-5]) { // Check stem length before removal
        if strings.HasSuffix(word, "ational") {
            word = strings.TrimSuffix(word, "ational") + "ate"
        } else if strings.HasSuffix(word, "tional") {
            word = strings.TrimSuffix(word, "tional") + "tion"
        } else if strings.HasSuffix(word, "alize") {
            word = strings.TrimSuffix(word, "alize") + "al"
        } else if strings.HasSuffix(word, "icate") {
            word = strings.TrimSuffix(word, "icate") + "ic"
        } else if strings.HasSuffix(word, "iciti") {
            word = strings.TrimSuffix(word, "iciti") + "ic"
        } else if strings.HasSuffix(word, "fulness") {
            word = strings.TrimSuffix(word, "fulness")
        }
    }
    
    // Step 3: Removing -al, -ence, -able, -ize, -ant, etc.
    if len(word) > 4 && hasVowel(word[:len(word)-3]) {
        if strings.HasSuffix(word, "al") {
            word = strings.TrimSuffix(word, "al")
        } else if strings.HasSuffix(word, "ance") {
            word = strings.TrimSuffix(word, "ance")
        } else if strings.HasSuffix(word, "ence") {
            word = strings.TrimSuffix(word, "ence")
        } else if strings.HasSuffix(word, "er") {
            word = strings.TrimSuffix(word, "er")
        } else if strings.HasSuffix(word, "ic") {
            word = strings.TrimSuffix(word, "ic")
        } else if strings.HasSuffix(word, "able") {
            word = strings.TrimSuffix(word, "able")
        } else if strings.HasSuffix(word, "ant") {
            word = strings.TrimSuffix(word, "ant")
        } else if strings.HasSuffix(word, "ize") {
            word = strings.TrimSuffix(word, "ize")
        }
    }

    return word
}

// tokenNGramFilter generates N-gram tokens (e.g., bi-grams and tri-grams)
// from a list of base tokens. This is useful for phrase matching and context.
func tokenNGramFilter(tokens []string, minN, maxN int) []string {
    if len(tokens) == 0 {
        return tokens
    }

    // Start with the original tokens (unigrams)
    nGramTokens := make([]string, 0, len(tokens)*maxN)
    if minN <= 1 {
        nGramTokens = append(nGramTokens, tokens...)
    }

    // Generate N-grams from minN up to maxN
    for n := minN; n <= maxN; n++ {
        if n <= 1 {
            continue // Skip unigrams if already added or if minN starts at 1
        }
        
        // Ensure we have enough tokens to form an N-gram
        if len(tokens) < n {
            break 
        }

        // Iterate through the tokens to form the N-grams
        for i := 0; i <= len(tokens)-n; i++ {
            nGram := strings.Join(tokens[i:i+n], " ")
            nGramTokens = append(nGramTokens, nGram)
        }
    }
    
    // We remove duplicates since "high quality" might appear multiple times 
    // in a longer string like "very high quality item" and "high quality new".
    // This simple deduplication is important before indexing.
    return removeDuplicates(nGramTokens)
}

// removeDuplicates is a simple helper function to clean up the token list.
func removeDuplicates(tokens []string) []string {
    keys := make(map[string]bool)
    list := []string{}
    for _, entry := range tokens {
        if _, value := keys[entry]; !value {
            keys[entry] = true
            list = append(list, entry)
        }
    }
    return list
}