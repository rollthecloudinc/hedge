package os

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"strings"
	"text/template"

	opensearch "github.com/opensearch-project/opensearch-go"
)

type SearchQueryBuilder interface {
	Build() map[string]interface{}
	GetIndex() string
	GetCollectionKey() string
}

type TemplateBuilder struct {
	Index         string
	Data          interface{}
	Template      *template.Template
	Name          string
	CollectionKey string
}

func (t TemplateBuilder) GetIndex() string {
	return t.Index
}

func (t TemplateBuilder) GetCollectionKey() string {
	return t.CollectionKey
}

func (t TemplateBuilder) Build() map[string]interface{} {
	var tb bytes.Buffer
	err := t.Template.ExecuteTemplate(&tb, t.Name, t.Data)
	if err != nil {
		log.Printf("Build Query Error: %s", err.Error())
	}

	var query map[string]interface{}
	err = json.Unmarshal(tb.Bytes(), &query)
	if err != nil {
		log.Printf("Unmarshall Query Error: %s", err.Error())
	}

	return query
}

func ExecuteQuery(esClient *opensearch.Client, builder SearchQueryBuilder) []interface{} {
	query := builder.Build()

	var qb bytes.Buffer
	if err := json.NewEncoder(&qb).Encode(query); err != nil {
		log.Fatalf("Error encoding search query: %s", err)
	}
	log.Printf("Search Query: %s", qb.String())

	return ExecuteSearch(esClient, &query, builder.GetIndex(), builder.GetCollectionKey())
}

func ExecuteSearch(esClient *opensearch.Client, query *map[string]interface{}, index string, collectionKey string) []interface{} {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		log.Fatalf("Error encoding query: %s", err)
	}
	res, err := esClient.Search(
		esClient.Search.WithContext(context.Background()),
		esClient.Search.WithIndex(index),
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
	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		log.Fatalf("Error parsing the response body: %s", err)
	}
	pieces := strings.Split(collectionKey, ".")
	target := r[pieces[0]]
	for i, piece := range pieces {
		if i > 0 {
			target = target.(map[string]interface{})[piece]
		}
	}
	return target.([]interface{})
	// return r["hits"].(map[string]interface{})["hits"].([]interface{})
	/*var docs []interface{}
	for _, hit := range r["hits"].(map[string]interface{})["hits"].([]interface{}) {
		docs = append(docs, hit)
	}
	return docs*/
}
