package entity

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
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
	"goclassifieds/lib/gov"
	"goclassifieds/lib/os"
	repo "goclassifieds/lib/repo"
	"goclassifieds/lib/utils"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	session "github.com/aws/aws-sdk-go/aws/session"
	lambda "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/mitchellh/mapstructure"
	"github.com/shurcooL/githubv4"
	"github.com/tangzero/inflector"

	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	s3 "github.com/aws/aws-sdk-go/service/s3"
	esapi "github.com/elastic/go-elasticsearch/esapi"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
	"github.com/go-playground/validator/v10"
	"github.com/gocql/gocql"
	"github.com/google/go-github/v46/github"
	opensearch "github.com/opensearch-project/opensearch-go"
	opensearchapi "github.com/opensearch-project/opensearch-go/opensearchapi"
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
	SingularName        string
	PluralName          string
	IdKey               string
	Stage               string
	CloudName           string
	LogUsageLambdaInput *utils.LogUsageLambdaInput
}

type ValidateEntityRequest struct {
	EntityName          string
	EntityType          string
	UserId              string
	Site                string
	Entity              map[string]interface{}
	LogUsageLambdaInput *utils.LogUsageLambdaInput
}

type EnforceContractRequest struct {
	EntityName          string
	EntityType          string
	UserId              string
	Site                string
	Entity              map[string]interface{}
	Contract            map[string]interface{}
	LogUsageLambdaInput *utils.LogUsageLambdaInput
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

type EnforceContractResponse struct {
	Entity       map[string]interface{}
	Valid        bool
	Unauthorized bool
	Errors       []map[string]interface{}
}

type VariableBindings struct {
	Values []interface{}
}

type EntityDataResponse struct {
	Data []map[string]interface{}
}

type CreateEntityResponse struct {
	Success bool
	Entity  map[string]interface{}
	Errors  []map[string]interface{}
}

type UpdateEntityResponse struct {
	Success bool
	Entity  map[string]interface{}
	Errors  []map[string]interface{}
}

type EntityValidationResponse struct {
	Entity map[string]interface{}
	Errors []map[string]interface{}
}

type DefaultManagerConfig struct {
	EsClient            *elasticsearch7.Client
	OsClient            *opensearch.Client
	GithubV4Client      *githubv4.Client
	Session             *session.Session
	Lambda              *lambda.Lambda
	Template            *template.Template
	UserId              string
	SingularName        string
	PluralName          string
	Index               string
	BucketName          string
	Stage               string
	Site                string
	CloudName           string
	LogUsageLambdaInput *utils.LogUsageLambdaInput
	BeforeSave          EntityHook
	AfterSave           EntityHook
	BeforeFind          EntityCollectionHook
	AfterFind           EntityCollectionHook
}

type EntityAdaptorConfig struct {
	Session    *session.Session
	Lambda     *lambda.Lambda
	Cognito    *cognitoidentityprovider.CognitoIdentityProvider
	Elastic    *elasticsearch7.Client
	Opensearch *opensearch.Client
	Template   *template.Template
	Cql        *gocql.Session
	Bindings   *VariableBindings
}

type EntityManager struct {
	Config          EntityConfig
	Creator         Creator
	Updator         Updator
	Validators      map[string]Validator
	Loaders         map[string]Loader
	Finders         map[string]Finder
	Storages        map[string]Storage
	Authorizers     map[string]Authorization
	Hooks           map[Hooks]EntityHook
	CollectionHooks map[string]EntityCollectionHook
}

type Manager interface {
	Create(entity map[string]interface{}) (*CreateEntityResponse, error)
	Update(entity map[string]interface{}) (*UpdateEntityResponse, error)
	Validate(name string, entity map[string]interface{}) (*EntityValidationResponse, error)
	Purge(storage string, entities ...map[string]interface{})
	Save(entity map[string]interface{}, storage string)
	Load(id string, loader string) map[string]interface{}
	Find(finder string, query string, data *EntityFinderDataBag) []map[string]interface{}
	Allow(id string, op string, loader string) (bool, map[string]interface{})
	AddFinder(name string, finder Finder)
	AddLoader(name string, loader Loader)
	AddStorage(name string, storage Storage)
	AddValidator(name string, validator Validator)
	AddAuthorizer(name string, authorizer Authorization)
	SetHook(name Hooks, entityHook EntityHook)
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
	Create(entity map[string]interface{}, m *EntityManager) (*CreateEntityResponse, error)
}

type Updator interface {
	Update(entity map[string]interface{}, m *EntityManager) (*UpdateEntityResponse, error)
}

type Validator interface {
	Validate(entity map[string]interface{}, m *EntityManager) (*EntityValidationResponse, error)
}

type Finder interface {
	Find(query string, data *EntityFinderDataBag) []map[string]interface{}
}

type Authorization interface {
	CanWrite(id string, m *EntityManager) (bool, map[string]interface{})
}

type S3AdaptorConfig struct {
	Bucket  string           `json:"bucket"`
	Prefix  string           `json:"prefix"`
	Session *session.Session `json:"-"`
}

type CognitoAdaptorConfig struct {
	Client     *cognitoidentityprovider.CognitoIdentityProvider `json:"-"`
	UserPoolId string                                           `json:"userPoolId"`
	Transform  CognitoTransformation                            `json:"-"`
}

type ElasticAdaptorConfig struct {
	Client *elasticsearch7.Client `json:"-"`
	Index  string                 `json:"index"`
}

type OpensearchAdaptorConfig struct {
	Client *opensearch.Client `json:"-"`
	Index  string             `json:"index"`
}

type CqlAdaptorConfig struct {
	Session *gocql.Session `json:"-"`
	Table   string         `json:"table"`
}

type GithubFileUploadConfig struct {
	Client   *githubv4.Client `json:"-"`
	Repo     string           `json:"repo"`
	Branch   string           `json:"branch"`
	Path     string           `json:"path"`
	UserName string           `json:"userName"`
}

type GithubRestFileUploadConfig struct {
	Client   *github.Client `json:"-"`
	Repo     string         `json:"repo"`
	Branch   string         `json:"branch"`
	Path     string         `json:"path"`
	UserName string         `json:"userName"`
}

type ElasticTemplateFinderConfig struct {
	Client        *elasticsearch7.Client `json:"-"`
	Index         string                 `json:"index"`
	Template      *template.Template     `json:"-"`
	CollectionKey string                 `json:"collectionKey"`
	ObjectKey     string                 `json:"objectKey"`
}

type OpensearchTemplateFinderConfig struct {
	Client        *opensearch.Client `json:"-"`
	Index         string             `json:"index"`
	Template      *template.Template `json:"-"`
	CollectionKey string             `json:"collectionKey"`
	ObjectKey     string             `json:"objectKey"`
}

type CqlTemplateFinderConfig struct {
	Session  *gocql.Session     `json:"-"`
	Table    string             `json:"table"`
	Template *template.Template `json:"-"`
	Bindings *VariableBindings  `json:"-"`
	Aliases  map[string]string  `json:"aliases"`
}

type OwnerAuthorizationConfig struct {
	UserId string `json:"userId"`
	Site   string `json:"site"`
}

type ResourceOrOwnerAuthorizationConfig struct {
	UserId              string `json:"userId"`
	Site                string `json:"site"`
	Resource            gov.ResourceTypes
	Asset               string
	Lambda              *lambda.Lambda  `json:"-"`
	AdditionalResources *[]gov.Resource `json:"additional_resources"`
}

type ResourceAuthorizationConfig struct {
	UserId              string `json:"userId"`
	Site                string `json:"site"`
	Resource            gov.ResourceTypes
	Asset               string
	Lambda              *lambda.Lambda  `json:"-"`
	AdditionalResources *[]gov.Resource `json:"additional_resources"`
}

type ResourceAuthorizationEmbeddedConfig struct {
	UserId              string `json:"userId"`
	Site                string `json:"site"`
	Resource            gov.ResourceTypes
	Asset               string
	Lambda              *lambda.Lambda `json:"-"`
	CassSession         *gocql.Session
	GrantAccessManager  Manager
	AdditionalResources *[]gov.Resource `json:"additional_resources"`
}

type DefaultCreatorConfig struct {
	Lambda *lambda.Lambda `json:"-"`
	UserId string         `json:"userId"`
	Site   string         `json:"site"`
	Save   string         `json:"save"`
}

type DefaultUpdatorConfig struct {
	Lambda *lambda.Lambda `json:"-"`
	UserId string         `json:"userId"`
	Site   string         `json:"site"`
	Save   string         `json:"save"`
}

type DefaultValidatorConfig struct {
	Lambda *lambda.Lambda `json:"-"`
	UserId string         `json:"userId"`
	Site   string         `json:"site"`
}

type ContractValidatorConfig struct {
	Lambda   *lambda.Lambda `json:"-"`
	Client   *github.Client `json:"-"`
	UserId   string         `json:"userId"`
	Site     string         `json:"site"`
	Repo     string         `json:"repo"`
	Branch   string         `json:"branch"`
	Contract string         `json:"contract"`
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

type GithubFileLoaderAdaptor struct {
	Config GithubFileUploadConfig `json:"config"`
}

type GithubRestFileLoaderAdaptor struct {
	Config GithubRestFileUploadConfig `json:"config"`
}

type GithubRestLoaderAdaptor struct {
	Config GithubRestFileUploadConfig `json:"config"`
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

type OpensearchStorageAdaptor struct {
	Config OpensearchAdaptorConfig `json:"config"`
}

type CqlStorageAdaptor struct {
	Config CqlAdaptorConfig `json:"config"`
}

type CqlAutoDiscoveryExpansionStorageAdaptor struct {
	Config CqlAdaptorConfig `json:"config"`
}

type GithubFileUploadAdaptor struct {
	Config GithubFileUploadConfig `json:"config"`
}

type GithubRestFileUploadAdaptor struct {
	Config GithubRestFileUploadConfig `json:"config"`
}

type ElasticTemplateFinder struct {
	Config ElasticTemplateFinderConfig `json:"config"`
}

type OpensearchTemplateFinder struct {
	Config OpensearchTemplateFinderConfig `json:"config"`
}

type CqlTemplateFinder struct {
	Config CqlTemplateFinderConfig `json:"config"`
}

type NoopAuthorizationAdaptor struct {
}

type OwnerAuthorizationAdaptor struct {
	Config OwnerAuthorizationConfig `json:"config"`
}

type ResourceOrOwnerAuthorizationAdaptor struct {
	Config ResourceOrOwnerAuthorizationConfig `json:"config"`
}

type ResourceAuthorizationAdaptor struct {
	Config ResourceAuthorizationConfig `json:"config"`
}

type ResourceAuthorizationEmbeddedAdaptor struct {
	Config ResourceAuthorizationEmbeddedConfig `json:"config"`
}

type DefaultCreatorAdaptor struct {
	Config DefaultCreatorConfig `json:"config"`
}

type DefaultUpdatorAdaptor struct {
	Config DefaultUpdatorConfig `json:"config"`
}

type EntityTypeCreatorAdaptor struct {
	Config DefaultCreatorConfig `json:"config"`
}

type DefaultValidatorAdaptor struct {
	Config DefaultValidatorConfig `json:"config"`
}

type ContractValidatorAdaptor struct {
	Config ContractValidatorConfig `json:"config"`
}

type DefaultEntityTypeFinderConfig struct {
	Template *template.Template `json:"-"`
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

func (m EntityManager) Create(entity map[string]interface{}) (*CreateEntityResponse, error) {
	return m.Creator.Create(entity, &m)
}

func (m EntityManager) Update(entity map[string]interface{}) (*UpdateEntityResponse, error) {
	return m.Updator.Update(entity, &m)
}

func (m EntityManager) Validate(name string, entity map[string]interface{}) (*EntityValidationResponse, error) {
	return m.Validators[name].Validate(entity, &m)
}

func (m EntityManager) Purge(storage string, entities ...map[string]interface{}) {
	// id := fmt.Sprint(ent[m.Config.IdKey])
	m.Storages[storage].Purge(&m, entities...)
}

func (m EntityManager) Save(entity map[string]interface{}, storage string) {

	log.Print("EntityManager:save " + storage)

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

func (m EntityManager) AddStorage(name string, storage Storage) {
	m.Storages[name] = storage
}

func (m EntityManager) AddFinder(name string, finder Finder) {
	m.Finders[name] = finder
}

func (m EntityManager) AddLoader(name string, loader Loader) {
	m.Loaders[name] = loader
}

func (m EntityManager) AddAuthorizer(name string, authorizer Authorization) {
	m.Authorizers[name] = authorizer
}

func (m EntityManager) AddValidator(name string, validator Validator) {
	m.Validators[name] = validator
}

func (m EntityManager) SetHook(name Hooks, entityHook EntityHook) {
	m.Hooks[name] = entityHook
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

func (s GithubFileLoaderAdaptor) Load(id string, m *EntityManager) map[string]interface{} {
	log.Printf("BEGIN GithubFileUploadAdaptor::LOAD %s", id)
	var obj map[string]interface{}
	pieces := strings.Split(s.Config.Repo, "/")
	var q struct {
		Repository struct {
			Object struct {
				ObjectFragment struct {
					Text string
				} `graphql:"... on Blob"`
			} `graphql:"object(expression: $exp)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}
	qVars := map[string]interface{}{
		"exp":   githubv4.String(s.Config.Branch + ":" + s.Config.Path + "/" + id + ".json"),
		"owner": githubv4.String(pieces[0]),
		"name":  githubv4.String(pieces[1]),
	}
	err := s.Config.Client.Query(context.Background(), &q, qVars)
	if err != nil {
		log.Print("Github latest file failure.")
		log.Panic(err)
	}
	log.Printf(q.Repository.Object.ObjectFragment.Text)
	json.Unmarshal([]byte(q.Repository.Object.ObjectFragment.Text), &obj)
	log.Printf("END GithubFileUploadAdaptor::LOAD %s", id)
	return obj
}

func (s GithubRestFileLoaderAdaptor) Load(id string, m *EntityManager) map[string]interface{} {
	log.Printf("BEGIN GithubRestFileUploadAdaptor::LOAD %s", id)
	var obj map[string]interface{}
	pieces := strings.Split(s.Config.Repo, "/")
	opts := &github.RepositoryContentGetOptions{
		Ref: s.Config.Branch,
	}
	suffix := ""
	if id != "" {
		suffix = "/" + id
	}
	log.Print("Github Rest Fetch " + pieces[0] + "/" + pieces[1] + ":" + s.Config.Path + suffix)
	file, _, res, err := s.Config.Client.Repositories.GetContents(context.Background(), pieces[0], pieces[1], s.Config.Path+suffix, opts)
	if err != nil && res.StatusCode != 404 {
		log.Print("Github get content failure.")
		log.Panic(err)
	}
	if err == nil && file != nil && file.Content != nil {
		content, _ := base64.StdEncoding.DecodeString(*file.Content)
		//if err != nil {
			log.Printf(string(content))
			json.Unmarshal(content, &obj)
			log.Printf("The object id is %s", obj["id"].(string))
		//}
	}
	log.Printf("END GithubRestFileUploadAdaptor::LOAD %s", id)
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
	log.Printf("store in bucket: %s", s.Config.Bucket)
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

func (s OpensearchStorageAdaptor) Store(id string, entity map[string]interface{}) {
	log.Print("OpensearchStorageAdaptor:TOP")
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(entity); err != nil {
		log.Fatalf("Error encoding body: %s", err)
	}
	log.Print("OpensearchStorageAdaptor index " + s.Config.Index + "for id " + id)
	req := opensearchapi.IndexRequest{
		Index:      s.Config.Index,
		DocumentID: id,
		Body:       &buf,
		Refresh:    "true",
	}
	res, err := req.Do(context.Background(), s.Config.Client)

	if res != nil && res.IsError() {
		var body []byte
		res.Body.Read(body)
		log.Print("OpensearchStorageAdaptor response error status "+res.Status(), string(body))
	}
	if err != nil {
		log.Fatalf("Error getting response: %s", err)
	}
}

func (s ElasticStorageAdaptor) Purge(m *EntityManager, entities ...map[string]interface{}) error {
	return nil
}

func (s OpensearchStorageAdaptor) Purge(m *EntityManager, entities ...map[string]interface{}) error {
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

func (s CqlAutoDiscoveryExpansionStorageAdaptor) Store(id string, entity map[string]interface{}) {

	log.Print("CqlAutoDiscoveryExpansionStorageAdaptor")

	targetField := ""

	for key, value := range entity {
		xType := fmt.Sprintf("%T", value)
		if xType == "[]interface {}" {
			targetField = key
			break
		}
	}

	if targetField != "" {

		log.Printf("Discovered target field for entity: %s", targetField)

		batch := gocql.NewBatch(gocql.UnloggedBatch)
		batch.SetConsistency(gocql.LocalQuorum)

		for _, nestedItem := range entity[targetField].([]interface{}) {

			values := make([]interface{}, 0)
			cols := make([]string, 0)
			bind := make([]string, 0)

			for field, value := range entity {
				if field != targetField {
					bind = append(bind, "?")
					cols = append(cols, strings.ToLower(field))
					xType := fmt.Sprintf("%T", value)
					log.Printf("%s : %s", strings.ToLower(field), xType)
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
			}

			for field, value := range nestedItem.(map[string]interface{}) {
				bind = append(bind, "?")
				cols = append(cols, strings.ToLower(field))
				xType := fmt.Sprintf("%T", value)
				log.Printf("%s : %s", strings.ToLower(field), xType)
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
			log.Print(stmt)
			log.Print(values...)
			batch.Query(stmt, values...)

		}

		err := s.Config.Session.ExecuteBatch(batch)
		if err != nil {
			log.Print("Expanded CQL query failure.")
			log.Panic(err)
		}

	}

}

func (s CqlAutoDiscoveryExpansionStorageAdaptor) Purge(m *EntityManager, entities ...map[string]interface{}) error {
	return nil
}

func (s GithubFileUploadAdaptor) Store(id string, entity map[string]interface{}) {

	dataBuffer := bytes.Buffer{}
	encoder := json.NewEncoder(&dataBuffer)
	encoder.SetIndent("", "\t")
	encoder.Encode(entity)
	data := []byte(dataBuffer.String())
	params := repo.CommitParams{
		Repo:     s.Config.Repo,
		Branch:   s.Config.Branch,
		Path:     s.Config.Path + "/" + id + ".json",
		Data:     &data,
		UserName: s.Config.UserName,
	}

	repo.Commit(
		s.Config.Client,
		&params,
	)

}

func (s GithubFileUploadAdaptor) Purge(m *EntityManager, entities ...map[string]interface{}) error {
	return nil
}

func (s GithubRestFileUploadAdaptor) Store(id string, entity map[string]interface{}) {

	dataBuffer := bytes.Buffer{}
	encoder := json.NewEncoder(&dataBuffer)
	encoder.SetIndent("", "\t")
	encoder.SetEscapeHTML(false)
	encoder.Encode(entity)
	data := []byte(dataBuffer.String())
	params := repo.CommitParams{
		Repo:     s.Config.Repo,
		Branch:   s.Config.Branch,
		Path:     s.Config.Path + "/" + id + ".json",
		Data:     &data,
		UserName: s.Config.UserName,
	}

	repo.CommitRestOptimized(
		s.Config.Client,
		&params,
	)

}

func (s GithubRestFileUploadAdaptor) Purge(m *EntityManager, entities ...map[string]interface{}) error {
	return nil
}

func (a NoopAuthorizationAdaptor) CanWrite(id string, m *EntityManager) (bool, map[string]interface{}) {
	return true, nil
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

func (a ResourceOrOwnerAuthorizationAdaptor) CanWrite(id string, m *EntityManager) (bool, map[string]interface{}) {

	grantAccessRequest := gov.GrantAccessRequest{
		User:                a.Config.UserId,
		Type:                gov.User,
		Resource:            a.Config.Resource,
		Operation:           gov.Write,
		Asset:               a.Config.Asset,
		AdditionalResources: *a.Config.AdditionalResources,
		LogUsageLambdaInput: m.Config.LogUsageLambdaInput,
	}

	payload, err := json.Marshal(grantAccessRequest)
	if err != nil {
		log.Printf("Error marshalling grant access request: %s", err.Error())
		return false, nil
	}

	log.Print("before grant access invoke")

	res, err := a.Config.Lambda.Invoke(&lambda.InvokeInput{FunctionName: aws.String("goclassifieds-api-" + m.Config.Stage + "-GrantAccess"), Payload: payload})
	if err != nil {
		log.Printf("error invoking grant access: %s", err.Error())
		return false, nil
	}

	var grantRes gov.GrantAccessResponse
	json.Unmarshal(res.Payload, &grantRes)

	b, _ := json.Marshal(grantRes)
	log.Print(string(b))

	entity := m.Load(id, "default")

	if entity != nil {
		// log.Printf("Check ownership of %s", id)
		userId := fmt.Sprint(entity["userId"])
		// log.Printf("Check Entity Ownership: %s == %s", userId, a.Config.UserId)
		return (userId == a.Config.UserId), entity

	}

	return grantRes.Grant, nil
}

func (a ResourceAuthorizationAdaptor) CanWrite(id string, m *EntityManager) (bool, map[string]interface{}) {

	grantAccessRequest := gov.GrantAccessRequest{
		User:                a.Config.UserId,
		Type:                gov.User,
		Resource:            a.Config.Resource,
		Operation:           gov.Write,
		Asset:               a.Config.Asset,
		AdditionalResources: *a.Config.AdditionalResources,
		LogUsageLambdaInput: m.Config.LogUsageLambdaInput,
	}

	payload, err := json.Marshal(grantAccessRequest)
	if err != nil {
		log.Printf("Error marshalling grant access request: %s", err.Error())
		return false, nil
	}

	log.Print("before grant access invoke")

	res, err := a.Config.Lambda.Invoke(&lambda.InvokeInput{FunctionName: aws.String("goclassifieds-api-" + m.Config.Stage + "-GrantAccess"), Payload: payload})
	if err != nil {
		log.Printf("error invoking grant access: %s", err.Error())
		return false, nil
	}

	var grantRes gov.GrantAccessResponse
	json.Unmarshal(res.Payload, &grantRes)

	b, _ := json.Marshal(grantRes)
	log.Print(string(b))

	return grantRes.Grant, nil
}

func (a ResourceAuthorizationEmbeddedAdaptor) CanWrite(id string, m *EntityManager) (bool, map[string]interface{}) {

	grantAccessRequest := gov.GrantAccessRequest{
		User:                a.Config.UserId,
		Type:                gov.User,
		Resource:            a.Config.Resource,
		Operation:           gov.Write,
		Asset:               a.Config.Asset,
		AdditionalResources: *a.Config.AdditionalResources,
		LogUsageLambdaInput: m.Config.LogUsageLambdaInput,
	}

	allAttributes := make([]EntityAttribute, 0)
	data := &EntityFinderDataBag{
		Attributes: allAttributes,
		Metadata: map[string]interface{}{
			"user":     grantAccessRequest.User,
			"type":     grantAccessRequest.Type,
			"resource": grantAccessRequest.Resource,
			"asset":    grantAccessRequest.Asset,
			"op":       grantAccessRequest.Operation,
		},
	}
	results := a.Config.GrantAccessManager.Find("default", "grant_access", data)

	grant := len(results) != 0

	if len(grantAccessRequest.AdditionalResources) != 0 {
		for _, r := range grantAccessRequest.AdditionalResources {
			if r.User == grantAccessRequest.User && r.Type == grantAccessRequest.Type && r.Resource == grantAccessRequest.Resource && r.Asset == grantAccessRequest.Asset && r.Operation == grantAccessRequest.Operation {
				grant = true
				break
			}
		}
	}

	return grant, nil
}

func (v DefaultValidatorAdaptor) Validate(entity map[string]interface{}, m *EntityManager) (*EntityValidationResponse, error) {

	valRes := &EntityValidationResponse{}

	request := ValidateEntityRequest{
		EntityName:          m.Config.SingularName,
		Entity:              entity,
		UserId:              v.Config.UserId,
		Site:                v.Config.Site,
		LogUsageLambdaInput: m.Config.LogUsageLambdaInput,
	}

	payload, err := json.Marshal(request)
	if err != nil {
		log.Printf("Error marshalling entity validation request: %s", err.Error())
		valRes.Entity = entity
		return valRes, errors.New("Error marshalling entity validation request")
	}

	var validateRes ValidateEntityResponse

	res, err := v.Config.Lambda.Invoke(&lambda.InvokeInput{FunctionName: aws.String("goclassifieds-api-" + m.Config.Stage + "-ValidateEntity"), Payload: payload})
	if err != nil {
		log.Printf("error invoking entity validation: %s", err.Error())
		valRes.Entity = entity
		return valRes, errors.New("Error invoking validation")
	}

	json.Unmarshal(res.Payload, &validateRes)

	if validateRes.Unauthorized {
		log.Printf("Unauthorized to create entity")
		valRes.Entity = entity
		return valRes, errors.New("Unauthorized to create entity")
	}

	if validateRes.Valid {
		log.Printf("Lambda Response valid default")
		valRes.Entity = validateRes.Entity
		return valRes, nil
	}

	valRes.Entity = entity
	return valRes, errors.New("Entity invalid")
}

func (v ContractValidatorAdaptor) Validate(entity map[string]interface{}, m *EntityManager) (*EntityValidationResponse, error) {

	// Test schema - replace with discovery
	/*schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"foo": map[string]interface{}{
				"type": "integer",
			},
			"bar": map[string]interface{}{
				"type": "string",
			},
		},
		"required":             []string{"foo"},
		"additionalProperties": false,
	}*/

	valRes := &EntityValidationResponse{}
	var contract map[string]interface{}

	pieces := strings.Split(v.Config.Repo, "/")
	opts := &github.RepositoryContentGetOptions{
		Ref: v.Config.Branch,
	}

	file, _, res, err := v.Config.Client.Repositories.GetContents(context.Background(), pieces[0], pieces[1], v.Config.Contract, opts)
	if err != nil || res.StatusCode != 200 {
		log.Print("No contract detected for " + v.Config.Contract)
		entity["userId"] = v.Config.UserId
		valRes.Entity = entity
		return valRes, nil
	}
	if err == nil && file != nil && file.Content != nil {
		content, err := base64.StdEncoding.DecodeString(*file.Content)
		if err == nil {
			json.Unmarshal(content, &contract)
		} else {
			entity["userId"] = v.Config.UserId
			valRes.Entity = entity
			return valRes, errors.New("Invalid contract unable to parse.")
		}
	}

	request := EnforceContractRequest{
		EntityName:          m.Config.SingularName,
		Entity:              entity,
		Contract:            contract,
		UserId:              v.Config.UserId,
		Site:                v.Config.Site,
		LogUsageLambdaInput: m.Config.LogUsageLambdaInput,
	}

	payload, err := json.Marshal(request)
	if err != nil {
		log.Printf("Error marshalling entity validation request: %s", err.Error())
		valRes.Entity = entity
		return valRes, errors.New("Error marshalling entity validation request")
	}

	var contractRes EnforceContractResponse

	res2, err2 := v.Config.Lambda.Invoke(&lambda.InvokeInput{FunctionName: aws.String("goclassifieds-api-" + m.Config.Stage + "-EnforceContract"), Payload: payload})
	if err2 != nil {
		log.Printf("error invoking entity validation: %s", err2.Error())
		valRes.Entity = entity
		return valRes, errors.New("Error invoking validation")
	}

	json.Unmarshal(res2.Payload, &contractRes)

	if contractRes.Unauthorized {
		log.Printf("Unauthorized to create entity")
		valRes.Entity = entity
		return valRes, errors.New("Unauthorized to create entity")
	}

	if contractRes.Valid {
		log.Printf("Lambda Response valid contract")
		valRes.Entity = contractRes.Entity
		return valRes, nil
	}

	valRes.Entity = entity
	valRes.Errors = contractRes.Errors
	return valRes, errors.New("Entity invalid")

}

func (c DefaultCreatorAdaptor) Create(entity map[string]interface{}, m *EntityManager) (*CreateEntityResponse, error) {

	res := &CreateEntityResponse{}

	write, _ := m.Allow("", "write", "default")
	if !write {
		log.Printf("not allowed to write entity %s", entity)
		res.Entity = entity
		res.Success = false
		return res, errors.New("unauthorized to write entity.")
	}

	validateRes, err := m.Validate("default", entity)

	/*request := ValidateEntityRequest{
		EntityName:          m.Config.SingularName,
		Entity:              entity,
		UserId:              c.Config.UserId,
		Site:                c.Config.Site,
		LogUsageLambdaInput: m.Config.LogUsageLambdaInput,
	}

	write, _ := m.Allow("", "write", "default")
	if !write {
		log.Printf("not allowed to write entity %s", entity)
		return entity, errors.New("unauthorized to write entity.")
	}

	payload, err := json.Marshal(request)
	if err != nil {
		log.Printf("Error marshalling entity validation request: %s", err.Error())
		return entity, errors.New("Error marshalling entity validation request")
	}

	var validateRes ValidateEntityResponse

	if m.Config.CloudName == "azure" {

		entity["userId"] = request.UserId

		validateRes = ValidateEntityResponse{
			Entity:       entity,
			Valid:        true,
			Unauthorized: false,
		}

	} else {

		res, err := c.Config.Lambda.Invoke(&lambda.InvokeInput{FunctionName: aws.String("goclassifieds-api-" + m.Config.Stage + "-ValidateEntity"), Payload: payload})
		if err != nil {
			log.Printf("error invoking entity validation: %s", err.Error())
			return entity, errors.New("Error invoking validation")
		}

		json.Unmarshal(res.Payload, &validateRes)
	}

	if validateRes.Unauthorized {
		log.Printf("Unauthorized to create entity")
		return entity, errors.New("Unauthorized to create entity")
	}

	if validateRes.Valid {
		log.Printf("Lambda Response valid")
		m.Save(validateRes.Entity, c.Config.Save)
		return validateRes.Entity, nil
	}*/

	if err == nil {
		log.Printf("Entity passes validation")
		m.Save(validateRes.Entity, c.Config.Save)
		res.Entity = validateRes.Entity
		res.Success = true
		return res, nil
	}

	res.Entity = entity
	res.Success = false
	res.Errors = validateRes.Errors

	return res, nil
}

func (c EntityTypeCreatorAdaptor) Create(entity map[string]interface{}, m *EntityManager) (*CreateEntityResponse, error) {

	log.Print("EntityTypeCreatorAdaptor: 1")

	res := &CreateEntityResponse{}

	jsonData, err := json.Marshal(entity)
	if err != nil {
		res.Entity = entity
		res.Success = false
		return res, err
	}

	log.Print("EntityTypeCreatorAdaptor: 2")

	var obj EntityType
	err = json.Unmarshal(jsonData, &obj)
	if err != nil {
		res.Entity = entity
		res.Success = false
		return res, err
	}

	log.Print("EntityTypeCreatorAdaptor: 3")

	obj.Id = utils.GenerateId()
	obj.UserId = c.Config.UserId

	log.Print("EntityTypeCreatorAdaptor: 4")

	validate := validator.New()
	err = validate.Struct(obj)
	if err != nil {
		res.Entity = entity
		res.Success = false
		return res, err.(validator.ValidationErrors)
	}

	log.Print("EntityTypeCreatorAdaptor: 5")

	newEntity, _ := TypeToEntity(&obj)
	m.Save(newEntity, c.Config.Save)

	log.Print("EntityTypeCreatorAdaptor: 6")

	res.Entity = newEntity
	res.Success = true

	return res, nil

}

func (c DefaultUpdatorAdaptor) Update(entity map[string]interface{}, m *EntityManager) (*UpdateEntityResponse, error) {

	res := &UpdateEntityResponse{}

	write, _ := m.Allow(entity[m.Config.IdKey].(string), "write", "default")
	if !write {
		log.Printf("not allowed to write to entity %s", entity[m.Config.IdKey].(string))
		res.Entity = entity
		res.Success = false
		return res, errors.New("unauthorized to write to entity.")
	}

	validateRes, err := m.Validate("default", entity)

	/*request := ValidateEntityRequest{
		EntityName:          m.Config.SingularName,
		Entity:              entity,
		UserId:              c.Config.UserId,
		LogUsageLambdaInput: m.Config.LogUsageLambdaInput,
	}

	write, _ := m.Allow(entity[m.Config.IdKey].(string), "write", "default")
	if !write {
		log.Printf("not allowed to write to entity %s", entity[m.Config.IdKey].(string))
		return entity, errors.New("unauthorized to write to entity.")
	}

	payload, err := json.Marshal(request)
	if err != nil {
		log.Printf("Error marshalling entity validation request: %s", err.Error())
		return entity, errors.New("Error marshalling entity validation request")
	}

	var validateRes ValidateEntityResponse

	if m.Config.CloudName == "azure" {

		entity["userId"] = request.UserId

		validateRes = ValidateEntityResponse{
			Entity:       entity,
			Valid:        true,
			Unauthorized: false,
		}

	} else {

		res, err := c.Config.Lambda.Invoke(&lambda.InvokeInput{FunctionName: aws.String("goclassifieds-api-" + m.Config.Stage + "-ValidateEntity"), Payload: payload})
		if err != nil {
			log.Printf("error invoking entity validation: %s", err.Error())
			return entity, errors.New("Error invoking validation")
		}

		json.Unmarshal(res.Payload, &validateRes)
	}

	if validateRes.Unauthorized {
		log.Printf("Unauthorized to update entity")
		return entity, errors.New("Unauthorized to update entity")
	}

	if validateRes.Valid {
		log.Printf("Lambda Response valid")
		m.Save(validateRes.Entity, c.Config.Save)
		return validateRes.Entity, nil
	}*/

	if err == nil {
		log.Printf("Entity passes validation")
		m.Save(validateRes.Entity, c.Config.Save)
		res.Entity = validateRes.Entity
		res.Success = true
		return res, nil
	}

	res.Success = false
	res.Entity = entity
	res.Errors = validateRes.Errors

	return res, nil
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

func (f OpensearchTemplateFinder) Find(query string, data *EntityFinderDataBag) []map[string]interface{} {

	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(data); err != nil {
		log.Fatalf("Error encoding search query: %s", err)
	}
	log.Printf("template data: %s", b.String())

	hits := os.ExecuteQuery(f.Config.Client, os.TemplateBuilder{
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

	log.Printf("cql query: %s", tb.String())
	for _, value := range f.Config.Bindings.Values {
		log.Printf("binding: %v", value)
	}

	rows := make([]map[string]interface{}, 0)
	iter := f.Config.Session.Query(tb.String(), f.Config.Bindings.Values...).Consistency(gocql.LocalQuorum).Iter()
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

	var loader Loader
	var finder Finder
	var storage Storage
	var authorizer Authorization
	var creator Creator

	switch name {
	case "s3/loader":
		loader = S3LoaderAdaptor{
			Config: S3AdaptorConfig{
				Session: s.Session,
			},
		}
		break
	case "cognito/loader":
		loader = CognitoLoaderAdaptor{
			Config: CognitoAdaptorConfig{
				Client: s.Cognito,
			},
		}
		break
	case "finder/loader":
		loader = FinderLoaderAdaptor{}
		break
	case "elastic/templatefinder":
		finder = ElasticTemplateFinder{
			Config: ElasticTemplateFinderConfig{
				Client:   s.Elastic,
				Template: s.Template,
			},
		}
		break
	case "entitytypefinder":
		finder = DefaultEntityTypeFinder{
			Config: DefaultEntityTypeFinderConfig{
				Template: s.Template,
			},
		}
		break
	case "cql/templatefinder":
		finder = CqlTemplateFinder{
			Config: CqlTemplateFinderConfig{
				Session:  s.Cql,
				Template: s.Template,
				Bindings: s.Bindings,
			},
		}
		break
	case "s3/storage":
		storage = S3StorageAdaptor{
			Config: S3AdaptorConfig{
				Session: s.Session,
			},
		}
		break
	case "elastic/storage":
		storage = ElasticStorageAdaptor{
			Config: ElasticAdaptorConfig{
				Client: s.Elastic,
			},
		}
		break
	case "cql/storage":
		storage = CqlStorageAdaptor{
			Config: CqlAdaptorConfig{
				Session: s.Cql,
			},
		}
		break
	case "owner/authorizer":
		authorizer = OwnerAuthorizationAdaptor{}
		break
	case "default/creator":
		creator = DefaultCreatorAdaptor{
			Config: DefaultCreatorConfig{
				Lambda: s.Lambda,
			},
		}
		break
	case "entitytype/creator":
		creator = EntityTypeCreatorAdaptor{
			Config: DefaultCreatorConfig{
				Lambda: s.Lambda,
			},
		}
		break
	default:
		return nil, errors.New("adaptor does not exist by that name")
	}

	//factory := map[string]interface{}{
	/*"s3/loader": S3LoaderAdaptor{
		Config: S3AdaptorConfig{
			Session: s.Session,
		},
	},*/
	/*"cognito/loader": CognitoLoaderAdaptor{
		Config: CognitoAdaptorConfig{
			Client: s.Cognito,
		},
	},*/
	/*"finder/loader": FinderLoaderAdaptor{},*/
	/*"elastic/templatefinder": ElasticTemplateFinder{
		Config: ElasticTemplateFinderConfig{
			Client:   s.Elastic,
			Template: s.Template,
		},
	},*/
	/*"entitytypefinder": DefaultEntityTypeFinder{
		Config: DefaultEntityTypeFinderConfig{
			Template: s.Template,
		},
	},*/
	/*"cql/templateFinder": CqlTemplateFinder{
		Config: CqlTemplateFinderConfig{
			Session:  s.Cql,
			Template: s.Template,
			Bindings: s.Bindings,
		},
	},*/
	/*"s3/storage": S3StorageAdaptor{
		Config: S3AdaptorConfig{
			Session: s.Session,
		},
	},*/
	/*"elastic/storage": ElasticStorageAdaptor{
		Config: ElasticAdaptorConfig{
			Client: s.Elastic,
		},
	},*/
	/*"cql/storage": CqlStorageAdaptor{
		Config: CqlAdaptorConfig{
			Session: s.Cql,
		},
	},*/
	/*"owner/authorizer": OwnerAuthorizationAdaptor{},*/
	/*"default/creator": DefaultCreatorAdaptor{
		Config: DefaultCreatorConfig{
			Lambda: s.Lambda,
		},
	},*/
	/*"entitytype/creator": EntityTypeCreatorAdaptor{
		Config: DefaultCreatorConfig{
			Lambda: s.Lambda,
		},
	},*/
	//}

	/*if _, ok := factory[name]; !ok {
		return nil, errors.New("adaptor does not exist by that name")
	}*/

	/*jsonData, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}*/

	if strings.Index(name, "finder") > -1 {
		log.Print(name)
		log.Printf("%v", finder)
		err := mapstructure.Decode(c, finder)
		if err != nil {
			log.Print(err)
			return nil, err
		}
		return finder, nil
	} else if strings.Index(name, "loader") > -1 {
		log.Print(name)
		log.Printf("%v", loader)
		err := mapstructure.Decode(c, loader)
		if err != nil {
			log.Print(err)
			return nil, err
		}
		return loader, nil
	} else if strings.Index(name, "storage") > -1 {
		log.Print(name)
		log.Printf("%v", storage)
		err := mapstructure.Decode(c, storage)
		if err != nil {
			log.Print(err)
			return nil, err
		}
		return storage, nil
	} else if strings.Index(name, "authorizer") > -1 {
		log.Print(name)
		log.Printf("%v", authorizer)
		err := mapstructure.Decode(c, authorizer)
		if err != nil {
			log.Print(err)
			return nil, err
		}
		return authorizer, nil
	} else if strings.Index(name, "creator") > -1 {
		log.Print(name)
		log.Printf("%v", creator)
		err := mapstructure.Decode(c, creator)
		if err != nil {
			log.Print(err)
			return nil, err
		}
		return creator, nil
	} else {
		log.Print("unmatched")
		return nil, errors.New("unmatched adaptor")
	}

}

func GetManager(entityName string, c map[string]interface{}, s *EntityAdaptorConfig) (*EntityManager, error) {

	adaptors := []string{
		"finders",
		"loaders",
		"storages",
		"authorizers",
	}

	finders := make(map[string]Finder)
	loaders := make(map[string]Loader)
	storages := make(map[string]Storage)
	authorizers := make(map[string]Authorization)
	var creator Creator

	if item, ok := c["creator"]; ok {
		instance, err := GetAdaptor(fmt.Sprint(item.(map[string]interface{})["factory"]), s, item.(map[string]interface{}))
		if err != nil {
			return &EntityManager{}, err
		}
		creator = instance.(Creator)
	}

	for _, adaptor := range adaptors {
		if _, ok := c[adaptor]; ok {
			for name, item := range c[adaptor].(map[string]interface{}) {
				instance, err := GetAdaptor(fmt.Sprint(item.(map[string]interface{})["factory"]), s, item.(map[string]interface{}))
				if err != nil {
					return &EntityManager{}, err
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
				default:
					return &EntityManager{}, errors.New("invalid adaptor detected")
				}
			}
		}
	}

	manager := EntityManager{
		Config: EntityConfig{
			SingularName: entityName,
			PluralName:   inflector.Pluralize(entityName),
			IdKey:        "id",
			Stage:        "",
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
			SingularName:        config.SingularName,
			PluralName:          config.PluralName,
			IdKey:               "id",
			Stage:               config.Stage,
			CloudName:           config.CloudName,
			LogUsageLambdaInput: config.LogUsageLambdaInput,
		},
		Creator: DefaultCreatorAdaptor{
			Config: DefaultCreatorConfig{
				Lambda: config.Lambda,
				UserId: config.UserId,
				Site:   config.Site,
				Save:   "default",
			},
		},
		Updator: DefaultUpdatorAdaptor{
			Config: DefaultUpdatorConfig{
				Lambda: config.Lambda,
				UserId: config.UserId,
				Site:   config.Site,
				Save:   "default",
			},
		},
		Validators: map[string]Validator{
			"default": DefaultValidatorAdaptor{
				Config: DefaultValidatorConfig{
					Lambda: config.Lambda,
					UserId: config.UserId,
					Site:   config.Site,
				},
			},
		},
		Finders: map[string]Finder{
			"default": OpensearchTemplateFinder{
				Config: OpensearchTemplateFinderConfig{
					Index:         config.Index,
					Client:        config.OsClient,
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
					Bucket:  config.BucketName,
					Prefix:  config.PluralName + "/",
				},
			},
		},
		Storages: map[string]Storage{
			"default": S3StorageAdaptor{
				Config: S3AdaptorConfig{
					Session: config.Session,
					Bucket:  config.BucketName,
					Prefix:  config.PluralName + "/",
				},
			},
			"opensearch": OpensearchStorageAdaptor{
				Config: OpensearchAdaptorConfig{
					Index:  config.Index,
					Client: config.OsClient,
				},
			},
		},
		Authorizers: map[string]Authorization{},
		Hooks: map[Hooks]EntityHook{},
	}
}

func NewEntityTypeManager(config DefaultManagerConfig) EntityManager {
	return EntityManager{
		Config: EntityConfig{
			SingularName:        "type",
			PluralName:          "types",
			IdKey:               "id",
			Stage:               config.Stage,
			CloudName:           config.CloudName,
			LogUsageLambdaInput: config.LogUsageLambdaInput,
		},
		Creator: EntityTypeCreatorAdaptor{
			Config: DefaultCreatorConfig{
				Lambda: config.Lambda,
				UserId: config.UserId,
				Site:   config.Site,
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
			"default": OpensearchStorageAdaptor{
				Config: OpensearchAdaptorConfig{
					Index:  "classified_types",
					Client: config.OsClient,
				},
			},
		},
		Authorizers: map[string]Authorization{},
		Hooks: map[Hooks]EntityHook{},
	}
}