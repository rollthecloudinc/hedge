package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"text/template"

	"goclassifieds/lib/entity"
	"goclassifieds/lib/es"
	"goclassifieds/lib/profiles"
	"goclassifieds/lib/utils"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	session "github.com/aws/aws-sdk-go/aws/session"
	ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/gin"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
)

var ginLambda *ginadapter.GinLambda

type ActionFunc func(context *gin.Context, ac *ActionContext)

type ActionContext struct {
	EsClient        *elasticsearch7.Client
	Session         *session.Session
	ProfilesManager entity.Manager
	QueryTemplate   *template.Template
}

func GetProfileListItems(context *gin.Context, ac *ActionContext) {
	hits := es.ExecuteQuery(ac.EsClient, es.TemplateBuilder{
		Index:    "classified_profiles",
		Name:     "profilelistitems",
		Template: ac.QueryTemplate,
		Data: profiles.ProfileListItemsQuery{
			UserId:   utils.GetSubject(context),
			ParentId: context.Query("parentId"),
		},
	})
	items := make([]profiles.Profile, len(hits))
	for index, hit := range hits {
		mapstructure.Decode(hit.(map[string]interface{})["_source"], &items[index])
	}
	context.JSON(200, items)
}

func GetProfileNavItems(context *gin.Context, ac *ActionContext) {
	hits := es.ExecuteQuery(ac.EsClient, es.TemplateBuilder{
		Index:    "classified_profiles",
		Name:     "profilenavitems1",
		Template: ac.QueryTemplate,
		Data: profiles.ProfileNavItemsQuery1{
			UserId: utils.GetSubject(context),
		},
	})
	rootIds := make(map[string]bool)
	for _, hit := range hits {
		id := fmt.Sprint(hit.(map[string]interface{})["_source"].(map[string]interface{})["id"])
		parentId := fmt.Sprint(hit.(map[string]interface{})["_source"].(map[string]interface{})["parentId"])
		_, ok := rootIds[id]
		if !ok && (parentId == "" || parentId == "<nil>") {
			rootIds[id] = true
		}
	}
	for _, hit := range hits {
		parentId := fmt.Sprint(hit.(map[string]interface{})["_source"].(map[string]interface{})["parentId"])
		_, ok := rootIds[parentId]
		if !ok && parentId != "" && parentId != "<nil>" {
			rootIds[parentId] = true
		}
	}
	i := 0
	ids := make([]string, len(rootIds))
	for key := range rootIds {
		ids[i] = key
		i++
	}
	hits = es.ExecuteQuery(ac.EsClient, es.TemplateBuilder{
		Index:    "classified_profiles",
		Name:     "profilenavitems2",
		Template: ac.QueryTemplate,
		Data: profiles.ProfileNavItemsQuery2{
			Ids:  ids,
			Last: i - 1,
		},
	})
	items := make([]profiles.ProfileNavItem, len(hits))
	for index, hit := range hits {
		mapstructure.Decode(hit.(map[string]interface{})["_source"], &items[index])
	}
	context.JSON(200, items)
}

func CreateProfile(context *gin.Context, ac *ActionContext) {
	var obj profiles.Profile
	if err := context.ShouldBind(&obj); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	obj.Id = utils.GenerateId()
	obj.Status = profiles.Submitted // @todo: Enums not being validated :(
	obj.UserId = utils.GetSubject(context)
	obj.EntityPermissions = profiles.ProfilePermissions{
		ReadUserIds:   []string{obj.UserId},
		WriteUserIds:  []string{obj.UserId},
		DeleteUserIds: []string{obj.UserId},
	}
	newEntity, _ := profiles.ToEntity(&obj)
	ac.ProfilesManager.Save(newEntity, "s3")
	context.JSON(200, newEntity)
}

func DeclareAction(action ActionFunc, ac ActionContext) gin.HandlerFunc {
	return func(context *gin.Context) {
		log.Printf("handler context: %v", context)
		ac.ProfilesManager = entity.NewDefaultManager(entity.DefaultManagerConfig{
			SingularName: "profile",
			PluralName:   "profiles",
			Index:        "classified_profiles",
			EsClient:     ac.EsClient,
			Session:      ac.Session,
			UserId:       utils.GetSubject(context),
		})
		action(context, &ac)
	}
}

func init() {
	// stdout and stderr are sent to AWS CloudWatch Logs
	log.Printf("Gin cold start")

	elasticCfg := elasticsearch7.Config{
		Addresses: []string{
			"https://i12sa6lx3y:v75zs8pgyd@classifieds-4537380016.us-east-1.bonsaisearch.net:443",
		},
	}

	esClient, err := elasticsearch7.NewClient(elasticCfg)
	if err != nil {

	}

	sess := session.Must(session.NewSession())

	t, err := template.ParseFiles("api/profiles/queries.json.tmpl")
	if err != nil {
		log.Printf("Error: %s", err.Error())
	}

	actionContext := ActionContext{
		EsClient:      esClient,
		Session:       sess,
		QueryTemplate: t,
	}

	r := gin.Default()
	r.GET("/profiles/profilelistitems", DeclareAction(GetProfileListItems, actionContext))
	r.GET("/profiles/profilenavitems", DeclareAction(GetProfileNavItems, actionContext))
	r.POST("/profiles/profile", DeclareAction(CreateProfile, actionContext))

	ginLambda = ginadapter.New(r)
}

func Handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// If no name is provided in the HTTP request body, throw an error
	/*var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(req); err != nil {
		log.Fatalf("Error encoding request: %s", err)
	}
	log.Printf("request: %s", buf)*/

	// If no name is provided in the HTTP request body, throw an error
	res, err := ginLambda.ProxyWithContext(ctx, req)
	res.Headers["Access-Control-Allow-Origin"] = "*"
	return res, err
}

func main() {
	lambda.Start(Handler)
}
