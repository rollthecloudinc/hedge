package main

import (
	"context"
	"goclassifieds/lib/ads"
	"goclassifieds/lib/entity"
	"goclassifieds/lib/utils"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/mitchellh/mapstructure"
)

func handler(ctx context.Context, payload *entity.ValidateEntityRequest) (entity.ValidateEntityResponse, error) {
	log.Print("Inside validate")
	log.Printf("Entity: %s", payload.EntityName)

	var obj ads.Ad
	err := mapstructure.Decode(payload.Entity, &obj)
	if err != nil {
		return entity.ValidateEntityResponse{
			Entity: payload.Entity,
			Valid:  false,
		}, err
	}

	obj.Id = utils.GenerateId()
	obj.Status = ads.Submitted // @todo: Enums not being validated :(
	obj.UserId = payload.UserId
	newEntity, _ := ads.ToEntity(&obj)

	return entity.ValidateEntityResponse{
		Entity: newEntity,
		Valid:  true,
	}, nil
}

func main() {
	// Make the handler available for Remote Procedure Call by AWS Lambda
	lambda.Start(handler)
}
