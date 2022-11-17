package main

import (
	"goclassifieds/lib/utils"
	"log"
	"os"

	"github.com/MicahParks/keyfunc"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/golang-jwt/jwt/v4"
)

var handler Handler

type Handler func(req *events.APIGatewayProxyRequest) (events.APIGatewayCustomAuthorizerResponse, error)

type ActionContext struct {
	UserPoolId string
}

// Authorizer custom api authorizer
func Authorizer(request *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayCustomAuthorizerResponse, error) {
	token1 := request.Headers["authorization"]

	utils.LogUsageForHttpRequest(request)

	log.Printf("%v", request)
	log.Printf("token is %s", token1)

	if token1 == "" {
		log.Print("unauthorized request pass thru")
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
		}, nil
	}

	token := token1[7:]
	log.Printf("token after is %s", token)

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
	log.SetFlags(0)
	lambda.Start(handler)
}
