package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"goclassifieds/lib/entity"
	"goclassifieds/lib/repo"

	"golang.org/x/oauth2"
	"github.com/aws/aws-lambda-go/lambda"         // Used for defining the Lambda handler
	lambdaSvc "github.com/aws/aws-sdk-go/service/lambda" // AWS SDK for interacting with Lambda
	"github.com/aws/aws-sdk-go/aws/session"       // AWS session for SDK service creation
)

func handler(ctx context.Context, event map[string]interface{}) (entity.AfterSaveExecEntityResponse, error) {

		// Navigate through the map to get the 'repoName'
	input, ok := event["Input"].(map[string]interface{})
	if !ok {
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to parse 'Input' field from event")
	}

	ent, ok := input["entity"].(map[string]interface{})
	if !ok {
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to parse 'entity' field from Input")
	}

	repoName, ok := ent["repoName"].(string)
	if !ok {
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to parse 'repoName' from entity")
	}

	// Log or use the 'repoName'
	log.Printf("Repository Name: %s", repoName)
	
	owner := "rollthecloudinc"  // Hardcoded owner (to be replaced by payload.Entity in later iterations)
	// repoName := "site12"
	repoBuildName := repoName + "-build"         // Hardcoded repo name (to be replaced by payload.Entity)

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

	privateKey, publicKey, err := repo.GenerateSSHKeyPair()
	if err != nil {
		log.Print("Failed to ssh key pair")
		return entity.AfterSaveExecEntityResponse{}, err
	}

	log.Printf("ssh private key: %s", privateKey)
	log.Printf("ssh public key: %s", publicKey)

	err = repo.CreateGithubDeployKey(context.Background(), httpClient, owner, repoBuildName, "Automated SSH Deploy Key", publicKey, false)
	if err != nil {
		log.Print("Failed to create deploy deploy")
		return entity.AfterSaveExecEntityResponse{}, err
	}

	// Now add the private key to the source code repo as a repo secret.
	// Step: Create environment secrets
	err = repo.CreateGithubRepositorySecret(context.Background(), httpClient, lClient, owner, repoName, "SSH_DEPLOY_KEY", privateKey, stage)
	if err != nil {
		log.Print("Failed to create SSH_DEPLOY_KEY repository secret")
		return entity.AfterSaveExecEntityResponse{}, err
	}

	log.Printf("Repository secret %s created successfullly.", "SSH_DEPLOY_KEY")

	return entity.AfterSaveExecEntityResponse{}, nil
}

func main() {
	log.SetFlags(0) // Output logs without timestamps
	// Make the handler available for invocation by AWS Lambda
	lambda.Start(handler)
}