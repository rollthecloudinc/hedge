package main

import (
	"context"
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
