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
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/google/go-github/github"
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
		emailInput := &ses.SendTemplatedEmailInput{
			Source:               aws.String("Security <sso@druidcloud.dev>"),
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
	lambda.Start(handler)
}
