package main

import (
	"log"

	"goclassifieds/lib/shapeshift"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var handler shapeshift.Handler

type Handler func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error)

func init() {
	ac := shapeshift.ShapeshiftActionContext()
	handler = shapeshift.InitializeHandler(ac)
}

func main() {
	log.SetFlags(0)
	lambda.Start(handler)
}
