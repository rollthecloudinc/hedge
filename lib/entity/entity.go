package entity

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	session "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	esapi "github.com/elastic/go-elasticsearch/esapi"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
)

type EntityConfig struct {
	SingularName string
	PluralName   string
	IdKey        string
}

type EntityManager struct {
	Config   EntityConfig
	Loaders  map[string]Loader
	Storages map[string]Storage
}

type Manager interface {
	Save(entity map[string]interface{}, storage string)
}

type Storage interface {
	Store(id string, entity map[string]interface{})
}

type Loader interface {
	Load(id string, loader string)
}

type S3AdaptorConfig struct {
	Bucket  string
	Prefix  string
	Session *session.Session
}

type ElasticAdaptorConfig struct {
	Client *elasticsearch7.Client
	Index  string
}

type S3LoaderAdaptor struct {
	Config S3AdaptorConfig
}

type S3StorageAdaptor struct {
	Config S3AdaptorConfig
}

type ElasticStorageAdaptor struct {
	Config ElasticAdaptorConfig
}

func (m EntityManager) Save(entity map[string]interface{}, storage string) {
	id := fmt.Sprint(entity[m.Config.IdKey])
	m.Storages[storage].Store(id, entity)
}

func (s S3StorageAdaptor) Store(id string, entity map[string]interface{}) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&entity); err != nil {
		log.Fatal(err)
	}
	var buf2 bytes.Buffer
	gz := gzip.NewWriter(&buf2)
	if _, err := gz.Write(buf.Bytes()); err != nil {
		log.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		log.Fatal(err)
	}
	uploader := s3manager.NewUploader(s.Config.Session)
	_, err := uploader.Upload(&s3manager.UploadInput{
		Bucket:          aws.String(s.Config.Bucket),
		Key:             aws.String(s.Config.Prefix + "" + id + ".json.gz"),
		Body:            &buf2,
		ContentType:     aws.String("application/json"),
		ContentEncoding: aws.String("gzip"),
	})
	if err != nil {
		log.Fatal(err)
	}
}

func (s ElasticStorageAdaptor) Store(id string, entity map[string]interface{}) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(entity); err != nil {
		log.Fatalf("Error encoding body: %s", err)
	}
	req := esapi.IndexRequest{
		Index:      s.Config.Index,
		DocumentID: id,
		Body:       &buf,
		Refresh:    "true",
	}
	_, err := req.Do(context.Background(), s.Config.Client)
	if err != nil {
		log.Fatalf("Error getting response: %s", err)
	}
}
