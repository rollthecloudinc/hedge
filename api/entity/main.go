package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"text/template"

	entity "goclassifieds/lib/entity"
	utils "goclassifieds/lib/utils"

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

type ActionContext struct {
	EsClient      *elasticsearch7.Client
	Session       *session.Session
	Lambda        *lambda2.Lambda
	TypeManager   entity.Manager
	EntityManager entity.Manager
	EntityName    string
	Template      *template.Template
}

type TypeTemplateData struct {
}

/*func GetEntityTypes(context *gin.Context, ac *ActionContext) {

	var tb bytes.Buffer
	err := ac.Template.ExecuteTemplate(&tb, ac.EntityName, TypeTemplateData{})
	if err != nil {
		log.Printf("Entity Type Error: %s", err.Error())
	}

	var types []map[string]interface{}
	err = json.Unmarshal(tb.Bytes(), &types)
	if err != nil {
		log.Printf("Unmarshall Entity Types Error: %s", err.Error())
	}

	typedTypes := make([]entity.EntityType, len(types))
	for index, jType := range types {
		mapstructure.Decode(jType, &typedTypes[index])
	}

	context.JSON(200, typedTypes)
}*/

/*func GetEntityType(context *gin.Context, ac *ActionContext) {

	typeId = context.Param("typeId")

	var tb bytes.Buffer
	err := ac.Template.ExecuteTemplate(&tb, ac.EntityName, TypeTemplateData{})
	if err != nil {
		log.Printf("Entity Type Error: %s", err.Error())
	}

	var types []map[string]interface{}
	err = json.Unmarshal(tb.Bytes(), &types)
	if err != nil {
		log.Printf("Unmarshall Entity Types Error: %s", err.Error())
	}

	var typedType entity.EntityType
	for _, jType := range types {
		id := fmt.Sprint(jType["id"])
		if id == typeId {
			mapstructure.Decode(jType, &typedType)
			break
		}
	}

	context.JSON(200, typedType)
}*/

func GetEntities(context *gin.Context, ac *ActionContext) {
	query := context.Param("queryName")
	id, err := uuid.Parse(query)
	if err != nil {
		typeId := context.Param("typeId")
		allAttributes := make([]entity.EntityAttribute, 0)
		if typeId != "" {
			objType := ac.TypeManager.Load(context.Param("typeId"), "default")
			var entType entity.EntityType
			mapstructure.Decode(objType, &entType)
			for _, attribute := range entType.Attributes {
				flatAttributes := entity.FlattenEntityAttribute(attribute)
				for _, flatAttribute := range flatAttributes {
					allAttributes = append(allAttributes, flatAttribute)
				}
			}
		}
		data := entity.EntityFinderDataBag{
			Query:      context.Request.URL.Query(),
			Attributes: allAttributes,
		}
		entities := ac.EntityManager.Find("default", query, &data)
		context.JSON(200, entities)
	} else {
		ent := ac.EntityManager.Load(id.String(), "default")
		context.JSON(200, ent)
	}
}

func CreateEntity(context *gin.Context, ac *ActionContext) {
	log.Print("CreateEntity 1")
	var e map[string]interface{}
	body, err := ioutil.ReadAll(context.Request.Body)
	if err != nil {
		log.Printf("Error json binding: %s", err.Error())
	}
	log.Print("CreateEntity 2")
	json.Unmarshal(body, &e)
	log.Print("CreateEntity 3")
	newEntity, err := ac.EntityManager.Create(e)
	if err != nil {
		log.Print("CreateEntity 4 %s", err.Error())
		context.JSON(500, err)
		return
	}
	log.Print("CreateEntity 5")
	context.JSON(200, newEntity)
}

func DeclareAction(action ActionFunc, ac ActionContext) gin.HandlerFunc {
	log.Print("DeclareAction 1")
	return func(context *gin.Context) {
		log.Print("DeclareAction 2")
		entityName := context.Param("entityName")
		pluralName := inflector.Pluralize(entityName)
		singularName := inflector.Singularize(entityName)
		ac.EntityName = singularName
		ac.TypeManager = entity.NewEntityTypeManager(entity.DefaultManagerConfig{
			SingularName: "type",
			PluralName:   "typyes",
			Index:        "classified_types",
			EsClient:     ac.EsClient,
			Session:      ac.Session,
			Lambda:       ac.Lambda,
			Template:     ac.Template,
			UserId:       utils.GetSubject(context),
		})
		if singularName == "type" {
			log.Print("DeclareAction 3")
			ac.EntityManager = ac.TypeManager
		} else {
			log.Print("DeclareAction 4")
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
		log.Print("DeclareAction 5")
		action(context, &ac)
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

	t, err := template.ParseFiles("api/entity/types.json.tmpl", "api/entity/queries.json.tmpl")
	if err != nil {
		log.Printf("Error: %s", err.Error())
	}

	actionContext := ActionContext{
		EsClient: esClient,
		Session:  sess,
		Lambda:   lClient,
		Template: t,
	}

	r := gin.Default()
	//r.GET("/entity/:entityName/types", DeclareAction(GetEntityTypes, actionContext))
	//r.GET("/entity/type/:typeId", DeclareAction(GetEntityType, actionContext))
	r.GET("/entity/:entityName", DeclareAction(GetEntities, actionContext))
	r.GET("/entity/:entityName/:queryName", DeclareAction(GetEntities, actionContext))
	r.POST("/entity/:entityName", DeclareAction(CreateEntity, actionContext))
	// r.GET("/entity/:entityName/entities", DeclareAction(CreateEntity, actionContext))

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
