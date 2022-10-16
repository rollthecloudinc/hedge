package main

import (
	"log"

	"github.com/aws/aws-lambda-go/lambda"
)

func handler() {
	log.Print("renewable_report run")
}

func main() {
	// Make the handler available for Remote Procedure Call by AWS Lambda
	log.Print("renewable_report start")
	lambda.Start(handler)
}
