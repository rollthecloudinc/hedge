package main

import (
	"context"
	"encoding/json"
	"errors"
	"goclassifieds/lib/ads"
	"goclassifieds/lib/entity"
	"goclassifieds/lib/utils"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/go-playground/validator/v10"
)

func handler(ctx context.Context, payload *entity.ValidateEntityRequest) (entity.ValidateEntityResponse, error) {
	log.Print("Inside validate")
	log.Printf("Entity: %s", payload.EntityName)

	invalid := entity.ValidateEntityResponse{
		Entity:       payload.Entity,
		Valid:        false,
		Unauthorized: true,
	}

	if payload.UserId == "" {
		return invalid, errors.New("Unauthorized to create entity")
	}

	invalid.Unauthorized = false

	jsonData, err := json.Marshal(payload.Entity)
	if err != nil {
		return invalid, err
	}

	var obj ads.Ad
	err = json.Unmarshal(jsonData, &obj)
	if err != nil {
		return invalid, err
	}

	submitted := ads.Submitted

	obj.Id = utils.GenerateId()
	obj.Status = &submitted // @todo: Enums not being validated :(
	obj.UserId = payload.UserId

	validate := validator.New()
	err = validate.Struct(obj)
	if err != nil {
		return invalid, err.(validator.ValidationErrors)
	}

	newEntity, _ := ads.ToEntity(&obj)

	return entity.ValidateEntityResponse{
		Entity:       newEntity,
		Valid:        true,
		Unauthorized: false,
	}, nil
}

func main() {
	// Make the handler available for Remote Procedure Call by AWS Lambda
	lambda.Start(handler)
}
