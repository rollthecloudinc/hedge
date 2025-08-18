package main

import (
	"context"
	"goclassifieds/lib/entity"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
)

func handler(ctx context.Context, payload *entity.AfterSaveExecEntityRequest) (entity.AfterSaveExecEntityResponse, error) {

	/**
	 * This is where all the code goes to create action SECRETS
	 * for a site. Both for repo and enviironment.
	 */
	log.Print("Start site workflow")

	return entity.AfterSaveExecEntityResponse{}, nil
}

func main() {
	log.SetFlags(0)
	// Make the handler available for Remote Procedure Call by AWS Lambda
	lambda.Start(handler)
}
