package shapeshift

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"

	"goclassifieds/lib/entity"
	"goclassifieds/lib/gov"
	"goclassifieds/lib/repo"
	"goclassifieds/lib/sign"
	"goclassifieds/lib/utils"

	"github.com/aws/aws-lambda-go/events"
	session "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	lambda2 "github.com/aws/aws-sdk-go/service/lambda"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
	"github.com/gocql/gocql"
	"github.com/google/go-github/v46/github"
	"github.com/mitchellh/mapstructure"
	opensearch "github.com/opensearch-project/opensearch-go"
	"github.com/shurcooL/githubv4"
	"github.com/tangzero/inflector"
	"golang.org/x/oauth2"
)

type Handler func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error)

type TemplateQueryFunc func(e string, data *entity.EntityFinderDataBag) []map[string]interface{}
type TemplateLambdaFunc func(e string, userId string, data []map[string]interface{}) entity.EntityDataResponse
type TemplateUserIdFunc func(req *events.APIGatewayProxyRequest) string
type TemplateBindValueFunc func(value interface{}) string

type ActionContext struct {
	EsClient            *elasticsearch7.Client
	OsClient            *opensearch.Client
	CassSession         *gocql.Session
	GithubV4Client      *githubv4.Client
	GithubRestClient    *github.Client
	Session             *session.Session
	Lambda              *lambda2.Lambda
	Cognito             *cognitoidentityprovider.CognitoIdentityProvider
	TypeManager         entity.Manager
	EntityManager       entity.Manager
	GrantAccessManager  entity.Manager
	EntityName          string
	Template            *template.Template
	TemplateName        string
	Implementation      string
	BucketName          string
	Stage               string
	Site                string
	UserPoolId          string
	GithubAppPem        []byte
	AdditionalResources *[]gov.Resource
	CloudName           string
	Bindings            *entity.VariableBindings
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
	var id string
	if len(pathPieces) > 3 && pathPieces[3] == "shapeshifter" {
		id = pathPieces[len(pathPieces)-1]
	} else {
		id = pathPieces[3]
	}
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
	//pathPieces := strings.Split(req.Path, "/")
	res := events.APIGatewayProxyResponse{StatusCode: 500}
	body := []byte(req.Body)
	json.Unmarshal(body, &e)
	createRes, err := ac.EntityManager.Create(e)
	if err != nil {
		if strings.Contains(err.Error(), "unauthorized") {
			res.StatusCode = 403
			res.Body = err.Error()
		}
		return res, nil
	} else if createRes.Success == false {
		validationErrors, _ := json.Marshal(createRes.Errors)
		res.Body = string(validationErrors)
		return res, err
	}
	resBody, err := json.Marshal(createRes.Entity)
	if err != nil {
		return res, err
	}
	//if len(pathPieces) > 3 && pathPieces[3] == "shapeshifter" {
	// log.Print("Index Entity")
	// ac.EntityManager.Save(createRes.Entity, "opensearch") // @todo: Just remove for now to get it up and running with Canva Hackathon
	//}
	res.StatusCode = 200
	res.Headers = map[string]string{
		"Content-Type": "application/json",
	}
	res.Body = string(resBody)
	return res, nil
}

func UpdateEntity(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {
	var e map[string]interface{}
	//pathPieces := strings.Split(req.Path, "/")
	res := events.APIGatewayProxyResponse{StatusCode: 500}
	body := []byte(req.Body)
	json.Unmarshal(body, &e)
	updateRes, err := ac.EntityManager.Update(e)
	if err != nil {
		return res, err
	} else if updateRes.Success == false {
		validationErrors, _ := json.Marshal(updateRes.Errors)
		res.Body = string(validationErrors)
		return res, err
	}
	resBody, err := json.Marshal(updateRes.Entity)
	if err != nil {
		return res, err
	}
	// @todo: Ignore just to get this working for Canva integration
	//if len(pathPieces) > 3 && pathPieces[3] == "shapeshifter" {
	// log.Print("Index Entity")
	// ac.EntityManager.Save(updateRes.Entity, "opensearch")
	//}
	res.StatusCode = 200
	res.Headers = map[string]string{
		"Content-Type": "application/json",
	}
	res.Body = string(resBody)
	return res, nil
}

func InitializeHandler(c *ActionContext) Handler {
	return func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

		var clusteringOwner string
		var clusteringRepo string
		var catalogFile string
		var clusteringEnabled bool

		if os.Getenv("CLUSTERING_ENABLED") == "true" {
			clusteringEnabled = true
		} else {
			clusteringEnabled = false
		}

		usageLog := &utils.LogUsageLambdaInput{
			UserId:       GetUserId(req),
			Username:     GetUsername(req),
			Resource:     req.Resource,
			Path:         req.Path,
			RequestId:    req.RequestContext.RequestID,
			Intensities:  "null",
			Regions:      "null",
			Region:       "null",
			Service:      "null",
			Repository:   "null",
			Organization: "null",
		}
		_, hedged := req.Headers["x-hedge-region"]
		if hedged {
			usageLog.Intensities = req.Headers["x-hedge-intensities"]
			usageLog.Regions = req.Headers["x-hedge-regions"]
			usageLog.Region = req.Headers["x-hedge-region"]
			usageLog.Service = req.Headers["x-hedge-service"]
		}
		_, hasOwner := req.PathParameters["owner"]
		if hasOwner {
			clusteringOwner = req.PathParameters["owner"]
			usageLog.Organization = req.PathParameters["owner"]
		}
		_, hasRepo := req.PathParameters["repo"]
		if hasRepo {
			clusteringRepo = req.PathParameters["repo"]
			usageLog.Repository = req.PathParameters["repo"]
		}

		utils.LogUsageForLambdaWithInput(usageLog)

		log.Print("Before RequestActionContext()")
		ac := RequestActionContext(c, req)
		log.Print("After RequestActionContext()")

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
			SingularName:        "type",
			PluralName:          "types",
			Index:               "classified_types",
			EsClient:            ac.EsClient,
			OsClient:            ac.OsClient,
			GithubV4Client:      ac.GithubV4Client,
			Session:             ac.Session,
			Lambda:              ac.Lambda,
			Template:            ac.Template,
			UserId:              userId,
			BucketName:          ac.BucketName,
			Stage:               ac.Stage,
			CloudName:           ac.CloudName,
			LogUsageLambdaInput: usageLog,
		})

		searchIndex := "classified_" + pluralName
		if singularName == "shapeshifter" {
			proxyPieces := strings.Split(req.PathParameters["proxy"], "/")
			searchIndex = req.PathParameters["owner"] + "__" + req.PathParameters["repo"] + "__" + strings.Join(proxyPieces[0:len(proxyPieces)-1], "__")
			log.Print("search Index: " + searchIndex)
		}

		if singularName == "shapeshifter" {
			proxyPieces := strings.Split(req.PathParameters["proxy"], "/")
			directoryPath := strings.Join(proxyPieces[0:len(proxyPieces)-1], "/")
			c, err := repo.EnsureCatalog(context.Background(), ac.GithubRestClient, req.PathParameters["owner"], req.PathParameters["repo"], directoryPath, os.Getenv("GITHUB_BRANCH"), clusteringEnabled)
			if err != nil {
				log.Printf("Unable to ensure catalog")
				return events.APIGatewayProxyResponse{StatusCode: 500}, nil
			} else {
				catalogFile = c
				catalogPieces := strings.Split(catalogFile, "/")
				chapter := catalogPieces[len(catalogPieces)-2]
				log.Printf("The chapter is %s", chapter)
				repoPieces := strings.Split(req.PathParameters["repo"], "-")
				clusteringRepoPrefix := strings.Join(repoPieces[0:len(repoPieces)-1],"-")
				log.Printf("The clustering repo prefix is %s", clusteringRepoPrefix)
				if chapter != "0" {
					clusteringRepo = clusteringRepoPrefix + "-" + chapter + "-objects"
				}
			}
		}

		if singularName == "type" {
			ac.EntityManager = ac.TypeManager
		} else {
			ac.EntityManager = entity.NewDefaultManager(entity.DefaultManagerConfig{
				SingularName:        singularName,
				PluralName:          pluralName,
				Index:               searchIndex,
				EsClient:            ac.EsClient,
				OsClient:            ac.OsClient,
				GithubV4Client:      ac.GithubV4Client,
				Session:             ac.Session,
				Lambda:              ac.Lambda,
				Template:            ac.Template,
				UserId:              userId,
				BucketName:          ac.BucketName,
				Stage:               ac.Stage,
				Site:                ac.Site,
				CloudName:           ac.CloudName,
				LogUsageLambdaInput: usageLog,
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
			// Github Installation will indirectly enforce access to repository.
			// ac.EntityManager.AddAuthorizer("default", entity.NoopAuthorizationAdaptor{})
			if ac.CloudName == "azure" {
				ac.EntityManager.AddAuthorizer("default", entity.ResourceAuthorizationEmbeddedAdaptor{
					Config: entity.ResourceAuthorizationEmbeddedConfig{
						UserId:              userId,
						Site:                ac.Site,
						Resource:            gov.GithubRepo,
						Asset:               req.PathParameters["owner"] + "/" + req.PathParameters["repo"],
						Lambda:              ac.Lambda,
						CassSession:         ac.CassSession,
						GrantAccessManager:  ac.GrantAccessManager,
						AdditionalResources: ac.AdditionalResources,
					},
				})
			} else {
				ac.EntityManager.AddAuthorizer("default", entity.ResourceAuthorizationAdaptor{
					Config: entity.ResourceAuthorizationConfig{
						UserId:              userId,
						Site:                ac.Site,
						Resource:            gov.GithubRepo,
						Asset:               req.PathParameters["owner"] + "/" + req.PathParameters["repo"],
						Lambda:              ac.Lambda,
						AdditionalResources: ac.AdditionalResources,
					},
				})
			}
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
					Client:   ac.GithubV4Client,
					Repo:     "rollthecloudinc/" + req.PathParameters["site"] + "-objects", // @todo: Hard coded to test integration for now.
					Branch:   os.Getenv("GITHUB_BRANCH"),                                   // This will cone env vars from inside json file passed via serverless.
					Path:     "panelpage",
					UserName: GetUsername(req), // path to place stuff. This will probably be a separate repo or directory udnerneath assets.
				},
			})
			ac.EntityManager.AddStorage("default", entity.GithubFileUploadAdaptor{
				Config: entity.GithubFileUploadConfig{
					Client:   ac.GithubV4Client,
					Repo:     "rollthecloudinc/" + req.PathParameters["site"] + "-objects", // @todo: Hard coded to test integration for now.
					Branch:   os.Getenv("GITHUB_BRANCH"),                                   // This will cone env vars from inside json file passed via serverless.
					Path:     "panelpage",
					UserName: GetUsername(req), // path to place stuff. This will probably be a separate repo or directory udnerneath assets.
				},
			})
		} else if singularName == "shapeshifter" {
			proxyPieces := strings.Split(req.PathParameters["proxy"], "/")
			var loaderPath string
			if req.HTTPMethod == "GET" {
				loaderPath = strings.Join(proxyPieces[0:len(proxyPieces)-1], "/")
			} else {
				loaderPath = req.PathParameters["proxy"]
			}
			// @todo: The loader will need to use the right one yeeh!
			log.Print("REPORT RequestId: " + req.RequestContext.RequestID + " Organization: " + req.PathParameters["owner"] + " Repository: " + req.PathParameters["repo"])
			
			loaderClusteringRep := req.PathParameters["owner"] + "/" + req.PathParameters["repo"]
			
			// @todo: Find the chapter that will be used to load the entity.
			//proxyPieces := strings.Split(req.PathParameters["proxy"], "/")
			directoryPath := strings.Join(proxyPieces[0:len(proxyPieces)-1], "/")
			fileNameGuid := strings.Split(proxyPieces[len(proxyPieces)-1],".")[0]
			loadChapter, err := repo.FindChapterByGUID(context.Background(), ac.GithubRestClient, req.PathParameters["owner"], req.PathParameters["repo"], directoryPath, fileNameGuid, os.Getenv("GITHUB_BRANCH"))
			if err != nil {
				log.Print("Error looking up chapter for entity %s", proxyPieces[len(proxyPieces)-1])
			}
			log.Printf("Loading entity from chapter %s", loadChapter)
			repoPieces := strings.Split(req.PathParameters["repo"], "-")
			clusteringRepoPrefix := strings.Join(repoPieces[0:len(repoPieces)-1],"-")
			log.Printf("The clustering repo prefix is %s", clusteringRepoPrefix)
			if loadChapter != "0" {
				loaderClusteringRep = req.PathParameters["owner"] + "/" + clusteringRepoPrefix + "-" + loadChapter + "-objects"
			}
			log.Printf("Entity will be loaded from the clustering repo %s", loaderClusteringRep)

			ac.EntityManager.AddLoader("default", entity.GithubRestFileLoaderAdaptor{
				Config: entity.GithubRestFileUploadConfig{
					Client:   ac.GithubRestClient,
					Repo:     loaderClusteringRep,
					Branch:   os.Getenv("GITHUB_BRANCH"),
					Path:     loaderPath,
					UserName: GetUsername(req),
				},
			})
			updateClusteringRepo := clusteringOwner + "/" + clusteringRepo
			if (req.HTTPMethod == "PUT") {
				updateClusteringRepo = loaderClusteringRep
			}
			ac.EntityManager.AddStorage("default", entity.GithubRestFileUploadAdaptor{
				Config: entity.GithubRestFileUploadConfig{
					Client:   ac.GithubRestClient,
					// Repo:     req.PathParameters["owner"] + "/" + req.PathParameters["repo"],
					Repo:     updateClusteringRepo,
					Branch:   os.Getenv("GITHUB_BRANCH"),
					Path:     strings.Join(proxyPieces[0:len(proxyPieces)-1], "/"),
					UserName: GetUsername(req),
				},
			})
			ac.EntityManager.AddValidator("default", entity.ContractValidatorAdaptor{
				Config: entity.ContractValidatorConfig{
					Lambda:   ac.Lambda,
					UserId:   userId,
					Site:     ac.Site,
					Client:   ac.GithubRestClient,
					Repo:     req.PathParameters["owner"] + "/" + req.PathParameters["repo"],
					Branch:   os.Getenv("GITHUB_BRANCH"),
					Contract: "/contracts/" + proxyPieces[0] + ".json",
				},
			})
		}

		if (singularName == "shapeshifter" && req.HTTPMethod == "POST") {
			// var catalogFile string
			ac.EntityManager.SetHook(entity.BeforeSave, func(ent map[string]interface{}, m *entity.EntityManager) (map[string]interface{}, error) {
				log.Print("Before shapeshift save")
				log.Printf("The clustering owner and repo are %s/%s", clusteringOwner, clusteringRepo)
				/*proxyPieces := strings.Split(req.PathParameters["proxy"], "/")
				directoryPath := strings.Join(proxyPieces[0:len(proxyPieces)-1], "/")
				c, err := repo.EnsureCatalog(context.Background(), ac.GithubRestClient, req.PathParameters["owner"], req.PathParameters["repo"], directoryPath)
				if err != nil {
					log.Printf("Unable to ensure catalog")
					return nil, fmt.Errorf("Unable to ensure catalog.")
				} else {
					catalogFile = c
				}*/
				if clusteringEnabled {
					err := repo.EnsureRepoCreate(ac.GithubRestClient, clusteringOwner, clusteringRepo, "clustering repo for " + req.PathParameters["owner"] + "/" + req.PathParameters["repo"], false)
					if err != nil {
						return nil, fmt.Errorf("Could not ensure repo")
					}
				}
				return ent, nil
			})
			ac.EntityManager.SetHook(entity.AfterSave, func(ent map[string]interface{}, m *entity.EntityManager) (map[string]interface{}, error) {
				log.Print("After shapeshift save")
				id, ok := ent["id"].(string)
				if !ok {
					log.Print("Error: 'id' is not a string or is missing")
					return nil, fmt.Errorf("'id' is not a string or is missing")
				}
				log.Printf("Entity ID: %s", id)
				// proxyPieces := strings.Split(req.PathParameters["proxy"], "/")
				// directoryPath := strings.Join(proxyPieces[0:len(proxyPieces)-1], "/")
				// file, err := repo.EnsureCatalog(context.Background(), ac.GithubRestClient, req.PathParameters["owner"], req.PathParameters["repo"], directoryPath)
				/*if err != nil {
					log.Print("Unable to ensure catalog")
					return nil, fmt.Errorf("Unable to ensure catalog.")
				}*/
				log.Print("Append id to catalog file " + catalogFile)
				repo.AppendToFile(context.Background(), ac.GithubRestClient, req.PathParameters["owner"], req.PathParameters["repo"], catalogFile, id, os.Getenv("GITHUB_BRANCH"))
				return ent, nil
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

		usageLog := &utils.LogUsageLambdaInput{}

		entityManager := entity.NewDefaultManager(entity.DefaultManagerConfig{
			SingularName:        singularName,
			PluralName:          pluralName,
			Index:               "classified_" + pluralName,
			EsClient:            ac.EsClient,
			OsClient:            ac.OsClient,
			Session:             ac.Session,
			Lambda:              ac.Lambda,
			Template:            ac.Template,
			UserId:              "",
			BucketName:          ac.BucketName,
			Stage:               ac.Stage,
			LogUsageLambdaInput: usageLog,
			CloudName:           ac.CloudName,
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

func TemplateBindValue(ac *ActionContext) TemplateBindValueFunc {
	return func(value interface{}) string {
		ac.Bindings.Values = append(ac.Bindings.Values, value)
		return "?"
	}
}

func RequestActionContext(ac *ActionContext, req *events.APIGatewayProxyRequest) *ActionContext {

	pathPieces := strings.Split(req.Path, "/")

	var githubToken string
	var githubRestClient *github.Client
	var srcToken oauth2.TokenSource
	additionalResources := make([]gov.Resource, 0)

	if len(pathPieces) < 4 || pathPieces[3] != "shapeshifter" {
		githubToken = os.Getenv("GITHUB_TOKEN")
		srcToken = oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: githubToken},
		)
	} else {
		getTokenInput := &repo.GetInstallationTokenInput{
			GithubAppPem: ac.GithubAppPem,
			Owner:        req.PathParameters["owner"],
			GithubAppId:  os.Getenv("GITHUB_APP_ID"),
		}
		installationToken, err := repo.GetInstallationToken(getTokenInput)
		if err != nil {
			log.Print("Error generating installation token", err.Error())
		}
		srcToken := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: *installationToken.Token},
		)

		username := GetUsername(req)
		username = "ng-druid" // force it just to see if we can get a write going...

		if username == os.Getenv("DEFAULT_SIGNING_USERNAME") || username == req.PathParameters["owner"] {
			log.Print("Granting explicit permission for " + username + " to " + req.PathParameters["owner"] + "/" + req.PathParameters["repo"])
			resource := gov.Resource{
				User:      GetUserId(req),
				Type:      gov.User,
				Resource:  gov.GithubRepo,
				Asset:     req.PathParameters["owner"] + "/" + req.PathParameters["repo"],
				Operation: gov.Write,
			}
			additionalResources = append(additionalResources, resource)
		}

		httpClient := oauth2.NewClient(context.Background(), srcToken)
		githubRestClient = github.NewClient(httpClient)
	}

	httpClient := oauth2.NewClient(context.Background(), srcToken)
	log.Print("Created token")

	githubV4Client := githubv4.NewClient(httpClient)
	log.Print("Created github v4 client")
	//githubRestClient := github.NewClient(httpClient)

	token := req.Headers["authorization"][7:]
	log.Print("token: " + token)

	awsSigner := sign.AwsSigner{
		Service:        "es",
		Region:         "us-east-1",
		Session:        ac.Session,
		IdentityPoolId: os.Getenv("IDENTITY_POOL_ID"),
		Issuer:         os.Getenv("ISSUER"),
		Token:          token,
	}

	opensearchCfg := opensearch.Config{
		Addresses: []string{os.Getenv("ELASTIC_URL")},
		Signer:    awsSigner,
	}

	osClient, err := opensearch.NewClient(opensearchCfg)
	if err != nil {
		log.Printf("Opensearch Error: %s", err.Error())
	}

	var grantAccessManager entity.Manager
	if ac.CloudName == "azure" {
		grantAccessResourceParams := &gov.ResourceManagerParams{
			Session:  ac.CassSession,
			Request:  &gov.GrantAccessRequest{},
			Template: ac.Template,
			// Resource:  fmt.Sprint(payload.Resource),
			// Operation: fmt.Sprint(payload.Operation),
		}
		grantAccessManager, _ = GrantAccessManager(grantAccessResourceParams, ac.Bindings)
	}

	return &ActionContext{
		EsClient:            ac.EsClient,
		OsClient:            osClient,
		CassSession:         ac.CassSession,
		GithubV4Client:      githubV4Client,
		GithubRestClient:    githubRestClient,
		Session:             ac.Session,
		Lambda:              ac.Lambda,
		Template:            ac.Template,
		Implementation:      "default",
		BucketName:          ac.BucketName,
		Stage:               ac.Stage,
		Site:                req.PathParameters["site"],
		UserPoolId:          ac.UserPoolId,
		AdditionalResources: &additionalResources,
		CloudName:           ac.CloudName,
		GrantAccessManager:  grantAccessManager,
		Bindings:            ac.Bindings,
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
	field := "cognito:username"
	/*if os.Getenv("STAGE") == "prod" {
		field = "cognito:username"
	}*/
	log.Printf("claims are %v", req.RequestContext.Authorizer["claims"])
	if req.RequestContext.Authorizer["claims"] != nil {
		username = fmt.Sprint(req.RequestContext.Authorizer["claims"].(map[string]interface{})[field])
		if username == "<nil>" {
			username = ""
		}
	} else if req.RequestContext.Authorizer[field] != nil {
		username = req.RequestContext.Authorizer[field].(string)
	}
	return username
}

func GrantAccessManager(params *gov.ResourceManagerParams, bindings *entity.VariableBindings) (entity.Manager, error) {
	entityName := "resource"
	manager := entity.EntityManager{
		Config: entity.EntityConfig{
			SingularName: entityName,
			PluralName:   inflector.Pluralize(entityName),
			IdKey:        "id",
			Stage:        os.Getenv("STAGE"),
		},
		Validators: map[string]entity.Validator{
			"default": entity.DefaultValidatorAdaptor{},
		},
		Creator:  entity.DefaultCreatorAdaptor{},
		Storages: map[string]entity.Storage{},
		Finders: map[string]entity.Finder{
			"default": entity.CqlTemplateFinder{
				Config: entity.CqlTemplateFinderConfig{
					Session:  params.Session,
					Template: params.Template,
					Table:    inflector.Pluralize(entityName) + "2",
					Bindings: bindings,
					Aliases:  map[string]string{},
				},
			},
		},
		Hooks:           map[entity.Hooks]entity.EntityHook{},
		CollectionHooks: map[string]entity.EntityCollectionHook{},
	}

	return manager, nil
}

func ShapeshiftActionContext() *ActionContext {
	log.Printf("Gin cold start")

	elasticCfg := elasticsearch7.Config{
		Addresses: []string{os.Getenv("ELASTIC_URL")},
	}

	esClient, err := elasticsearch7.NewClient(elasticCfg)
	if err != nil {
	}

	sess := session.Must(session.NewSession())
	lClient := lambda2.New(sess)
	cogClient := cognitoidentityprovider.New(sess)

	var cassSession *gocql.Session
	if os.Getenv("CLOUD_NAME") == "azure" {
		cluster := gocql.NewCluster("cassandra.us-east-1.amazonaws.com")
		cluster.Keyspace = "ClassifiedsDev"
		cluster.Port = 9142
		cluster.Consistency = gocql.LocalOne // gocql.LocalQuorum
		cluster.Authenticator = &gocql.PasswordAuthenticator{Username: os.Getenv("KEYSPACE_USERNAME"), Password: os.Getenv("KEYSPACE_PASSWORD")}
		cluster.SslOpts = &gocql.SslOptions{Config: &tls.Config{ServerName: "cassandra.us-east-1.amazonaws.com"}, CaPath: "api/chat/AmazonRootCA1.pem", EnableHostVerification: true}
		cluster.PoolConfig = gocql.PoolConfig{HostSelectionPolicy: /*gocql.TokenAwareHostPolicy(*/ gocql.DCAwareRoundRobinPolicy("us-east-1") /*)*/}
		cassSession, err = cluster.CreateSession()
		if err != nil {
			log.Print("Error connecting to keyspaces cassandra cluster.")
			log.Fatal(err)
		}
	}

	pem, err := os.ReadFile("rtc-vertigo-" + os.Getenv("STAGE") + ".private-key.pem")
	if err != nil {
		log.Print("Error reading github app pem file", err.Error())
	}

	actionContext := &ActionContext{
		EsClient:     esClient,
		Session:      sess,
		CassSession:  cassSession,
		Lambda:       lClient,
		Cognito:      cogClient,
		BucketName:   os.Getenv("BUCKET_NAME"),
		Stage:        os.Getenv("STAGE"),
		UserPoolId:   os.Getenv("USER_POOL_ID"),
		GithubAppPem: pem,
		CloudName:    os.Getenv("CLOUD_NAME"),
		Bindings:     &entity.VariableBindings{Values: make([]interface{}, 0)},
	}

	log.Printf("entity bucket storage: %s", actionContext.BucketName)

	funcMap := template.FuncMap{
		"query":     TemplateQuery(actionContext),
		"lambda":    TemplateLambda(actionContext),
		"userId":    TemplateUserId(actionContext),
		"bindValue": TemplateBindValue(actionContext),
	}

	t, err := template.New("").Funcs(funcMap).ParseFiles("types.json.tmpl", "queries.json.tmpl")
	if os.Getenv("CLOUD_NAME") == "azure" {
		t, err = t.Parse(gov.Query())
		if err != nil {
			log.Print("Error parsing gov.Query() template.")
		}
	}

	if err != nil {
		log.Printf("Error: %s", err.Error())
	}

	actionContext.Template = t

	return actionContext
}
