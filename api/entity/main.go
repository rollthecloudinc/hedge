package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"

	"goclassifieds/lib/entity"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	session "github.com/aws/aws-sdk-go/aws/session"
	lambda2 "github.com/aws/aws-sdk-go/service/lambda"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
	"github.com/mitchellh/mapstructure"
	"github.com/tangzero/inflector"
)

// var ginLambda *ginadapter.GinLambda
var handler Handler

type Handler func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error)

type TemplateQueryFunc func(e string, data *entity.EntityFinderDataBag) []map[string]interface{}
type TemplateLambdaFunc func(e string, userId string, data []map[string]interface{}) entity.EntityDataResponse

type ActionContext struct {
	EsClient       *elasticsearch7.Client
	Session        *session.Session
	Lambda         *lambda2.Lambda
	TypeManager    entity.Manager
	EntityManager  entity.Manager
	EntityName     string
	Template       *template.Template
	TemplateName   string
	Implementation string
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

	typeId := req.QueryStringParameters["typeId"]
	allAttributes := make([]entity.EntityAttribute, 0)

	if typeId != "" {
		objType := ac.TypeManager.Load(typeId, "default")
		var entType entity.EntityType
		mapstructure.Decode(objType, &entType)
		var b bytes.Buffer
		if err := json.NewEncoder(&b).Encode(objType); err != nil {
			log.Fatalf("Error encoding obj type: %s", err)
		}
		log.Printf("obj type: %s", b.String())
		for _, attribute := range entType.Attributes {
			flatAttributes := entity.FlattenEntityAttribute(attribute)
			for _, flatAttribute := range flatAttributes {
				log.Printf("attribute: %s", flatAttribute.Name)
				allAttributes = append(allAttributes, flatAttribute)
			}
		}
	}
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

func GetEntity(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {
	var res events.APIGatewayProxyResponse
	pathPieces := strings.Split(req.Path, "/")
	id := pathPieces[2]
	log.Printf("entity by id: %s", id)
	ent := ac.EntityManager.Load(id, ac.Implementation)
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

		if index := strings.Index(entityName, "list"); index > -1 {
			entityName = inflector.Pluralize(entityName[0:index])
			if entityName == "features" {
				entityName = "ads"
				ac.TemplateName = "featurelistitems"
				ac.Implementation = "features"
			}
		} else if entityName == "adprofileitems" {
			entityName = "profiles"
			ac.TemplateName = "profilenavitems"
		} else if entityName == "adtypes" {
			entityName = "types"
			ac.TemplateName = "all"
		}

		pluralName := inflector.Pluralize(entityName)
		singularName := inflector.Singularize(entityName)
		ac.EntityName = singularName
		userId := GetUserId(req)

		log.Printf("entity plural name: %s", pluralName)
		log.Printf("entity singular name: %s", singularName)

		ac.TypeManager = entity.NewEntityTypeManager(entity.DefaultManagerConfig{
			SingularName: "type",
			PluralName:   "types",
			Index:        "classified_types",
			EsClient:     ac.EsClient,
			Session:      ac.Session,
			Lambda:       ac.Lambda,
			Template:     ac.Template,
			UserId:       userId,
		})

		if singularName == "type" {
			ac.EntityManager = ac.TypeManager
		} else {
			ac.EntityManager = entity.NewDefaultManager(entity.DefaultManagerConfig{
				SingularName: singularName,
				PluralName:   pluralName,
				Index:        "classified_" + pluralName,
				EsClient:     ac.EsClient,
				Session:      ac.Session,
				Lambda:       ac.Lambda,
				Template:     ac.Template,
				UserId:       userId,
			})
		}

		if singularName == "ad" {
			collectionKey := "aggregations.features.features_filtered.feature_names.buckets"
			if req.QueryStringParameters["searchString"] == "" {
				collectionKey = "aggregations.features.feature_names.buckets"
			}
			ac.EntityManager.AddFinder("features", entity.ElasticTemplateFinder{
				Config: entity.ElasticTemplateFinderConfig{
					Index:         "classified_" + pluralName,
					Client:        ac.EsClient,
					Template:      ac.Template,
					CollectionKey: collectionKey,
					ObjectKey:     "",
				},
			})
		}

		if entityName == pluralName && req.HTTPMethod == "GET" {
			return GetEntities(req, ac)
		} else if entityName == singularName && req.HTTPMethod == "GET" {
			return GetEntity(req, ac)
		} else if entityName == singularName && req.HTTPMethod == "POST" {
			return CreateEntity(req, ac)
		}

		return events.APIGatewayProxyResponse{StatusCode: 500}, nil
	}
}

func TemplateQuery(ac *ActionContext) TemplateQueryFunc {

	return func(e string, data *entity.EntityFinderDataBag) []map[string]interface{} {

		pieces := strings.Split(e, "/")
		pluralName := inflector.Pluralize(pieces[0])
		singularName := inflector.Singularize(pieces[0])

		query := pluralName
		if len(pieces) == 2 {
			query = pieces[1]
		}

		entityManager := entity.NewDefaultManager(entity.DefaultManagerConfig{
			SingularName: singularName,
			PluralName:   pluralName,
			Index:        "classified_" + pluralName,
			EsClient:     ac.EsClient,
			Session:      ac.Session,
			Lambda:       ac.Lambda,
			Template:     ac.Template,
			UserId:       "",
		})

		/*data := entity.EntityFinderDataBag{
			Query:  queryData,
			UserId: userId,
		}*/

		/*data := entity.EntityFinderDataBag{
			Query:  make(map[string][]string, 0),
			UserId: "",
		}*/

		// @todo: allow third piece to specify type so that attributes can be replaced in data bag.
		// Will need to clone data bag and swap out attributes.
		// Limit nesting to avoid infinite loop -- only three levels allowed. Should be enough.

		entities := entityManager.Find("default", query, data)

		log.Print("TemplateQuery 8")
		return entities

	}

}

func TemplateLambda(ac *ActionContext) TemplateLambdaFunc {
	return func(e string, userId string, data []map[string]interface{}) entity.EntityDataResponse {

		pieces := strings.Split(e, "/")
		pluralName := inflector.Pluralize(pieces[0])
		singularName := inflector.Singularize(pieces[0])

		functionName := pluralName
		if len(pieces) == 2 {
			functionName = "goclassifieds-api-dev-" + pieces[1]
		}

		request := entity.EntityDataRequest{
			EntityName: singularName,
			UserId:     userId,
			Data:       data,
		}

		res, err := entity.ExecuteEntityLambda(ac.Lambda, functionName, &request)
		if err != nil {
			log.Printf("error invoking template lambda: %s", err.Error())
		}

		return res

	}
}

func RequestActionContext(ac *ActionContext) *ActionContext {
	return &ActionContext{
		EsClient:       ac.EsClient,
		Session:        ac.Session,
		Lambda:         ac.Lambda,
		Template:       ac.Template,
		Implementation: "default",
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
	log.Printf("Gin cold start")

	elasticCfg := elasticsearch7.Config{
		Addresses: []string{os.Getenv("ELASTIC_URL")},
	}

	esClient, err := elasticsearch7.NewClient(elasticCfg)
	if err != nil {

	}

	sess := session.Must(session.NewSession())
	lClient := lambda2.New(sess)

	actionContext := ActionContext{
		EsClient: esClient,
		Session:  sess,
		Lambda:   lClient,
	}

	funcMap := template.FuncMap{
		"query":  TemplateQuery(&actionContext),
		"lambda": TemplateLambda(&actionContext),
	}

	t, err := template.New("").Funcs(funcMap).ParseFiles("api/entity/types.json.tmpl", "api/entity/queries.json.tmpl")

	if err != nil {
		log.Printf("Error: %s", err.Error())
	}

	actionContext.Template = t

	handler = InitializeHandler(&actionContext)
}

func main() {
	lambda.Start(handler)
}
