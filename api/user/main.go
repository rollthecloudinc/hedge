package main

import (
	"context"
	"encoding/json"
	"goclassifieds/lib/entity"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

var handler Handler

type Handler func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error)

type ActionContext struct {
	Session       *session.Session
	Client        *cognitoidentityprovider.CognitoIdentityProvider
	EntityManager entity.Manager
	UserPoolId    string
	Stage         string
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
	res := events.APIGatewayProxyResponse{}
	code := req.QueryStringParameters["code"]
	// state := req.QueryStringParameters["state"]
	config := oauth2.Config{
		ClientID:     "Iv1.c72ed0518c7be356",
		ClientSecret: "X",
		RedirectURL:  "",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
		},
		Scopes: []string{},
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
	cogUser := &cognitoidentityprovider.SignUpInput{
		Username: user.Login,
		Password: aws.String("Test1234!"),
		ClientId: aws.String(os.Getenv("COGNITO_APP_CLIENT_ID")),
		UserAttributes: []*cognitoidentityprovider.AttributeType{
			{
				Name:  aws.String("email"),
				Value: primaryEmail,
			},
			{
				Name:  aws.String("custom:githubAccessToken"),
				Value: aws.String(token.AccessToken),
			},
			{
				Name:  aws.String("custom:githubRefreshToken"),
				Value: aws.String(token.RefreshToken),
			},
		},
	}
	u, err := ac.Client.SignUp(cogUser)
	if err != nil {
		log.Print(err.Error())
		res.StatusCode = 500
		return res, nil
	}
	updateInput := &cognitoidentityprovider.AdminUpdateUserAttributesInput{
		UserPoolId: aws.String(ac.UserPoolId),
		Username:   user.Login,
		UserAttributes: []*cognitoidentityprovider.AttributeType{
			{
				Name:  aws.String("email_verified"),
				Value: aws.String("true"),
			},
		},
	}
	_, err = ac.Client.AdminUpdateUserAttributes(updateInput)
	if err != nil {
		log.Print(err.Error())
		res.StatusCode = 500
		return res, nil
	}
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
	res.Body = u.String()
	return res, nil
}

func InitializeHandler(c *ActionContext) Handler {
	return func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

		ac := RequestActionContext(c)
		ac.EntityManager = NewManager(ac)

		if req.HTTPMethod == "GET" && strings.Index(req.Path, "publicuserprofile") > -1 {
			return GetEntity(req, ac)
		} else if req.HTTPMethod == "GET" && strings.Index(req.Path, "github/signup") > -1 {
			return GithubSignup(req, ac)
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

func RequestActionContext(ac *ActionContext) *ActionContext {
	return &ActionContext{
		Session:    ac.Session,
		Client:     ac.Client,
		Stage:      ac.Stage,
		UserPoolId: ac.UserPoolId,
	}
}

func init() {
	log.Printf("cold start")

	sess := session.Must(session.NewSession())
	client := cognitoidentityprovider.New(sess)

	actionContext := ActionContext{
		Session:    sess,
		Client:     client,
		UserPoolId: os.Getenv("USER_POOL_ID"),
		Stage:      os.Getenv("STAGE"),
	}

	handler = InitializeHandler(&actionContext)
}

func main() {
	lambda.Start(handler)
}
