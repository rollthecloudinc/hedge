package main

import (
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var handler Handler

type Handler func(req *events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error)

type ActionContext struct {
	UserId string
}

func InitializeHandler(c *ActionContext) Handler {
	return func(req *events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
		//ac := RequestActionContext(c)
		// ac.UserId = GetUserId(req)
		fmt.Printf("%#v", req)
		return events.APIGatewayProxyResponse{StatusCode: 200}, nil
	}
}

func RequestActionContext(ac *ActionContext) *ActionContext {
	return &ActionContext{}
}

func init() {
	actionContext := ActionContext{}
	handler = InitializeHandler(&actionContext)
}

func main() {
	lambda.Start(handler)
}
