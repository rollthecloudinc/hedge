package main

import (
	"encoding/json"
	"goclassifieds/lib/entity"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
)

var handler Handler

type Handler func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error)

type ActionContext struct {
	Session       *session.Session
	Client        *cognitoidentityprovider.CognitoIdentityProvider
	EntityManager entity.Manager
	UserPoolId    string
}

func GetEntity(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {
	var res events.APIGatewayProxyResponse
	pathPieces := strings.Split(req.Path, "/")
	id := pathPieces[2]
	log.Printf("entity by id: %s", id)
	ent := ac.EntityManager.Load(id, "default")
	body, err := json.Marshal(ent)
	if err != nil {
		return res, err
	}
	res.StatusCode = 200
	res.Headers = map[string]string{
		"Content-Type": "application/json",
	}
	res.Body = string(body[:])
	return res, nil
}

func InitializeHandler(c *ActionContext) Handler {
	return func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

		ac := RequestActionContext(c)
		ac.EntityManager = NewManager(ac)

		if req.HTTPMethod == "GET" {
			return GetEntity(req, ac)
		}

		return events.APIGatewayProxyResponse{StatusCode: 500}, nil
	}
}

func NewManager(ac *ActionContext) entity.EntityManager {
	return entity.EntityManager{
		Loaders: map[string]entity.Loader{
			"default": entity.CognitoLoaderAdaptor{
				Config: entity.CognitoAdaptorConfig{
					Client:     ac.Client,
					UserPoolId: ac.UserPoolId,
					Transform: func(user *cognitoidentityprovider.UserType) (map[string]interface{}, error) {
						ent := make(map[string]interface{})
						ent["userName"] = user.Username
						for _, attr := range user.Attributes {
							if *attr.Name == "sub" {
								ent["id"] = attr.Value
							}
						}
						return ent, nil
					},
				},
			},
		},
	}
}

func RequestActionContext(ac *ActionContext) *ActionContext {
	return &ActionContext{
		Session: ac.Session,
		Client:  ac.Client,
	}
}

func init() {
	log.Printf("cold start")

	sess := session.Must(session.NewSession())
	client := cognitoidentityprovider.New(sess)

	actionContext := ActionContext{
		Session:    sess,
		Client:     client,
		UserPoolId: os.Getenv("USER_POOL_ID"),
	}

	handler = InitializeHandler(&actionContext)
}

func main() {
	lambda.Start(handler)
}
