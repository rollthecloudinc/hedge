package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"
	"time"

	"goclassifieds/lib/entity"
	"goclassifieds/lib/utils"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	session "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/apigatewaymanagementapi"
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
	Gateway        *apigatewaymanagementapi.ApiGatewayManagementApi
	Template       *template.Template
	EntityManager  entity.EntityManager
	UserId         string
	EntityName     string
	TemplateName   string
	Implementation string
	Stage          string
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

		utils.LogUsageForHttpRequest(req)

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
		Gateway:        c.Gateway,
		Implementation: "default",
		Bindings:       &entity.VariableBindings{Values: make([]interface{}, 0)},
		Stage:          c.Stage,
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
	manager := entity.EntityManager{
		Config: entity.EntityConfig{
			SingularName: ac.EntityName,
			PluralName:   inflector.Pluralize(ac.EntityName),
			IdKey:        "id",
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
						"adid":           "adId",
						"profileid":      "profileId",
					},
				},
			},
		},
		Hooks: map[entity.Hooks]entity.EntityHook{},
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
			"default/gridlayouts": func(entities []map[string]interface{}, m *entity.EntityManager) ([]map[string]interface{}, error, entity.HookSignals) {
				items := make(map[string]interface{})
				for _, ent := range entities {
					gridItem := map[string]interface{}{
						"cols":   ent["cols"],
						"rows":   ent["rows"],
						"x":      ent["x"],
						"y":      ent["y"],
						"weight": ent["weight"],
					}
					id := fmt.Sprint(ent["id"])
					if _, ok := items[id]; !ok {
						items[id] = map[string]interface{}{
							"id":        ent["id"],
							"site":      ent["site"],
							"gridItems": make([]map[string]interface{}, 0),
						}
					}
					items[id].(map[string]interface{})["gridItems"] = append(items[id].(map[string]interface{})["gridItems"].([]map[string]interface{}), gridItem)
				}
				rows := make([]map[string]interface{}, 0)
				for _, ent := range items {
					rows = append(rows, ent.(map[string]interface{}))
				}
				return rows, nil, entity.HookContinue
			},
		},
	}

	if ac.EntityName == "chatmessage" {
		manager.Hooks[entity.AfterSave] = func(ent map[string]interface{}, m *entity.EntityManager) (map[string]interface{}, error) {
			log.Print("After chat message save")
			data, _ := json.Marshal(ent)
			connManager := entity.EntityManager{
				Config: entity.EntityConfig{
					SingularName: "chatconnection",
					PluralName:   "chatconnections",
					IdKey:        "connId",
					Stage:        ac.Stage,
				},
				Finders: map[string]entity.Finder{
					"default": entity.CqlTemplateFinder{
						Config: entity.CqlTemplateFinderConfig{
							Session:  ac.Session,
							Template: ac.Template,
							Table:    "chatconnections",
							Bindings: ac.Bindings,
							Aliases: map[string]string{
								"connid":    "connId",
								"userid":    "userId",
								"createdat": "createdAt",
							},
						},
					},
				},
				CollectionHooks: map[string]entity.EntityCollectionHook{
					"default/_chatconnections": entity.PipeCollectionHooks(
						entity.FilterEntities(func(ent map[string]interface{}) bool {
							return ent["createdAt"].(time.Time).After(time.Now().Add(-1 * time.Hour))
						}),
						entity.MergeEntities(func(m *entity.EntityManager) []map[string]interface{} {
							allAttributes := make([]entity.EntityAttribute, 0)
							data := entity.EntityFinderDataBag{
								Req:        req,
								Attributes: allAttributes,
								Metadata: map[string]interface{}{
									"recipientId": ent["recipientId"],
								},
							}
							return m.Find("default", "_chatconnections_inverse", &data)
						}),
					),
					"default/_chatconnections_inverse": entity.PipeCollectionHooks(
						entity.FilterEntities(func(ent map[string]interface{}) bool {
							return ent["createdAt"].(time.Time).After(time.Now().Add(-1 * time.Hour))
						}),
					),
				},
			}
			allAttributes := make([]entity.EntityAttribute, 0)
			dataBag := entity.EntityFinderDataBag{
				Req:        req,
				Attributes: allAttributes,
			}
			connections := connManager.Find("default", "_chatconnections", &dataBag)
			for _, conn := range connections {
				log.Printf("chat connection = %s", conn["connId"])
				_, err := ac.Gateway.PostToConnection(&apigatewaymanagementapi.PostToConnectionInput{
					ConnectionId: aws.String(fmt.Sprint(conn["connId"])),
					Data:         data,
				})
				if err != nil {
					log.Print(err)
				}
			}
			return ent, nil
		}
	}

	if ac.EntityName == "gridlayout" {
		manager.Storages["expansion"] = entity.CqlAutoDiscoveryExpansionStorageAdaptor{
			Config: entity.CqlAdaptorConfig{
				Session: ac.Session,
				Table:   "gridlayouts",
			},
		}
		manager.Creator = entity.DefaultCreatorAdaptor{
			Config: entity.DefaultCreatorConfig{
				Lambda: ac.Lambda,
				UserId: ac.UserId,
				Save:   "expansion",
			},
		}
	}

	return manager
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
	cluster.Authenticator = &gocql.PasswordAuthenticator{Username: os.Getenv("KEYSPACE_USERNAME"), Password: os.Getenv("KEYSPACE_PASSWORD")}
	cluster.SslOpts = &gocql.SslOptions{Config: &tls.Config{ServerName: "cassandra.us-east-1.amazonaws.com"}, CaPath: "api/chat/AmazonRootCA1.pem", EnableHostVerification: true}
	cluster.PoolConfig = gocql.PoolConfig{HostSelectionPolicy: /*gocql.TokenAwareHostPolicy(*/ gocql.DCAwareRoundRobinPolicy("us-east-1") /*)*/}
	cSession, err := cluster.CreateSession()
	if err != nil {
		log.Fatal(err)
	}

	sess := session.Must(session.NewSession())
	lClient := lambda2.New(sess)
	gateway := apigatewaymanagementapi.New(sess, aws.NewConfig().WithEndpoint(os.Getenv("APIGATEWAY_ENDPOINT")))

	actionContext := ActionContext{
		Session: cSession,
		Lambda:  lClient,
		Gateway: gateway,
		Stage:   os.Getenv("STAGE"),
	}

	handler = InitializeHandler(&actionContext)

	log.Print("chat started")
}

func main() {
	log.SetFlags(0)
	lambda.Start(handler)
}
