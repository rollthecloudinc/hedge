package utils

import (
	"encoding/json" // For working with JSON (e.g., unmarshaling request bodies)
	"fmt"           // For string formatting and creating errors
	"github.com/aws/aws-lambda-go/events" // For APIGatewayProxyRequest & APIGatewayProxyResponse
)

func CopyRequest(req *events.APIGatewayProxyRequest, modifications func(*events.APIGatewayProxyRequest)) *events.APIGatewayProxyRequest {
	// Create a deep copy of the original request
	newReq := &events.APIGatewayProxyRequest{
		Resource:                        req.Resource,
		Path:                            req.Path,
		HTTPMethod:                      req.HTTPMethod,
		Headers:                         copyMap(req.Headers),
		QueryStringParameters:           copyMap(req.QueryStringParameters),
		PathParameters:                  copyMap(req.PathParameters),
		StageVariables:                  copyMap(req.StageVariables),
		RequestContext:                  req.RequestContext,
		Body:                            req.Body,
		IsBase64Encoded:                 req.IsBase64Encoded,
		MultiValueHeaders:               copyMultiMap(req.MultiValueHeaders),
		MultiValueQueryStringParameters: copyMultiMap(req.MultiValueQueryStringParameters),
	}

	// Apply modifications
	if modifications != nil {
		modifications(newReq)
	}

	return newReq
}

// Helper functions to copy maps to maintain immutability
func copyMap(original map[string]string) map[string]string {
	if original == nil {
		return nil
	}
	copied := make(map[string]string)
	for k, v := range original {
		copied[k] = v
	}
	return copied
}

func copyMultiMap(original map[string][]string) map[string][]string {
	if original == nil {
		return nil
	}
	copied := make(map[string][]string)
	for k, v := range original {
		copied[k] = append([]string{}, v...)
	}
	return copied
}

func PluckPropertyFromJSON(body string, propertyName string) (string, error) {
	if body == "" {
		return "", fmt.Errorf("request body is empty")
	}

	// Parse the JSON body into a map
	var data map[string]interface{}
	err := json.Unmarshal([]byte(body), &data)
	if err != nil {
		return "", fmt.Errorf("failed to parse JSON: %v", err)
	}

	// Extract the desired property
	value, ok := data[propertyName]
	if !ok {
		return "", fmt.Errorf("property %s not found in JSON body", propertyName)
	}

	// Ensure the value is a string
	strValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("property %s is not a string", propertyName)
	}

	return strValue, nil
}