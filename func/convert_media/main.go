package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	lambda2 "github.com/aws/aws-sdk-go/service/lambda"
)

var stage string

func handler(ctx context.Context, s3Event events.S3Event) {

	sess := session.Must(session.NewSession())
	lClient := lambda2.New(sess)

	for _, record := range s3Event.Records {

		log.Printf("%+v", record)

		payload := []byte("")

		res, err := lClient.Invoke(&lambda2.InvokeInput{FunctionName: aws.String("goclassifieds-api-" + stage + "-CssToJson"), Payload: payload})
		if err != nil {
			log.Printf("error invoking entity validation: %s", err.Error())
			log.Printf("response: %s", res)
		}

		// pieces := strings.Split(record.S3.Object.Key, "/")

	}
}

func main() {
	stage = os.Getenv("STAGE")
	lambda.Start(handler)
}
