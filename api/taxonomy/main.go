package main

import (
	"context"
	"fmt"
	"goclassifieds/lib/entity"
	"goclassifieds/lib/es"
	"goclassifieds/lib/utils"
	"goclassifieds/lib/vocab"
	"log"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/session"
	ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/gin"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
)

var ginLambda *ginadapter.GinLambda

type ActionFunc func(context *gin.Context, ac *ActionContext)

type ActionContext struct {
	EsClient     *elasticsearch7.Client
	Session      *session.Session
	VocabManager entity.Manager
}

func GetVocabListItems(context *gin.Context, ac *ActionContext) {
	userId := utils.GetSubject(context)
	query := vocab.BuildVocabSearchQuery(userId)
	hits := es.ExecuteSearch(ac.EsClient, &query, "classified_vocabularies")
	vocabs := make([]vocab.Vocabulary, len(hits))
	for index, hit := range hits {
		mapstructure.Decode(hit.(map[string]interface{})["_source"], &vocabs[index])
	}
	context.JSON(200, vocabs)
}

func CreateVocab(context *gin.Context, ac *ActionContext) {
	var obj vocab.Vocabulary
	if err := context.ShouldBind(&obj); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	obj.Id = utils.GenerateId()
	obj.UserId = utils.GetSubject(context)
	newEntity, _ := vocab.ToEntity(&obj)
	ac.VocabManager.Save(newEntity, "s3")
	context.JSON(200, newEntity)
}

func UpdateVocab(context *gin.Context, ac *ActionContext) {
	// log.Printf("UpdateVocab : %s", context.Param("id"))
	canWrite, oldEntity := ac.VocabManager.Allow(context.Param("id"), "write", "s3")
	if !canWrite {
		context.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to write to vocabulary."})
		return
	}
	var obj vocab.Vocabulary
	if err := context.ShouldBind(&obj); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	obj.Id = fmt.Sprint(oldEntity["id"])
	obj.UserId = fmt.Sprint(oldEntity["userId"])
	newEntity, _ := vocab.ToEntity(&obj)
	ac.VocabManager.Save(newEntity, "s3")
	context.JSON(200, newEntity)
}

func DeclareAction(action ActionFunc, ac ActionContext) gin.HandlerFunc {
	return func(context *gin.Context) {
		userId := utils.GetSubject(context)
		ac.VocabManager = vocab.CreateVocabManager(ac.EsClient, ac.Session, userId)
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
	// vocabManager := vocab.CreateVocabManager(esClient, sess)

	actionContext := ActionContext{
		EsClient: esClient,
		Session:  sess,
	}

	// taxonomyController := TaxonomyController{VocabManager: &vocabManager, EsClient: esClient, Session: sess}

	r := gin.Default()
	r.GET("/taxonomy/vocabularylistitems", DeclareAction(GetVocabListItems, actionContext))
	r.POST("/taxonomy/vocabulary", DeclareAction(CreateVocab, actionContext))
	r.PUT("/taxonomy/vocabulary/:id", DeclareAction(UpdateVocab, actionContext))

	ginLambda = ginadapter.New(r)
}

func Handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// If no name is provided in the HTTP request body, throw an error
	/*var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(req); err != nil {
		log.Fatalf("Error encoding request: %s", err)
	}
	log.Printf("request: %s", buf)*/

	res, err := ginLambda.ProxyWithContext(ctx, req)
	res.Headers["Access-Control-Allow-Origin"] = "*"
	return res, err
}

func main() {
	lambda.Start(Handler)
}
