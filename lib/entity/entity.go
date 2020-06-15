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
	"strings"
	"text/template"
	"time"

	"goclassifieds/lib/attr"
	"goclassifieds/lib/es"
	"goclassifieds/lib/utils"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	session "github.com/aws/aws-sdk-go/aws/session"
	lambda "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/mitchellh/mapstructure"
	"github.com/tangzero/inflector"

	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	s3 "github.com/aws/aws-sdk-go/service/s3"
	esapi "github.com/elastic/go-elasticsearch/esapi"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
	"github.com/go-playground/validator/v10"
	"github.com/gocql/gocql"
)

type Hooks int32

const (
	BeforeSave Hooks = iota
	AfterSave
	BeforeFind
	AfterFind
)

type HookSignals int32

const (
	HookContinue HookSignals = iota
	HookBreak
	HookSkip
)

type EntityHook func(entity map[string]interface{}, m *EntityManager) (map[string]interface{}, error)
type EntityCollectionHook func(entities []map[string]interface{}, m *EntityManager) ([]map[string]interface{}, error, HookSignals)
type CognitoTransformation func(user *cognitoidentityprovider.UserType) (map[string]interface{}, error)

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
	Req        *events.APIGatewayProxyRequest
	Attributes []EntityAttribute
	Metadata   map[string]interface{}
}

type ValidateEntityResponse struct {
	Entity       map[string]interface{}
	Valid        bool
	Unauthorized bool
}

type VariableBindings struct {
	Values []interface{}
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
	BeforeFind   EntityCollectionHook
	AfterFind    EntityCollectionHook
}

type EntityAdaptorConfig struct {
	Session  *session.Session
	Lambda   *lambda.Lambda
	Cognito  *cognitoidentityprovider.CognitoIdentityProvider
	Elastic  *elasticsearch7.Client
	Template *template.Template
	Cql      *gocql.Session
	Bindings *VariableBindings
}

type EntityManager struct {
	Config          EntityConfig
	Creator         Creator
	Loaders         map[string]Loader
	Finders         map[string]Finder
	Storages        map[string]Storage
	Authorizers     map[string]Authorization
	Hooks           map[Hooks]EntityHook
	CollectionHooks map[string]EntityCollectionHook
}

type Manager interface {
	Create(entity map[string]interface{}) (map[string]interface{}, error)
	Update(entity map[string]interface{})
	Purge(storage string, entities ...map[string]interface{})
	Save(entity map[string]interface{}, storage string)
	Load(id string, loader string) map[string]interface{}
	Find(finder string, query string, data *EntityFinderDataBag) []map[string]interface{}
	Allow(id string, op string, loader string) (bool, map[string]interface{})
	AddFinder(name string, finder Finder)
	ExecuteHook(hook Hooks, entity map[string]interface{}) (map[string]interface{}, error)
	ExecuteCollectionHook(hook string, entities []map[string]interface{}) ([]map[string]interface{}, error)
}

type Storage interface {
	Store(id string, entity map[string]interface{})
	Purge(m *EntityManager, entities ...map[string]interface{}) error
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
	Bucket  string `json:"bucket"`
	Prefix  string `json:"prefix"`
	Session *session.Session
}

type CognitoAdaptorConfig struct {
	Client     *cognitoidentityprovider.CognitoIdentityProvider
	UserPoolId string `json:"userPoolId"`
	Transform  CognitoTransformation
}

type ElasticAdaptorConfig struct {
	Client *elasticsearch7.Client
	Index  string `json:"index"`
}

type CqlAdaptorConfig struct {
	Session *gocql.Session
	Table   string `json:"table"`
}

type ElasticTemplateFinderConfig struct {
	Client        *elasticsearch7.Client
	Index         string `json:"index"`
	Template      *template.Template
	CollectionKey string `json:"collectionKey"`
	ObjectKey     string `json:"objectKey"`
}

type CqlTemplateFinderConfig struct {
	Session  *gocql.Session
	Table    string `json:"table"`
	Template *template.Template
	Bindings *VariableBindings
	Aliases  map[string]string `json:"aliases"`
}

type OwnerAuthorizationConfig struct {
	UserId string `json:"userId"`
}

type DefaultCreatorConfig struct {
	Lambda *lambda.Lambda
	UserId string `json:"userId"`
	Save   string `json:"save"`
}

type S3LoaderAdaptor struct {
	Config S3AdaptorConfig `json:"config"`
}

type S3MediaLoaderAdaptor struct {
	Config S3AdaptorConfig `json:"config"`
}

type CognitoLoaderAdaptor struct {
	Config CognitoAdaptorConfig `json:"config"`
}

type FinderLoaderAdaptor struct {
	Finder string `json:"finder"`
}

type S3StorageAdaptor struct {
	Config S3AdaptorConfig `json:"config"`
}

type ElasticStorageAdaptor struct {
	Config ElasticAdaptorConfig `json:"config"`
}

type CqlStorageAdaptor struct {
	Config CqlAdaptorConfig `json:"config"`
}

type ElasticTemplateFinder struct {
	Config ElasticTemplateFinderConfig `json:"config"`
}

type CqlTemplateFinder struct {
	Config CqlTemplateFinderConfig `json:"config"`
}

type OwnerAuthorizationAdaptor struct {
	Config OwnerAuthorizationConfig `json:"config"`
}

type DefaultCreatorAdaptor struct {
	Config DefaultCreatorConfig `json:"config"`
}

type EntityTypeCreatorAdaptor struct {
	Config DefaultCreatorConfig `json:"config"`
}

type DefaultEntityTypeFinderConfig struct {
	Template *template.Template
}

type DefaultEntityTypeFinder struct {
	Config DefaultEntityTypeFinderConfig `json:"config"`
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

func (m EntityManager) Purge(storage string, entities ...map[string]interface{}) {
	// id := fmt.Sprint(ent[m.Config.IdKey])
	m.Storages[storage].Purge(&m, entities...)
}

func (m EntityManager) Save(entity map[string]interface{}, storage string) {

	ent, err := m.ExecuteHook(BeforeSave, entity)

	if err != nil {
		log.Print(err)
	}

	id := fmt.Sprint(ent[m.Config.IdKey])
	m.Storages[storage].Store(id, ent)

	if _, err := m.ExecuteHook(AfterSave, entity); err != nil {
		log.Print(err)
	}
}

func (m EntityManager) AddFinder(name string, finder Finder) {
	m.Finders[name] = finder
}

func (m EntityManager) Find(finder string, query string, data *EntityFinderDataBag) []map[string]interface{} {

	entities := make([]map[string]interface{}, 0)

	entities, err := m.ExecuteCollectionHook(finder+"/"+query, entities)
	if err != nil {
		log.Print(err)
	}

	for _, entity := range m.Finders[finder].Find(query, data) {
		entities = append(entities, entity)
	}

	entities, err = m.ExecuteCollectionHook(finder+"/"+query, entities)
	if err != nil {
		log.Print(err)
	}

	return entities
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

func (m EntityManager) ExecuteHook(hook Hooks, entity map[string]interface{}) (map[string]interface{}, error) {
	if hookFunc, ok := m.Hooks[hook]; ok {
		return hookFunc(entity, &m)
	}
	return entity, nil
}

func (m EntityManager) ExecuteCollectionHook(hook string, entities []map[string]interface{}) ([]map[string]interface{}, error) {
	if hookFunc, ok := m.CollectionHooks[hook]; ok {
		entities, err, _ := hookFunc(entities, &m)
		return entities, err
	}
	return entities, nil
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

func (l CognitoLoaderAdaptor) Load(id string, m *EntityManager) map[string]interface{} {
	res, err := l.Config.Client.ListUsers(&cognitoidentityprovider.ListUsersInput{
		Filter:     aws.String("sub=\"" + id + "\""),
		UserPoolId: aws.String(l.Config.UserPoolId),
		Limit:      aws.Int64(1),
	})
	if err != nil {
		log.Print(err)
	}
	var obj map[string]interface{}
	if l.Config.Transform != nil {
		obj, _ = l.Config.Transform(res.Users[0])
	} else {
		jsonData, err := json.Marshal(res.Users[0])
		if err != nil {
			log.Print(err)
		}
		err = json.Unmarshal(jsonData, &obj)
		if err != nil {
			log.Print(err)
		}
	}
	return obj
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

func (s S3StorageAdaptor) Purge(m *EntityManager, entities ...map[string]interface{}) error {
	return nil
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

func (s ElasticStorageAdaptor) Purge(m *EntityManager, entities ...map[string]interface{}) error {
	return nil
}

func (s CqlStorageAdaptor) Store(id string, entity map[string]interface{}) {
	values := make([]interface{}, 0)
	cols := make([]string, 0)
	bind := make([]string, 0)
	for field, value := range entity {
		bind = append(bind, "?")
		cols = append(cols, strings.ToLower(field))
		xType := fmt.Sprintf("%T", value)
		log.Printf("%s = %s", field, xType)
		if xType == "string" {
			t1, e := time.Parse(time.RFC3339, value.(string))
			if e == nil {
				log.Printf("is time: %s", value)
				values = append(values, t1)
			} else {
				log.Printf("is not time: %s", value)
				values = append(values, value)
			}
		} else {
			log.Printf("is not string: %v", value)
			values = append(values, value)
		}
	}
	stmt := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`, s.Config.Table, strings.Join(cols[:], ","), strings.Join(bind[:], ","))
	log.Print("after exec")
	if err := s.Config.Session.Query(stmt, values...).Exec(); err != nil {
		log.Print("after exec error")
		log.Fatal(err)
	}
}

func (s CqlStorageAdaptor) Purge(m *EntityManager, entities ...map[string]interface{}) error {
	for _, ent := range entities {
		log.Printf("Purge = %s", ent[m.Config.IdKey])
	}
	return nil
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
		Index:         f.Config.Index,
		Name:          query,
		Template:      f.Config.Template,
		Data:          data,
		CollectionKey: f.Config.CollectionKey,
	})

	docs := make([]map[string]interface{}, len(hits))
	for index, hit := range hits {
		if f.Config.ObjectKey != "" {
			mapstructure.Decode(hit.(map[string]interface{})[f.Config.ObjectKey], &docs[index])
		} else {
			docs[index] = hit.(map[string]interface{})
		}
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
		log.Printf("DefaultEntityTypeFinder:Find %s", name)
		if data.Req == nil || data.Req.QueryStringParameters == nil || (data.Req.QueryStringParameters != nil && data.Req.QueryStringParameters["name"] == name) {
			filteredTypes = append(filteredTypes, entType)
		}
	}

	return filteredTypes

}

func (f CqlTemplateFinder) Find(query string, data *EntityFinderDataBag) []map[string]interface{} {

	f.Config.Bindings.Values = make([]interface{}, 0)

	var tb bytes.Buffer
	err := f.Config.Template.ExecuteTemplate(&tb, query, data)
	if err != nil {
		log.Printf("Build CQL Query Error: %s", err.Error())
	}

	log.Printf("cql query: ", tb.String())
	for _, value := range f.Config.Bindings.Values {
		log.Printf("binding: %v", value)
	}

	rows := make([]map[string]interface{}, 0)
	iter := f.Config.Session.Query(tb.String(), f.Config.Bindings.Values...).Iter()
	for {
		rawRow := make(map[string]interface{})
		row := make(map[string]interface{})

		if !iter.MapScan(rawRow) {
			break
		}

		for field, val := range rawRow {
			if _, ok := f.Config.Aliases[field]; ok {
				row[f.Config.Aliases[field]] = val
			} else {
				row[field] = val
			}
		}

		rows = append(rows, row)

	}

	return rows
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

func PipeCollectionHooks(hooks ...EntityCollectionHook) EntityCollectionHook {
	return func(entities []map[string]interface{}, m *EntityManager) ([]map[string]interface{}, error, HookSignals) {
		collection := entities[:]
		for _, hook := range hooks {
			c, err, sig := hook(entities, m)
			if err != nil {
				log.Print(err)
			}
			if sig == HookBreak {
				return collection, nil, HookContinue
			} else if sig == HookSkip {
				continue
			}
			collection = c
		}
		return collection, nil, HookContinue
	}
}

func MergeEntities(h func(m *EntityManager) []map[string]interface{}) EntityCollectionHook {
	return func(entities []map[string]interface{}, m *EntityManager) ([]map[string]interface{}, error, HookSignals) {
		for _, ent := range h(m) {
			entities = append(entities, ent)
		}
		return entities, nil, HookContinue
	}
}

func FilterEntities(h func(ent map[string]interface{}) bool) EntityCollectionHook {
	return func(entities []map[string]interface{}, m *EntityManager) ([]map[string]interface{}, error, HookSignals) {
		filtered := make([]map[string]interface{}, 0)
		for _, ent := range entities {
			if h(ent) {
				filtered = append(filtered, ent)
			}
		}
		return filtered, nil, HookContinue
	}
}

func GetAdaptor(name string, s *EntityAdaptorConfig, c map[string]interface{}) (interface{}, error) {

	factory := map[string]interface{}{
		"s3/loader": S3LoaderAdaptor{
			Config: S3AdaptorConfig{
				Session: s.Session,
			},
		},
		"cognito/loader": CognitoLoaderAdaptor{
			Config: CognitoAdaptorConfig{
				Client: s.Cognito,
			},
		},
		"finder/loader": FinderLoaderAdaptor{},
		"elastic/templatefinder": ElasticTemplateFinder{
			Config: ElasticTemplateFinderConfig{
				Client:   s.Elastic,
				Template: s.Template,
			},
		},
		"entitytypefinder": DefaultEntityTypeFinder{
			Config: DefaultEntityTypeFinderConfig{
				Template: s.Template,
			},
		},
		"cql/templateFinder": CqlTemplateFinder{
			Config: CqlTemplateFinderConfig{
				Session:  s.Cql,
				Template: s.Template,
				Bindings: s.Bindings,
			},
		},
		"s3/storage": S3StorageAdaptor{
			Config: S3AdaptorConfig{
				Session: s.Session,
			},
		},
		"elastic/storage": ElasticStorageAdaptor{
			Config: ElasticAdaptorConfig{
				Client: s.Elastic,
			},
		},
		"cql/storage": CqlStorageAdaptor{
			Config: CqlAdaptorConfig{
				Session: s.Cql,
			},
		},
		"owner/authorizer": OwnerAuthorizationAdaptor{},
		"default/creator": DefaultCreatorAdaptor{
			Config: DefaultCreatorConfig{
				Lambda: s.Lambda,
			},
		},
		"entitytype/creator": EntityTypeCreatorAdaptor{
			Config: DefaultCreatorConfig{
				Lambda: s.Lambda,
			},
		},
	}

	if _, ok := factory[name]; !ok {
		return nil, errors.New("adaptor does not exist by that name")
	}

	jsonData, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(jsonData, factory[name])
	if err != nil {
		return nil, err
	}

	return factory[name], nil

}

func GetManager(entityName string, c map[string]interface{}, s *EntityAdaptorConfig) (*EntityManager, error) {

	adaptors := []string{
		"finders",
		"loaders",
		"storages",
		"creator",
		"authorization",
	}

	var finders map[string]Finder
	var loaders map[string]Loader
	var storages map[string]Storage
	var authorizers map[string]Authorization
	var creator Creator

	for _, adaptor := range adaptors {
		if _, ok := c[adaptor]; ok {
			for name, item := range c[adaptor].(map[string]interface{}) {
				instance, err := GetAdaptor(fmt.Sprint(item.(map[string]string)["factory"]), s, item.(map[string]interface{}))
				if err != nil {
					return nil, err
				}
				switch adaptor {
				case "finders":
					finders[name] = instance.(Finder)
					break
				case "loaders":
					loaders[name] = instance.(Loader)
					break
				case "storages":
					storages[name] = instance.(Storage)
					break
				case "authorizers":
					authorizers[name] = instance.(Authorization)
					break
				case "creator":
					creator = instance.(Creator)
					break
				default:
					return nil, errors.New("invalid adaptor detected")
				}
			}
		}
	}

	manager := EntityManager{
		Config: EntityConfig{
			SingularName: entityName,
			PluralName:   inflector.Pluralize(entityName),
			IdKey:        "id",
		},
		Finders:     finders,
		Loaders:     loaders,
		Storages:    storages,
		Authorizers: authorizers,
		Creator:     creator,
	}

	return &manager, nil

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
					Index:         config.Index,
					Client:        config.EsClient,
					Template:      config.Template,
					CollectionKey: "hits.hits",
					ObjectKey:     "_source",
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
