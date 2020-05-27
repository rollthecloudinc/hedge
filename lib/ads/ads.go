package ads

import (
	"bytes"
	"encoding/json"
	attr "goclassifieds/lib/attr"
	vocab "goclassifieds/lib/vocab"
	"log"
	"strconv"
	"strings"
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

type AdType struct {
	Id         AdTypes           `form:"id" json:"id" binding:"required"`
	Name       string            `form:"name" json:"name" binding:"required"`
	Attributes []AdTypeAttribute `form:"attributes[]" json:"attributes" binding:"required"`
	Filters    []AdTypeAttribute `form:"filters[]" json:"filters" binding:"required"`
}

type AdTypeAttribute struct {
	Name       string              `form:"name" json:"name" binding:"required"`
	Type       attr.AttributeTypes `form:"type" json:"type" binding:"required"`
	Label      string              `form:"label" json:"label" binding:"required"`
	Required   bool                `form:"required" json:"required" binding:"required"`
	Widget     string              `form:"widget" json:"widget" binding:"required"`
	Attributes []AdTypeAttribute   `form:"attributes[]" json:"attributes" binding:"required"`
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

func GetAdTypes() []AdType {
	return []AdType{
		{
			Id:         General,
			Name:       "general",
			Attributes: []AdTypeAttribute{},
			Filters:    []AdTypeAttribute{},
		},
		{
			Id:   RealEstate,
			Name: "realestate",
			Attributes: []AdTypeAttribute{
				{
					Name:       "price",
					Type:       attr.Number,
					Label:      "Asking Price",
					Required:   true,
					Widget:     "text",
					Attributes: []AdTypeAttribute{},
				},
				{
					Name:       "beds",
					Type:       attr.Number,
					Label:      "Beds",
					Required:   true,
					Widget:     "text",
					Attributes: []AdTypeAttribute{},
				},
				{
					Name:       "baths",
					Type:       attr.Number,
					Label:      "Baths",
					Required:   true,
					Widget:     "text",
					Attributes: []AdTypeAttribute{},
				},
				{
					Name:       "sqft",
					Type:       attr.Number,
					Label:      "Sqft",
					Required:   true,
					Widget:     "text",
					Attributes: []AdTypeAttribute{},
				},
			},
			Filters: []AdTypeAttribute{},
		},
		{
			Id:         Rental,
			Name:       "rentals",
			Attributes: []AdTypeAttribute{},
			Filters:    []AdTypeAttribute{},
		},
		{
			Id:   Auto,
			Name: "auto",
			Attributes: []AdTypeAttribute{
				{
					Name:     "ymm",
					Type:     attr.Complex,
					Label:    "YMM",
					Required: false,
					Widget:   "ymm_selector",
					Attributes: []AdTypeAttribute{
						{
							Name:       "year",
							Type:       attr.Number,
							Label:      "Year",
							Required:   false,
							Widget:     "text",
							Attributes: []AdTypeAttribute{},
						},
						{
							Name:       "make",
							Type:       attr.Text,
							Label:      "Make",
							Required:   false,
							Widget:     "text",
							Attributes: []AdTypeAttribute{},
						},
						{
							Name:       "model",
							Type:       attr.Text,
							Label:      "Model",
							Required:   false,
							Widget:     "text",
							Attributes: []AdTypeAttribute{},
						},
					},
				},
			},
			Filters: []AdTypeAttribute{},
		},
	}
}

func GetAdType(adTypeId AdTypes) AdType {
	adTypes := GetAdTypes()
	var match AdType
	for _, adType := range adTypes {
		if adType.Id == adTypeId {
			match = adType
		}
	}
	return match
}

func MapAdType(s string) AdTypes {
	switch s {
	case "0":
		return General
	case "1":
		return RealEstate
	case "2":
		return Rental
	case "3":
		return Auto
	default:
		return -1
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
