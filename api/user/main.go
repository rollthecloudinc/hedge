package main

import (
	"context"
	"encoding/json"
	"goclassifieds/lib/entity"
	"goclassifieds/lib/sign"
	"goclassifieds/lib/utils"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/google/go-github/github"
	"github.com/opensearch-project/opensearch-go"
	"github.com/sethvargo/go-password/password"
	"golang.org/x/oauth2"
)

var handler Handler

type Handler func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error)

type ActionContext struct {
	Session       *session.Session
	Client        *cognitoidentityprovider.CognitoIdentityProvider
	SesClient     *ses.SES
	EntityManager entity.Manager
	UserPoolId    string
	Stage         string
}

type MyTemplateData struct {
	Name           string `json:"name"`
	FavoriteAnimal string `json:"favoriteanimal"`
}

type TempPasswordData struct {
	Name         string `json:"name"`
	TempPassword string `json:"tempPassword"`
}

type Auth0LogEntityManagerInput struct {
	OsClient *opensearch.Client
}

func GetEntity(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {
	var res events.APIGatewayProxyResponse
	pathPieces := strings.Split(req.Path, "/")
	id := pathPieces[2]
	log.Printf("entity by id: %s", id)
	ent := ac.EntityManager.Load(id, "default")
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

func GithubSignup(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {
	userExists := false
	res := events.APIGatewayProxyResponse{}
	code := req.QueryStringParameters["code"]
	// state := req.QueryStringParameters["state"]
	config := oauth2.Config{
		ClientID:     os.Getenv("GITHUB_APP_CLIENT_ID"),
		ClientSecret: os.Getenv("GITHUB_APP_CLIENT_SECRET"),
		RedirectURL:  "",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
		},
		Scopes: []string{ /*"repo"*/ },
	}
	ctx := context.Background()
	token, err := config.Exchange(ctx, code)
	if err != nil {
		log.Print(err.Error())
		res.StatusCode = 500
		return res, nil
	}
	res.StatusCode = 200
	// log.Print("code = " + code + " | state =" + state + " | accessToken = " + token.AccessToken + " | refreshToken = " + token.RefreshToken)
	ts := oauth2.StaticTokenSource(token)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		log.Print(err.Error())
		res.StatusCode = 500
		return res, nil
	}
	primaryEmail := user.Email
	log.Print(user.String())
	listOptions := &github.ListOptions{Page: 1, PerPage: 10}
	emails, _, err := client.Users.ListEmails(ctx, listOptions)
	if err != nil {
		log.Print(err.Error())
		res.StatusCode = 500
		return res, nil
	}
	for _, email := range emails {
		log.Print(email.GetEmail())
		if email.GetPrimary() == true && email.GetVerified() == true {
			log.Print("identified verified primary email: " + email.GetEmail())
			primaryEmail = email.Email
		}
	}
	tempPassword, err := password.Generate(15, 5, 2, false, false)
	log.Print("temp password " + tempPassword)
	cogUser := &cognitoidentityprovider.SignUpInput{
		Username: user.Login,
		Password: aws.String(tempPassword),
		ClientId: aws.String(os.Getenv("COGNITO_APP_CLIENT_ID")),
		UserAttributes: []*cognitoidentityprovider.AttributeType{
			{
				Name:  aws.String("email"),
				Value: primaryEmail,
			},
		},
	}
	u, err := ac.Client.SignUp(cogUser)
	if err != nil && err.Error() != "UsernameExistsException: User already exists" {
		log.Print(err.Error())
		res.StatusCode = 500
		return res, nil
	}
	if err != nil && err.Error() == "UsernameExistsException: User already exists" {
		userExists = true
	}
	updateInput := &cognitoidentityprovider.AdminUpdateUserAttributesInput{
		UserPoolId: aws.String(ac.UserPoolId),
		Username:   user.Login,
		UserAttributes: []*cognitoidentityprovider.AttributeType{
			{
				Name:  aws.String("email_verified"),
				Value: aws.String("true"),
			},
			{
				Name:  aws.String("custom:githubAccessToken"),
				Value: aws.String(token.AccessToken),
			},
			{
				Name:  aws.String("custom:githubRefreshToken"),
				Value: aws.String(token.RefreshToken),
			},
			/*{
				Name:  aws.String("custom:githubLogin"),
				Value: user.Login,
			},*/
		},
	}
	_, err = ac.Client.AdminUpdateUserAttributes(updateInput)
	if err != nil {
		log.Print(err.Error())
		res.StatusCode = 500
		return res, nil
	}
	if !userExists {
		confirmInput := &cognitoidentityprovider.AdminConfirmSignUpInput{
			UserPoolId: aws.String(ac.UserPoolId),
			Username:   user.Login,
		}
		_, err = ac.Client.AdminConfirmSignUp(confirmInput)
		if err != nil {
			log.Print(err.Error())
			res.StatusCode = 500
			return res, nil
		}
		resetPass := &cognitoidentityprovider.AdminResetUserPasswordInput{
			UserPoolId: aws.String(ac.UserPoolId),
			Username:   user.Login,
		}
		_, err = ac.Client.AdminResetUserPassword(resetPass)
		if err != nil {
			log.Print(err.Error())
			res.StatusCode = 500
			return res, nil
		}
		emailData := TempPasswordData{
			Name:         user.GetLogin(),
			TempPassword: tempPassword,
		}
		jsonData, err := json.Marshal(emailData)
		if err != nil {
			log.Print(err.Error())
			res.StatusCode = 500
			return res, nil
		}
		defaultEmail := "climateaware-dev.eco"
		if ac.Stage == "prod" {
			defaultEmail = "climateaware.eco"
		}
		emailInput := &ses.SendTemplatedEmailInput{
			Source:               aws.String("Security <sso@" + defaultEmail + ">"),
			Template:             aws.String("TempPassword"),
			ConfigurationSetName: aws.String("TempPassword"),
			Destination: &ses.Destination{
				ToAddresses: []*string{primaryEmail},
			},
			TemplateData: aws.String(string(jsonData)),
		}
		_, err = ac.SesClient.SendTemplatedEmail(emailInput)
		if err != nil {
			log.Print(err.Error())
			res.StatusCode = 500
			return res, nil
		}
		// @todo: Create serverless collection
		// - uuid
		// - map uuid to owner in new cassandra collection
		// - DevAuthAdmin
		// - I think a dedicated role needs to be created anyway
		// - The user will be linked to this role.
		// -- or we put this inside the marketplace event... - access to plan
	}
	res.Body = u.String()
	return res, nil
}

func GithubMarketplaceEvent(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {
	res := events.APIGatewayProxyResponse{}
	res.StatusCode = 200
	res.Body = "{ \"message\": \"success\" }"
	log.Print("GithubMarketplaceEvent What")
	return res, nil
}

func OktaLogEvent(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {

	res := events.APIGatewayProxyResponse{}

	log.Print("OktaLogeEvent")
	// log.Print(req.Headers["Authorization"]);
	log.Print(req.Body)

	// Split the body of the request into lines
	lines := strings.Split(req.Body, "\n")

	// Prepare a bulk request
	var bulkBody strings.Builder
	for _, line := range lines {
		// Each line is a separate JSON object
		// Add action and metadata
		internalId := utils.GenerateId()
		bulkBody.WriteString(`{ "index" : { "_index" : "auth0_log", "_id" : "` + internalId + `" } }`)
		bulkBody.WriteString("\n")

		// Add source data
		bulkBody.WriteString(line)
		bulkBody.WriteString("\n")
	}

	log.Print(bulkBody.String())

	userPasswordAwsSigner := sign.UserPasswordAwsSigner{
		// Service:            "aoss",
		Service:            "es",
		Region:             "us-east-1",
		Session:            ac.Session,
		IdentityPoolId:     os.Getenv("IDENTITY_POOL_ID"),
		Issuer:             os.Getenv("ISSUER"),
		Username:           os.Getenv("DEFAULT_SIGNING_USERNAME"),
		Password:           os.Getenv("DEFAULT_SIGNING_PASSWORD"),
		CognitoAppClientId: os.Getenv("COGNITO_APP_CLIENT_ID"),
	}

	// addr := []string = []string{"https://xelsubdau4tag8glbeb9.us-east-1.aoss.amazonaws.com"}
	// addr := []string{"https://xelsubdau4tag8glbeb9.us-east-1.aoss.amazonaws.com"}
	// fuck serverless for now... perhaps not compatible with cognito... idk
	opensearchCfg := opensearch.Config{
		Addresses: []string{os.Getenv("ELASTIC_URL")},
		// Addresses: addr,
		Signer: userPasswordAwsSigner,
		// EnableDebugLogger: true,
	}

	osClient, err := opensearch.NewClient(opensearchCfg)
	if err != nil {
		log.Printf("Opensearch Connection Error: %s", err.Error())
		return events.APIGatewayProxyResponse{}, err
	}

	log.Print("Established connection to opensearch serverless 123")

	/*auth0LogManagerInput := &Auth0LogEntityManagerInput{
		OsClient: osClient,
	}

	auth0LogManager := Auth0LogEntityManager(auth0LogManagerInput)

	for _, line := range lines {

		if line != "" {

			var e map[string]interface{}
			internalId := utils.GenerateId()
			err = json.Unmarshal([]byte(line), &e)
			if err != nil {
				log.Print(`Error unmarshaling ` + internalId)
			} else {
				// e["userId"] = payload.UserId
				e["id"] = internalId
				auth0LogManager.Save(e, "default")
			}

		}

	}*/

	// Send the bulk request
	// _, err = osClient.Bulk(strings.NewReader(bulkBody.String()), osClient.Bulk.WithRefresh("true"))
	// bulkBytes := []byte(bulkBody.String())
	bulkRes, err := osClient.Bulk(strings.NewReader(bulkBody.String()))
	if err != nil {
		// Handle error
		log.Print(bulkRes)
		log.Printf("Opensearch Query Error: %s", err.Error())
		return events.APIGatewayProxyResponse{}, err
	}

	res.StatusCode = 200
	res.Body = "{ \"message\": \"success\" }"

	return res, nil
}

func AossPost(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {

	res := events.APIGatewayProxyResponse{}

	log.Print("AossPost")
	// log.Print(req.Headers["Authorization"]);
	log.Print(req.Body)

	// Split the body of the request into lines
	//lines := strings.Split(req.Body, "\n")

	// Prepare a bulk request
	/*var bulkBody strings.Builder
	for _, line := range lines {
		// Each line is a separate JSON object
		// Add action and metadata
		internalId := utils.GenerateId()
		bulkBody.WriteString(`{ "index" : { "_index" : "auth0_log", "_id" : "` + internalId + `" } }`)
		bulkBody.WriteString("\n")

		// Add source data
		bulkBody.WriteString(line)
		bulkBody.WriteString("\n")
	}*/

	// log.Print(bulkBody.String())

	userPasswordAwsSigner := sign.UserPasswordAwsSigner{
		Service: "aoss",
		// Service:            "es",
		Region:             "us-east-1",
		Session:            ac.Session,
		IdentityPoolId:     os.Getenv("IDENTITY_POOL_ID"),
		Issuer:             os.Getenv("ISSUER"),
		Username:           os.Getenv("DEFAULT_SIGNING_USERNAME"),
		Password:           os.Getenv("DEFAULT_SIGNING_PASSWORD"),
		CognitoAppClientId: os.Getenv("COGNITO_APP_CLIENT_ID"),
	}

	// addr := []string = []string{"https://xelsubdau4tag8glbeb9.us-east-1.aoss.amazonaws.com"}
	addr := []string{"https://xelsubdau4tag8glbeb9.us-east-1.aoss.amazonaws.com"}
	// fuck serverless for now... perhaps not compatible with cognito... idk
	opensearchCfg := opensearch.Config{
		// Addresses: []string{os.Getenv("ELASTIC_URL")},
		Addresses: addr,
		Signer:    userPasswordAwsSigner,
		// EnableDebugLogger: true,
	}

	osClient, err := opensearch.NewClient(opensearchCfg)
	if err != nil {
		log.Printf("Opensearch Connection Error: %s", err.Error())
		return events.APIGatewayProxyResponse{}, err
	}

	log.Print("Established connection to opensearch serverless 123")

	auth0LogManagerInput := &Auth0LogEntityManagerInput{
		OsClient: osClient,
	}

	auth0LogManager := Auth0LogEntityManager(auth0LogManagerInput)

	/*for _, line := range lines {

	if line != "" {*/

	var e map[string]interface{}
	internalId := utils.GenerateId()
	err = json.Unmarshal([]byte(req.Body), &e)
	if err != nil {
		log.Print(`Error unmarshaling ` + internalId)
	} else {
		// e["userId"] = payload.UserId
		e["id"] = internalId
		auth0LogManager.Save(e, "default")
	}

	/*}

	}*/

	// Send the bulk request
	// _, err = osClient.Bulk(strings.NewReader(bulkBody.String()), osClient.Bulk.WithRefresh("true"))
	// bulkBytes := []byte(bulkBody.String())
	/*bulkRes, err := osClient.Bulk(strings.NewReader(bulkBody.String()))
	if err != nil {
		// Handle error
		log.Print(bulkRes)
		log.Printf("Opensearch Query Error: %s", err.Error())
		return events.APIGatewayProxyResponse{}, err
	}*/

	res.StatusCode = 200
	res.Body = "{ \"message\": \"success\" }"

	return res, nil
}

func InitializeHandler(c *ActionContext) Handler {
	return func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

		usageLog := &utils.LogUsageLambdaInput{
			// UserId: GetUserId(req),
			//Username:     GetUsername(req),
			UserId:       "null",
			Username:     "null",
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
			usageLog.Organization = req.PathParameters["owner"]
		}
		_, hasRepo := req.PathParameters["repo"]
		if hasRepo {
			usageLog.Repository = req.PathParameters["repo"]
		}

		utils.LogUsageForLambdaWithInput(usageLog)

		ac := RequestActionContext(c)
		ac.EntityManager = NewManager(ac)
		// ac.EntityManager.Config.LogUsageForLambdaWithInput = usageLog

		if req.HTTPMethod == "GET" && strings.Index(req.Path, "publicuserprofile") > -1 {
			return GetEntity(req, ac)
		} else if req.HTTPMethod == "GET" && strings.Index(req.Path, "github/signup") > -1 {
			return GithubSignup(req, ac)
		} else if req.HTTPMethod == "POST" && strings.Index(req.Path, "github/marketplace/event") > -1 {
			return GithubMarketplaceEvent(req, ac)
		} else if req.HTTPMethod == "POST" && strings.Index(req.Path, "okta/log/event") > -1 {
			return OktaLogEvent(req, ac)
		} else if req.HTTPMethod == "POST" && strings.Index(req.Path, "user/aoss") > -1 {
			return AossPost(req, ac)
		}

		return events.APIGatewayProxyResponse{StatusCode: 500}, nil
	}
}

func NewManager(ac *ActionContext) entity.EntityManager {
	return entity.EntityManager{
		Loaders: map[string]entity.Loader{
			"default": entity.CognitoLoaderAdaptor{
				Config: entity.CognitoAdaptorConfig{
					Client:     ac.Client,
					UserPoolId: ac.UserPoolId,
					Transform: func(user *cognitoidentityprovider.UserType) (map[string]interface{}, error) {
						ent := make(map[string]interface{})
						ent["userName"] = user.Username
						for _, attr := range user.Attributes {
							if *attr.Name == "sub" {
								ent["id"] = attr.Value
							}
						}
						return ent, nil
					},
				},
			},
		},
	}
}

func Auth0LogEntityManager(input *Auth0LogEntityManagerInput) *entity.EntityManager {
	manager := entity.NewDefaultManager(entity.DefaultManagerConfig{
		SingularName: "auth0_log",
		PluralName:   "auth0_logs",
		Stage:        os.Getenv("STAGE"),
	})
	manager.AddAuthorizer("default", entity.NoopAuthorizationAdaptor{})
	manager.AddStorage("default", entity.OpensearchStorageAdaptor{
		Config: entity.OpensearchAdaptorConfig{
			Index:  "auth0_log",
			Client: input.OsClient,
		},
	})
	return &manager
}

func RequestActionContext(ac *ActionContext) *ActionContext {
	return &ActionContext{
		Session:    ac.Session,
		Client:     ac.Client,
		SesClient:  ac.SesClient,
		Stage:      ac.Stage,
		UserPoolId: ac.UserPoolId,
	}
}

func init() {
	log.Printf("cold start")

	sess := session.Must(session.NewSession())
	client := cognitoidentityprovider.New(sess)
	sesClient := ses.New(sess)

	actionContext := ActionContext{
		Session:    sess,
		Client:     client,
		SesClient:  sesClient,
		UserPoolId: os.Getenv("USER_POOL_ID"),
		Stage:      os.Getenv("STAGE"),
	}

	handler = InitializeHandler(&actionContext)
}

func main() {
	log.SetFlags(0)
	lambda.Start(handler)
}
