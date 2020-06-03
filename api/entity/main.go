package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"strings"
	"text/template"

	"goclassifieds/lib/entity"
	"goclassifieds/lib/utils"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	session "github.com/aws/aws-sdk-go/aws/session"
	lambda2 "github.com/aws/aws-sdk-go/service/lambda"
	ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/gin"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"github.com/tangzero/inflector"
)

var ginLambda *ginadapter.GinLambda

type ActionFunc func(context *gin.Context, ac *ActionContext)

type TemplateQueryFunc func(e string, data *entity.EntityFinderDataBag) []map[string]interface{}

type ActionContext struct {
	EsClient      *elasticsearch7.Client
	Session       *session.Session
	Lambda        *lambda2.Lambda
	TypeManager   entity.Manager
	EntityManager entity.Manager
	EntityName    string
	Template      *template.Template
}

func GetEntities(context *gin.Context, ac *ActionContext) {
	query := context.Param("queryName")
	if query == "" {
		query = inflector.Pluralize(ac.EntityName)
	}
	id, err := uuid.Parse(query)
	if err != nil {
		typeId := context.Query("typeId")
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
			Query:      context.Request.URL.Query(),
			Attributes: allAttributes,
			UserId:     utils.GetSubject(context),
		}
		entities := ac.EntityManager.Find("default", query, &data)
		context.JSON(200, entities)
	} else {
		log.Printf("entity by id: %s", id)
		ent := ac.EntityManager.Load(id.String(), "default")
		context.JSON(200, ent)
	}
}

func CreateEntity(context *gin.Context, ac *ActionContext) {
	var e map[string]interface{}
	body, err := ioutil.ReadAll(context.Request.Body)
	if err != nil {
		log.Printf("Error json binding: %s", err.Error())
	}
	json.Unmarshal(body, &e)
	newEntity, err := ac.EntityManager.Create(e)
	if err != nil {
		context.JSON(500, err)
		return
	}
	context.JSON(200, newEntity)
}

func DeclareAction(action ActionFunc, ac ActionContext) gin.HandlerFunc {
	return func(context *gin.Context) {
		entityName := context.Param("entityName")
		pluralName := inflector.Pluralize(entityName)
		singularName := inflector.Singularize(entityName)
		ac.EntityName = singularName
		ac.TypeManager = entity.NewEntityTypeManager(entity.DefaultManagerConfig{
			SingularName: "type",
			PluralName:   "types",
			Index:        "classified_types",
			EsClient:     ac.EsClient,
			Session:      ac.Session,
			Lambda:       ac.Lambda,
			Template:     ac.Template,
			UserId:       utils.GetSubject(context),
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
				UserId:       utils.GetSubject(context),
			})
		}
		action(context, &ac)
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
		"query": TemplateQuery(&actionContext),
	}

	t, err := template.New("").Funcs(funcMap).ParseFiles("api/entity/types.json.tmpl", "api/entity/queries.json.tmpl")

	if err != nil {
		log.Printf("Error: %s", err.Error())
	}

	actionContext.Template = t

	r := gin.Default()
	r.GET("/entity/:entityName", DeclareAction(GetEntities, actionContext))
	r.GET("/entity/:entityName/:queryName", DeclareAction(GetEntities, actionContext))
	r.POST("/entity/:entityName", DeclareAction(CreateEntity, actionContext))

	ginLambda = ginadapter.New(r)
}

func Handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// If no name is provided in the HTTP request body, throw an error
	res, err := ginLambda.ProxyWithContext(ctx, req)
	res.Headers["Access-Control-Allow-Origin"] = "*"
	return res, err
}

func main() {
	lambda.Start(Handler)
}
