package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"goclassifieds/lib/gov"
	"goclassifieds/lib/repo"
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
	Session          *session.Session
	BucketName       string
	GithubV4Client   *githubv4.Client
	GithubRestClient *github.Client
	Lambda           *lambda2.Lambda
	Stage            string
}

func UploadMediaFile(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {

	res := events.APIGatewayProxyResponse{StatusCode: 403}

	suffix := ""
	if os.Getenv("GITHUB_BRANCH") == "master" {
		suffix = "-prod"
	}

	asset := "rollthecloudinc/" + req.PathParameters["site"] + "-objects" + suffix
	grantAccessRequest := gov.GrantAccessRequest{
		User:      GetUserId(req),
		Type:      gov.User,
		Resource:  gov.GithubRepo,
		Operation: gov.Write,
		Asset:     asset,
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

	body, err := base64.StdEncoding.DecodeString(req.Body)
	if err != nil {
		return res, err
	}

	r := http.Request{
		Method: req.HTTPMethod,
		Header: map[string][]string{
			"Content-Type": {req.Headers["Content-Type"]},
		},
		Body: ioutil.NopCloser(bytes.NewBuffer(body)),
	}

	file, header, err := r.FormFile("File")
	defer file.Close()
	if err != nil {
		return res, err
	}

	// contentType := header.Header.Get("Content-Type")
	//ext, _ := mime.ExtensionsByType(contentType)
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

	// Necessary to commit to github but not for s3
	dataBuffer := bytes.NewBuffer(nil)
	if _, err := io.Copy(dataBuffer, file); err != nil {
		return res, err
	}

	d := []byte(dataBuffer.String())
	params := repo.CommitParams{
		Repo:   asset,
		Branch: os.Getenv("GITHUB_BRANCH"),
		Path:   "media/" + id + "." + ext,
		Data:   &d,
	}

	repo.Commit(
		ac.GithubV4Client,
		&params,
	)

	/*userId := GetUserId(req)
	uploader := s3manager.NewUploader(ac.Session)
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(ac.BucketName),
		Key:         aws.String(data["path"]),
		Body:        file,
		ContentType: aws.String(data["contentType"]),
		Metadata:    map[string]*string{"userId": &userId},
	})
	if err != nil {
		return res, err
	}*/

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

	pathPieces := strings.Split(req.Path, "/")
	siteName := pathPieces[1]
	file, _ := url.QueryUnescape(pathPieces[3]) // pathPieces[2]

	log.Print("requested media site: " + siteName)
	log.Print("requested media file: " + file)

	// buf := aws.NewWriteAtBuffer([]byte{})

	// downloader := s3manager.NewDownloader(ac.Session)

	/*_, err := downloader.Download(buf, &s3.GetObjectInput{
		Bucket: aws.String(ac.BucketName),
		Key:    aws.String("media/" + file),
	})

	if err != nil {
		return res, err
	}*/

	ext := strings.Split(pathPieces[len(pathPieces)-1], ".")
	contentType := mime.TypeByExtension(ext[len(ext)-1])

	log.Print("content type: " + contentType)
	if ext[len(ext)-1] == "md" {
		contentType = "text/markdown"
	} else if ext[len(ext)-1] == "svg" {
		contentType = "image/svg+xml"
	}

	suffix := ""
	if os.Getenv("GITHUB_BRANCH") == "master" {
		suffix = "-prod"
	}

	owner := "rollthecloudinc"
	repo := siteName + "-objects" + suffix

	var q struct {
		Repository struct {
			Object struct {
				ObjectFragment struct {
					Oid githubv4.GitObjectID
				} `graphql:"... on Blob"`
			} `graphql:"object(expression: $exp)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}
	qVars := map[string]interface{}{
		"exp":   githubv4.String(os.Getenv("GITHUB_BRANCH") + ":media/" + file),
		"owner": githubv4.String(owner),
		"name":  githubv4.String(repo),
	}

	err := ac.GithubV4Client.Query(context.Background(), &q, qVars)
	if err != nil {
		log.Print("Github latest file failure.")
		log.Panic(err)
	}

	oid := q.Repository.Object.ObjectFragment.Oid
	log.Print("Github file object id " + oid)

	blob, _, err := ac.GithubRestClient.Git.GetBlob(context.Background(), owner, repo, string(oid))
	if err != nil {
		log.Print("Github get blob failure.")
		log.Panic(err)
	}

	res.StatusCode = 200
	res.Headers = map[string]string{
		"Content-Type": contentType,
	}
	res.Body = blob.GetContent()
	res.IsBase64Encoded = true
	return res, nil
}

func InitializeHandler(ac ActionContext) Handler {
	return func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

		_, hedged := req.Headers["x-hedge-region"]
		if hedged {
			log.Print("REPORT RequestId: " + req.RequestContext.RequestID + " Function: " + os.Getenv("AWS_LAMBDA_FUNCTION_NAME") + " Path: " + req.Path + " Resource: " + req.Resource + " X-HEDGE-REGIONS: " + req.Headers["x-hedge-regions"] + " X-HEDGE-INTENSITIES: " + req.Headers["x-hedge-intensities"] + " X-HEDGE-REGION: " + req.Headers["x-hedge-region"] + " X-HEDGE-SERVICE: " + req.Headers["x-hedge-service"])
		} else {
			log.Print("REPORT RequestId: " + req.RequestContext.RequestID + " Function: " + os.Getenv("AWS_LAMBDA_FUNCTION_NAME") + " Path: " + req.Path + " Resource: " + req.Resource)
		}

		if req.HTTPMethod == "POST" {
			return UploadMediaFile(req, &ac)
		} else {
			return GetMediaFile(req, &ac)
		}
	}
}

func GetUserId(req *events.APIGatewayProxyRequest) string {
	userId := ""
	if req.RequestContext.Authorizer["claims"] != nil {
		userId = fmt.Sprint(req.RequestContext.Authorizer["claims"].(map[string]interface{})["sub"])
		if userId == "<nil>" {
			userId = ""
		}
	}
	return userId
}

func init() {
	log.Printf("Gin cold start")
	sess := session.Must(session.NewSession())
	lClient := lambda2.New(sess)

	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	httpClient := oauth2.NewClient(context.Background(), src)

	githubV4Client := githubv4.NewClient(httpClient)
	githubRestClient := github.NewClient(httpClient)

	actionContext := ActionContext{
		Session:          sess,
		BucketName:       os.Getenv("BUCKET_NAME"),
		GithubV4Client:   githubV4Client,
		GithubRestClient: githubRestClient,
		Stage:            os.Getenv("STAGE"),
		Lambda:           lClient,
	}
	handler = InitializeHandler(actionContext)
}

func main() {
	lambda.Start(handler)
}
