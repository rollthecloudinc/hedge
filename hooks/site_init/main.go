package main

import (
	"context"
	"goclassifieds/lib/entity"
	"goclassifieds/lib/repo"
	"log"
	"fmt"
	"sync"
	"os"
	"github.com/aws/aws-lambda-go/lambda"
	"golang.org/x/oauth2"
	"github.com/google/go-github/v46/github"
)

type RepoInitParams struct {
	TemplateOwner string
	TemplateRepo  string
	NewRepoOwner  string
	NewRepoName   string
	Description   string
	Private       bool
}

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

	// @todo: This will be retrieved from payload.
	owner := "rollthecloudinc"

	// @Params entity.repoName
	// @params Owner

	repoParams := []RepoInitParams{
		{owner, "spearhead", owner, repoName, "Spearhead website source code.", false},
		{owner, "spearhead-objects", owner, repoName + "-objects", "Sreadhead website objects repository.", false},
		{owner, "spearhead-objects-prod", owner, repoName + "-objects-prod", "Spearhead website objects production repository", false},
		{owner, "spearhead-build", owner, repoName + "-build", "Spearhead website distribution.", false},
		// Add as many parameters as needed
	}

	// @todo: Payload property values all empty.
	// Use hard coded values for now.
	/*fmt.Printf("Payload received: %+v\n", payload)
	fmt.Printf("Entity field: %+v\n", payload.Entity)
	fmt.Printf("Storage field: %s\n", payload.Storage)
	fmt.Printf("Stage field: %s\n", payload.Stage)*/

	/**
	* This is where all the code goes to create the repos.
	*/
	log.Print("Create repos")

	pem, err := os.ReadFile("rtc-vertigo-" + os.Getenv("STAGE") + ".private-key.pem")
	if err != nil {
		log.Print(err)
		return entity.AfterSaveExecEntityResponse{}, err
	} else {
		log.Print("Github app pem file has been loaded.")
		log.Print(pem)
	}

	getTokenInput := &repo.GetInstallationTokenInput{
		GithubAppPem: pem,
		Owner:        owner,
		GithubAppId:  os.Getenv("GITHUB_APP_ID"),
	}
	installationToken, err := repo.GetInstallationToken(getTokenInput)
	if err != nil {
		log.Print("Error generating installation token", err.Error())
		return entity.AfterSaveExecEntityResponse{}, err
	}
	srcToken := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: *installationToken.Token},
	)
	// log.Printf("The source token: %s", srcToken)

	httpClient := oauth2.NewClient(context.Background(), srcToken)
	githubRestClient := github.NewClient(httpClient)

		var wg sync.WaitGroup
		errChan := make(chan error, len(repoParams)) // Buffer for the maximum number of goroutines

		// Run each initialization in parallel
		for _, params := range repoParams {
			wg.Add(1)
			go func(params RepoInitParams) {
				defer wg.Done()
				err := AutomateRepoInit(
					context.Background(),
					githubRestClient,
					params.TemplateOwner,
					params.TemplateRepo,
					params.NewRepoOwner,
					params.NewRepoName,
					params.Description,
					params.Private,
				)
				if err != nil {
					errChan <- err
				}
			}(params)
		}

		// Wait for all goroutines to finish
		wg.Wait()
		close(errChan)

		// Check for errors
		var hasErrors bool
		for err := range errChan {
			hasErrors = true
			log.Printf("Error: %s\n", err.Error())
		}

		if hasErrors {
			log.Print("One or more repository initializations failed.")
			return entity.AfterSaveExecEntityResponse{}, err
		} else {
			log.Println("Successfully automated repository initialization for all repositories.")
		}

	return entity.AfterSaveExecEntityResponse{}, nil

}

func AutomateRepoInit(ctx context.Context, githubRestClient *github.Client, templateOwner, templateRepo, newRepoOwner, newRepoName, description string, private bool) error {
	log.Printf("Creating repository '%s/%s' from template '%s/%s'...", newRepoOwner, newRepoName, templateOwner, templateRepo)

	// Step 1: Create repository from template
	err := repo.CreateFromTemplate(ctx, githubRestClient.Client(), templateOwner, templateRepo, newRepoOwner, newRepoName, description, private)
	if err != nil {
		return fmt.Errorf("failed to create repository from template: %w", err)
	}

	log.Printf("Repository '%s/%s' successfully created from template. Proceeding with dev branch creation...", newRepoOwner, newRepoName)

	// Step 2: Create the "dev" branch
	err = repo.CreateDevBranch(githubRestClient, newRepoOwner, newRepoName)
	if err != nil {
		return fmt.Errorf("failed to create dev branch: %w", err)
	}

	log.Println("Dev branch successfully created.")
	return nil
}

func main() {
	log.SetFlags(0)
	// Make the handler available for Remote Procedure Call by AWS Lambda
	lambda.Start(handler)
}
