package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	lambda2 "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

var stage string

type CssToJsonRequest struct {
	Content string
}

func handler(ctx context.Context, s3Event events.S3Event) {

	sess := session.Must(session.NewSession())
	lClient := lambda2.New(sess)

	for _, record := range s3Event.Records {

		log.Printf("%+v", record)

		content, _ := download(sess, &record)
		payload, _ := request(content)

		log.Printf("%s", payload)

		res, err := lClient.Invoke(&lambda2.InvokeInput{FunctionName: aws.String("goclassifieds-api-" + stage + "-CssToJson"), Payload: payload})
		if err != nil {
			log.Printf("error invoking entity validation: %s", err.Error())
			log.Printf("response: %s", res)
		}

	}
}

func request(content []byte) ([]byte, error) {

	request := CssToJsonRequest{
		Content: string(content),
	}

	payload, err := json.Marshal(request)
	if err != nil {
		log.Printf("Error marshalling css to json request: %s", err.Error())
		return nil, errors.New("Error marshalling css to json request")
	}

	return payload, nil
}

func download(sess *session.Session, record *events.S3EventRecord) ([]byte, error) {

	buf := aws.NewWriteAtBuffer([]byte{})

	downloader := s3manager.NewDownloader(sess)

	_, err := downloader.Download(buf, &s3.GetObjectInput{
		Bucket: aws.String(record.S3.Bucket.Name),
		Key:    aws.String(record.S3.Object.Key),
	})

	if err != nil {
		return []byte(""), err
	}

	return buf.Bytes(), nil

}

func main() {
	stage = os.Getenv("STAGE")
	lambda.Start(handler)
}
