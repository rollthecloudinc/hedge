package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"

	"goclassifieds/lib/entity"
	"goclassifieds/lib/repo"

	"golang.org/x/oauth2"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/google/go-github/v46/github"
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

	siteName := repoName
	owner := "rollthecloudinc"
	// repoName := "site13"
	// repoBuildName := "site11-build"

	// Validate environment variables
	stage := os.Getenv("STAGE")
	if stage == "" {
		err := fmt.Errorf("environment variable STAGE is missing")
		log.Print(err)
		return entity.AfterSaveExecEntityResponse{}, err
	}

	githubAppID := os.Getenv("GITHUB_APP_ID")
	if githubAppID == "" {
		err := fmt.Errorf("environment variable GITHUB_APP_ID is missing")
		log.Print(err)
		return entity.AfterSaveExecEntityResponse{}, err
	}

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
	log.Print("GitHub installation token generated successfully.")

	// Create OAuth2 HTTP client
	srcToken := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: *installationToken.Token})
	httpClient := oauth2.NewClient(ctx, srcToken)
	githubRestClient := github.NewClient(httpClient)

	err = modifyBranchFiles(ctx, githubRestClient, owner, repoName, siteName, stage, "dev")
	if err != nil {
		return entity.AfterSaveExecEntityResponse{}, err
	}

	err = modifyBranchFiles(ctx, githubRestClient, owner, repoName, siteName, stage, "master")
	if err != nil {
		return entity.AfterSaveExecEntityResponse{}, err
	}

	return entity.AfterSaveExecEntityResponse{}, nil
}

func modifyBranchFiles(ctx context.Context, githubRestClient *github.Client, owner string, repoName string, siteName string, stage string, branch string) error {
	var entries []*github.TreeEntry

	err := makeEnvironmentFileChanges(ctx, githubRestClient, &entries, owner, repoName, siteName, "environment.prod.ts", stage, branch)
	if err != nil {
		return err
	}

	err = makeEnvironmentFileChanges(ctx, githubRestClient, &entries, owner, repoName, siteName, "environment.dev.ts", stage, branch)
	if err != nil {
		return err
	}

	branchRef, _, err := githubRestClient.Repositories.GetBranch(ctx, owner, repoName, branch, true)
	if err != nil {
		log.Printf("Failed to get branch '%s': %v", branch, err)
		return fmt.Errorf("failed to get branch '%s': %w", branch, err)
	}

	tree, _, err := githubRestClient.Git.CreateTree(ctx, owner, repoName, *branchRef.Commit.SHA, entries)
	if err != nil {
		log.Printf("Failed to create tree: %v", err)
		return fmt.Errorf("failed to create tree: %w", err)
	}

	newCommit := &github.Commit{
		Parents: []*github.Commit{
			{SHA: branchRef.Commit.SHA},
		},
		Tree: tree,
		Author: &github.CommitAuthor{
			Name:  github.String("Vertigo Bot"),
			Email: github.String("bot@example.com"),
		},
		Message: github.String("Vertigo commit"),
	}

	commit, _, err := githubRestClient.Git.CreateCommit(ctx, owner, repoName, newCommit)
	if err != nil {
		log.Printf("Failed to create commit: %v", err)
		return fmt.Errorf("failed to create commit: %w", err)
	}

	updateRef := &github.Reference{
		Ref: github.String("refs/heads/" + branch),
		Object: &github.GitObject{
			SHA:  commit.SHA,
			Type: github.String("commit"),
		},
	}

	_, _, err = githubRestClient.Git.UpdateRef(ctx, owner, repoName, updateRef, true)
	if err != nil {
		log.Printf("Failed to update ref: %v", err)
		return fmt.Errorf("failed to update ref: %w", err)
	}

	log.Printf("Successfully updated branch '%s'.", branch)
	return nil
}

func makeEnvironmentFileChanges(ctx context.Context, githubRestClient *github.Client, entries *[]*github.TreeEntry, owner string, repoName string, siteName string, fileName string, stage string, branch string) error {
	opts := &github.RepositoryContentGetOptions{
		Ref: branch,
	}

	file, _, _, err := githubRestClient.Repositories.GetContents(ctx, owner, repoName, "projects/spear/src/environments/"+fileName, opts)
	if err != nil {
		log.Printf("Failed to get file '%s': %v", fileName, err)
		return fmt.Errorf("failed to get file '%s': %w", fileName, err)
	}

	fileContent, err := base64.StdEncoding.DecodeString(*file.Content)
	if err != nil {
		log.Printf("Failed to decode file content: %v", err)
		return fmt.Errorf("failed to decode file content: %w", err)
	}

	modifiedContent, err := modifyEnvironmentFileContent(string(fileContent), siteName)
	if err != nil {
		log.Printf("Failed to modify file content: %v", err)
		return fmt.Errorf("failed to modify file content: %w", err)
	}

	blob, _, err := githubRestClient.Git.CreateBlob(ctx, owner, repoName, makeBlob(modifiedContent))
	if err != nil {
		log.Printf("Failed to create blob: %v", err)
		return fmt.Errorf("failed to create blob: %w", err)
	}

	*entries = append(*entries, &github.TreeEntry{
		Path: github.String("projects/spear/src/environments/" + fileName),
		Mode: github.String("100644"),
		Type: github.String("blob"),
		SHA:  blob.SHA,
	})

	return nil
}

func makeBlob(content string) *github.Blob {
	buf := bytes.Buffer{}
	encoder := base64.NewEncoder(base64.StdEncoding, &buf)
	encoder.Write([]byte(content))
	encoder.Close()

	return &github.Blob{
		Content:  github.String(buf.String()),
		Encoding: github.String("base64"),
	}
}

func modifyEnvironmentFileContent(content string, site string) (string, error) {
	return strings.Replace(content, "spearhead", site, -1), nil
}

func main() {
	log.SetFlags(0)
	lambda.Start(handler)
}