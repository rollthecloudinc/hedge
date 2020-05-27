package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	ads "goclassifieds/lib/ads"
	entity "goclassifieds/lib/entity"
	es "goclassifieds/lib/es"
	utils "goclassifieds/lib/utils"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	session "github.com/aws/aws-sdk-go/aws/session"
	ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/gin"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
)

var ginLambda *ginadapter.GinLambda

type ActionFunc func(context *gin.Context, ac *ActionContext)

type ActionContext struct {
	EsClient   *elasticsearch7.Client
	Session    *session.Session
	AdsManager entity.Manager
}

func GetAdListItems(context *gin.Context, ac *ActionContext) {
	var req ads.AdListitemsRequest
	if err := context.ShouldBind(&req); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	query := ads.BuildAdsSearchQuery(&req)
	hits := es.ExecuteSearch(ac.EsClient, &query, "classified_ads")
	ads := make([]ads.Ad, len(hits))
	for index, hit := range hits {
		mapstructure.Decode(hit.(map[string]interface{})["_source"], &ads[index])
	}
	context.JSON(200, ads)
}

func CreateAd(context *gin.Context, ac *ActionContext) {
	var obj ads.Ad
	if err := context.ShouldBind(&obj); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	obj.Id = utils.GenerateId()
	obj.Status = ads.Submitted // @todo: Enums not being validated :(
	obj.UserId = utils.GetSubject(context)
	newEntity, _ := ads.ToEntity(&obj)
	ac.AdsManager.Save(newEntity, "s3")
	context.JSON(200, newEntity)
}

func GetAdTypes(context *gin.Context, ac *ActionContext) {
	adTypes := ads.GetAdTypes()
	jsonData, _ := json.Marshal(adTypes)
	var entities []map[string]interface{}
	json.Unmarshal(jsonData, &entities)
	context.JSON(200, entities)
}

func GetAdType(context *gin.Context, ac *ActionContext) {
	adTypeId, _ := context.Params.Get("adTypeId")
	adType := ads.GetAdType(ads.MapAdType(adTypeId))
	jsonData, _ := json.Marshal(adType)
	var entity map[string]interface{}
	json.Unmarshal(jsonData, &entity)
	context.JSON(200, entity)
}

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
		ac.AdsManager = entity.NewDefaultManager(entity.DefaultManagerConfig{
			SingularName: "ad",
			PluralName:   "ads",
			Index:        "classified_ads",
			EsClient:     ac.EsClient,
			Session:      ac.Session,
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

	actionContext := ActionContext{
		EsClient: esClient,
		Session:  sess,
	}

	r := gin.Default()
	r.GET("/ads/adlistitems", DeclareAction(GetAdListItems, actionContext))
	r.POST("/ads/ad", DeclareAction(CreateAd, actionContext))
	r.GET("/ads/adtypes", DeclareAction(GetAdTypes, actionContext))
	r.GET("/ads/adtype/:adTypeId", DeclareAction(GetAdType, actionContext))

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
