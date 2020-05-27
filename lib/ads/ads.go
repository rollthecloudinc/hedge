package ads

import (
	"bytes"
	"encoding/json"
	"goclassifieds/lib/attr"
	"goclassifieds/lib/vocab"
	"log"
	"strconv"
	"strings"
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
	TypeId       string   `form:"typeId" binding:"required"`
	SearchString string   `form:"searchString"`
	Location     string   `form:"location"`
	Features     []string `form:"features[]"`
	Page         int      `form:"page"`
}

type Ad struct {
	Id          string                `json:"id" validate:"required"`
	TypeId      string                `form:"typeId" json:"typeId" binding:"typeId" validate:"required"`
	Status      *AdStatuses           `form:"status" json:"status" validate:"required"`
	Title       string                `form:"title" json:"title" binding:"required" validate:"required"`
	Description string                `form:"description" json:"description" binding:"required" validate:"required"`
	Location    [2]float64            `form:"location[]" json:"location" binding:"required" validate:"required"`
	UserId      string                `form:"userId" json:"userId" validate:"required"`
	ProfileId   string                `form:"profileId" json:"profileId"`
	CityDisplay string                `form:"cityDisplay" json:"cityDisplay" binding:"required" validate:"required"`
	Images      []AdImage             `form:"images[]" json:"images" validate:"dive"`
	Attributes  []attr.AttributeValue `form:"attributes[]" json:"attributes" validate:"dive"`
	FeatureSets []vocab.Vocabulary    `form:"featureSets[]" json:"featureSets" validate:"dive"`
}

type AdImage struct {
	Id     string `form:"id" json:"id" binding:"required" validate:"required"`
	Path   string `form:"path" json:"path" binding:"required" validate:"required"`
	Weight int    `form:"weight" json:"weight" binding:"required" validate:"required"`
}

func ToEntity(ad *Ad) (map[string]interface{}, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(ad); err != nil {
		log.Fatalf("Error encoding query: %s", err)
	}
	jsonData, err := json.Marshal(ad)
	if err != nil {
		return nil, err
	}
	var entity map[string]interface{}
	err = json.Unmarshal(jsonData, &entity)
	return entity, nil
}

func BuildAdsSearchQuery(req *AdListitemsRequest) map[string]interface{} {
	filterMust := []interface{}{
		map[string]interface{}{
			"term": map[string]interface{}{
				"typeId": map[string]interface{}{
					"value": req.TypeId,
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
