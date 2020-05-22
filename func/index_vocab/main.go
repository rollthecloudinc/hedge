package main

import (
	"context"
	"goclassifieds/lib/vocab"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/session"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
)

func handler(ctx context.Context, s3Event events.S3Event) {

	elasticCfg := elasticsearch7.Config{
		Addresses: []string{
			"https://i12sa6lx3y:v75zs8pgyd@classifieds-4537380016.us-east-1.bonsaisearch.net:443",
		},
	}

	esClient, err := elasticsearch7.NewClient(elasticCfg)
	if err != nil {

	}

	sess := session.Must(session.NewSession())
	vocabManager := vocab.CreateVocabManager(esClient, sess)

	for _, record := range s3Event.Records {
		id := record.S3.Object.Key[13 : len(record.S3.Object.Key)-8]
		vocab := vocabManager.Load(id, "s3")
		vocabManager.Save(vocab, "elastic")
	}
}

func main() {
	// Make the handler available for Remote Procedure Call by AWS Lambda
	lambda.Start(handler)
}
