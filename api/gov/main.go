package main

import (
	"encoding/json"
	"goclassifieds/lib/gov"
	"goclassifieds/lib/utils"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	lambda2 "github.com/aws/aws-sdk-go/service/lambda"
)

var handler Handler

type Handler func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error)

type ActionContext struct {
	Stage  string
	Lambda *lambda2.Lambda
}

func GetGrant(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {
	var res events.APIGatewayProxyResponse

	grantAccessRequest := gov.GrantAccessRequest{
		User:      req.PathParameters["user"],
		Type:      gov.UserTypeMap[req.PathParameters["type"]],
		Resource:  gov.ResourceTypeMap[req.PathParameters["resource"]],
		Operation: gov.OperationMap[req.PathParameters["op"]],
		Asset:     req.PathParameters["proxy"],
	}

	log.Print(req)

	payload, err := json.Marshal(grantAccessRequest)
	if err != nil {
		log.Printf("Error marshalling grant access request: %s", err.Error())
		res.StatusCode = 500
		return res, err
	}

	lambdaRes, err := ac.Lambda.Invoke(&lambda2.InvokeInput{FunctionName: aws.String("goclassifieds-api-" + ac.Stage + "-GrantAccess"), Payload: payload})
	if err != nil {
		log.Printf("error invoking grant_access: %s", err.Error())
		res.StatusCode = 500
		return res, err
	}

	//var grantRes gov.GrantAccessResponse
	//json.Unmarshal(lambdaRes.Payload, &grantRes)

	res.Body = string(lambdaRes.Payload)
	res.StatusCode = 200
	return res, nil
}

func InitializeHandler(c *ActionContext) Handler {
	return func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

		usageLog := &utils.LogUsageLambdaInput{
			UserId:       req.PathParameters["user"],
			Username:     "null",
			Resource:     req.Resource,
			Path:         req.Path,
			RequestId:    req.RequestContext.RequestID,
			Intensities:  "null",
			Regions:      "null",
			Region:       "null",
			Service:      "null",
			Repository:   "null",
			Organization: "null",
		}
		_, hedged := req.Headers["x-hedge-region"]
		if hedged {
			usageLog.Intensities = req.Headers["x-hedge-intensities"]
			usageLog.Regions = req.Headers["x-hedge-regions"]
			usageLog.Region = req.Headers["x-hedge-region"]
			usageLog.Service = req.Headers["x-hedge-service"]
		}
		_, hasOwner := req.PathParameters["owner"]
		if hasOwner {
			usageLog.Organization = req.PathParameters["owner"]
		}
		_, hasRepo := req.PathParameters["repo"]
		if hasRepo {
			usageLog.Repository = req.PathParameters["repo"]
		}

		utils.LogUsageForLambdaWithInput(usageLog)

		ac := RequestActionContext(c)

		//ac.UserId = GetUserId(req)

		if req.HTTPMethod == "GET" {
			return GetGrant(req, ac)
		} /*else if entityName == pluralName && req.HTTPMethod == "GET" {
			return GetEntities(req, ac)
		}*/

		return events.APIGatewayProxyResponse{StatusCode: 400}, nil
	}
}

func RequestActionContext(c *ActionContext) *ActionContext {

	ac := &ActionContext{
		Stage:  c.Stage,
		Lambda: c.Lambda,
	}

	return ac

}

func init() {
	log.Printf("gov start")

	sess := session.Must(session.NewSession())
	lClient := lambda2.New(sess)
	//gateway := apigatewaymanagementapi.New(sess, aws.NewConfig().WithEndpoint(os.Getenv("APIGATEWAY_ENDPOINT")))

	actionContext := ActionContext{
		Stage:  os.Getenv("STAGE"),
		Lambda: lClient,
	}

	handler = InitializeHandler(&actionContext)

	log.Print("gov started")
}

func main() {
	log.SetFlags(0)
	lambda.Start(handler)
}
