package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"goclassifieds/libs/attr"
	es "goclassifieds/libs/es"
	utils "goclassifieds/libs/utils"
	"goclassifieds/libs/vocab"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	session "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/gin"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
	"github.com/gin-gonic/gin"
)

var ginLambda *ginadapter.GinLambda

type AdTypes int32

const (
	General AdTypes = iota
	RealEstate
	Rental
	Auto
	Job
)

type AdStatuses int32

const (
	Submitted AdStatuses = iota
	Approved
	Rejected
	Expired
	Deleted
)

type AdListitemsRequest struct {
	AdType       int      `form:"adType" binding:"required"`
	SearchString string   `form:"searchString"`
	Location     string   `form:"location"`
	Features     []string `form:"features[]"`
	Page         int      `form:"page"`
}

type Ad struct {
	Id          string
	AdType      AdTypes               `form:"adType" binding:"required"`
	Status      AdStatuses            `form:"status"`
	Title       string                `form:"title" binding:"required"`
	Description string                `form:"description" binding:"required"`
	Location    [2]float64            `form:"location[]" binding:"required"`
	UserId      string                `form:"userId"`
	ProfileId   string                `form:"profileId"`
	CityDisplay string                `form:"cityDisplay" binding:"required"`
	Images      []AdImage             `form:"images[]"`
	Attributes  []attr.AttributeValue `form:"attributes[]"`
	FeatureSets []vocab.Vocabulary    `form:"featureSets[]"`
}

type AdImage struct {
	Id     string `form:"id" binding:"required"`
	Path   string `form:"path" binding:"required"`
	Weight int    `form:"weight" binding:"required"`
}

type AdsController struct {
	EsClient *elasticsearch7.Client
	Session  *session.Session
}

func (c *AdsController) GetAdListItems(context *gin.Context) {
	var req AdListitemsRequest
	if err := context.ShouldBind(&req); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	query := buildAdsSearchQuery(&req)
	ads := es.ExecuteSearch(c.EsClient, &query, "classified_ads")
	for _, ad := range ads {
		log.Printf(" * ID=%s, %s", ad.(map[string]interface{})["_id"], ad.(map[string]interface{})["_source"])
	}
	context.JSON(200, ads)
}

func (c *AdsController) CreateAd(context *gin.Context) {
	var ad Ad
	if err := context.ShouldBind(&ad); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ad.Id = utils.GenerateId()
	ad.Status = Submitted // @todo: Enums not being validated :(
	ad.UserId = utils.GetSubject(context)
	storeAd(c.Session, &ad)
	context.JSON(200, ad)
}

func storeAd(sess *session.Session, ad *Ad) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(ad); err != nil {
		log.Fatalf("Error encoding query: %s", err)
	}
	uploader := s3manager.NewUploader(sess)
	_, err := uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String("classifieds-ui-dev"),
		Key:         aws.String("ads/" + ad.Id + ".json.gz"),
		Body:        &buf,
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		log.Printf("s3 upload error: %s", err)
	}
}

func buildAdsSearchQuery(req *AdListitemsRequest) map[string]interface{} {
	filterMust := []interface{}{
		map[string]interface{}{
			"term": map[string]interface{}{
				"adType": map[string]interface{}{
					"value": req.AdType,
				},
			},
		},
	}

	if req.Location != "" {
		cords := strings.Split(req.Location, ",")
		lat, e := strconv.ParseFloat(cords[1], 64)
		if e != nil {

		}
		lon, e := strconv.ParseFloat(cords[0], 64)
		if e != nil {

		}
		geoFilter := map[string]interface{}{
			"geo_distance": map[string]interface{}{
				"validation_method": "ignore_malformed",
				"distance":          "10m",
				"distance_type":     "arc",
				"location": map[string]interface{}{
					"lat": lat,
					"lon": lon,
				},
			},
		}
		filterMust = append(filterMust, geoFilter)
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []interface{}{
					map[string]interface{}{
						"bool": map[string]interface{}{
							"must": filterMust,
						},
					},
				},
			},
		},
	}

	if req.SearchString != "" || req.Features != nil {

		var matchMust []interface{}

		if req.SearchString != "" {
			matchSearchString := map[string]interface{}{
				"match": map[string]interface{}{
					"title": map[string]interface{}{
						"query": req.SearchString,
					},
				},
			}
			matchMust = append(matchMust, matchSearchString)
		}

		if req.Features != nil {
			matchMust = buildAdFeaturesSearchQuery(matchMust, req.Features)
		}

		query["query"].(map[string]interface{})["bool"].(map[string]interface{})["must"] = matchMust

	}
	return query
}

func buildAdFeaturesSearchQuery(query []interface{}, features []string) []interface{} {
	for _, feature := range features {
		featureFilter := map[string]interface{}{
			"nested": map[string]interface{}{
				"path": "features",
				"query": map[string]interface{}{
					"bool": map[string]interface{}{
						"must": map[string]interface{}{
							"match": map[string]interface{}{
								"features.humanName": map[string]interface{}{
									"query": feature,
								},
							},
						},
					},
				},
			},
		}
		query = append(query, featureFilter)
	}
	return query
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

	adsController := AdsController{EsClient: esClient, Session: sess}

	r := gin.Default()
	r.GET("/ads/adlistitems", adsController.GetAdListItems)
	r.POST("/ads/ad", adsController.CreateAd)

	ginLambda = ginadapter.New(r)
}

func Handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// If no name is provided in the HTTP request body, throw an error
	return ginLambda.ProxyWithContext(ctx, req)
}

func main() {
	lambda.Start(Handler)
}
