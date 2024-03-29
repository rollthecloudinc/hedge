package main

import (
	"log"
	"os"

	"goclassifieds/lib/utils"

	"github.com/MicahParks/keyfunc"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/golang-jwt/jwt/v4"
)

var handler Handler

type Handler func(req *events.APIGatewayWebsocketProxyRequest) (events.APIGatewayCustomAuthorizerResponse, error)

type ActionContext struct {
	UserPoolId string
}

// Authorizer custom api authorizer
func Authorizer(request *events.APIGatewayWebsocketProxyRequest, ac *ActionContext) (events.APIGatewayCustomAuthorizerResponse, error) {
	token := request.QueryStringParameters["token"]

	usageLog := &utils.LogUsageLambdaInput{
		// UserId: GetUserId(req),
		//Username:     GetUsername(req),
		UserId:       "null",
		Username:     "null",
		Resource:     request.RequestContext.RouteKey,
		Path:         request.RequestContext.EventType,
		RequestId:    request.RequestContext.RequestID,
		Intensities:  "null",
		Regions:      "null",
		Region:       "null",
		Service:      "null",
		Repository:   "null",
		Organization: "null",
	}
	_, hedged := request.Headers["x-hedge-region"]
	if hedged {
		usageLog.Intensities = request.Headers["x-hedge-intensities"]
		usageLog.Regions = request.Headers["x-hedge-regions"]
		usageLog.Region = request.Headers["x-hedge-region"]
		usageLog.Service = request.Headers["x-hedge-service"]
	}

	utils.LogUsageForLambdaWithInput(usageLog)

	// ctx := context.Background()
	// Fetch all keys
	// jwkSet, err := jwk.Fetch(ctx, "https://cognito-idp.us-east-1.amazonaws.com/"+ac.UserPoolId+"/.well-known/jwks.json")
	jwks, err := keyfunc.Get("https://cognito-idp.us-east-1.amazonaws.com/"+ac.UserPoolId+"/.well-known/jwks.json", keyfunc.Options{})
	if err != nil {
		log.Fatalln("Unable to fetch keys")
	}

	log.Print("fectehd keys")

	// Verify
	t, err := jwt.Parse(token, jwks.Keyfunc)
	if err != nil || !t.Valid {
		log.Print(err)
		log.Fatalln("Unauthorized")
	}

	log.Print("authorized")

	claims := t.Claims.(jwt.MapClaims)

	log.Print("got claims")

	claims["cognito:groups"] = nil //claims["cognito:groups"]
	claims["cognito:roles"] = nil  //claims["cognito:groups"]

	return events.APIGatewayCustomAuthorizerResponse{
		PrincipalID: "me",
		PolicyDocument: events.APIGatewayCustomAuthorizerPolicy{
			Version: "2012-10-17",
			Statement: []events.IAMPolicyStatement{
				{
					Action:   []string{"execute-api:Invoke"},
					Effect:   "Allow",
					Resource: []string{"*"},
					// Resource: []string{request.Resource},
				},
			},
		},
		Context: claims,
	}, nil
}

func InitializeHandler(ac ActionContext) Handler {
	return func(req *events.APIGatewayWebsocketProxyRequest) (events.APIGatewayCustomAuthorizerResponse, error) {
		return Authorizer(req, &ac)
	}
}

func init() {
	log.Printf("Gin cold start")
	actionContext := ActionContext{
		UserPoolId: os.Getenv("USER_POOL_ID"),
	}
	handler = InitializeHandler(actionContext)
}

func main() {
	log.SetFlags(0)
	lambda.Start(handler)
}
