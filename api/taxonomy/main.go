package main

import (
	"bytes"
	"context"
	"encoding/json"
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

type TaxonomyController struct {
	EsClient     *elasticsearch7.Client
	Session      *session.Session
	VocabManager entity.Manager
}

func (c *TaxonomyController) GetVocabListItems(context *gin.Context) {
	userId := utils.GetSubject(context)
	query := vocab.BuildVocabSearchQuery(userId)
	hits := es.ExecuteSearch(c.EsClient, &query, "classified_vocabularies")
	vocabs := make([]vocab.Vocabulary, len(hits))
	for index, hit := range hits {
		mapstructure.Decode(hit.(map[string]interface{})["_source"], &vocabs[index])
	}
	context.JSON(200, vocabs)
}

func (c *TaxonomyController) CreateVocab(context *gin.Context) {
	var vocab vocab.Vocabulary
	if err := context.ShouldBind(&vocab); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	vocab.Id = utils.GenerateId()
	vocab.UserId = utils.GetSubject(context)
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(vocab); err != nil {
		log.Fatalf("Error encoding query: %s", err)
	}
	vocabJson, err := json.Marshal(vocab)
	if err != nil {
		return
	}
	var vocabMap map[string]interface{}
	err = json.Unmarshal(vocabJson, &vocabMap)
	c.VocabManager.Save(vocabMap, "s3")
	context.JSON(200, vocab)
}

func (c *TaxonomyController) UpdateVocab(context *gin.Context) {
	// log.Printf("UpdateVocab : %s", context.Param("id"))
	canWrite, e := c.VocabManager.Allow(context.Param("id"), "write", "s3")
	if !canWrite {
		context.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to write to vocabulary."})
		return
	}
	var vocab vocab.Vocabulary
	if err := context.ShouldBind(&vocab); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	vocab.Id = fmt.Sprint(e["id"])
	vocab.UserId = fmt.Sprint(e["userId"])
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(vocab); err != nil {
		log.Fatalf("Error encoding query: %s", err)
	}
	vocabJson, err := json.Marshal(vocab)
	if err != nil {
		return
	}
	var vocabMap map[string]interface{}
	err = json.Unmarshal(vocabJson, &vocabMap)
	c.VocabManager.Save(vocabMap, "s3")
	context.JSON(200, vocab)
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

	vocabManager := vocab.CreateVocabManager(esClient, sess)

	taxonomyController := TaxonomyController{VocabManager: &vocabManager, EsClient: esClient, Session: sess}

	r := gin.Default()
	r.GET("/taxonomy/vocabularylistitems", taxonomyController.GetVocabListItems)
	r.POST("/taxonomy/vocabulary", taxonomyController.CreateVocab)
	r.PUT("/taxonomy/vocabulary/:id", taxonomyController.UpdateVocab)

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
