package main

import (
	"log"
	"os"

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

	// ctx := context.Background()
	// Fetch all keys
	// jwkSet, err := jwk.Fetch(ctx, "https://cognito-idp.us-east-1.amazonaws.com/"+ac.UserPoolId+"/.well-known/jwks.json")
	jwks, err := keyfunc.Get("https://cognito-idp.us-east-1.amazonaws.com/"+ac.UserPoolId+"/.well-known/jwks.json", keyfunc.Options{})
	if err != nil {
		log.Fatalln("Unable to fetch keys")
	}

	// Verify
	t, err := jwt.Parse(token, jwks.Keyfunc)
	if err != nil || !t.Valid {
		log.Print(err)
		log.Fatalln("Unauthorized")
	}

	claims := t.Claims.(jwt.MapClaims)

	return events.APIGatewayCustomAuthorizerResponse{
		PrincipalID: "me",
		PolicyDocument: events.APIGatewayCustomAuthorizerPolicy{
			Version: "2012-10-17",
			Statement: []events.IAMPolicyStatement{
				{
					Action:   []string{"execute-api:*"},
					Effect:   "Allow",
					Resource: []string{"*"},
					// Resource: []string{request.Re},
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
	lambda.Start(handler)
}
