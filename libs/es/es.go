package es

import (
	"bytes"
	"context"
	"encoding/json"
	"log"

	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
)

func ExecuteSearch(esClient *elasticsearch7.Client, query *map[string]interface{}, index string) []interface{} {
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
	return r["hits"].(map[string]interface{})["hits"].([]interface{})
	/*var docs []interface{}
	for _, hit := range r["hits"].(map[string]interface{})["hits"].([]interface{}) {
		docs = append(docs, hit)
	}
	return docs*/
}
