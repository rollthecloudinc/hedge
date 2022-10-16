package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
)

func handler(ctx context.Context, b json.RawMessage) {
	log.Print("renewable_report run")
}

func main() {
	// Make the handler available for Remote Procedure Call by AWS Lambda
	log.Print("renewable_report start")
	lambda.Start(handler)
}
