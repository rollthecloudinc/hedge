package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"goclassifieds/lib/entity"
	"goclassifieds/lib/utils"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	session "github.com/aws/aws-sdk-go/aws/session"
	lambda2 "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/gocql/gocql"
)

var handler Handler

type Handler func(req *events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error)

type ActionContext struct {
	Session     *gocql.Session
	Lambda      *lambda2.Lambda
	UserId      string
	Stage       string
	ConnManager entity.EntityManager
}

func Connect(req *events.APIGatewayWebsocketProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {
	log.Print("connect")
	//log.Print("user id = " + ac.UserId)
	obj := make(map[string]interface{})
	obj["connId"] = req.RequestContext.ConnectionID
	obj["userId"] = ac.UserId
	obj["createdAt"] = time.Now()
	_, err := ac.ConnManager.Create(obj)
	if err != nil {
		log.Print(err)
		return events.APIGatewayProxyResponse{StatusCode: 500}, err
	}
	return events.APIGatewayProxyResponse{StatusCode: 200}, nil
}

func Disconnect(req *events.APIGatewayWebsocketProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {
	log.Print("disconnect")
	obj := make(map[string]interface{})
	obj["connId"] = req.RequestContext.ConnectionID
	ac.ConnManager.Purge("default", obj)
	return events.APIGatewayProxyResponse{StatusCode: 200}, nil
}

func InitializeHandler(c *ActionContext) Handler {
	return func(req *events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {

		usageLog := &utils.LogUsageLambdaInput{
			UserId: GetUserId(req),
			//Username:     GetUsername(req),
			Username:     "null",
			Resource:     req.RequestContext.RouteKey,
			Path:         req.RequestContext.EventType,
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
		/*_, hasOwner := req.PathParameters["owner"]
		if hasOwner {
			usageLog.Organization = req.PathParameters["owner"]
		}
		_, hasRepo := req.PathParameters["repo"]
		if hasRepo {
			usageLog.Repository = req.PathParameters["repo"]
		}*/

		utils.LogUsageForLambdaWithInput(usageLog)

		ac := RequestActionContext(c)

		ac.UserId = GetUserId(req)
		ac.ConnManager = CreateConnectionManager(ac)
		ac.ConnManager.Config.LogUsageLambdaInput = usageLog

		b, _ := json.Marshal(req)
		log.Print(string(b))

		if req.RequestContext.RouteKey == "$connect" {
			return Connect(req, ac)
		} else if req.RequestContext.RouteKey == "$disconnect" {
			return Disconnect(req, ac)
		} else if req.RequestContext.RouteKey == "$default" {
			return events.APIGatewayProxyResponse{StatusCode: 200}, nil
		}

		return events.APIGatewayProxyResponse{StatusCode: 500}, nil
	}
}

func CreateConnectionManager(ac *ActionContext) entity.EntityManager {
	manager := entity.EntityManager{
		Config: entity.EntityConfig{
			SingularName: "chatconnection",
			PluralName:   "chatconnections",
			IdKey:        "connId",
			Stage:        ac.Stage,
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
		Authorizers: map[string]entity.Authorization{
			"default": entity.NoopAuthorizationAdaptor{},
		},
		Validators: map[string]entity.Validator{
			"default": entity.NoopValidatorAdaptor{},
		},
	}
	return manager
}

func RequestActionContext(c *ActionContext) *ActionContext {
	return &ActionContext{
		Session: c.Session,
		Lambda:  c.Lambda,
		Stage:   c.Stage,
	}
}

func GetUserId(req *events.APIGatewayWebsocketProxyRequest) string {
	userId := ""
	if req.RequestContext.Authorizer.(map[string]interface{})["sub"] != nil {
		userId = fmt.Sprint(req.RequestContext.Authorizer.(map[string]interface{})["sub"])
		if userId == "<nil>" {
			userId = ""
		}
	}
	return userId
}

func init() {
	log.Print("init")
	cluster := gocql.NewCluster("cassandra.us-east-1.amazonaws.com")
	cluster.Keyspace = "ClassifiedsDev"
	cluster.Port = 9142
	cluster.Consistency = gocql.LocalQuorum
	cluster.Authenticator = &gocql.PasswordAuthenticator{Username: os.Getenv("KEYSPACE_USERNAME"), Password: os.Getenv("KEYSPACE_PASSWORD")}
	cluster.SslOpts = &gocql.SslOptions{Config: &tls.Config{ServerName: "cassandra.us-east-1.amazonaws.com"}, CaPath: "AmazonRootCA1.pem", EnableHostVerification: true}
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
		Stage:   os.Getenv("STAGE"),
	}

	handler = InitializeHandler(&actionContext)
}

func main() {
	log.SetFlags(0)
	log.Print("start xxx")
	lambda.Start(handler)
}
