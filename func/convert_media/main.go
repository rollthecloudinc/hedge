package main

import (
	"context"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	session "github.com/aws/aws-sdk-go/aws/session"
)

func handler(ctx context.Context, s3Event events.S3Event) {

	sess := session.Must(session.NewSession())

	for _, record := range s3Event.Records {

		pieces := strings.Split(record.S3.Object.Key, "/")

	}
}

func main() {
	lambda.Start(handler)
}
