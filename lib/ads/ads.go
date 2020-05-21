package ads

import (
	"bytes"
	"context"
	"encoding/json"
	attr "goclassifieds/lib/attr"
	vocab "goclassifieds/lib/vocab"
	"log"

	esapi "github.com/elastic/go-elasticsearch/esapi"
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

func IndexAd(esClient *elasticsearch7.Client, ad *Ad) *esapi.Response {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(ad); err != nil {
		log.Fatalf("Error encoding body: %s", err)
	}
	req := esapi.IndexRequest{
		Index:      "classified_ads",
		DocumentID: ad.Id,
		Body:       &buf,
		Refresh:    "true",
	}
	res, err := req.Do(context.Background(), esClient)
	if err != nil {
		log.Fatalf("Error getting response: %s", err)
	}
	return res
}
