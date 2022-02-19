package main

import (
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func HandleRequest(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	body := "{\"works\": true}"
	return events.APIGatewayProxyResponse{StatusCode: 200, Body: body, Headers: map[string]string{"Content-Type": "application/json"}}, nil
}

func main() {
	lambda.Start(HandleRequest)
}
