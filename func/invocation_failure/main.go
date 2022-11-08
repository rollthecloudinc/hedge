package main

import (
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
)

/*func handler(ctx context.Context, s3Event events.S3Event) {
}*/

func handler(request interface{}) (string, error) {
	b, _ := json.Marshal(request)
	log.Print(string(b))
	return "failure", nil
}

func main() {
	log.Print("start")
	lambda.Start(handler)
}
