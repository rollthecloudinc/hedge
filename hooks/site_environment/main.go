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

func handler(ctx context.Context, payload *entity.AfterSaveExecEntityRequest) (entity.AfterSaveExecEntityResponse, error) {

	owner := "rollthecloudinc"  // Hardcoded owner (to be replaced by payload.Entity in later iterations)
	repoName := "site12"         // Hardcoded repo name (to be replaced by payload.Entity)
	repoBuildName := "site12-build"

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

	// Create environment secrets (example secret used here)
	secretName := "DESTINATION_REPO"
	secretValue := repoBuildName
	err = createRepositorySecret(ctx, httpClient, lClient, owner, repoName, stage, secretName, secretValue)
	if err != nil {
		log.Printf("Failed to create repository secrets: %v", err)
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to create repository secrets: %w", err)
	}

	secretName = "DESTINATION_USER"
	secretValue = owner
	err = createRepositorySecret(ctx, httpClient, lClient, owner, repoName, stage, secretName, secretValue)
	if err != nil {
		log.Printf("Failed to create repository secrets: %v", err)
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to create repository secrets: %w", err)
	}

	secretName = "ENVIRONMENT_NAME"
	secretValue = "dev"
	environmentName := "dev"
	err = createEnvironmentSecret(ctx, httpClient, lClient, owner, repoName, environmentName, stage, secretName, secretValue)
	if err != nil {
		log.Printf("Failed to create environment secret: %v", err)
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to create environment secrets: %w", err)
	}

	secretName = "ENVIRONMENT_NAME"
	secretValue = "prod"
	environmentName = "prod"
	err = createEnvironmentSecret(ctx, httpClient, lClient, owner, repoName, environmentName, stage, secretName, secretValue)
	if err != nil {
		log.Printf("Failed to create environment secret: %v", err)
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to create environment secrets: %w", err)
	}

	secretName = "OBJECTS_REPO"
	secretValue = owner + "/" + repoName + "-objects"
	environmentName = "dev"
	err = createEnvironmentSecret(ctx, httpClient, lClient, owner, repoName, environmentName, stage, secretName, secretValue)
	if err != nil {
		log.Printf("Failed to create environment secret: %v", err)
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to create environment secrets: %w", err)
	}

	secretName = "OBJECTS_REPO"
	secretValue = owner + "/" + repoName + "-objects-prod"
	environmentName = "prod"
	err = createEnvironmentSecret(ctx, httpClient, lClient, owner, repoName, environmentName, stage, secretName, secretValue)
	if err != nil {
		log.Printf("Failed to create environment secret: %v", err)
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to create environment secrets: %w", err)
	}

	secretName = "TARGET_BRANCH"
	secretValue = "dev"
	environmentName = "dev"
	err = createEnvironmentSecret(ctx, httpClient, lClient, owner, repoName, environmentName, stage, secretName, secretValue)
	if err != nil {
		log.Printf("Failed to create environment secret: %v", err)
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to create environment secrets: %w", err)
	}

	secretName = "TARGET_BRANCH"
	secretValue = "master"
	environmentName = "prod"
	err = createEnvironmentSecret(ctx, httpClient, lClient, owner, repoName, environmentName, stage, secretName, secretValue)
	if err != nil {
		log.Printf("Failed to create environment secret: %v", err)
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to create environment secrets: %w", err)
	}

	log.Printf("Secrets successfully created for repository '%s/%s'.", owner, repoName)

	return entity.AfterSaveExecEntityResponse{}, nil
}


func createRepositorySecret(ctx context.Context, httpClient *http.Client, lClient *lambdaSvc.Lambda, owner string, repoName string, stage string, secretName string, secretValue string) error {
	// Retrieve secret values from environment variables
	//secretName := "DESTINATION_REPO"
	if secretValue == "" {
		return fmt.Errorf("repository variable '%s' is missing", secretName)
	}

	// Call the appropriate function to create the GitHub repository secret
	err := repo.CreateGithubRepositorySecret(ctx, httpClient, lClient, owner, repoName, secretName, secretValue, stage)
	if err != nil {
		log.Printf("Failed to create repository secret: %v", err)
		return fmt.Errorf("failed to create GitHub repository secret: %w", err)
	}

	log.Printf("Repository secret '%s' created successfully.", secretName)
	return nil
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