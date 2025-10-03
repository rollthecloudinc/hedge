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
    "github.com/google/go-github/v46/github"
)

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

// Modifiers struct holds the operation for a simple condition.
type Modifiers struct {
    Operation Operation `json:"operation"`
}

type Term struct {
    Field     string     `json:"field"`
    Value     string     `json:"value"`
    Modifiers *Modifiers `json:"modifiers,omitempty"`
}

type Filter struct {
    Field     string     `json:"field"`
    Value     string     `json:"value"`
    Modifiers *Modifiers `json:"modifiers,omitempty"`
}

type Match struct {
    Field     string     `json:"field"`
    Value     string     `json:"value"`
    Modifiers *Modifiers `json:"modifiers,omitempty"`
}

type Case struct {
    Term   *Term   `json:"term,omitempty"`
    Bool   *Bool   `json:"bool,omitempty"`
    Filter *Filter `json:"filter,omitempty"`
    Match  *Match  `json:"match,omitempty"`
}

type Bool struct {
    All  []Case `json:"all,omitempty"`
    None []Case `json:"none,omitempty"`
    One  []Case `json:"one,omitempty"`
    Not  []Case `json:"not,omitempty"`
}

type Query struct {
    Bool  Bool   `json:"bool"`
    Index string `json:"index"`
	Composite map[string]interface{} `json:"composite"`
}

// ----------------------------------------------------
// Condition Interface and Implementations
// ----------------------------------------------------

// Condition is an interface that all simple condition structs must satisfy.
type Condition interface {
    GetField() string
    GetValue() string
    GetModifiers() *Modifiers
}

func (t Term) GetField() string         { return t.Field }
func (t Term) GetValue() string         { return t.Value }
func (t Term) GetModifiers() *Modifiers { return t.Modifiers }

func (f Filter) GetField() string         { return f.Field }
func (f Filter) GetValue() string         { return f.Value }
func (f Filter) GetModifiers() *Modifiers { return f.Modifiers }

func (m Match) GetField() string         { return m.Field }
func (m Match) GetValue() string         { return m.Value }
func (m Match) GetModifiers() *Modifiers { return m.Modifiers }

// ----------------------------------------------------
// Dot Notation Resolver (NEW FUNCTION)
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
            return "", false // Field part not found
        }

        if i == len(parts)-1 {
            // Last part of the path: return the value as a string
            switch v := val.(type) {
            case string:
                return v, true
            case float64:
                // JSON numbers unmarshal as float64; convert to string for comparison
                return strconv.FormatFloat(v, 'f', -1, 64), true
            case int:
                return strconv.Itoa(v), true
            case bool:
                return strconv.FormatBool(v), true
            default:
                // Final value is an object or array, which cannot be compared as a string/number
                return "", false
            }
        } else {
            // Not the last part: continue traversing
            nextMap, ok := val.(map[string]interface{})
            if !ok {
                // Intermediate path element is not a map (i.e., not a nested object)
                return "", false
            }
            current = nextMap
        }
    }
    return "", false // Should be unreachable
}

// ----------------------------------------------------
// Date Parsing Helpers
// ----------------------------------------------------

var dateFormats = []string{
    time.RFC3339,
    "2006-01-02",
    "1/2/2006",
    "01/02/2006",
    "2006-01-02 15:04:05",
}

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
// EvaluateBool (Actual Comparison Logic)
// ----------------------------------------------------

func EvaluateBool(c Condition, targetValue string, op Operation) bool {
    conditionValue := c.GetValue()

    // 1. --- Date/Time Comparison Attempt ---
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

    // 2. --- String and Text Operations ---
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

    // 3. --- Numeric Comparison Operations ---
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
            log.Printf("Warning: Failed to parse values for numeric operation %v. Target: '%s', Condition: '%s'.", op, targetValue, conditionValue)
            return false
        }
    }

    // 4. --- Set Operations (In/NotIn) ---
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

    log.Printf("Error: Operation %v is either unimplemented or invalid for the provided data types.", op)
    return false
}

// ----------------------------------------------------
// Bool.Evaluate (MODIFIED SIGNATURE)
// ----------------------------------------------------

// Evaluate recursively processes the nested Bool structure.
// NOTE: Data type changed to map[string]interface{} to support dot notation.
func (b *Bool) Evaluate(data map[string]interface{}) bool {
    // 1. ALL (AND logic)
    if len(b.All) > 0 {
        for _, c := range b.All {
            if !c.Evaluate(data) {
                return false
            }
        }
        return true
    }

    // 2. ONE (OR logic)
    if len(b.One) > 0 {
        for _, c := range b.One {
            if c.Evaluate(data) {
                return true
            }
        }
        return false
    }

    // 3. NONE (NOT OR logic)
    if len(b.None) > 0 {
        for _, c := range b.None {
            if c.Evaluate(data) {
                return false
            }
        }
        return true
    }

    // 4. NOT (Negation logic - typically on a single case)
    if len(b.Not) > 0 {
        return !b.Not[0].Evaluate(data)
    }

    return true
}

// ----------------------------------------------------
// Case.Evaluate (MODIFIED LOGIC)
// ----------------------------------------------------

// Evaluate processes a single Case. It uses resolveDotNotation to find field values.
func (c *Case) Evaluate(data map[string]interface{}) bool { // CHANGED DATA TYPE
    // A) Handle nested Boolean logic
    if c.Bool != nil {
        return c.Bool.Evaluate(data)
    }

    // B) Handle simple conditions (Term, Filter, Match)
    var condition Condition
    var defaultOp Operation = Equal 

    if c.Term != nil {
        condition = *c.Term
    } else if c.Filter != nil {
        condition = *c.Filter
    } else if c.Match != nil {
        condition = *c.Match
    } else {
        return true
    }
    
    // Extract Operation from Modifiers
    if condition.GetModifiers() != nil {
        defaultOp = condition.GetModifiers().Operation
    }

    // 1. **Resolve the field value using dot notation**
    targetValue, exists := resolveDotNotation(data, condition.GetField()) // USES NEW HELPER
    if !exists {
        // Field not found (or intermediate path failed)
        return false
    }

    // 2. Evaluate the condition
    return EvaluateBool(condition, targetValue, defaultOp)
}

// ----------------------------------------------------
// GetIndexById (GitHub Helper)
// ----------------------------------------------------

type GetIndexConfigurationInput struct {
    GithubClient       *github.Client `json:"-"`
    Stage              string    `json:"stage"`     
    Repo               string    `json:"repo"`
    Branch             string    `json:"branch"`
    Id                 string    `json:"id"`
}

func GetIndexById(c *GetIndexConfigurationInput) (map[string]interface{}, error) {

    var contract map[string]interface{}

    pieces := strings.Split(c.Repo, "/")
    opts := &github.RepositoryContentGetOptions{
        Ref: c.Branch,
    }
    file, _, res, err := c.GithubClient.Repositories.GetContents(context.Background(), pieces[0], pieces[1], "index/" + c.Id + ".json", opts)
    if err != nil || res.StatusCode != 200 {
        log.Print("No index detected for " + c.Id)
        return contract, nil
    }
    if err == nil && file != nil && file.Content != nil {
        content, err := base64.StdEncoding.DecodeString(*file.Content)
        if err == nil {
            json.Unmarshal(content, &contract)
        } else {
            return contract, errors.New("Invalid index unable to parse.")
        }
    }
    return contract, nil
}