package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"goclassifieds/lib/ads"
	"io/ioutil"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	s3 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

func handler(ctx context.Context, s3Event events.S3Event) {

	sess := session.Must(session.NewSession())
	downloader := s3manager.NewDownloader(sess)

	for _, record := range s3Event.Records {

		buf := aws.NewWriteAtBuffer([]byte{})
		rec := record.S3

		_, err := downloader.Download(buf, &s3.GetObjectInput{
			Bucket: aws.String(rec.Bucket.Name),
			Key:    aws.String(rec.Object.Key),
		})

		if err != nil {
			log.Fatalf("failed to download file, %v", err)
		}

		gz, err := gzip.NewReader(bytes.NewReader(buf.Bytes()))
		if err != nil {
			log.Fatal(err)
		}

		defer gz.Close()

		text, _ := ioutil.ReadAll(gz)
		// log.Printf("ad: %s", content)

		ad := ads.Ad{}
		json.Unmarshal(text, &ad)

		log.Printf("ad: %v", ad)

	}
}

func main() {
	// Make the handler available for Remote Procedure Call by AWS Lambda
	lambda.Start(handler)
}
