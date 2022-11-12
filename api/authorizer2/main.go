package main

import (
	"context"
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
	token1 := request.Headers["authorization"]

	_, hedged := request.Headers["x-hedge-region"]
	if hedged {
		log.Print("REPORT RequestId: " + request.RequestContext.RequestID + " Function: " + os.Getenv("AWS_LAMBDA_FUNCTION_NAME") + " Path: " + request.Path + " Resource: " + request.Resource + " X-HEDGE-REGIONS: " + request.Headers["x-hedge-regions"] + " X-HEDGE-INTENSITIES: " + request.Headers["x-hedge-intensities"] + " X-HEDGE-REGION: " + request.Headers["x-hedge-region"] + " X-HEDGE-SERVICE: " + request.Headers["x-hedge-service"])
	} else {
		log.Print("REPORT RequestId: " + request.RequestContext.RequestID + " Function: " + os.Getenv("AWS_LAMBDA_FUNCTION_NAME") + " Path: " + request.Path + " Resource: " + request.Resource)
	}

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

	// Fetch all keys
	jwkSet, err := jwk.Fetch(context.Background(), "https://cognito-idp.us-east-1.amazonaws.com/"+ac.UserPoolId+"/.well-known/jwks.json")
	if err != nil {
		log.Fatalln("Unable to fetch keys")
	}

	// Verify
	t, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		key, _ := jwkSet.LookupKeyID(t.Header["kid"].(string))
		var k interface{}
		err = key.Raw(&k)
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
	// claimsMap := make(map[string]interface{})
	// claimsMap["claims"] = claims

	log.Printf("users claims %v", claims)

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
