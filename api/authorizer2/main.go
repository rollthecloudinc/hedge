package main

import (
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/dgrijalva/jwt-go"
	"github.com/lestrrat-go/jwx/jwk"
)

var handler Handler

type Handler func(req *events.APIGatewayProxyRequest) (events.APIGatewayCustomAuthorizerResponse, error)

type ActionContext struct {
	UserPoolId string
}

// Authorizer custom api authorizer
func Authorizer(request *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayCustomAuthorizerResponse, error) {
	token := request.Headers["Authorization"]

	// Fetch all keys
	jwkSet, err := jwk.Fetch("https://cognito-idp.us-east-1.amazonaws.com/" + ac.UserPoolId + "/.well-known/jwks.json")
	if err != nil {
		log.Fatalln("Unable to fetch keys")
	}

	// Verify
	t, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		keys := jwkSet.LookupKeyID(t.Header["kid"].(string))
		var k interface{}
		err = keys[0].Raw(&k)
		if err != nil {
			return nil, err
		}
		return k, nil
	})
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
	return func(req *events.APIGatewayProxyRequest) (events.APIGatewayCustomAuthorizerResponse, error) {
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
