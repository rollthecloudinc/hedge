package main

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/gin"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
	"github.com/gin-gonic/gin"
)

var ginLambda *ginadapter.GinLambda

type AdsController struct {
	EsClient *elasticsearch7.Client
}

func (c *AdsController) GetAds(context *gin.Context) {
	/*votePack, err := c.Database.GetVotePack("c5039ecd-e774-4c19-a2b9-600c2134784d")
	if err != nil{
			context.String(404, "Votepack Not Found")
	}*/
	context.JSON(200, gin.H{
		"message": "pong",
	})
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

	adsController := AdsController{EsClient: esClient}

	r := gin.Default()
	r.GET("/ads", adsController.GetAds)

	ginLambda = ginadapter.New(r)
}

func Handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// If no name is provided in the HTTP request body, throw an error
	return ginLambda.ProxyWithContext(ctx, req)
}

func main() {
	lambda.Start(Handler)
}
