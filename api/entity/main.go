package main

import (
	"bytes"
	"context"
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
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"github.com/tangzero/inflector"
)

// var ginLambda *ginadapter.GinLambda
var actions map[string]ActionHandler

type ActionHandler func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error)
type ActionFunc func(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error)

type TemplateQueryFunc func(e string, data *entity.EntityFinderDataBag) []map[string]interface{}
type TemplateLambdaFunc func(e string, userId string, data []map[string]interface{}) entity.EntityDataResponse

type ActionContext struct {
	EsClient      *elasticsearch7.Client
	Session       *session.Session
	Lambda        *lambda2.Lambda
	TypeManager   entity.Manager
	EntityManager entity.Manager
	EntityName    string
	Template      *template.Template
}

func GetEntities(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {
	var res events.APIGatewayProxyResponse
	pathPieces := strings.Split(req.Path, "/")
	query := pathPieces[3]
	if query == "" {
		query = inflector.Pluralize(ac.EntityName)
	}
	id, err := uuid.Parse(query)
	if err != nil {
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
			Query:      req.MultiValueQueryStringParameters,
			Attributes: allAttributes,
			UserId:     GetUserId(req),
		}
		entities := ac.EntityManager.Find("default", query, &data)
		//context.JSON(200, entities)
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
	} else {
		log.Printf("entity by id: %s", id)
		ac.EntityManager.Load(id.String(), "default")
		res.StatusCode = 200
		return res, nil
		// context.JSON(200, ent)
	}
}

func CreateEntity(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {
	var e map[string]interface{}
	var res events.APIGatewayProxyResponse
	/*(body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return res, err
	}*/
	body := []byte(req.Body)
	json.Unmarshal(body, &e)
	_, err := ac.EntityManager.Create(e)
	if err != nil {
		// context.JSON(500, err)
		return res, err
	}
	res.StatusCode = 200
	return res, nil
	// context.JSON(200, newEntity)
}

func DeclareAction(action ActionFunc, ac ActionContext) ActionHandler {
	return func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
		pathPieces := strings.Split(req.Path, "/")
		entityName := pathPieces[2]
		pluralName := inflector.Pluralize(entityName)
		singularName := inflector.Singularize(entityName)
		ac.EntityName = singularName
		userId := GetUserId(req)
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
		return action(req, &ac)
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
	// stdout and stderr are sent to AWS CloudWatch Logs
	log.Printf("Gin cold start")

	elasticCfg := elasticsearch7.Config{
		Addresses: []string{
			"https://i12sa6lx3y:v75zs8pgyd@classifieds-4537380016.us-east-1.bonsaisearch.net:443",
		},
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

	/*r := gin.Default()
	r.GET("/entity/:entityName", DeclareAction(GetEntities, actionContext))
	r.GET("/entity/:entityName/:queryName", DeclareAction(GetEntities, actionContext))
	r.POST("/entity/:entityName", DeclareAction(CreateEntity, actionContext))

	ginLambda = ginadapter.New(r)*/

	actions = map[string]ActionHandler{
		"persist": DeclareAction(CreateEntity, actionContext),
		"view":    DeclareAction(GetEntities, actionContext),
	}
}

func Handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// If no name is provided in the HTTP request body, throw an error
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(req); err != nil {
		log.Fatalf("Error encoding request: %s", err)
	}
	log.Printf("request: %s", buf)

	res, err := actions["view"](&req)

	// If no name is provided in the HTTP request body, throw an error
	//res, err := ginLambda.ProxyWithContext(ctx, req)
	//res.Headers["Access-Control-Allow-Origin"] = "*"

	return res, err
}

func main() {
	lambda.Start(Handler)
}
