package main

import (
	"bytes"
	"context"
	"encoding/json"
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

func (c *AdsController) GetAdListItems(context *gin.Context) {
	/*votePack, err := c.Database.GetVotePack("c5039ecd-e774-4c19-a2b9-600c2134784d")
	if err != nil{
			context.String(404, "Votepack Not Found")
	}*/
	var (
		r map[string]interface{}
	)
	FetchAds(c.EsClient, &r)
	for _, hit := range r["hits"].(map[string]interface{})["hits"].([]interface{}) {
		log.Printf(" * ID=%s, %s", hit.(map[string]interface{})["_id"], hit.(map[string]interface{})["_source"])
	}
	context.JSON(200, r)
}

func FetchAds(esClient *elasticsearch7.Client, r *map[string]interface{}) {
	var buf bytes.Buffer
	// Perform the search request.
	res, err := esClient.Search(
		esClient.Search.WithContext(context.Background()),
		esClient.Search.WithIndex("classified_ads"),
		esClient.Search.WithBody(&buf),
		// esClient.Search.WithTrackTotalHits(true),
		esClient.Search.WithPretty(),
	)
	if err != nil {
		log.Fatalf("Error getting response: %s", err)
	}
	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			log.Fatalf("Error parsing the response body: %s", err)
		} else {
			// Print the response status and error information.
			log.Fatalf("[%s] %s: %s",
				res.Status(),
				e["error"].(map[string]interface{})["type"],
				e["error"].(map[string]interface{})["reason"],
			)
		}
	}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		log.Fatalf("Error parsing the response body: %s", err)
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

	adsController := AdsController{EsClient: esClient}

	r := gin.Default()
	r.GET("/ads/adlistitems", adsController.GetAdListItems)

	ginLambda = ginadapter.New(r)
}

func Handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// If no name is provided in the HTTP request body, throw an error
	return ginLambda.ProxyWithContext(ctx, req)
}

func main() {
	lambda.Start(Handler)
}
