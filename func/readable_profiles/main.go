package main

import (
	"context"
	"fmt"
	"goclassifieds/lib/entity"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
)

func handler(ctx context.Context, payload *entity.EntityDataRequest) (entity.EntityDataResponse, error) {
	log.Print("Inside readable_profiles")
	log.Printf("Entity: %s", payload.EntityName)

	rootIds := make(map[string]bool)
	for _, p := range payload.Data {
		id := fmt.Sprint(p["id"])
		parentId := fmt.Sprint(p["parentId"])
		_, ok := rootIds[id]
		if !ok && (parentId == "" || parentId == "<nil>") {
			rootIds[id] = true
		}
	}
	for _, p := range payload.Data {
		parentId := fmt.Sprint(p["parentId"])
		_, ok := rootIds[parentId]
		if !ok && parentId != "" && parentId != "<nil>" {
			rootIds[parentId] = true
		}
	}
	i := 0
	ids := make([]map[string]interface{}, len(rootIds))
	for key := range rootIds {
		ids[i] = map[string]interface{}{"value": key}
		i++
	}

	return entity.EntityDataResponse{
		Data: ids,
	}, nil
}

func main() {
	// Make the handler available for Remote Procedure Call by AWS Lambda
	lambda.Start(handler)
}
