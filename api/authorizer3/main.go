package main

import (
	"goclassifieds/lib/utils"
	"log"
	"os"
	"errors"

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
		// For now lock it down but eventually needs to support anonymous users.
		/*log.Print("unauthorized request pass thru")
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
		}, nil*/
		log.Print("Unauthorized requires token")
		return events.APIGatewayCustomAuthorizerResponse{}, errors.New("Unauthorized requires token")
	}

	token := token1[7:]
	log.Printf("token after is %s", token)

	// jwks, err := keyfunc.Get("https://cognito-idp.us-east-1.amazonaws.com/"+ac.UserPoolId+"/.well-known/jwks.json", keyfunc.Options{})

	// --------------------- Canva integration for jwks ----------------------

	appId := "AAGJBJc7pZs"
	jwks, err := keyfunc.Get("https://api.canva.com/rest/v1/apps/" + appId + "/jwks", keyfunc.Options{})

	// --------------------- End Canva integration for jwks ----------------------
	
	if err != nil {
		log.Print("Unable to fetch keys")
	}

	log.Print("fectehd keys")

	// Verify
	t, err := jwt.Parse(token, jwks.Keyfunc)
	if err != nil || !t.Valid {
		log.Print(err)
		log.Print("Unauthorized")
		return events.APIGatewayCustomAuthorizerResponse{}, err
	}

	log.Print("authorized")

	claims := t.Claims.(jwt.MapClaims)

	log.Print("got claims")

	// -------- Canva Integration for Claims Compatibility ---------------------------------
	// remap to source of truth when shared between providers like: user id/sub, user name, etc.
	// Otherwise overload with claims that can be pulled out once request identified as a specific auth host ie.cognito, canva, etc.

	// log expected claims for canva
	log.Printf("Canva userId: %s", claims["userId"])
	log.Printf("Canva designId: %s", claims["brandId"])
	log.Printf("Canva teamId: %s", claims["aud"])

	// Cognito is the source of truth so map Canva userId to sub which means the same thing.
	claims["sub"] = claims["userId"]

	// Canva does not have usernames just optional display names. So for now just reuse the userId.
	// We could use the connect api to grab the display name if it exists... but why really...
	claims["cognito:username"] = claims["userId"]

	// --------- End Canva Integration for Claims Compatibility -------------------

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
