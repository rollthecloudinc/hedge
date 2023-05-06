package main

import (
	"context"
	"crypto/tls"
	"goclassifieds/lib/entity"
	"goclassifieds/lib/gov"
	"goclassifieds/lib/utils"
	"log"
	"os"
	"text/template"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/gocql/gocql"
	"github.com/tangzero/inflector"
)

type TemplateBindValueFunc func(value interface{}) string

func handler(ctx context.Context, payload *gov.GrantAccessRequest) (gov.GrantAccessResponse, error) {

	utils.LogUsageForLambdaWithInput(payload.LogUsageLambdaInput)

	cluster := gocql.NewCluster("cassandra.us-east-1.amazonaws.com")
	cluster.Keyspace = "ClassifiedsDev"
	cluster.Port = 9142
	cluster.Consistency = gocql.LocalOne // gocql.LocalQuorum
	cluster.Authenticator = &gocql.PasswordAuthenticator{Username: os.Getenv("KEYSPACE_USERNAME"), Password: os.Getenv("KEYSPACE_PASSWORD")}
	cluster.SslOpts = &gocql.SslOptions{Config: &tls.Config{ServerName: "cassandra.us-east-1.amazonaws.com"}, CaPath: "api/chat/AmazonRootCA1.pem", EnableHostVerification: true}
	cluster.PoolConfig = gocql.PoolConfig{HostSelectionPolicy: /*gocql.TokenAwareHostPolicy(*/ gocql.DCAwareRoundRobinPolicy("us-east-1") /*)*/}
	cSession, err := cluster.CreateSession()
	if err != nil {
		log.Fatal(err)
	}

	resourceParams := &gov.ResourceManagerParams{
		Session: cSession,
		Request: payload,
		// Resource:  fmt.Sprint(payload.Resource),
		// Operation: fmt.Sprint(payload.Operation),
	}

	resourceManager, _ := ResourceManager(resourceParams)
	allAttributes := make([]entity.EntityAttribute, 0)
	data := &entity.EntityFinderDataBag{
		Attributes: allAttributes,
		Metadata: map[string]interface{}{
			"user":     payload.User,
			"type":     payload.Type,
			"resource": payload.Resource,
			"asset":    payload.Asset,
			"op":       payload.Operation,
		},
	}
	results := resourceManager.Find("default", "grant_access", data)

	/*b, _ := json.Marshal(results)
	log.Print(string(b))*/

	grant := len(results) != 0

	if len(payload.AdditionalResources) != 0 {
		for _, r := range payload.AdditionalResources {
			if r.User == payload.User && r.Type == payload.Type && r.Resource == payload.Resource && r.Asset == payload.Asset && r.Operation == payload.Operation {
				grant = true
				break
			}
		}
	}

	return gov.GrantAccessResponse{
		Grant: grant,
	}, nil

}

func ResourceManager(params *gov.ResourceManagerParams) (entity.Manager, error) {
	entityName := "resource"
	bindings := &entity.VariableBindings{Values: make([]interface{}, 0)}
	funcMap := template.FuncMap{
		"bindValue": TemplateBindValue(bindings),
	}
	t, err := template.New("").Funcs(funcMap).Parse(gov.Query())
	if err != nil {
		log.Printf("Error: %s", err.Error())
		return entity.EntityManager{}, err
	}
	manager := entity.EntityManager{
		Config: entity.EntityConfig{
			SingularName: entityName,
			PluralName:   inflector.Pluralize(entityName),
			IdKey:        "id",
			Stage:        os.Getenv("STAGE"),
		},
		Creator: entity.DefaultCreatorAdaptor{},
		Validators: map[string]entity.Validator{
			"default": entity.DefaultValidatorAdaptor{},
		},
		Storages: map[string]entity.Storage{},
		Finders: map[string]entity.Finder{
			"default": entity.CqlTemplateFinder{
				Config: entity.CqlTemplateFinderConfig{
					Session:  params.Session,
					Template: t,
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

func TemplateBindValue(bindings *entity.VariableBindings) TemplateBindValueFunc {
	return func(value interface{}) string {
		bindings.Values = append(bindings.Values, value)
		return "?"
	}
}

func main() {
	log.SetFlags(0)
	// Make the handler available for Remote Procedure Call by AWS Lambda
	lambda.Start(handler)
}
