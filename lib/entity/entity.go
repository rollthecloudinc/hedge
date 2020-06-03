package entity

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"text/template"

	"goclassifieds/lib/attr"
	"goclassifieds/lib/es"
	"goclassifieds/lib/utils"

	"github.com/aws/aws-sdk-go/aws"
	session "github.com/aws/aws-sdk-go/aws/session"
	lambda "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/mitchellh/mapstructure"

	s3 "github.com/aws/aws-sdk-go/service/s3"
	esapi "github.com/elastic/go-elasticsearch/esapi"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
	"github.com/go-playground/validator/v10"
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

type EntityDataRequest struct {
	EntityName string
	EntityType string
	UserId     string
	Entity     map[string]interface{}
	Data       []map[string]interface{}
}

type EntityFinderDataBag struct {
	Query      map[string][]string
	Attributes []EntityAttribute
	UserId     string
}

type ValidateEntityResponse struct {
	Entity       map[string]interface{}
	Valid        bool
	Unauthorized bool
}

type EntityDataResponse struct {
	Data []map[string]interface{}
}

type DefaultManagerConfig struct {
	EsClient     *elasticsearch7.Client
	Session      *session.Session
	Lambda       *lambda.Lambda
	Template     *template.Template
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
	Finders     map[string]Finder
	Storages    map[string]Storage
	Authorizers map[string]Authorization
}

type Manager interface {
	Create(entity map[string]interface{}) (map[string]interface{}, error)
	Update(entity map[string]interface{})
	Delete(entity map[string]interface{})
	Save(entity map[string]interface{}, storage string)
	Load(id string, loader string) map[string]interface{}
	Find(finder string, query string, data *EntityFinderDataBag) []map[string]interface{}
	Allow(id string, op string, loader string) (bool, map[string]interface{})
}

type Storage interface {
	Store(id string, entity map[string]interface{})
}

type Loader interface {
	Load(id string, m *EntityManager) map[string]interface{}
}

type Creator interface {
	Create(entity map[string]interface{}, m *EntityManager) (map[string]interface{}, error)
}

type Finder interface {
	Find(query string, data *EntityFinderDataBag) []map[string]interface{}
}

type Authorization interface {
	CanWrite(id string, m *EntityManager) (bool, map[string]interface{})
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

type ElasticTemplateFinderConfig struct {
	Client   *elasticsearch7.Client
	Index    string
	Template *template.Template
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

type FinderLoaderAdaptor struct {
	Finder string
}

type S3StorageAdaptor struct {
	Config S3AdaptorConfig
}

type ElasticStorageAdaptor struct {
	Config ElasticAdaptorConfig
}

type ElasticTemplateFinder struct {
	Config ElasticTemplateFinderConfig
}

type OwnerAuthorizationAdaptor struct {
	Config OwnerAuthorizationConfig
}

type DefaultCreatorAdaptor struct {
	Config DefaultCreatorConfig
}

type EntityTypeCreatorAdaptor struct {
	Config DefaultCreatorConfig
}

type DefaultEntityTypeFinderConfig struct {
	Template *template.Template
}

type DefaultEntityTypeFinder struct {
	Config DefaultEntityTypeFinderConfig
}

type EntityType struct {
	Id         string            `form:"id" json:"id" binding:"required" validate:"required"`
	UserId     string            `form:"userId" json:"userId" binding:"required" validate:"required"`
	Owner      string            `form:"owner" json:"owner" validate:"required"`
	OwnerId    string            `form:"ownerId" json:"ownerId"`
	ParentId   string            `form:"parentId" json:"parentId"`
	Name       string            `form:"name" json:"name" binding:"required" validate:"required"`
	Overlay    bool              `form:"overlay" json:"overlay" binding:"required"`
	Target     string            `form:"target" json:"target" binding:"required" validate:"required"`
	Attributes []EntityAttribute `form:"attributes[]" json:"attributes" binding:"required" validate:"dive"`
	Filters    []EntityAttribute `form:"filters[]" json:"filters" binding:"required" validate:"dive"`
}

type EntityAttribute struct {
	Name       string                 `form:"name" json:"name" binding:"required" validate:"required"`
	Type       *attr.AttributeTypes   `form:"type" json:"type" binding:"required" validate:"required"`
	Label      string                 `form:"label" json:"label" binding:"required" validate:"required"`
	Required   bool                   `form:"required" json:"required" binding:"required"`
	Widget     string                 `form:"widget" json:"widget" binding:"required" validate:"required"`
	Settings   map[string]interface{} `form:"settings" json:"settings"`
	Attributes []EntityAttribute      `form:"attributes[]" json:"attributes" binding:"required" validate:"dive"`
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
	m.Storages[storage].Store(id, entity)
}

func (m EntityManager) Find(finder string, query string, data *EntityFinderDataBag) []map[string]interface{} {
	return m.Finders[finder].Find(query, data)
}

func (m EntityManager) Load(id string, loader string) map[string]interface{} {
	return m.Loaders[loader].Load(id, &m)
}

func (m EntityManager) Allow(id string, op string, loader string) (bool, map[string]interface{}) {
	if op == "write" {
		return m.Authorizers["default"].CanWrite(id, &m)
	} else {
		return false, nil
	}
}

func (l S3LoaderAdaptor) Load(id string, m *EntityManager) map[string]interface{} {

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

func (l FinderLoaderAdaptor) Load(id string, m *EntityManager) map[string]interface{} {
	data := EntityFinderDataBag{}
	entities := m.Find("default", l.Finder, &data)
	var match map[string]interface{}
	for _, ent := range entities {
		k := fmt.Sprint(ent[m.Config.IdKey])
		if k == id {
			match = ent
			break
		}
	}
	return match
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

func (a OwnerAuthorizationAdaptor) CanWrite(id string, m *EntityManager) (bool, map[string]interface{}) {
	// log.Printf("Check ownership of %s", id)
	entity := m.Load(id, "default")
	if entity == nil {
		return false, nil
	}
	userId := fmt.Sprint(entity["userId"])
	// log.Printf("Check Entity Ownership: %s == %s", userId, a.Config.UserId)
	return (userId == a.Config.UserId), entity
}

func (c DefaultCreatorAdaptor) Create(entity map[string]interface{}, m *EntityManager) (map[string]interface{}, error) {

	request := ValidateEntityRequest{
		EntityName: m.Config.SingularName,
		Entity:     entity,
		UserId:     c.Config.UserId,
	}

	payload, err := json.Marshal(request)
	if err != nil {
		log.Printf("Error marshalling entity validation request: %s", err.Error())
		return entity, errors.New("Error marshalling entity validation request")
	}

	res, err := c.Config.Lambda.Invoke(&lambda.InvokeInput{FunctionName: aws.String("goclassifieds-api-dev-ValidateEntity"), Payload: payload})
	if err != nil {
		log.Printf("error invoking entity validation: %s", err.Error())
		return entity, errors.New("Error invoking validation")
	}

	var validateRes ValidateEntityResponse
	json.Unmarshal(res.Payload, &validateRes)

	if validateRes.Unauthorized {
		log.Printf("Unauthorized to create entity")
		return entity, errors.New("Unauthorized to create entity")
	}

	if validateRes.Valid {
		log.Printf("Lambda Response valid")
		m.Save(validateRes.Entity, c.Config.Save)
		return validateRes.Entity, nil
	}

	return entity, errors.New("Entity invalid")
}

func (c EntityTypeCreatorAdaptor) Create(entity map[string]interface{}, m *EntityManager) (map[string]interface{}, error) {

	log.Print("EntityTypeCreatorAdaptor: 1")

	jsonData, err := json.Marshal(entity)
	if err != nil {
		return entity, err
	}

	log.Print("EntityTypeCreatorAdaptor: 2")

	var obj EntityType
	err = json.Unmarshal(jsonData, &obj)
	if err != nil {
		return entity, err
	}

	log.Print("EntityTypeCreatorAdaptor: 3")

	obj.Id = utils.GenerateId()
	obj.UserId = c.Config.UserId

	log.Print("EntityTypeCreatorAdaptor: 4")

	validate := validator.New()
	err = validate.Struct(obj)
	if err != nil {
		return entity, err.(validator.ValidationErrors)
	}

	log.Print("EntityTypeCreatorAdaptor: 5")

	newEntity, _ := TypeToEntity(&obj)
	m.Save(newEntity, c.Config.Save)

	log.Print("EntityTypeCreatorAdaptor: 6")

	return newEntity, nil

}

func (f ElasticTemplateFinder) Find(query string, data *EntityFinderDataBag) []map[string]interface{} {

	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(data); err != nil {
		log.Fatalf("Error encoding search query: %s", err)
	}
	log.Printf("template data: %s", b.String())

	hits := es.ExecuteQuery(f.Config.Client, es.TemplateBuilder{
		Index:    f.Config.Index,
		Name:     query,
		Template: f.Config.Template,
		Data:     data,
	})

	docs := make([]map[string]interface{}, len(hits))
	for index, hit := range hits {
		mapstructure.Decode(hit.(map[string]interface{})["_source"], &docs[index])
	}

	return docs

}

func (f DefaultEntityTypeFinder) Find(query string, data *EntityFinderDataBag) []map[string]interface{} {

	var tb bytes.Buffer
	err := f.Config.Template.ExecuteTemplate(&tb, query, data)
	if err != nil {
		log.Printf("Entity Type Error: %s", err.Error())
	}

	var types []map[string]interface{}
	err = json.Unmarshal(tb.Bytes(), &types)
	if err != nil {
		log.Printf("Unmarshall Entity Types Error: %s", err.Error())
	}

	filteredTypes := make([]map[string]interface{}, 0)
	for _, entType := range types {
		name := fmt.Sprint(entType["name"])
		if data.Query["name"] == nil || data.Query["name"][0] == "" || data.Query["name"][0] == name {
			filteredTypes = append(filteredTypes, entType)
		}
	}

	return filteredTypes

}

func TypeToEntity(entityType *EntityType) (map[string]interface{}, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(entityType); err != nil {
		log.Fatalf("Error encoding query: %s", err)
	}
	jsonData, err := json.Marshal(entityType)
	if err != nil {
		return nil, err
	}
	var entity map[string]interface{}
	err = json.Unmarshal(jsonData, &entity)
	return entity, nil
}

func FlattenEntityAttribute(attribute EntityAttribute) []EntityAttribute {
	leafNodes := make([]EntityAttribute, 0)
	if attribute.Attributes == nil || len(attribute.Attributes) == 0 {
		leafNodes = append(leafNodes, attribute)
	} else {
		for _, attr := range attribute.Attributes {
			flatChildren := FlattenEntityAttribute(attr)
			for _, flatChild := range flatChildren {
				leafNodes = append(leafNodes, flatChild)
			}
		}
	}
	return leafNodes
}

func ExecuteEntityLambda(lam *lambda.Lambda, functionName string, request *EntityDataRequest) (EntityDataResponse, error) {

	deadResponse := EntityDataResponse{}

	payload, err := json.Marshal(request)
	if err != nil {
		log.Printf("Error marshalling entity lambda request: %s", err.Error())
		return deadResponse, errors.New("Error marshalling entity lambda request")
	}

	res, err := lam.Invoke(&lambda.InvokeInput{FunctionName: aws.String(functionName), Payload: payload})
	if err != nil {
		log.Printf("error invoking entity lambda: %s", err.Error())
		return deadResponse, errors.New("Error invoking entity lambda")
	}

	var dataRes EntityDataResponse
	json.Unmarshal(res.Payload, &dataRes)

	return dataRes, nil

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
				Save:   "default",
			},
		},
		Finders: map[string]Finder{
			"default": ElasticTemplateFinder{
				Config: ElasticTemplateFinderConfig{
					Index:    config.Index,
					Client:   config.EsClient,
					Template: config.Template,
				},
			},
		},
		Loaders: map[string]Loader{
			"default": S3LoaderAdaptor{
				Config: S3AdaptorConfig{
					Session: config.Session,
					Bucket:  "classifieds-ui-dev",
					Prefix:  config.PluralName + "/",
				},
			},
		},
		Storages: map[string]Storage{
			"default": S3StorageAdaptor{
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
	}
}

func NewEntityTypeManager(config DefaultManagerConfig) EntityManager {
	return EntityManager{
		Config: EntityConfig{
			SingularName: "type",
			PluralName:   "types",
			IdKey:        "id",
		},
		Creator: EntityTypeCreatorAdaptor{
			Config: DefaultCreatorConfig{
				Lambda: config.Lambda,
				UserId: config.UserId,
				Save:   "default",
			},
		},
		Finders: map[string]Finder{
			"default": DefaultEntityTypeFinder{
				Config: DefaultEntityTypeFinderConfig{
					Template: config.Template,
				},
			},
		},
		Loaders: map[string]Loader{
			"default": FinderLoaderAdaptor{
				Finder: "all",
			},
		},
		Storages: map[string]Storage{
			"default": ElasticStorageAdaptor{
				Config: ElasticAdaptorConfig{
					Index:  "classified_types",
					Client: config.EsClient,
				},
			},
		},
	}
}
