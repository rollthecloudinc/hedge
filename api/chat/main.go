package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"text/template"

	"goclassifieds/lib/entity"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	session "github.com/aws/aws-sdk-go/aws/session"
	lambda2 "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/gocql/gocql"
	"github.com/tangzero/inflector"
)

var handler Handler

type Handler func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error)
type TemplateBindValueFunc func(value interface{}) string

type ActionContext struct {
	Session        *gocql.Session
	Lambda         *lambda2.Lambda
	Template       *template.Template
	EntityManager  entity.EntityManager
	UserId         string
	EntityName     string
	TemplateName   string
	Implementation string
	Bindings       *entity.VariableBindings
}

func GetEntities(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {
	var res events.APIGatewayProxyResponse
	pathPieces := strings.Split(req.Path, "/")

	query := inflector.Pluralize(ac.EntityName)
	if len(pathPieces) == 3 && pathPieces[2] != "" {
		query = pathPieces[2]
	} else if ac.TemplateName != "" {
		query = ac.TemplateName
	}

	log.Printf("entity: %s | query: %s", ac.EntityName, query)

	allAttributes := make([]entity.EntityAttribute, 0)
	data := entity.EntityFinderDataBag{
		Req:        req,
		Attributes: allAttributes,
	}
	entities := ac.EntityManager.Find(ac.Implementation, query, &data)
	body, err := json.Marshal(entities)
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

		pathPieces := strings.Split(req.Path, "/")
		entityName := pathPieces[1]

		pluralName := inflector.Pluralize(entityName)
		singularName := inflector.Singularize(entityName)
		ac.EntityName = singularName
		ac.UserId = GetUserId(req)
		ac.EntityManager = NewManager(ac, req)

		if entityName == singularName && req.HTTPMethod == "POST" {
			return CreateEntity(req, ac)
		} else if entityName == pluralName && req.HTTPMethod == "GET" {
			return GetEntities(req, ac)
		}

		return events.APIGatewayProxyResponse{StatusCode: 500}, nil
	}
}

func RequestActionContext(c *ActionContext) *ActionContext {

	ac := &ActionContext{
		Session:        c.Session,
		Lambda:         c.Lambda,
		Implementation: "default",
		Bindings:       &entity.VariableBindings{Values: make([]interface{}, 0)},
	}

	funcMap := template.FuncMap{
		"bindValue": TemplateBindValue(ac),
	}

	t, err := template.New("").Funcs(funcMap).ParseFiles("api/chat/queries.tmpl")

	if err != nil {
		log.Printf("Error: %s", err.Error())
	}

	ac.Template = t

	return ac

}

func NewManager(ac *ActionContext, req *events.APIGatewayProxyRequest) entity.EntityManager {
	return entity.EntityManager{
		Config: entity.EntityConfig{
			SingularName: ac.EntityName,
			PluralName:   inflector.Pluralize(ac.EntityName),
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
					Table:   inflector.Pluralize(ac.EntityName),
				},
			},
		},
		Finders: map[string]entity.Finder{
			"default": entity.CqlTemplateFinder{
				Config: entity.CqlTemplateFinderConfig{
					Session:  ac.Session,
					Template: ac.Template,
					Table:    inflector.Pluralize(ac.EntityName),
					Bindings: ac.Bindings,
					Aliases: map[string]string{
						"recipientid":    "recipientId",
						"userid":         "userId",
						"senderid":       "senderId",
						"recipientlabel": "recipientLabel",
						"createdat":      "createdAt",
					},
				},
			},
		},
		CollectionHooks: map[string]entity.EntityCollectionHook{
			"default/chatmessages": entity.PipeCollectionHooks(
				entity.MergeEntities(func(m *entity.EntityManager) []map[string]interface{} {
					allAttributes := make([]entity.EntityAttribute, 0)
					data := entity.EntityFinderDataBag{
						Req:        req,
						Attributes: allAttributes,
					}
					return m.Find("default", "_chatmessages_inverse", &data)
				},
				)),
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

func TemplateBindValue(ac *ActionContext) TemplateBindValueFunc {
	return func(value interface{}) string {
		ac.Bindings.Values = append(ac.Bindings.Values, value)
		return "?"
	}
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
