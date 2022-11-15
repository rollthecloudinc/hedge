package utils

import (
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
)

func LogUsageForHttpRequest(req *events.APIGatewayProxyRequest) {

	_, hedged := req.Headers["x-hedge-region"]
	if hedged {
		log.Print("REPORT RequestId: " + req.RequestContext.RequestID + " Function: " + os.Getenv("AWS_LAMBDA_FUNCTION_NAME") + " Path: " + req.Path + " Resource: " + req.Resource + " X-HEDGE-REGIONS: " + req.Headers["x-hedge-regions"] + " X-HEDGE-INTENSITIES: " + req.Headers["x-hedge-intensities"] + " X-HEDGE-REGION: " + req.Headers["x-hedge-region"] + " X-HEDGE-SERVICE: " + req.Headers["x-hedge-service"])
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
