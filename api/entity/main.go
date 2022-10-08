package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"
	"time"

	"goclassifieds/lib/entity"
	"goclassifieds/lib/gov"
	"goclassifieds/lib/sign"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	session "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	lambda2 "github.com/aws/aws-sdk-go/service/lambda"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/go-github/v46/github"
	"github.com/mitchellh/mapstructure"
	opensearch "github.com/opensearch-project/opensearch-go"
	"github.com/shurcooL/githubv4"
	"github.com/tangzero/inflector"
	"golang.org/x/oauth2"
)

// var ginLambda *ginadapter.GinLambda
var handler Handler

type Handler func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error)

type TemplateQueryFunc func(e string, data *entity.EntityFinderDataBag) []map[string]interface{}
type TemplateLambdaFunc func(e string, userId string, data []map[string]interface{}) entity.EntityDataResponse
type TemplateUserIdFunc func(req *events.APIGatewayProxyRequest) string

type ActionContext struct {
	EsClient         *elasticsearch7.Client
	OsClient         *opensearch.Client
	GithubV4Client   *githubv4.Client
	GithubRestClient *github.Client
	Session          *session.Session
	Lambda           *lambda2.Lambda
	Cognito          *cognitoidentityprovider.CognitoIdentityProvider
	TypeManager      entity.Manager
	EntityManager    entity.Manager
	EntityName       string
	Template         *template.Template
	TemplateName     string
	Implementation   string
	BucketName       string
	Stage            string
	Site             string
	UserPoolId       string
	GithubAppPem     []byte
}

func GetEntities(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {
	var res events.APIGatewayProxyResponse
	pathPieces := strings.Split(req.Path, "/")

	query := inflector.Pluralize(ac.EntityName)
	if len(pathPieces) == 4 && pathPieces[3] != "" {
		query = pathPieces[3]
	} else if ac.TemplateName != "" {
		query = ac.TemplateName
	}

	log.Printf("entity: %s | query: %s", ac.EntityName, query)

	typeId := req.QueryStringParameters["typeId"]
	allAttributes := make([]entity.EntityAttribute, 0)

	if typeId != "" {
		objType := ac.TypeManager.Load(typeId, "default")
		var entType entity.EntityType
		mapstructure.Decode(objType, &entType)
		var b bytes.Buffer
		if err := json.NewEncoder(&b).Encode(objType); err != nil {
			log.Fatalf("Error encoding obj type: %s", err)
		}
		log.Printf("obj type: %s", b.String())
		for _, attribute := range entType.Attributes {
			flatAttributes := entity.FlattenEntityAttribute(attribute)
			for _, flatAttribute := range flatAttributes {
				log.Printf("attribute: %s", flatAttribute.Name)
				allAttributes = append(allAttributes, flatAttribute)
			}
		}
	}
	data := entity.EntityFinderDataBag{
		Req:        req,
		Attributes: allAttributes,
	}
	entities := ac.EntityManager.Find(ac.Implementation, query, &data)
	body, err := json.Marshal(entities)
	if err != nil {
		return res, err
	}
	res.StatusCode = 200
	res.Headers = map[string]string{
		"Content-Type": "application/json",
	}
	res.Body = string(body[:])
	return res, nil
}

func GetEntity(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {
	var res events.APIGatewayProxyResponse
	pathPieces := strings.Split(req.Path, "/")
	id := pathPieces[3]
	log.Printf("entity by id: %s", id)
	ent := ac.EntityManager.Load(id, ac.Implementation)
	body, err := json.Marshal(ent)
	if err != nil {
		return res, err
	}
	res.StatusCode = 200
	res.Headers = map[string]string{
		"Content-Type": "application/json",
	}
	res.Body = string(body[:])
	return res, nil
}

func CreateEntity(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {
	var e map[string]interface{}
	res := events.APIGatewayProxyResponse{StatusCode: 500}
	body := []byte(req.Body)
	json.Unmarshal(body, &e)
	newEntity, err := ac.EntityManager.Create(e)
	if err != nil {
		if strings.Contains(err.Error(), "unauthorized") {
			res.StatusCode = 403
			res.Body = err.Error()
		}
		return res, nil
	}
	resBody, err := json.Marshal(newEntity)
	if err != nil {
		return res, err
	}
	res.StatusCode = 200
	res.Headers = map[string]string{
		"Content-Type": "application/json",
	}
	res.Body = string(resBody)
	return res, nil
}

func UpdateEntity(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {
	var e map[string]interface{}
	res := events.APIGatewayProxyResponse{StatusCode: 500}
	body := []byte(req.Body)
	json.Unmarshal(body, &e)
	newEntity, err := ac.EntityManager.Update(e)
	if err != nil {
		return res, err
	}
	resBody, err := json.Marshal(newEntity)
	if err != nil {
		return res, err
	}
	res.StatusCode = 200
	res.Headers = map[string]string{
		"Content-Type": "application/json",
	}
	res.Body = string(resBody)
	return res, nil
}

func InitializeHandler(c *ActionContext) Handler {
	return func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

		ac := RequestActionContext(c, req)

		b, _ := json.Marshal(req)
		log.Print(string(b))

		pathPieces := strings.Split(req.Path, "/")
		entityName := pathPieces[2]

		if len(pathPieces) > 3 && pathPieces[3] == "shapeshifter" {
			entityName = pathPieces[3]
		}

		if index := strings.Index(entityName, "list"); index > -1 && entityName != "shapeshifter" {
			entityName = inflector.Pluralize(entityName[0:index])
			if entityName == "features" {
				entityName = "ads"
				ac.TemplateName = "featurelistitems"
				ac.Implementation = "features"
			}
		} else if entityName == "adprofileitems" {
			entityName = "profiles"
			ac.TemplateName = "profilenavitems"
		} else if entityName == "adtypes" {
			entityName = "types"
			ac.TemplateName = "all"
		} else if entityName == "adprofile" {
			entityName = "profile"
		}

		pluralName := inflector.Pluralize(entityName)
		singularName := inflector.Singularize(entityName)
		ac.EntityName = singularName
		userId := GetUserId(req)

		log.Printf("entity plural name: %s", pluralName)
		log.Printf("entity singular name: %s", singularName)

		ac.TypeManager = entity.NewEntityTypeManager(entity.DefaultManagerConfig{
			SingularName:   "type",
			PluralName:     "types",
			Index:          "classified_types",
			EsClient:       ac.EsClient,
			OsClient:       ac.OsClient,
			GithubV4Client: ac.GithubV4Client,
			Session:        ac.Session,
			Lambda:         ac.Lambda,
			Template:       ac.Template,
			UserId:         userId,
			BucketName:     ac.BucketName,
			Stage:          ac.Stage,
		})

		if singularName == "type" {
			ac.EntityManager = ac.TypeManager
		} else {
			ac.EntityManager = entity.NewDefaultManager(entity.DefaultManagerConfig{
				SingularName:   singularName,
				PluralName:     pluralName,
				Index:          "classified_" + pluralName,
				EsClient:       ac.EsClient,
				OsClient:       ac.OsClient,
				GithubV4Client: ac.GithubV4Client,
				Session:        ac.Session,
				Lambda:         ac.Lambda,
				Template:       ac.Template,
				UserId:         userId,
				BucketName:     ac.BucketName,
				Stage:          ac.Stage,
				Site:           ac.Site,
			})
			/*manager, err := entity.GetManager(
				singularName,
				map[string]interface{}{
					"creator": map[string]interface{}{
						"factory": "default/creator",
						"config": map[string]interface{}{
							"userId": userId,
							"save":   "default",
						},
					},
					"finders": map[string]interface{}{
						"default": map[string]interface{}{
							"factory": "elastic/templatefinder",
							"config": map[string]interface{}{
								"index":         "classifieds_" + pluralName,
								"collectionKey": "hits.hits",
								"objectKey":     "_source",
							},
						},
					},
					"loaders": map[string]interface{}{
						"default": map[string]interface{}{
							"factory": "s3/loader",
							"config": map[string]interface{}{
								"bucket": "classifieds-ui-dev",
								"prefix": pluralName + "/",
							},
						},
					},
					"storages": map[string]interface{}{
						"default": map[string]interface{}{
							"factory": "s3/storage",
							"config": map[string]interface{}{
								"prefix": pluralName + "/",
								"bucket": "classifieds-ui-dev",
							},
						},
						"elastic": map[string]interface{}{
							"factory": "elastic/storage",
							"config": map[string]interface{}{
								"index": "classifieds_" + pluralName,
							},
						},
					},
				},
				&entity.EntityAdaptorConfig{
					Session:  ac.Session,
					Template: ac.Template,
					Lambda:   ac.Lambda,
					Elastic:  ac.EsClient,
				},
			)*/
			/*if err != nil {
				log.Print("Error creating entity manager")
				log.Print(err)
			}
			ac.EntityManager = manager*/
		}

		// Default to using owner authoization fo all entities.
		if singularName == "panelpage" {
			suffix := ""
			if os.Getenv("STAGE") == "prod" {
				suffix = "-prod"
			}
			ac.EntityManager.AddAuthorizer("default", entity.ResourceOrOwnerAuthorizationAdaptor{
				Config: entity.ResourceOrOwnerAuthorizationConfig{
					UserId:   userId,
					Site:     ac.Site,
					Resource: gov.GithubRepo,
					Asset:    "rollthecloudinc/" + req.PathParameters["site"] + "-objects" + suffix,
					Lambda:   ac.Lambda,
				},
			})
		} else if singularName == "shapeshifter" {
			// ac.EntityManager.AddAuthorizer("default", entity.NoopAuthorizationAdaptor{})
			ac.EntityManager.AddAuthorizer("default", entity.ResourceOrOwnerAuthorizationAdaptor{
				Config: entity.ResourceOrOwnerAuthorizationConfig{
					UserId:   userId,
					Site:     ac.Site,
					Resource: gov.GithubRepo,
					Asset:    req.PathParameters["owner"] + "/" + req.PathParameters["repo"],
					Lambda:   ac.Lambda,
				},
			})
		} else {
			ac.EntityManager.AddAuthorizer("default", entity.OwnerAuthorizationAdaptor{
				Config: entity.OwnerAuthorizationConfig{
					UserId: userId,
					Site:   ac.Site,
				},
			})
		}

		if singularName == "ad" {
			collectionKey := "aggregations.features.features_filtered.feature_names.buckets"
			if req.QueryStringParameters["featureSearchString"] == "" {
				collectionKey = "aggregations.features.feature_names.buckets"
			}
			ac.EntityManager.AddFinder("features", entity.OpensearchTemplateFinder{
				Config: entity.OpensearchTemplateFinderConfig{
					Index:         "classified_" + pluralName,
					Client:        ac.OsClient,
					Template:      ac.Template,
					CollectionKey: collectionKey,
					ObjectKey:     "",
				},
			})
		}

		if singularName == "panelpage" {
			ac.EntityManager.AddLoader("default", entity.GithubFileLoaderAdaptor{
				Config: entity.GithubFileUploadConfig{
					Client: ac.GithubV4Client,
					Repo:   "rollthecloudinc/" + req.PathParameters["site"] + "-objects", // @todo: Hard coded to test integration for now.
					Branch: os.Getenv("GITHUB_BRANCH"),                                   // This will cone env vars from inside json file passed via serverless.
					Path:   "panelpage",                                                  // path to place stuff. This will probably be a separate repo or directory udnerneath assets.
				},
			})
			ac.EntityManager.AddStorage("default", entity.GithubFileUploadAdaptor{
				Config: entity.GithubFileUploadConfig{
					Client: ac.GithubV4Client,
					Repo:   "rollthecloudinc/" + req.PathParameters["site"] + "-objects", // @todo: Hard coded to test integration for now.
					Branch: os.Getenv("GITHUB_BRANCH"),                                   // This will cone env vars from inside json file passed via serverless.
					Path:   "panelpage",                                                  // path to place stuff. This will probably be a separate repo or directory udnerneath assets.
				},
			})
		} else if singularName == "shapeshifter" {
			ac.EntityManager.AddLoader("default", entity.GithubFileLoaderAdaptor{
				Config: entity.GithubFileUploadConfig{
					Client: ac.GithubV4Client,
					Repo:   req.PathParameters["owner"] + "/" + req.PathParameters["repo"],
					Branch: os.Getenv("GITHUB_BRANCH"),
					Path:   req.PathParameters["proxy"],
				},
			})
			ac.EntityManager.AddStorage("default", entity.GithubRestFileUploadAdaptor{
				Config: entity.GithubRestFileUploadConfig{
					Client: ac.GithubRestClient,
					Repo:   req.PathParameters["owner"] + "/" + req.PathParameters["repo"],
					Branch: os.Getenv("GITHUB_BRANCH"),
					Path:   req.PathParameters["proxy"],
				},
			})
		}

		if entityName == pluralName && req.HTTPMethod == "GET" {
			return GetEntities(req, ac)
		} else if entityName == singularName && req.HTTPMethod == "GET" {
			return GetEntity(req, ac)
		} else if entityName == singularName && req.HTTPMethod == "POST" {
			return CreateEntity(req, ac)
		} else if entityName == singularName && req.HTTPMethod == "PUT" {
			return UpdateEntity(req, ac)
		}

		return events.APIGatewayProxyResponse{StatusCode: 500}, nil
	}
}

func TemplateQuery(ac *ActionContext) TemplateQueryFunc {

	return func(e string, data *entity.EntityFinderDataBag) []map[string]interface{} {

		pieces := strings.Split(e, "/")
		pluralName := inflector.Pluralize(pieces[0])
		singularName := inflector.Singularize(pieces[0])

		query := pluralName
		if len(pieces) == 2 {
			query = pieces[1]
		}

		entityManager := entity.NewDefaultManager(entity.DefaultManagerConfig{
			SingularName: singularName,
			PluralName:   pluralName,
			Index:        "classified_" + pluralName,
			EsClient:     ac.EsClient,
			OsClient:     ac.OsClient,
			Session:      ac.Session,
			Lambda:       ac.Lambda,
			Template:     ac.Template,
			UserId:       "",
			BucketName:   ac.BucketName,
			Stage:        ac.Stage,
		})

		/*data := entity.EntityFinderDataBag{
			Query:  queryData,
			UserId: userId,
		}*/

		/*data := entity.EntityFinderDataBag{
			Query:  make(map[string][]string, 0),
			UserId: "",
		}*/

		// @todo: allow third piece to specify type so that attributes can be replaced in data bag.
		// Will need to clone data bag and swap out attributes.
		// Limit nesting to avoid infinite loop -- only three levels allowed. Should be enough.

		entities := entityManager.Find("default", query, data)

		log.Print("TemplateQuery 8")
		return entities

	}

}

func TemplateLambda(ac *ActionContext) TemplateLambdaFunc {
	return func(e string, userId string, data []map[string]interface{}) entity.EntityDataResponse {

		pieces := strings.Split(e, "/")
		pluralName := inflector.Pluralize(pieces[0])
		singularName := inflector.Singularize(pieces[0])

		functionName := pluralName
		if len(pieces) == 2 {
			functionName = "goclassifieds-api-" + ac.Stage + "-" + pieces[1]
		}

		request := entity.EntityDataRequest{
			EntityName: singularName,
			UserId:     userId,
			Data:       data,
		}

		res, err := entity.ExecuteEntityLambda(ac.Lambda, functionName, &request)
		if err != nil {
			log.Printf("error invoking template lambda: %s", err.Error())
		}

		return res

	}
}

func TemplateUserId(ac *ActionContext) TemplateUserIdFunc {
	return func(req *events.APIGatewayProxyRequest) string {
		return GetUserId(req)
	}
}

func RequestActionContext(ac *ActionContext, req *events.APIGatewayProxyRequest) *ActionContext {

	pathPieces := strings.Split(req.Path, "/")

	var githubToken string
	var githubRestClient *github.Client
	var srcToken oauth2.TokenSource

	if len(pathPieces) < 4 || pathPieces[3] != "shapeshifter" {
		githubToken = os.Getenv("GITHUB_TOKEN")
		srcToken = oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: githubToken},
		)
	} else {
		/*log.Print("shapeshifter detected")
		username := GetUsername(req)
		getUserInput := &cognitoidentityprovider.AdminGetUserInput{
			Username:   aws.String(username),
			UserPoolId: aws.String(ac.UserPoolId),
		}
		user, err := ac.Cognito.AdminGetUser(getUserInput)
		if err == nil {
			for _, attr := range user.UserAttributes {
				log.Print("attribute " + *attr.Name)
				if *attr.Name == "custom:githubAccessToken" {
					log.Print("custom:githubAccessToken detected")
					githubToken = *attr.Value
					log.Print("github token " + githubToken)
					break
				}
			}
		} else {
			log.Print(err.Error())
		}*/
		log.Print("shapeshifter detected")
		pk, err := jwt.ParseRSAPrivateKeyFromPEM(ac.GithubAppPem)
		if err != nil {
			log.Print("Error parsing github app pem")
		}
		log.Print("Parsed github app pem")
		token := jwt.New(jwt.SigningMethodRS256)
		claims := token.Claims.(jwt.MapClaims)
		claims["iat"] = time.Now().Add(-60 * time.Second).Unix()
		claims["exp"] = time.Now().Add(10 * time.Minute).Unix()
		claims["iss"] = os.Getenv("GITHUB_APP_ID")
		tokenString, err := token.SignedString(pk)
		if err != nil {
			log.Print("Error signing token", err.Error())
		}
		log.Print("Token string " + tokenString)
		listOpts := &github.ListOptions{
			Page:    1,
			PerPage: 100,
		}
		srcToken := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: tokenString},
		)
		httpClient := oauth2.NewClient(context.Background(), srcToken)
		githubRestClient = github.NewClient(httpClient)
		installations, _, err := githubRestClient.Apps.ListInstallations(context.Background(), listOpts)
		if err != nil {
			log.Print("Error listing installations", err.Error())
		}
		var targetInstallation *github.Installation
		if err == nil {
			log.Printf("Has instllations %d", len(installations))
			for _, installation := range installations {
				log.Print("installation account login ", installation.Account.Login)
				if *installation.Account.Login == req.PathParameters["owner"] {
					targetInstallation = installation
				}
			}
		}
		if targetInstallation != nil {
			log.Printf("matched installation %d", targetInstallation.ID)
			tokenOpts := &github.InstallationTokenOptions{}
			installationToken, _, err := githubRestClient.Apps.CreateInstallationToken(context.Background(), *targetInstallation.ID, tokenOpts)
			if err != nil {
				log.Print("Error generating instllation token", err.Error())
			}
			srcToken := oauth2.StaticTokenSource(
				&oauth2.Token{AccessToken: *installationToken.Token},
			)
			httpClient := oauth2.NewClient(context.Background(), srcToken)
			githubRestClient = github.NewClient(httpClient)
		}
	}

	httpClient := oauth2.NewClient(context.Background(), srcToken)
	githubV4Client := githubv4.NewClient(httpClient)
	//githubRestClient := github.NewClient(httpClient)

	return &ActionContext{
		EsClient:         ac.EsClient,
		OsClient:         ac.OsClient,
		GithubV4Client:   githubV4Client,
		GithubRestClient: githubRestClient,
		Session:          ac.Session,
		Lambda:           ac.Lambda,
		Template:         ac.Template,
		Implementation:   "default",
		BucketName:       ac.BucketName,
		Stage:            ac.Stage,
		Site:             req.PathParameters["site"],
		UserPoolId:       ac.UserPoolId,
	}
}

func GetUserId(req *events.APIGatewayProxyRequest) string {
	userId := ""
	log.Printf("claims are %v", req.RequestContext.Authorizer["claims"])
	if req.RequestContext.Authorizer["claims"] != nil {
		userId = fmt.Sprint(req.RequestContext.Authorizer["claims"].(map[string]interface{})["sub"])
		if userId == "<nil>" {
			userId = ""
		}
	} else if req.RequestContext.Authorizer["sub"] != nil {
		userId = req.RequestContext.Authorizer["sub"].(string)
	}
	return userId
}

func GetUsername(req *events.APIGatewayProxyRequest) string {
	username := ""
	log.Printf("claims are %v", req.RequestContext.Authorizer["claims"])
	if req.RequestContext.Authorizer["claims"] != nil {
		username = fmt.Sprint(req.RequestContext.Authorizer["claims"].(map[string]interface{})["username"])
		if username == "<nil>" {
			username = ""
		}
	} else if req.RequestContext.Authorizer["username"] != nil {
		username = req.RequestContext.Authorizer["username"].(string)
	}
	return username
}

func init() {
	log.Printf("Gin cold start")

	elasticCfg := elasticsearch7.Config{
		Addresses: []string{os.Getenv("ELASTIC_URL")},
	}

	awsSigner := sign.AwsSigner{
		Service: "es",
		Region:  "us-east-1",
	}

	opensearchCfg := opensearch.Config{
		Addresses: []string{os.Getenv("ELASTIC_URL")},
		Signer:    awsSigner,
	}

	esClient, err := elasticsearch7.NewClient(elasticCfg)
	if err != nil {

	}

	osClient, err := opensearch.NewClient(opensearchCfg)
	if err != nil {
		log.Printf("Opensearch Error: %s", err.Error())
	}

	/*src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	httpClient := oauth2.NewClient(context.Background(), src)

	githubV4Client := githubv4.NewClient(httpClient)*/

	sess := session.Must(session.NewSession())
	lClient := lambda2.New(sess)
	cogClient := cognitoidentityprovider.New(sess)

	pem, err := os.ReadFile("api/entity/rtc-vertigo-dev.private-key.pem")
	if err != nil {
		log.Print("Error reading github app pem file", err.Error())
	}

	actionContext := ActionContext{
		EsClient: esClient,
		OsClient: osClient,
		//GithubV4Client: githubV4Client,
		Session:      sess,
		Lambda:       lClient,
		Cognito:      cogClient,
		BucketName:   os.Getenv("BUCKET_NAME"),
		Stage:        os.Getenv("STAGE"),
		UserPoolId:   os.Getenv("USER_POOL_ID"),
		GithubAppPem: pem,
	}

	log.Printf("entity bucket storage: %s", actionContext.BucketName)

	funcMap := template.FuncMap{
		"query":  TemplateQuery(&actionContext),
		"lambda": TemplateLambda(&actionContext),
		"userId": TemplateUserId(&actionContext),
	}

	t, err := template.New("").Funcs(funcMap).ParseFiles("api/entity/types.json.tmpl", "api/entity/queries.json.tmpl")

	if err != nil {
		log.Printf("Error: %s", err.Error())
	}

	actionContext.Template = t

	handler = InitializeHandler(&actionContext)
}

func main() {
	lambda.Start(handler)
}
