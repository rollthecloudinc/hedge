package utils

import (
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
)

type LogUsageLambdaInput struct {
	Intensities  string
	Regions      string
	Region       string
	UserId       string
	Username     string
	Service      string
	Resource     string
	Path         string
	RequestId    string
	Repository   string
	Organization string
}

func LogUsageForHttpRequest(req *events.APIGatewayProxyRequest) {

	_, hedged := req.Headers["x-hedge-region"]
	if hedged {
		log.Print("REPORT RequestId: " + req.RequestContext.RequestID + " Function: " + os.Getenv("AWS_LAMBDA_FUNCTION_NAME") + " Path: " + req.Path + " Resource: " + req.Resource + " X-HEDGE-REGIONS: " + req.Headers["x-hedge-regions"] + " X-HEDGE-INTENSITIES: " + req.Headers["x-hedge-intensities"] + " X-HEDGE-REGION: " + req.Headers["x-hedge-region"] + " X-HEDGE-SERVICE: " + req.Headers["x-hedge-service"] + " UserId: " + GetUserIdFromHttpRequest(req) + " Username: " + GetUsernameFromHttpRequest(req))
	} else {
		log.Print("REPORT RequestId: " + req.RequestContext.RequestID + " Function: " + os.Getenv("AWS_LAMBDA_FUNCTION_NAME") + " Path: " + req.Path + " Resource: " + req.Resource)
	}

}

func LogUsageForWebsocketRequest(req *events.APIGatewayWebsocketProxyRequest) {

	_, hedged := req.Headers["x-hedge-region"]
	if hedged {
		log.Print("REPORT RequestId: " + req.RequestContext.RequestID + " Function: " + os.Getenv("AWS_LAMBDA_FUNCTION_NAME") + " Path: " + req.Path + " Resource: " + req.Resource + " X-HEDGE-REGIONS: " + req.Headers["x-hedge-regions"] + " X-HEDGE-INTENSITIES: " + req.Headers["x-hedge-intensities"] + " X-HEDGE-REGION: " + req.Headers["x-hedge-region"] + " X-HEDGE-SERVICE: " + req.Headers["x-hedge-service"])
	} else {
		log.Print("REPORT RequestId: " + req.RequestContext.RequestID + " Function: " + os.Getenv("AWS_LAMBDA_FUNCTION_NAME") + " Path: " + req.Path + " Resource: " + req.Resource)
	}

}

func LogUsageForLambda() {

	log.Print("REPORT Function: " + os.Getenv("AWS_LAMBDA_FUNCTION_NAME"))

}

func LogUsageForLambdaWithInput(input *LogUsageLambdaInput) {

	log.Print("REPORT RequestId: " + input.RequestId + " Function: " + os.Getenv("AWS_LAMBDA_FUNCTION_NAME") + " Path: " + input.Path + " Resource: " + input.Resource + " X-HEDGE-REGIONS: " + input.Regions + " X-HEDGE-INTENSITIES: " + input.Intensities + " X-HEDGE-REGION: " + input.Region + " X-HEDGE-SERVICE: " + input.Service + " UserId: " + input.UserId + " Username: " + input.Username + " Repository: " + input.Repository + " Organization: " + input.Organization)

}

func GetUserIdFromHttpRequest(req *events.APIGatewayProxyRequest) string {
	userId := ""
	log.Printf("claims are %v", req.RequestContext.Authorizer["claims"])
	if req.RequestContext.Authorizer["claims"] != nil {
		userId = fmt.Sprint(req.RequestContext.Authorizer["claims"].(map[string]interface{})["sub"])
		if userId == "<nil>" {
			userId = ""
		}
	} else if req.RequestContext.Authorizer["sub"] != nil {
		userId = req.RequestContext.Authorizer["sub"].(string)
	}
	return userId
}

func GetUsernameFromHttpRequest(req *events.APIGatewayProxyRequest) string {
	username := ""
	field := "cognito:username"
	/*if os.Getenv("STAGE") == "prod" {
		field = "cognito:username"
	}*/
	log.Printf("claims are %v", req.RequestContext.Authorizer["claims"])
	if req.RequestContext.Authorizer["claims"] != nil {
		username = fmt.Sprint(req.RequestContext.Authorizer["claims"].(map[string]interface{})[field])
		if username == "<nil>" {
			username = ""
		}
	} else if req.RequestContext.Authorizer[field] != nil {
		username = req.RequestContext.Authorizer[field].(string)
	}
	return username
}

/*func GetUserIdFromWebsocketRequest(req *events.APIGatewayWebsocketProxyRequest) string {
	userId := ""
	log.Printf("claims are %v", req.RequestContext.Authorizer)
	if req.RequestContext.Authorizer["claims"] != nil {
		userId = fmt.Sprint(req.RequestContext.Authorizer["claims"].(map[string]interface{})["sub"])
		if userId == "<nil>" {
			userId = ""
		}
	} else if req.RequestContext.Authorizer["sub"] != nil {
		userId = req.RequestContext.Authorizer["sub"].(string)
	}
	return userId
}

func GetUsernameFromWebsocketRequest(req *events.APIGatewayWebsocketProxyRequest) string {
	username := ""
	field := "cognito:username"
	log.Printf("claims are %v", req.RequestContext.Authorizer["claims"])
	if req.RequestContext.Authorizer["claims"] != nil {
		username = fmt.Sprint(req.RequestContext.Authorizer["claims"].(map[string]interface{})[field])
		if username == "<nil>" {
			username = ""
		}
	} else if req.RequestContext.Authorizer[field] != nil {
		username = req.RequestContext.Authorizer[field].(string)
	}
	return username
}*/
