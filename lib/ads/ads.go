package ads

import (
	attr "goclassifieds/lib/attr"
	"goclassifieds/lib/entity"
	vocab "goclassifieds/lib/vocab"
	"strconv"
	"strings"

	session "github.com/aws/aws-sdk-go/aws/session"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
)

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
	Id          string                `json:"id"`
	AdType      AdTypes               `form:"adType" json:"adType" binding:"required"`
	Status      AdStatuses            `form:"status" json:"status"`
	Title       string                `form:"title" json:"title" binding:"required"`
	Description string                `form:"description" json:"description" binding:"required"`
	Location    [2]float64            `form:"location[]" json:"location" binding:"required"`
	UserId      string                `form:"userId" json:"userId"`
	ProfileId   string                `form:"profileId" json:"profileId"`
	CityDisplay string                `form:"cityDisplay" json:"cityDisplay" binding:"required"`
	Images      []AdImage             `form:"images[]" json:"images"`
	Attributes  []attr.AttributeValue `form:"attributes[]" json:"attributes"`
	FeatureSets []vocab.Vocabulary    `form:"featureSets[]" json:"featureSets"`
}

type AdImage struct {
	Id     string `form:"id" json:"id" binding:"required"`
	Path   string `form:"path" json:"path" binding:"required"`
	Weight int    `form:"weight" json:"weight" binding:"required"`
}

func CreateAdManager(esClient *elasticsearch7.Client, session *session.Session) entity.EntityManager {
	return entity.EntityManager{
		Config: entity.EntityConfig{
			SingularName: "ad",
			PluralName:   "ads",
			IdKey:        "id",
		},
		Loaders: map[string]entity.Loader{
			"s3": entity.S3LoaderAdaptor{
				Config: entity.S3AdaptorConfig{
					Session: session,
					Bucket:  "classifieds-ui-dev",
					Prefix:  "ads/",
				},
			},
		},
		Storages: map[string]entity.Storage{
			"s3": entity.S3StorageAdaptor{
				Config: entity.S3AdaptorConfig{
					Session: session,
					Bucket:  "classifieds-ui-dev",
					Prefix:  "ads/",
				},
			},
			"elastic": entity.ElasticStorageAdaptor{
				Config: entity.ElasticAdaptorConfig{
					Index:  "classified_ads",
					Client: esClient,
				},
			},
		},
	}
}

func BuildAdsSearchQuery(req *AdListitemsRequest) map[string]interface{} {
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
