package entity

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"

	"goclassifieds/lib/attr"

	"github.com/aws/aws-sdk-go/aws"
	session "github.com/aws/aws-sdk-go/aws/session"
	lambda "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	s3 "github.com/aws/aws-sdk-go/service/s3"
	esapi "github.com/elastic/go-elasticsearch/esapi"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
)

type EntityHook func(enity map[string]interface{}) (bool, error)

type EntityConfig struct {
	SingularName string
	PluralName   string
	IdKey        string
}

type ValidateEntityRequest struct {
	EntityName string
	EntityType string
	UserId     string
	Entity     map[string]interface{}
}

type ValidateEntityResponse struct {
	Entity map[string]interface{}
	Valid  bool
}

type DefaultManagerConfig struct {
	EsClient     *elasticsearch7.Client
	Session      *session.Session
	Lambda       *lambda.Lambda
	UserId       string
	SingularName string
	PluralName   string
	Index        string
	BeforeSave   EntityHook
	AfterSave    EntityHook
}

type EntityManager struct {
	Config      EntityConfig
	Creator     Creator
	Loaders     map[string]Loader
	Storages    map[string]Storage
	Authorizers map[string]Authorization
	Hooks       EntityHooks
}

type EntityHooks struct {
	BeforeSave EntityHook
	AfterSave  EntityHook
}

type Manager interface {
	Create(entity map[string]interface{}) (map[string]interface{}, error)
	Update(entity map[string]interface{})
	Delete(entity map[string]interface{})
	Save(entity map[string]interface{}, storage string)
	Load(id string, loader string) map[string]interface{}
	Allow(id string, op string, loader string) (bool, map[string]interface{})
}

type Storage interface {
	Store(id string, entity map[string]interface{})
}

type Loader interface {
	Load(id string) map[string]interface{}
}

type Creator interface {
	Create(entity map[string]interface{}, m *EntityManager) (map[string]interface{}, error)
}

type Authorization interface {
	CanWrite(id string, loader Loader) (bool, map[string]interface{})
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

type OwnerAuthorizationConfig struct {
	UserId string
}

type DefaultCreatorConfig struct {
	Lambda *lambda.Lambda
	UserId string
	Save   string
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

type OwnerAuthorizationAdaptor struct {
	Config OwnerAuthorizationConfig
}

type DefaultCreatorAdaptor struct {
	Config DefaultCreatorConfig
}

type EntityType struct {
	Id         string            `form:"id" json:"id" binding:"required"`
	Owner      string            `form:"owner" json:"owner"`
	OwnerId    string            `form:"ownerId" json:"ownerId"`
	ParentId   string            `form:"parentId" json:"parentId"`
	Name       string            `form:"name" json:"name" binding:"required"`
	Overlay    bool              `form:"overlay" json:"overlay" binding:"required"`
	Target     string            `form:"target" json:"target" binding:"required"`
	Attributes []EntityAttribute `form:"attributes[]" json:"attributes" binding:"required"`
	Filters    []EntityAttribute `form:"filters[]" json:"filters" binding:"required"`
}

type EntityAttribute struct {
	Name       string                 `form:"name" json:"name" binding:"required"`
	Type       attr.AttributeTypes    `form:"type" json:"type" binding:"required"`
	Label      string                 `form:"label" json:"label" binding:"required"`
	Required   bool                   `form:"required" json:"required" binding:"required"`
	Widget     string                 `form:"widget" json:"widget" binding:"required"`
	Settings   map[string]interface{} `form:"settings" json:"settings"`
	Attributes []EntityAttribute      `form:"attributes[]" json:"attributes" binding:"required"`
}

func (m EntityManager) Create(entity map[string]interface{}) (map[string]interface{}, error) {
	return m.Creator.Create(entity, &m)
}

func (m EntityManager) Update(entity map[string]interface{}) {
}

func (m EntityManager) Delete(entity map[string]interface{}) {
}

func (m EntityManager) Save(entity map[string]interface{}, storage string) {
	id := fmt.Sprint(entity[m.Config.IdKey])
	if m.Hooks.BeforeSave != nil {
		abort, err := m.Hooks.BeforeSave(entity)
		if abort || err != nil {
			return
		}
	}
	m.Storages[storage].Store(id, entity)
	if m.Hooks.AfterSave != nil {
		m.Hooks.AfterSave(entity)
	}
}

func (m EntityManager) Load(id string, loader string) map[string]interface{} {
	return m.Loaders[loader].Load(id)
}

func (m EntityManager) Allow(id string, op string, loader string) (bool, map[string]interface{}) {
	if op == "write" {
		return m.Authorizers["default"].CanWrite(id, m.Loaders[loader])
	} else {
		return false, nil
	}
}

func (l S3LoaderAdaptor) Load(id string) map[string]interface{} {

	buf := aws.NewWriteAtBuffer([]byte{})

	downloader := s3manager.NewDownloader(l.Config.Session)

	_, err := downloader.Download(buf, &s3.GetObjectInput{
		Bucket: aws.String(l.Config.Bucket),
		Key:    aws.String(l.Config.Prefix + "" + id + ".json.gz"),
	})

	if err != nil {
		log.Fatalf("failed to download file, %v", err)
	}

	gz, err := gzip.NewReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		log.Fatal(err)
	}

	defer gz.Close()

	text, _ := ioutil.ReadAll(gz)

	var entity map[string]interface{}
	json.Unmarshal(text, &entity)
	return entity
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
	// @todo: invalidate cloudfront object.
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

func (a OwnerAuthorizationAdaptor) CanWrite(id string, loader Loader) (bool, map[string]interface{}) {
	// log.Printf("Check ownership of %s", id)
	entity := loader.Load(id)
	if entity == nil {
		return false, nil
	}
	userId := fmt.Sprint(entity["userId"])
	// log.Printf("Check Entity Ownership: %s == %s", userId, a.Config.UserId)
	return (userId == a.Config.UserId), entity
}

func (c DefaultCreatorAdaptor) Create(entity map[string]interface{}, m *EntityManager) (map[string]interface{}, error) {

	log.Print("Create: 1")

	request := ValidateEntityRequest{
		EntityName: m.Config.SingularName,
		Entity:     entity,
		UserId:     c.Config.UserId,
	}

	log.Print("Create: 2")

	payload, err := json.Marshal(request)
	if err != nil {
		log.Printf("Error marshalling entity validation request: %s", err.Error())
	}

	log.Print("Create: 3")

	res, err := c.Config.Lambda.Invoke(&lambda.InvokeInput{FunctionName: aws.String("goclassifieds-api-dev-ValidateEntity"), Payload: payload})
	if err != nil {
		log.Printf("error invoking entity validation: %s", err.Error())
	}

	var validateRes ValidateEntityResponse
	json.Unmarshal(res.Payload, &validateRes)

	if validateRes.Valid {
		log.Printf("Lambda Response valid")
		m.Save(validateRes.Entity, c.Config.Save)
		return validateRes.Entity, nil
	}

	log.Printf("Lambda Response invalid")
	return entity, nil
}

func NewDefaultManager(config DefaultManagerConfig) EntityManager {
	return EntityManager{
		Config: EntityConfig{
			SingularName: config.SingularName,
			PluralName:   config.PluralName,
			IdKey:        "id",
		},
		Creator: DefaultCreatorAdaptor{
			Config: DefaultCreatorConfig{
				Lambda: config.Lambda,
				UserId: config.UserId,
				Save:   "s3",
			},
		},
		Loaders: map[string]Loader{
			"s3": S3LoaderAdaptor{
				Config: S3AdaptorConfig{
					Session: config.Session,
					Bucket:  "classifieds-ui-dev",
					Prefix:  config.PluralName + "/",
				},
			},
		},
		Storages: map[string]Storage{
			"s3": S3StorageAdaptor{
				Config: S3AdaptorConfig{
					Session: config.Session,
					Bucket:  "classifieds-ui-dev",
					Prefix:  config.PluralName + "/",
				},
			},
			"elastic": ElasticStorageAdaptor{
				Config: ElasticAdaptorConfig{
					Index:  config.Index,
					Client: config.EsClient,
				},
			},
		},
		Hooks: EntityHooks{
			BeforeSave: config.BeforeSave,
			AfterSave:  config.AfterSave,
		},
	}
}
