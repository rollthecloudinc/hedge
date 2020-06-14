package main

import (
	"fmt"
	"log"

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
		log.Printf("connection id = %s", req.RequestContext.ConnectionID)
		log.Printf("user id = %s", GetUserId(req))
		return events.APIGatewayProxyResponse{StatusCode: 200}, nil
	}
}

func RequestActionContext(ac *ActionContext) *ActionContext {
	return &ActionContext{}
}

func GetUserId(req *events.APIGatewayWebsocketProxyRequest) string {
	userId := ""
	if req.RequestContext.Authorizer.(map[string]interface{})["sub"] != nil {
		userId = fmt.Sprint(req.RequestContext.Authorizer.(map[string]interface{})["sub"])
		if userId == "<nil>" {
			userId = ""
		}
	}
	return userId
}

func init() {
	actionContext := ActionContext{}
	handler = InitializeHandler(&actionContext)
}

func main() {
	lambda.Start(handler)
}
