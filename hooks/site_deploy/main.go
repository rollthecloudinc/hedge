package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"goclassifieds/lib/entity"
	"goclassifieds/lib/repo"

	"golang.org/x/oauth2"
	"github.com/aws/aws-lambda-go/lambda"         // Used for defining the Lambda handler
	lambdaSvc "github.com/aws/aws-sdk-go/service/lambda" // AWS SDK for interacting with Lambda
	"github.com/aws/aws-sdk-go/aws/session"       // AWS session for SDK service creation
)

func handler(ctx context.Context, event map[string]interface{}) (entity.AfterSaveExecEntityResponse, error) {

	// Navigate through the map to get the prodDeploymentToken
	input, ok := event["Input"].(map[string]interface{})
	if !ok {
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to parse 'Input' field from event")
	}

	siteStaticWebAppResult, ok := input["siteStaticWebAppResult"].(map[string]interface{})
	if !ok {
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to parse 'siteStaticWebAppResult' from Input")
	}

	devDeploymentToken, ok := siteStaticWebAppResult["devDeploymentToken"].(string)
	if !ok {
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to parse 'devDeploymentToken' from siteStaticWebAppResult")
	}

	prodDeploymentToken, ok := siteStaticWebAppResult["prodDeploymentToken"].(string)
	if !ok {
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to parse 'prodDeploymentToken' from siteStaticWebAppResult")
	}

	ent, ok := input["entity"].(map[string]interface{})
	if !ok {
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to parse 'entity' field from Input")
	}

	repoName := ent["repoName"].(string)

	// Log or use the 'repoName'
	//log.Printf("Repository Name: %s", repoName)

	// Log or use the prodDeploymentToken
	log.Printf("Dev Deployment Token: %s", devDeploymentToken)
	log.Printf("Production Deployment Token: %s", prodDeploymentToken)

	owner := "rollthecloudinc"  // Hardcoded owner (to be replaced by payload.Entity in later iterations)
	// repoName := "site13"         // Hardcoded repo name (to be replaced by payload.Entity)
	repoBuildName := repoName + "-build" //"site13-build"

	// Validate environment variables
	stage := os.Getenv("STAGE")
	if stage == "" {
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("environment variable STAGE is missing")
	}

	githubAppID := os.Getenv("GITHUB_APP_ID")
	if githubAppID == "" {
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("environment variable GITHUB_APP_ID is missing")
	}

	// Create an AWS session for Lambda client
	sess := session.Must(session.NewSession())
	lClient := lambdaSvc.New(sess)

	// Load GitHub app PEM file
	pemFilePath := fmt.Sprintf("rtc-vertigo-%s.private-key.pem", stage)
	pem, err := os.ReadFile(pemFilePath)
	if err != nil {
		log.Printf("Failed to read PEM file '%s': %v", pemFilePath, err)
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to load GitHub app PEM file: %w", err)
	}
	log.Print("GitHub app PEM file loaded successfully.")

	// Generate GitHub Installation Token
	getTokenInput := &repo.GetInstallationTokenInput{
		GithubAppPem: pem,
		Owner:        owner,
		GithubAppId:  githubAppID,
	}
	installationToken, err := repo.GetInstallationToken(getTokenInput)
	if err != nil {
		log.Printf("Error generating GitHub installation token for owner '%s': %v", owner, err)
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to generate GitHub installation token: %w", err)
	}

	// Create OAuth2 HTTP client
	srcToken := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: *installationToken.Token})
	httpClient := oauth2.NewClient(ctx, srcToken)

	secretName := "AZURE_STATIC_WEB_APPS_API_TOKEN"
	secretValue := devDeploymentToken
	environmentName := "dev"
	err = createEnvironmentSecret(ctx, httpClient, lClient, owner, repoBuildName, environmentName, stage, secretName, secretValue)
	if err != nil {
		log.Printf("Failed to create environment secret: %v", err)
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to create environment secrets: %w", err)
	}

	secretName = "AZURE_STATIC_WEB_APPS_API_TOKEN"
	secretValue = prodDeploymentToken
	environmentName = "prod"
	err = createEnvironmentSecret(ctx, httpClient, lClient, owner, repoBuildName, environmentName, stage, secretName, secretValue)
	if err != nil {
		log.Printf("Failed to create environment secret: %v", err)
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to create environment secrets: %w", err)
	}

	log.Printf("Secrets successfully created for repository '%s/%s'.", owner, repoName)

	return entity.AfterSaveExecEntityResponse{}, nil
}

func createEnvironmentSecret(ctx context.Context, httpClient *http.Client, lClient *lambdaSvc.Lambda, owner string, repoName string, environmentName string, stage string, secretName string, secretValue string) error {
	if secretValue == "" {
		return fmt.Errorf("repository variable '%s' is missing", secretName)
	}
	
	err := repo.CreateGithubEnvironmentSecret(context.Background(), httpClient, lClient, owner, repoName, environmentName, secretName, secretValue, stage)
	if err != nil {
		log.Printf("Failed to create environment secret: %v", err)
		return fmt.Errorf("failed to create GitHub environment secret: %w", err)
	}

	log.Printf("Environment secret '%s' created successfully.", secretName)
	return nil
}

func main() {
	log.SetFlags(0) // Output logs without timestamps
	// Make the handler available for invocation by AWS Lambda
	lambda.Start(handler)
}