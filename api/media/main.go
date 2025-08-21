package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"goclassifieds/lib/gov"
	"goclassifieds/lib/repo"
	"goclassifieds/lib/utils"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	session "github.com/aws/aws-sdk-go/aws/session"
	lambda2 "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/google/go-github/v46/github"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

// var ginLambda *ginadapter.GinLambda
var handler Handler

type Handler func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error)

type ActionContext struct {
	Session             *session.Session
	BucketName          string
	GithubV4Client      *githubv4.Client
	GithubRestClient    *github.Client
	Lambda              *lambda2.Lambda
	Stage               string
	Site                string
	GithubAppPem        []byte
	AdditionalResources *[]gov.Resource
	LogUsageLambdaInput *utils.LogUsageLambdaInput
}

func UploadMediaFile(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {

	res := events.APIGatewayProxyResponse{StatusCode: 403}
	var asset string

	suffix := ""
	if os.Getenv("GITHUB_BRANCH") == "master" {
		suffix = "-prod"
	}

	_, hasOwner := req.PathParameters["owner"]
	_, hasRepo := req.PathParameters["repo"]
	if hasOwner && hasRepo {
		asset = req.PathParameters["owner"] + "/" + req.PathParameters["repo"]
	} else {
		asset = "rollthecloudinc/" + req.PathParameters["site"] + "-objects" + suffix
	}
	grantAccessRequest := gov.GrantAccessRequest{
		User:                GetUserId(req),
		Type:                gov.User,
		Resource:            gov.GithubRepo,
		Operation:           gov.Write,
		Asset:               asset,
		AdditionalResources: *ac.AdditionalResources,
		LogUsageLambdaInput: ac.LogUsageLambdaInput,
	}

	payload, err := json.Marshal(grantAccessRequest)
	if err != nil {
		log.Printf("Error marshalling grant access request: %s", err.Error())
		res.StatusCode = 500
		return res, err
	}

	lambdaRes, err := ac.Lambda.Invoke(&lambda2.InvokeInput{FunctionName: aws.String("goclassifieds-api-" + ac.Stage + "-GrantAccess"), Payload: payload})
	if err != nil {
		log.Printf("error invoking grant_access: %s", err.Error())
		res.StatusCode = 500
		return res, err
	}

	var grantRes gov.GrantAccessResponse
	json.Unmarshal(lambdaRes.Payload, &grantRes)

	if !grantRes.Grant {
		res.StatusCode = 403
		res.Body = "Unauthorized to write files."
		return res, nil
	}

	log.Print("Encoding body")

	body, err := base64.StdEncoding.DecodeString(req.Body)
	if err != nil {
		return res, err
	}

	log.Print("Building Http Request")

	r := http.Request{
		Method: req.HTTPMethod,
		Header: map[string][]string{
			"Content-Type": {req.Headers["Content-Type"]},
		},
		Body: ioutil.NopCloser(bytes.NewBuffer(body)),
	}

	log.Print("Done Building Http Request")

	file, header, err := r.FormFile("File")
	if err != nil {
		log.Printf("error reading form file: %s", err.Error())
		return res, err
	}
	defer file.Close()

	log.Print("Setting file name")

	id := header.Filename
	ext := ""
	contentType := ""
	if pos := strings.LastIndexByte(header.Filename, '.'); pos != -1 {
		id = header.Filename[:pos]
		extIndex := pos + 1
		ext = header.Filename[extIndex:]
		contentType = mime.TypeByExtension(ext)
	}

	if contentType == "text/markdown" {
		ext = "md"
	}

	log.Print("data buffer")

	dataBuffer := bytes.NewBuffer(nil)
	if _, err := io.Copy(dataBuffer, file); err != nil {
		return res, err
	}

	d := []byte(dataBuffer.String())
	params := repo.CommitParams{
		Repo:     asset,
		Branch:   os.Getenv("GITHUB_BRANCH"),
		Path:     "media/" + id + "." + ext,
		Data:     &d,
		UserName: GetUsername(req),
	}

	log.Print("Commit rest optimized")

	repo.CommitRestOptimized(
		ac.GithubRestClient,
		&params,
	)

	data := map[string]string{
		"id":                 id,
		"path":               "media/" + id + "." + ext,
		"contentType":        contentType,
		"contentDisposition": header.Header.Get("Content-Disposition"),
		"length":             fmt.Sprint(header.Size),
	}

	res.StatusCode = 200
	res.Headers = map[string]string{
		"Content-Type": "application/json",
	}

	body, err = json.Marshal(data)
	res.Body = string(body)

	return res, nil
}

func GetMediaFile(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {
	res := events.APIGatewayProxyResponse{StatusCode: 500}

	_, hasOwner := req.PathParameters["owner"]
	_, hasRepo := req.PathParameters["repo"]

	pathPieces := strings.Split(req.Path, "/")

	var file string
	var owner string
	var repo string

	if hasOwner && hasRepo {
		file, _ = url.QueryUnescape(pathPieces[4])
		repo = req.PathParameters["repo"]
		owner = req.PathParameters["owner"]
	} else {
		file, _ = url.QueryUnescape(pathPieces[3])
		owner = "rollthecloudinc"
		suffix := ""
		if os.Getenv("GITHUB_BRANCH") == "master" {
			suffix = "-prod"
		}
		repo = pathPieces[1] + "-objects" + suffix
	}

	log.Print("requested media file: " + file)

	ext := strings.Split(pathPieces[len(pathPieces)-1], ".")
	contentType := mime.TypeByExtension(ext[len(ext)-1])

	log.Print("content type: " + contentType)
	if ext[len(ext)-1] == "md" {
		contentType = "text/markdown"
	} else if ext[len(ext)-1] == "svg" {
		contentType = "image/svg+xml"
	}

	path := "media/" + file

	opts := &github.RepositoryContentGetOptions{
		Ref: os.Getenv("GITHUB_BRANCH"),
	}
	content, _, resContent, err := ac.GithubRestClient.Repositories.GetContents(context.Background(), owner, repo, path, opts)
	if err != nil && resContent.StatusCode != 404 {
		log.Print("Github get content failure.")
		res.StatusCode = 400
	} else {
		res.StatusCode = 200
		res.Headers = map[string]string{
			"Content-Type": contentType,
		}
		res.Body = *content.Content
		res.IsBase64Encoded = true
	}
	return res, nil
}

func InitializeHandler(ac ActionContext) Handler {
	return func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

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
			usageLog.Organization = req.PathParameters["owner"]
		}
		_, hasRepo := req.PathParameters["repo"]
		if hasRepo {
			usageLog.Repository = req.PathParameters["repo"]
		}

		ac.LogUsageLambdaInput = usageLog
		utils.LogUsageForLambdaWithInput(usageLog)

		ac := RequestActionContext(&ac, req)

		log.Print("http method " + req.HTTPMethod)

		if req.HTTPMethod == "POST" {
			log.Print("UploadMediaFile")
			return UploadMediaFile(req, ac)
		} else {
			log.Print("GetMediaFile2")
			return GetMediaFile(req, ac)
		}
	}
}

func RequestActionContext(ac *ActionContext, req *events.APIGatewayProxyRequest) *ActionContext {

	var githubToken string
	var githubRestClient *github.Client
	var srcToken oauth2.TokenSource
	additionalResources := make([]gov.Resource, 0)

	_, hasOwner := req.PathParameters["owner"]
	_, hasRepo := req.PathParameters["repo"]
	if !hasOwner || !hasRepo || req.HTTPMethod == "GET" {
		githubToken = os.Getenv("GITHUB_TOKEN")
		srcToken = oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: githubToken},
		)
		httpClient := oauth2.NewClient(context.Background(), srcToken)
		githubRestClient = github.NewClient(httpClient)
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
		username = "angular.druid@gmail.com"

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
	githubV4Client := githubv4.NewClient(httpClient)
	//githubRestClient := github.NewClient(httpClient)

	//token := req.Headers["authorization"][7:]
	//log.Print("token: " + token)

	/*awsSigner := sign.AwsSigner{
		Service:        "es",
		Region:         "us-east-1",
		Session:        ac.Session,
		IdentityPoolId: os.Getenv("IDENTITY_POOL_ID"),
		Issuer:         os.Getenv("ISSUER"),
		Token:          token,
	}*/

	/*opensearchCfg := opensearch.Config{
		Addresses: []string{os.Getenv("ELASTIC_URL")},
		Signer:    awsSigner,
	}*/

	/*osClient, err := opensearch.NewClient(opensearchCfg)
	if err != nil {
		log.Printf("Opensearch Error: %s", err.Error())
	}*/

	return &ActionContext{
		//EsClient:            ac.EsClient,
		//OsClient:            osClient,
		GithubV4Client:   githubV4Client,
		GithubRestClient: githubRestClient,
		Session:          ac.Session,
		Lambda:           ac.Lambda,
		//Template:            ac.Template,
		// Implementation:      "default",
		BucketName:          ac.BucketName,
		Stage:               ac.Stage,
		Site:                req.PathParameters["site"],
		LogUsageLambdaInput: ac.LogUsageLambdaInput,
		AdditionalResources: &additionalResources,
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

func init() {
	log.Printf("Gin cold start 123")
	sess := session.Must(session.NewSession())
	lClient := lambda2.New(sess)

	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	httpClient := oauth2.NewClient(context.Background(), src)

	githubV4Client := githubv4.NewClient(httpClient)
	githubRestClient := github.NewClient(httpClient)

	log.Print("Before read pem file.")

	pem, err := os.ReadFile("rtc-vertigo-" + os.Getenv("STAGE") + ".private-key.pem")
	if err != nil {
		log.Print("Error reading github app pem file", err.Error())
	}

	log.Print("After read pem file.")

	actionContext := ActionContext{
		Session:          sess,
		BucketName:       os.Getenv("BUCKET_NAME"),
		GithubV4Client:   githubV4Client,
		GithubRestClient: githubRestClient,
		Stage:            os.Getenv("STAGE"),
		Lambda:           lClient,
		GithubAppPem:     pem,
	}
	handler = InitializeHandler(actionContext)
}

func main() {
	log.SetFlags(0)
	lambda.Start(handler)
}
