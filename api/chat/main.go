package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"

	"goclassifieds/lib/entity"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	session "github.com/aws/aws-sdk-go/aws/session"
	lambda2 "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/gocql/gocql"
)

var handler Handler

type Handler func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error)

type ActionContext struct {
	Session       *gocql.Session
	Lambda        *lambda2.Lambda
	EntityManager entity.EntityManager
	UserId        string
}

func CreateEntity(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {
	var e map[string]interface{}
	res := events.APIGatewayProxyResponse{StatusCode: 500}
	body := []byte(req.Body)
	json.Unmarshal(body, &e)
	newEntity, err := ac.EntityManager.Create(e)
	if err != nil {
		return res, err
	}
	resBody, err := json.Marshal(newEntity)
	if err != nil {
		return res, err
	}
	res.StatusCode = 200
	res.Headers = map[string]string{
		"Content-Type": "application/json",
	}
	res.Body = string(resBody)
	return res, nil
}

func InitializeHandler(c *ActionContext) Handler {
	return func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
		ac := RequestActionContext(c)
		ac.UserId = GetUserId(req)
		ac.EntityManager = NewManager(ac)
		return CreateEntity(req, ac)
		//return events.APIGatewayProxyResponse{StatusCode: 200}, nil
	}
}

func RequestActionContext(ac *ActionContext) *ActionContext {
	return &ActionContext{
		Session: ac.Session,
		Lambda:  ac.Lambda,
	}
}

func NewManager(ac *ActionContext) entity.EntityManager {
	return entity.EntityManager{
		Config: entity.EntityConfig{
			SingularName: "chatconnection",
			PluralName:   "chatconnections",
			IdKey:        "id",
		},
		Creator: entity.DefaultCreatorAdaptor{
			Config: entity.DefaultCreatorConfig{
				Lambda: ac.Lambda,
				UserId: ac.UserId,
				Save:   "default",
			},
		},
		Storages: map[string]entity.Storage{
			"default": entity.CqlStorageAdaptor{
				Config: entity.CqlAdaptorConfig{
					Session: ac.Session,
					Table:   "chatconnections",
				},
			},
		},
	}
}

func GetUserId(req *events.APIGatewayProxyRequest) string {
	userId := ""
	if req.RequestContext.Authorizer["claims"] != nil {
		userId = fmt.Sprint(req.RequestContext.Authorizer["claims"].(map[string]interface{})["sub"])
		if userId == "<nil>" {
			userId = ""
		}
	}
	return userId
}

func init() {
	log.Printf("chat start")

	cluster := gocql.NewCluster("cassandra.us-east-1.amazonaws.com")
	cluster.Keyspace = "ClassifiedsDev"
	cluster.Port = 9142
	cluster.Consistency = gocql.LocalQuorum
	cluster.Authenticator = &gocql.PasswordAuthenticator{Username: "tzmijewski-at-989992233821", Password: "oALqeCqjS3BgyiBp2Ram8kTUbhttAYoyUoL70hmz+tY="}
	cluster.SslOpts = &gocql.SslOptions{Config: &tls.Config{ServerName: "cassandra.us-east-1.amazonaws.com"}, CaPath: "api/chat/AmazonRootCA1.pem", EnableHostVerification: true}
	cluster.PoolConfig = gocql.PoolConfig{HostSelectionPolicy: /*gocql.TokenAwareHostPolicy(*/ gocql.DCAwareRoundRobinPolicy("us-east-1") /*)*/}
	cSession, err := cluster.CreateSession()
	if err != nil {
		log.Fatal(err)
	}

	sess := session.Must(session.NewSession())
	lClient := lambda2.New(sess)

	actionContext := ActionContext{
		Session: cSession,
		Lambda:  lClient,
	}

	handler = InitializeHandler(&actionContext)

	log.Print("chat started")
}

func main() {
	lambda.Start(handler)
}
