package main

import (
	"context"
	"goclassifieds/lib/entity"
	"log"
	"encoding/json"
	"os"
	"fmt"

	"goclassifieds/lib/repo"

	"github.com/aws/aws-lambda-go/lambda"
	"golang.org/x/oauth2"
	"github.com/google/go-github/v46/github"
)

func handler(ctx context.Context, event entity.AfterSaveExecEntityRequest) (entity.AfterSaveExecEntityResponse, error) {

	/**
	 * This is where all the code goes to create action SECRETS
	 * for a site. Both for repo and enviironment.
	 */
	log.Printf("Create index for entity %s in repo %s and owner %s", event.Contract, event.Repo, event.Owner)

	// Log the entire content of event.Entity
	entityJSON, err := json.Marshal(event.Entity) // Convert map[string]interface{} to JSON string for readable logging
	if err != nil {
		log.Printf("Error marshalling event.Entity: %s", err)
	} else {
		log.Printf("Entity content: %s", entityJSON)
	}

	repoName, ok := event.Entity["repoName"].(string)
	if ok {
		log.Printf("The new repo name will be %s", repoName)
	}

	githubAppID := os.Getenv("GITHUB_APP_ID")
	if githubAppID == "" {
		err := fmt.Errorf("environment variable GITHUB_APP_ID is missing")
		log.Print(err)
		return entity.AfterSaveExecEntityResponse{}, err
	}

	// Load GitHub app PEM file
	pemFilePath := fmt.Sprintf("rtc-vertigo-%s.private-key.pem", os.Getenv("STAGE"))
	pem, err := os.ReadFile(pemFilePath)
	if err != nil {
		log.Printf("Failed to read PEM file '%s': %v", pemFilePath, err)
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to load GitHub app PEM file: %w", err)
	}
	log.Print("GitHub app PEM file loaded successfully.")

	// Generate GitHub Installation Token
	getTokenInput := &repo.GetInstallationTokenInput{
		GithubAppPem: pem,
		Owner:        event.Owner,
		GithubAppId:  githubAppID,
	}
	installationToken, err := repo.GetInstallationToken(getTokenInput)
	if err != nil {
		log.Printf("Error generating GitHub installation token for owner '%s': %v", event.Owner, err)
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to generate GitHub installation token: %w", err)
	}
	log.Print("GitHub installation token generated successfully.")

	// Create OAuth2 HTTP client
	srcToken := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: *installationToken.Token})
	httpClient := oauth2.NewClient(ctx, srcToken)
	githubRestClient := github.NewClient(httpClient)

	// Step 1: Create repository from template
	templateOwner := "rollthecloudinc"
	templateRepo := "spearhead-index"
	description := "index repository"
	private := false // index repositories are private by default content should only be accessed via searches. Ah... make it public for now
	err = repo.CreateFromTemplate(ctx, githubRestClient.Client(), templateOwner, templateRepo, event.Owner, repoName, description, private)
	if err != nil {
		log.Printf("Error creating index repository: %s", err.Error())
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to create repository from template: %w", err)
	}

	log.Printf("Repository '%s/%s' successfully created from template. Proceeding with dev branch creation...", event.Owner, repoName)

	// Step 2: Create the "dev" branch
	err = repo.CreateDevBranch(githubRestClient, event.Owner, repoName)
	if err != nil {
		log.Printf("Error provisioning dev branch for index repository.")
		return entity.AfterSaveExecEntityResponse{}, fmt.Errorf("failed to create dev branch: %w", err)
	}

	return entity.AfterSaveExecEntityResponse{}, nil
}

func main() {
	log.SetFlags(0)
	// Make the handler available for Remote Procedure Call by AWS Lambda
	lambda.Start(handler)
}