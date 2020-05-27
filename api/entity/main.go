package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/mitchellh/mapstructure"
	"github.com/tangzero/inflector"
)

var ginLambda *ginadapter.GinLambda

type ActionFunc func(context *gin.Context, ac *ActionContext)

type ActionContext struct {
	EsClient      *elasticsearch7.Client
	Session       *session.Session
	Lambda        *lambda2.Lambda
	EntityManager entity.Manager
	EntityName    string
	EntityType    string
	Template      *template.Template
}

type TypeTemplateData struct {
}

func GetEntityTypes(context *gin.Context, ac *ActionContext) {

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
}

func GetEntityType(context *gin.Context, ac *ActionContext) {

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
		name := fmt.Sprint(jType["name"])
		if name == ac.EntityType {
			mapstructure.Decode(jType, &typedType)
			break
		}
	}

	context.JSON(200, typedType)
}

func CreateEntity(context *gin.Context, ac *ActionContext) {
	var e map[string]interface{}
	body, err := ioutil.ReadAll(context.Request.Body)
	if err != nil {
		log.Printf("Error json binding: %s", err.Error())
	}
	json.Unmarshal(body, &e)
	newEntity, _ := ac.EntityManager.Create(e)
	context.JSON(200, newEntity)
}

/*func GetAdType(context *gin.Context, ac *ActionContext) {
	adTypeId, _ := context.Params.Get("adTypeId")
	adType := ads.GetAdType(ads.MapAdType(adTypeId))
	jsonData, _ := json.Marshal(adType)
	var entity map[string]interface{}
	json.Unmarshal(jsonData, &entity)
	context.JSON(200, entity)
}*/

/*func BeforeAdSave(context *gin.Context, ac *ActionContext) entity.EntityHook {
	return func(entity map[string]interface{}) (bool, error) {
		log.Printf("Entity Hook Activated")
		var obj ads.Ad
		if err := context.ShouldBind(&obj); err != nil {
			context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return true, err
		}
		entity["Id"] = utils.GenerateId()
		entity["Status"] = ads.Submitted // @todo: Enums not being validated :(
		entity["UserId"] = utils.GetSubject(context)
		return false, nil
	}
}*/

func DeclareAction(action ActionFunc, ac ActionContext) gin.HandlerFunc {
	return func(context *gin.Context) {
		ac.EntityName = context.Param("entityName")
		ac.EntityType = context.Param("entityType")
		ac.EntityManager = entity.NewDefaultManager(entity.DefaultManagerConfig{
			SingularName: ac.EntityName,
			PluralName:   inflector.Pluralize(ac.EntityName),
			Index:        "classified_" + inflector.Pluralize(ac.EntityName),
			EsClient:     ac.EsClient,
			Session:      ac.Session,
			Lambda:       ac.Lambda,
			UserId:       utils.GetSubject(context),
			// BeforeSave: BeforeAdSave(context, &ac),
		})
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

	t, err := template.ParseFiles("api/entity/types.json.tmpl")
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
	r.GET("/entity/:entityName/types", DeclareAction(GetEntityTypes, actionContext))
	r.GET("/entity/:entityName/type/:entityType", DeclareAction(GetEntityType, actionContext))
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
