package main

import (
	"context"
	"goclassifieds/lib/ads"
	"goclassifieds/lib/attr"
	"goclassifieds/lib/cc"
	"goclassifieds/lib/entity"
	"goclassifieds/lib/vocab"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	session "github.com/aws/aws-sdk-go/aws/session"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
	opensearch "github.com/opensearch-project/opensearch-go"

	"github.com/mitchellh/mapstructure"
	"github.com/tangzero/inflector"
)

func handler(ctx context.Context, s3Event events.S3Event) {

	elasticCfg := elasticsearch7.Config{
		Addresses: []string{os.Getenv("ELASTIC_URL")},
	}

	esClient, err := elasticsearch7.NewClient(elasticCfg)
	if err != nil {

	}

	opensearchCfg := opensearch.Config{
		Addresses: []string{os.Getenv("ELASTIC_URL")},
	}

	osClient, err := opensearch.NewClient(opensearchCfg)
	if err != nil {

	}

	sess := session.Must(session.NewSession())

	for _, record := range s3Event.Records {

		pieces := strings.Split(record.S3.Object.Key, "/")

		pluralName := inflector.Pluralize(pieces[0])
		singularName := inflector.Singularize(pieces[0])

		entityManager := entity.NewDefaultManager(entity.DefaultManagerConfig{
			SingularName: singularName,
			PluralName:   pluralName,
			Index:        "classified_" + pluralName,
			EsClient:     esClient,
			OsClient:     osClient,
			Session:      sess,
			UserId:       "",
			Stage:        os.Getenv("STAGE"),
			BucketName:   os.Getenv("BUCKET_NAME"),
		})

		id := pieces[1][0 : len(pieces[1])-8]
		ent := entityManager.Load(id, "default")

		if singularName == "ad" {
			ent = IndexAd(ent)
		} else if singularName == "panelpage" {
			ent = IndexPanelPage(ent)
		}

		entityManager.Save(ent, "elastic")
	}
}

func IndexAd(obj map[string]interface{}) map[string]interface{} {

	var item ads.Ad
	mapstructure.Decode(obj, &item)

	allAttrValues := make([]attr.AttributeValue, 0)
	for _, attrValue := range item.Attributes {
		attributesFlattened := attr.FlattenAttributeValue(attrValue)
		for _, flatAttr := range attributesFlattened {
			attr.FinalizeAttributeValue(&flatAttr)
			allAttrValues = append(allAttrValues, flatAttr)
		}
	}
	item.Attributes = allAttrValues

	for index, featureSet := range item.FeatureSets {
		allFeatureTerms := make([]vocab.Term, 0)
		for _, term := range featureSet.Terms {
			flatTerms := vocab.FlattenTerm(term, true)
			for _, flatTerm := range flatTerms {
				allFeatureTerms = append(allFeatureTerms, flatTerm)
			}
		}
		item.FeatureSets[index].Terms = allFeatureTerms
	}

	ent, _ := ads.ToEntity(&item)
	return ent

}

func IndexPanelPage(obj map[string]interface{}) map[string]interface{} {

	var item cc.PanelPage
	mapstructure.Decode(obj, &item)

	item.GridItems = make([]cc.GridItem, 0)
	item.Contexts = make([]cc.InlineContext, 0)
	item.Panels = make([]cc.Panel, 0)
	item.RowSettings = make([]cc.LayoutSetting, 0)

	ent, _ := cc.ToPanelPageEntity(&item)
	return ent

}

func main() {
	lambda.Start(handler)
}
