package repo

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log"
	"strings"
	"time"
	"fmt"
	"os"
	"io"
	"math/rand"
	"path/filepath"
	"encoding/json"
	"net/http"
	"regexp"
	crand "crypto/rand"

	// "golang.org/x/crypto/nacl/box"
	// Import the kevinburke/nacl/box package, aliasing it to avoid conflict if you also use golang.org/x/crypto/nacl/box
	kb_nacl_box "github.com/kevinburke/nacl/box"

	"goclassifieds/lib/utils"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/go-github/v46/github"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"

	"github.com/aws/aws-sdk-go/aws"
	lambda "github.com/aws/aws-sdk-go/service/lambda"
)

type CommitParams struct {
	Repo     string
	Branch   string
	Path     string
	Data     *[]byte
	UserName string
}

type GithubUserInfo struct {
	Name  string
	Email string
}

type GetInstallationTokenInput struct {
	GithubAppPem []byte
	Owner        string
	GithubAppId  string
}


// Custom error in case the client is nil
type errGithubNoClient struct{}

// Constants for key sizes
const PublicKeySize = 32
const NonceSize = 24 // NaCl requires 24 bytes for the nonce

func (e *errGithubNoClient) Error() string {
	return "GitHub client is not provided"
}

func Commit(c *githubv4.Client, params *CommitParams) {

	log.Printf("BEGIN GithubFileUploadAdaptor::Store %s", params.Path)
	pieces := strings.Split(params.Repo, "/")
	var q struct {
		Repository struct {
			Object struct {
				Commit struct {
					History struct {
						Edges []struct {
							Node struct {
								Oid githubv4.GitObjectID
							}
						}
					} `graphql:"history(first:1)"`
				} `graphql:"... on Commit"`
			} `graphql:"object(expression: $branch)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}
	qVars := map[string]interface{}{
		"branch": githubv4.String(params.Branch),
		"owner":  githubv4.String(pieces[0]),
		"name":   githubv4.String(pieces[1]),
	}
	err := c.Query(context.Background(), &q, qVars)
	if err != nil {
		log.Print("Github latest commit failure.")
		log.Panic(err)
	}
	log.Printf("latest commit %s", q.Repository.Object.Commit.History.Edges[0].Node.Oid)
	var m struct {
		CreateCommitOnBranch struct {
			Commit struct {
				Url githubv4.String
			}
		} `graphql:"createCommitOnBranch(input: $input)"`
	}
	buf := bytes.Buffer{}
	// dataBuffer := bytes.Buffer{}
	// json.NewEncoder(&dataBuffer).Encode(entity)
	encoder := base64.NewEncoder(base64.StdEncoding, &buf)
	// encoder.Write([]byte(dataBuffer.String()))
	encoder.Write(*params.Data)
	encoder.Close()
	additions := make([]githubv4.FileAddition, 1)
	additions[0] = githubv4.FileAddition{
		// Path:     githubv4.String(p + "/" + id + ".json"),
		Path:     githubv4.String(params.Path),
		Contents: githubv4.Base64String(buf.String()),
	}
	input := githubv4.CreateCommitOnBranchInput{
		Branch: githubv4.CommittableBranch{
			RepositoryNameWithOwner: (*githubv4.String)(&params.Repo),
			BranchName:              (*githubv4.String)(&params.Branch),
		},
		Message: githubv4.CommitMessage{
			Headline: githubv4.String("Update File: " + params.Path),
		},
		ExpectedHeadOid: *githubv4.NewGitObjectID(q.Repository.Object.Commit.History.Edges[0].Node.Oid),
		FileChanges: &githubv4.FileChanges{
			Additions: &additions,
		},
	}
	err2 := c.Mutate(context.Background(), &m, input, nil)
	if err2 != nil {
		log.Print("Github file upload failure.")
		log.Panic(err2)
	}
	log.Printf("END GithubFileUploadAdaptor::Store %s", params.Path)

}

func CommitRest(c *github.Client, params *CommitParams) {
	pieces := strings.Split(params.Repo, "/")
	/*userInfo, err := GetUserInfo(c)
	if err != nil {
		log.Print("Github user info failure.")
		log.Panic(err)
	}*/
	log.Print("username: " + params.UserName)
	userInfo := &GithubUserInfo{
		Name:/*"Vertigo"*/ params.UserName,
		Email: "vertigo@rollthecloud.com",
	}
	branch, _, err := c.Repositories.GetBranch(context.Background(), pieces[0], pieces[1], params.Branch, true)
	if err != nil {
		log.Print("Github latest commit failure.")
		log.Panic(err)
	}
	log.Print("Repo " + params.Repo + " branch " + params.Branch + " latest commit SHA " + *branch.Commit.SHA)
	buf := bytes.Buffer{}
	encoder := base64.NewEncoder(base64.StdEncoding, &buf)
	encoder.Write(*params.Data)
	encoder.Close()
	b := &github.Blob{
		Content:  github.String(buf.String()),
		Encoding: github.String("base64"),
	}
	blob, _, err := c.Git.CreateBlob(context.Background(), pieces[0], pieces[1], b)
	if err != nil {
		log.Print("Github create blob failure.")
		log.Panic(err)
	}
	log.Print("Created blob " + *blob.SHA)
	log.Print(*blob.SHA + " path " + params.Path)
	entries := make([]*github.TreeEntry, 1)
	entries[0] = &github.TreeEntry{
		Path: github.String(params.Path),
		Mode: github.String("100644"),
		Type: github.String("blob"),
		SHA:  blob.SHA,
	}
	tree, _, err := c.Git.CreateTree(context.Background(), pieces[0], pieces[1], *branch.Commit.SHA, entries)
	if err != nil {
		log.Print("Github create tree failure.")
		log.Panic(err)
	}
	log.Print("Created tree " + *tree.SHA)
	log.Print("user name: " + userInfo.Name)
	log.Print("user email: " + userInfo.Email)
	parents := make([]*github.Commit, 1)
	parents[0] = &github.Commit{SHA: branch.Commit.SHA}
	newCommit := &github.Commit{
		Parents: parents,
		Tree:    tree,
		Author: &github.CommitAuthor{
			Name:  github.String("Todd Zmijewski"/*userInfo.Name*/),
			Email: github.String("angular.druid@gmail.com"/*userInfo.Email*/),
		},
		Message: github.String("Vertigo commit"),
	}
	commit, _, err := c.Git.CreateCommit(context.Background(), pieces[0], pieces[1], newCommit)
	if err != nil {
		log.Print("Github create commit failure.")
		log.Panic(err)
	}
	log.Print("Created commit " + *commit.SHA)
	updateRef := &github.Reference{
		Ref: github.String("refs/heads/" + params.Branch),
		Object: &github.GitObject{
			SHA:  commit.SHA,
			Type: github.String("commit"),
		},
	}
	ref, _, err := c.Git.UpdateRef(context.Background(), pieces[0], pieces[1], updateRef, true)
	if err != nil {
		log.Print("Github ref update failure.")
		log.Panic(err)
	}
	log.Print("Updated github ref " + *ref.NodeID)
	//log.Panic("made it")
}

// Entry point function that decides where to commit or save the data.
func CommitRestOptimized(c *github.Client, params *CommitParams) {
	if os.Getenv("SAVE_TO_FILE_SYSTEM") == "true" {
		CommitToFileSystem(params)
	} else {
		CommitToGitHub(c, params)
	}
}

// Function for committing directly to GitHub.
func CommitToGitHub(c *github.Client, params *CommitParams) {

	/*userInfo := &GithubUserInfo{
		Name:  params.UserName,
		Email: "vertigo@rollthecloud.com",
	}*/

	pieces := strings.Split(params.Repo, "/") // Split "org/repo" into ["org", "repo"]

	opts := &github.RepositoryContentGetOptions{
		Ref: params.Branch,
	}
	file, _, res, err := c.Repositories.GetContents(context.Background(), pieces[0], pieces[1], params.Path, opts)
	if err != nil && res.StatusCode != 404 {
		log.Print("GitHub get content failure.")
		log.Panic(err)
	}

	if res.StatusCode == 404 {
		// File does not exist, create it.
		createOpts := &github.RepositoryContentFileOptions{
			Branch:  github.String(params.Branch),
			Content: *params.Data,
			Message: github.String("Create file " + params.Path),
			Author: &github.CommitAuthor{
				Name:  github.String("Todd Zmijewski"/*userInfo.Name*/),
				Email: github.String("angular.druid@gmail.com"/*userInfo.Email*/),
			},
		}
		_, _, err := c.Repositories.CreateFile(context.Background(), pieces[0], pieces[1], params.Path, createOpts)
		if err != nil {
			log.Print("GitHub create failure.")
			log.Panic(err)
		}
		log.Print("Created GitHub file")
	} else {
		// File exists, update it.
		updateOpts := &github.RepositoryContentFileOptions{
			Branch:  github.String(params.Branch),
			Content: *params.Data,
			Message: github.String("Update file " + params.Path),
			Author: &github.CommitAuthor{
				Name:  github.String("Todd Zmijewski"/*userInfo.Name*/),
				Email: github.String("angular.druid@gmail.com"/*userInfo.Email*/),
			},
			SHA: file.SHA, // Required for the update request.
		}
		_, _, err := c.Repositories.UpdateFile(context.Background(), pieces[0], pieces[1], params.Path, updateOpts)
		if err != nil {
			log.Print("GitHub update failure.")
			log.Panic(err)
		}
		log.Print("Updated GitHub file")
	}
}

// Function for saving data to the local filesystem.
func CommitToFileSystem(params *CommitParams) {
	// Check if the FILESYSTEM_ROOT environment variable is set.
	root := os.Getenv("FILESYSTEM_ROOT")
	if root == "" {
		log.Panic("FILESYSTEM_ROOT environment variable is not set")
	}

	// Construct the full file path: root/repo/user/path
	filePath := filepath.Join(root, params.Repo, params.UserName, params.Path)

	// Ensure the directory structure exists
	dir := filepath.Dir(filePath)
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		log.Panic("Failed to create directories: ", err)
	}

	// Write the data to the file
	err = os.WriteFile(filePath, *params.Data, 0644)
	if err != nil {
		log.Panic("Failed to write data to file: ", err)
	}

	log.Print("Data saved to filesystem at: " + filePath)
}

func GetUserInfo(client *github.Client) (*GithubUserInfo, error) {
	user, _, err := client.Users.Get(context.Background(), "")
	if err != nil {
		log.Print(err.Error())
		return nil, err
	}
	primaryEmail := user.Email
	log.Print(user.String())
	listOptions := &github.ListOptions{Page: 1, PerPage: 10}
	emails, _, err := client.Users.ListEmails(context.Background(), listOptions)
	if err != nil {
		log.Print(err.Error())
		// @todo: Default for now
		primaryEmail = github.String("vertigo@rollthecloud.com")
		// return nil, err
	}
	if err == nil {
		for _, email := range emails {
			log.Print(email.GetEmail())
			if email.GetPrimary() == true && email.GetVerified() == true {
				log.Print("identified verified primary email: " + email.GetEmail())
				primaryEmail = email.Email
			}
		}
	}
	userInfo := &GithubUserInfo{
		Name:  *user.Login,
		Email: *primaryEmail,
	}
	return userInfo, nil
}

func GetInstallationToken(input *GetInstallationTokenInput) (*github.InstallationToken, error) {

	pk, err := jwt.ParseRSAPrivateKeyFromPEM(input.GithubAppPem)
	if err != nil {
		log.Print("Error parsing github app pem")
	}
	log.Print("Parsed github app pem")
	token := jwt.New(jwt.SigningMethodRS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["iat"] = time.Now().Add(-60 * time.Second).Unix()
	claims["exp"] = time.Now().Add(10 * time.Minute).Unix()
	claims["iss"] = input.GithubAppId
	tokenString, err := token.SignedString(pk)
	if err != nil {
		log.Print("Error signing token", err.Error())
		return &github.InstallationToken{}, err
	}
	log.Print("Token string " + tokenString)
	listOpts := &github.ListOptions{
		Page:    1,
		PerPage: 100,
	}
	srcToken := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: tokenString},
	)
	httpClient := oauth2.NewClient(context.Background(), srcToken)
	githubRestClient := github.NewClient(httpClient)
	installations, _, err := githubRestClient.Apps.ListInstallations(context.Background(), listOpts)
	if err != nil {
		log.Print("Error listing installations", err.Error())
		return &github.InstallationToken{}, err
	}
	var targetInstallation *github.Installation
	if err == nil {
		log.Printf("Has instllations %d", len(installations))
		for _, installation := range installations {
			log.Print("installation account login ", installation.Account.Login)
			if *installation.Account.Login == input.Owner {
				targetInstallation = installation
			}
		}
	}

	if targetInstallation != nil {
		log.Printf("matched installation %d", targetInstallation.ID)
		tokenOpts := &github.InstallationTokenOptions{}
		installationToken, _, err := githubRestClient.Apps.CreateInstallationToken(context.Background(), *targetInstallation.ID, tokenOpts)
		if err != nil {
			log.Print("Error generating instllation token", err.Error())
			return &github.InstallationToken{}, err
		}
		return installationToken, nil
	}

	return &github.InstallationToken{}, errors.New("No target installation matches owner " + input.Owner)

}

func AppendToFile(ctx context.Context, client *github.Client, owner, repo, filePath, guid string, branch string) error {
	opts := &github.RepositoryContentGetOptions{
		Ref: branch, // Specify the branch name here
	}

	// Step 1: Fetch the existing file contents.
	fileContent, _, res, err := client.Repositories.GetContents(ctx, owner, repo, filePath, opts)
	if err != nil {
		if res != nil && res.StatusCode == 404 {
			// If the file does not exist, we'll create it later.
			return fmt.Errorf("file not found in the repository: %w", err)
		}
		return fmt.Errorf("failed to get file contents: %w", err)
	}

	var existingContent string
	var sha string
	if fileContent != nil {
		decodedContent, err := fileContent.GetContent()
		if err != nil {
			return fmt.Errorf("failed to decode existing content: %w", err)
		}
		existingContent = decodedContent
		sha = fileContent.GetSHA()
	}

	// Step 2: Process the content as fixed-length GUID chunks.
	const guidLength = 32 // Hex-encoded 16 bytes = 32 characters.
	// Split content into fixed-length chunks, ignoring formatting.
	var allGUIDs []string
	for i := 0; i+guidLength <= len(existingContent); i += guidLength {
		allGUIDs = append(allGUIDs, existingContent[i:i+guidLength])
	}

	// Prepare the new GUID as a hex-encoded string.
	binaryGUID, err := utils.EncodeStringToFixedBytes(guid, 16)
	if err != nil {
		return fmt.Errorf("failed to encode GUID: %w", err)
	}
	newHexGUID := hex.EncodeToString(binaryGUID)

	// Append the new GUID to the list.
	allGUIDs = append(allGUIDs, newHexGUID)

	// Group GUIDs into lines of three, each line without spaces.
	var lines []string
	for i := 0; i < len(allGUIDs); i += 3 {
		endIndex := i + 3
		if endIndex > len(allGUIDs) {
			endIndex = len(allGUIDs)
		}
		lines = append(lines, strings.Join(allGUIDs[i:endIndex], ""))
	}

	// Join lines with newlines to form the final content.
	newContent := strings.Join(lines, "\n")

	// Step 3: Update the file in the repository.
	options := &github.RepositoryContentFileOptions{
		Message: github.String("Appending content via go-github"),
		Content: []byte(newContent),
		SHA:     github.String(sha), // Use the SHA for updates (omit for new files).
		Branch:  github.String(branch),
	}

	_, _, err = client.Repositories.UpdateFile(ctx, owner, repo, filePath, options)
	if err != nil {
		return fmt.Errorf("failed to update file in repository: %w", err)
	}

	fmt.Println("File successfully updated!")
	return nil
}

func CreateFileIfNotExists(ctx context.Context, client *github.Client, owner, repo, path, content string, branch string) error {
	
	//
	opts := &github.RepositoryContentGetOptions{
		Ref: branch, // Specify the branch name here
	}
	
	// Check if the file exists
	_, _, res, err := client.Repositories.GetContents(ctx, owner, repo, path, opts)
	if err != nil {
		if res != nil && res.StatusCode == 404 {
			// File doesn't exist, create it
			options := &github.RepositoryContentFileOptions{
				Message: github.String(fmt.Sprintf("Create %s via API", path)),
				Content: []byte(content),
				Branch:  github.String(branch),
			}
			_, _, err := client.Repositories.CreateFile(ctx, owner, repo, path, options)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", path, err)
			}
			fmt.Printf("File %s created successfully\n", path)
			return nil
		}
		return fmt.Errorf("failed to check existence of file %s: %w", path, err)
	}

	fmt.Printf("File %s already exists\n", path)
	return nil
}

func EnsureCatalog(ctx context.Context, client *github.Client, owner, repo, directoryPath string, branch string, clusteringEnabled bool, pageMax int) (string, error) {
	
	var chapter int
	var page int

	if clusteringEnabled {
		log.Print("Catalog ensuring with clustering.")
	} else {
		log.Print("Catalog ensuring without clustering chapter should default to 0.")
	}

	basePath := fmt.Sprintf("catalog/%s", directoryPath)

	// Seed the random number generator to get different results each time

	// Generate a random number between 0 and 10 (inclusive)
	// When clustering is disabled the chapter will always be 0 which is the master repo.
	if clusteringEnabled {
		rand.Seed(time.Now().UnixNano())
		chapter = rand.Intn(11) // 11 because Intn(n) generates [0, n)
	} else {
		chapter = 0
	}

	// Seed the random number generator to get different results each time

	// Generate a random number between 0 and 10 (inclusive)
	if pageMax == 0 {
		log.Print("One page in catalog per entity.")
		page = 0
	} else {
		log.Print("Multiple pages in catalog per entity.")
		rand.Seed(time.Now().UnixNano())
		page = rand.Intn(pageMax) // 11 because Intn(n) generates [0, n)
	}

	// Step 1: Ensure the first .gitkeep file exists in /catalog/{directoryPath}/.gitkeep
	gitkeepPath1 := fmt.Sprintf("%s/.gitkeep", basePath)
	err := CreateFileIfNotExists(ctx, client, owner, repo, gitkeepPath1, "", branch)
	if err != nil {
		log.Printf("one %s", err.Error())
		return "", fmt.Errorf("error ensuring %s: %w", gitkeepPath1, err)
	}

	// Step 2: Ensure the second .gitkeep file exists in /catalog/{directoryPath}/0/.gitkeep
	gitkeepPath2 := fmt.Sprintf("%s/%d/.gitkeep", basePath, chapter)
	err = CreateFileIfNotExists(ctx, client, owner, repo, gitkeepPath2, "", branch)
	if err != nil {
		log.Print("two %s", err.Error())
		return "", fmt.Errorf("error ensuring %s: %w", gitkeepPath2, err)
	}

	// Step 3: Ensure 0.txt file exists in /catalog/{directoryPath}/0/0.txt
	zeroFilePath := fmt.Sprintf("%s/%d/%d.txt", basePath, chapter, page)
	err = CreateFileIfNotExists(ctx, client, owner, repo, zeroFilePath, "", branch)
	if err != nil {
		log.Print("three")
		return "", fmt.Errorf("error ensuring %s: %w", zeroFilePath, err)
	}

	file := fmt.Sprintf("catalog/" + directoryPath + "/%d/%d.txt", chapter, page)
	return file, nil
}

// createRepo creates a GitHub repository for a specified owner/user or organization.
// Parameters:
// - client: A preconfigured GitHub client to make API calls
// - owner: The owner (GitHub username or organization) where the repository will be created
// - repoName: The name of the repository to be created
// - description: A description for the repository
// - private: Whether the repository should be private (true) or public (false)
func createRepo(client *github.Client, owner string, repoName string, description string, private bool) error {
	// Ensure client is not nil
	if client == nil {
		return &errGithubNoClient{}
	}

	// Define repository configuration
	repo := &github.Repository{
		Name:        github.String(repoName),
		Description: github.String(description),
		Private:     github.Bool(private),
	}

	// Context for the API call
	ctx := context.Background()

	// Make API call to create the repository
	// If the owner is an organization, specify it; otherwise, use an empty string for an authenticated user.
	newRepo, _, err := client.Repositories.Create(ctx, owner, repo)
	if err != nil {
		return err
	}

	// Log the repository's URL
	log.Printf("Repository created: %s\n", newRepo.GetHTMLURL())

	// Create a README file in the repository
	readmeContent := "# " + repoName + "\n\n" + description
	readmeOptions := &github.RepositoryContentFileOptions{
		Message: github.String("Add initial README file"),
		Content: []byte(readmeContent),
	}
	_, _, err = client.Repositories.CreateFile(ctx, owner, repoName, "README.md", readmeOptions)
	if err != nil {
		return fmt.Errorf("failed to create README file: %w", err)
	}

	log.Println("README.md file successfully created.")

	// Create the "dev" branch
	if err := CreateDevBranch(client, owner, repoName); err != nil {
		return fmt.Errorf("failed to create dev branch: %w", err)
	}

	log.Println("Dev branch successfully created.")

	// Log or return the repository's URL
	log.Printf("Repository created: %s\n", newRepo.GetHTMLURL())
	return nil
}

// repoExists checks if a repository with the given name already exists for the specified owner.
// Parameters:
// - client: A preconfigured GitHub client
// - owner: The owner (GitHub username or organization)
// - repoName: The name of the repository to check
// Returns:
// - true if the repository exists, false otherwise
// - error if the API call fails
func repoExists(client *github.Client, owner, repoName string) (bool, error) {
	// Ensure client is not nil
	if client == nil {
		return false, &errGithubNoClient{}
	}

	// Context for the API call
	ctx := context.Background()

	// Attempt to fetch the repository
	_, _, err := client.Repositories.Get(ctx, owner, repoName)
	if err != nil {
		// Check if the error is related to the repository not being found
		if _, isNotFound := err.(*github.ErrorResponse); isNotFound && err.(*github.ErrorResponse).Response.StatusCode == 404 {
			return false, nil // Repository does not exist
		}
		return false, err
	}

	// Repository exists
	return true, nil
}

// EnsureRepoCreate ensures the repository is created only if it doesn't already exist.
// Parameters:
// - client: A preconfigured GitHub client to make API calls
// - owner: The owner (GitHub username or organization)
// - repoName: The name of the repository to create
// - description: A description for the repository
// - private: Whether the repository should be private (true) or public (false)
// Returns:
// - nil if the repository is successfully created or already exists (no action needed)
// - error if any failure occurs
func EnsureRepoCreate(client *github.Client, owner, repoName, description string, private bool) error {
	// Check if the repository exists
	exists, err := repoExists(client, owner, repoName)
	if err != nil {
		return fmt.Errorf("failed to check if repository exists: %w", err)
	}

	if exists {
		log.Printf("Repository '%s/%s' already exists. No action needed.\n", owner, repoName)
		return nil // Repository already exists, no further action
	}

	// Create repository if it does not exist
	log.Printf("Creating repository '%s/%s'...\n", owner, repoName)
	err = createRepo(client, owner, repoName, description, private)
	if err != nil {
		return fmt.Errorf("failed to create repository: %w", err)
	}

	log.Printf("Repository '%s/%s' successfully created.\n", owner, repoName)
	return nil
}

// CreateDevBranch creates a "dev" branch in the repository based on the default branch.
func CreateDevBranch(client *github.Client, owner string, repoName string) error {
	// Context for the API call
	ctx := context.Background()

	// Get the default branch name (e.g., "main" or "master")
	repo, _, err := client.Repositories.Get(ctx, owner, repoName)
	if err != nil {
		return fmt.Errorf("failed to fetch repository details: %w", err)
	}
	defaultBranch := repo.GetDefaultBranch() // Example: "main" or "master"

	// Retry fetching the default branch reference with a timeout and an increasing delay
	var ref *github.Reference
	const maxRetries = 5       // Total number of retries
	const retryDelay = 2 * time.Second // Delay between retries

	for i := 1; i <= maxRetries; i++ {
		ref, _, err = client.Git.GetRef(ctx, owner, repoName, "refs/heads/"+defaultBranch)
		if err != nil {
			if i == maxRetries {
				// If the max retries are reached, return the error
				return fmt.Errorf("failed to fetch default branch reference after %d retries: %w", maxRetries, err)
			}
			// Log the retry attempt and wait before trying again
			log.Printf("Retry %d/%d: Waiting for default branch to become available...\n", i, maxRetries)
			time.Sleep(retryDelay)
			continue
		}
		// Successfully fetched the default branch reference
		break
	}

	// Create the "dev" branch pointing to the same commit as the default branch
	newBranchRef := &github.Reference{
		Ref: github.String("refs/heads/dev"),               // New branch name
		Object: &github.GitObject{SHA: ref.GetObject().SHA}, // Pointing to the same commit as the default branch
	}
	_, _, err = client.Git.CreateRef(ctx, owner, repoName, newBranchRef)
	if err != nil {
		return fmt.Errorf("failed to create 'dev' branch: %w", err)
	}

	log.Println("Dev branch successfully created.")
	return nil
}

func FindChapterByGUID(
	ctx context.Context, 
	client *github.Client, 
	owner, repo, directoryPath, guid, branch string,
) (string, error) {
    basePath := fmt.Sprintf("catalog/%s", directoryPath)

    // Step 1: Encode the provided GUID into a 16-byte fixed format for comparison
    encodedGuid, err := utils.EncodeStringToFixedBytes(guid, 16)
    if err != nil {
        return "", fmt.Errorf("error encoding GUID %s: %w", guid, err)
    }
    hexEncodedGuid := hex.EncodeToString(encodedGuid)
    log.Printf("Encoded GUID as hex: %s", hexEncodedGuid)

    // Step 2: Fetch the list of chapters from the base path
    _, chapters, res, err := client.Repositories.GetContents(ctx, owner, repo, basePath, &github.RepositoryContentGetOptions{Ref: branch})
    if err != nil {
        if res != nil && res.StatusCode == 404 {
            return "", fmt.Errorf("base path %s not found: %w", basePath, err)
        }
        return "", fmt.Errorf("failed to fetch chapters: %w", err)
    }

    // Step 3: Iterate over each chapter (directories within the base path)
    for _, chapter := range chapters {
        if chapter.GetType() != "dir" {
            continue // Skip non-directory entries
        }

        chapterPath := chapter.GetPath()
        log.Printf("Processing chapter: %s", chapterPath)

        // Step 4: Fetch the pages (files) within the chapter
        _, pages, _, err := client.Repositories.GetContents(ctx, owner, repo, chapterPath, &github.RepositoryContentGetOptions{Ref: branch})
        if err != nil {
            return "", fmt.Errorf("failed to fetch pages for chapter %s: %w", chapterPath, err)
        }

        // Step 5: Check each page (file) for the presence of the GUID
        for _, page := range pages {
            if page.GetType() == "dir" {
                continue // Skip subdirectories
            }

            pagePath := page.GetPath()
            log.Printf("Searching GUID in page: %s", pagePath)

            // Fetch the page content only when necessary
            pageFileContent, _, _, err := client.Repositories.GetContents(ctx, owner, repo, pagePath, &github.RepositoryContentGetOptions{Ref: branch})
            if err != nil {
                return "", fmt.Errorf("failed to fetch content for page %s: %w", pagePath, err)
            }

            rawContent, err := pageFileContent.GetContent() // Decode base64 content
            if err != nil {
                return "", fmt.Errorf("failed to decode content for page %s: %w", pagePath, err)
            }

            // Step 6: Manually process each line in the file content
            matchFound := false
            start := 0
            for i := 0; i < len(rawContent); i++ {
                if rawContent[i] == '\n' || i == len(rawContent)-1 {
                    // Extract the line (account for EOF case where '\n' might not exist at the end)
                    end := i
                    if i == len(rawContent)-1 && rawContent[i] != '\n' {
                        end = i + 1
                    }

                    line := strings.TrimSpace(rawContent[start:end])
                    start = i + 1 // Move to the next line start

                    if line == "" {
                        continue // Skip empty lines
                    }

                    // Step 6.1: Split the line into individual GUIDs (fixed-length chunks)
                    const guidLength = 32 // Hex-encoded GUID length
                    for j := 0; j+guidLength <= len(line); j += guidLength {
                        chunk := line[j : j+guidLength] // Extract each GUID chunk
                        log.Printf("Comparing GUID chunk %s with target GUID %s", chunk, hexEncodedGuid)
                        if chunk == hexEncodedGuid {
                            // Match found
                            log.Printf("Match found! GUID %s matches in chapter: %s, page: %s", guid, chapter.GetName(), page.GetName())
                            return chapter.GetName(), nil
                        }
                    }
                }
            }

            if matchFound {
                break
            }
        }
    }

    // If no matches are found across all files, return default "0"
    log.Printf("No match found for GUID: %s", guid)
    return "0", nil
}

// CreateFromTemplate creates a new repository from a template using the REST API.
func CreateFromTemplate(ctx context.Context, httpClient *http.Client, templateOwner, templateRepo, newRepoOwner, newRepoName, description string, private bool) error {
	// Construct the API URL for the template repository generation
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/generate", templateOwner, templateRepo)

	// Build the request payload
	payload := map[string]interface{}{
		"owner":       newRepoOwner, // Specify the repository owner dynamically
		"name":        newRepoName,
		"description": description,
		"private":     private,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create a POST request to the GitHub API
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add necessary headers to the request
	req.Header.Set("Authorization", "Bearer "+GetAccessTokenFromHttpClient(httpClient))
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	// Execute the request to create the repository from the template
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute API request: %w", err)
	}
	defer resp.Body.Close()

	// Handle non-success responses
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create repository from template: received status code %d", resp.StatusCode)
	}

	log.Printf("Repository '%s/%s' created successfully from template.", newRepoOwner, newRepoName)

	// Commit an initial file to ensure the repository is not empty
	if err := CommitInitialFile(httpClient, newRepoOwner, newRepoName, description); err != nil {
		return fmt.Errorf("failed to commit initial file: %w", err)
	}

	log.Printf("Initial file committed successfully to repository '%s/%s'.", newRepoOwner, newRepoName)

	return nil
}

// CommitInitialFile commits a simple README.md file to the repository to ensure the default branch is created.
func CommitInitialFile(httpClient *http.Client, repoOwner, repoName, description string) error {
	// Construct the API URL for committing the file
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/README.md", repoOwner, repoName)

	// Create the initial README content
	readmeContent := fmt.Sprintf("# %s\n\n%s", repoName, description)

	// Prepare the payload for committing the file
	payload := map[string]interface{}{
		"message": "Add initial README file",
		"content": base64.StdEncoding.EncodeToString([]byte(readmeContent)), // Base64-encoded content
		"branch":  "main", // Explicitly specify the default branch (commonly "main")
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create a PUT request to commit the file
	req, err := http.NewRequest("PUT", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add necessary headers
	req.Header.Set("Authorization", "Bearer "+GetAccessTokenFromHttpClient(httpClient))
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to commit initial file: %w", err)
	}
	defer resp.Body.Close()

	// Handle non-success responses
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to commit initial file: received status code %d", resp.StatusCode)
	}

	log.Println("Successfully committed initial README.md file.")
	return nil
}

// Helper to retrieve the access token associated with the HTTP client.
func GetAccessTokenFromHttpClient(client *http.Client) string {
	// Example: Use the transport or token source from the OAuth client to extract the token.
	// You'll modify this based on how tokens are passed within the existing implementation.
	tokenSource := client.Transport.(*oauth2.Transport).Source
	token, _ := tokenSource.Token()
	return token.AccessToken
}

// CreateEnvironment creates a GitHub environment using the HTTP client from go-github.
func CreateEnvironment(ctx context.Context, httpClient *http.Client, owner, repo, environmentName string) error {
	// Define the GitHub API URL for creating an environment
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/environments/%s", owner, repo, environmentName)

	// Define the payload (customize as needed)
	payload := map[string]interface{}{
		"reviewers": []interface{}{}, // You can define specific reviewers for deployment approvals here
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create the PUT request
	req, err := http.NewRequestWithContext(ctx, "PUT", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+GetAccessTokenFromHttpClient(httpClient))
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	// Execute the API call
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Check the response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create environment '%s': received HTTP status %d", environmentName, resp.StatusCode)
	}

	fmt.Printf("Environment '%s' created successfully!\n", environmentName)
	return nil
}

func CreateGithubRepositorySecret(ctx context.Context, httpClient *http.Client, lambdaClient *lambda.Lambda, owner, repo, secretName, secretValue string, stage string) error {
	// Step 1: Fetch the public key for the repository
	keyURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/secrets/public-key", owner, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", keyURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request to fetch public key: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+GetAccessTokenFromHttpClient(httpClient))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch public key: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to fetch public key: HTTP %d, response: %s", resp.StatusCode, string(body))
	}

	var keyResponse struct {
		Key   string `json:"key"`
		KeyID string `json:"key_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&keyResponse); err != nil {
		return fmt.Errorf("failed to decode public key response: %w", err)
	}
	if keyResponse.Key == "" || keyResponse.KeyID == "" {
		return fmt.Errorf("public key or key ID missing in GitHub response")
	}

	// Step 2: Encrypt the secret with Lambda encryption
	encryptedValue, err := EncryptSecretValueWithLambda(secretValue, keyResponse.Key, lambdaClient, stage)
	if err != nil {
		return fmt.Errorf("failed to encrypt secret value: %w", err)
	}

	// Step 3: Validate Base64-encoded value
	if !isValidBase64(encryptedValue) {
		return fmt.Errorf("invalid Base64-encoded encrypted value: %s", encryptedValue)
	}

	// Step 4: Send the encrypted secret to GitHub
	secretURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/secrets/%s", owner, repo, secretName)
	payload := map[string]string{
		"encrypted_value": encryptedValue,
		"key_id":          keyResponse.KeyID,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err = http.NewRequestWithContext(ctx, "PUT", secretURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request for secret creation: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+GetAccessTokenFromHttpClient(httpClient))
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err = httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create repository secret: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create repository secret: received HTTP status %d, response body: %s", resp.StatusCode, string(body))
	}

	log.Printf("Successfully created secret '%s' in repository '%s/%s'.", secretName, owner, repo)
	return nil
}

// EncryptSecretValue encrypts a secret value using github.com/kevinburke/nacl/box.Seal,
// manually prepending the ephemeral public key to mimic libsodium's crypto_box_seal.
func EncryptSecretValue(publicKey, secretValue string) (string, error) {
	log.Printf("DEBUG: Received public key string from GitHub: %s (length %d)", publicKey, len(publicKey))
	log.Printf("DEBUG: Secret value string: \"%s\" (length %d)", secretValue, len(secretValue)) // ADDED DEBUG LOG

	// Step 1: Decode the Base64-encoded public key (GitHub's public key) into raw bytes.
	publicKeyBytes, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil {
		return "", fmt.Errorf("failed to decode GitHub public key: %w", err)
	}

	log.Printf("DEBUG: Decoded public key bytes length: %d", len(publicKeyBytes))
	log.Printf("DEBUG: Decoded public key hex: %s", hex.EncodeToString(publicKeyBytes))

	var recipientPublicKey [32]byte
	if len(publicKeyBytes) != 32 {
		return "", fmt.Errorf("invalid length for GitHub public key: expected 32 bytes, got %d", len(publicKeyBytes))
	}
	copy(recipientPublicKey[:], publicKeyBytes)
	log.Printf("DEBUG: Copied recipientPublicKeyArray hex: %s", hex.EncodeToString(recipientPublicKey[:]))


	// Step 2: Generate an ephemeral key pair for the sender (our side).
	ephemeralPublicKey, ephemeralPrivateKey, err := kb_nacl_box.GenerateKey(crand.Reader)
	if err != nil {
		return "", fmt.Errorf("failed to generate ephemeral keypair for encryption: %w", err)
	}
	log.Printf("DEBUG: Generated ephemeral public key hex: %s", hex.EncodeToString(ephemeralPublicKey[:]))
	log.Printf("DEBUG: Generated ephemeral private key hex: %s", hex.EncodeToString(ephemeralPrivateKey[:]))


	// Step 3: Create an all-zero nonce for the box.Seal operation.
	var nonce [24]byte // Initializes to all zeros
	log.Printf("DEBUG: Nonce (all zeros) hex: %s", hex.EncodeToString(nonce[:]))


	// Step 4: Encrypt the plain-text value.
	// `nil` for 'out' argument, meaning Seal will allocate a new slice.
	sealedMessage := kb_nacl_box.Seal(nil, []byte(secretValue), &nonce, &recipientPublicKey, ephemeralPrivateKey)
	log.Printf("DEBUG: Sealed message (ciphertext) hex: %s", hex.EncodeToString(sealedMessage))
	log.Printf("DEBUG: Sealed message length: %d", len(sealedMessage))


	// Step 5: Prepend the ephemeral public key to the sealed message.
	encryptedValueWithEphemeralKey := make([]byte, len(ephemeralPublicKey)+len(sealedMessage))
	copy(encryptedValueWithEphemeralKey, ephemeralPublicKey[:])
	copy(encryptedValueWithEphemeralKey[len(ephemeralPublicKey):], sealedMessage)
	log.Printf("DEBUG: Final combined encrypted value hex: %s", hex.EncodeToString(encryptedValueWithEphemeralKey))
	log.Printf("DEBUG: Final combined encrypted value length: %d", len(encryptedValueWithEphemeralKey))


	// Step 6: Base64-encode the entire combined byte slice.
	base64Encoded := base64.StdEncoding.EncodeToString(encryptedValueWithEphemeralKey)
	log.Printf("DEBUG: Final Base64 encoded string: %s", base64Encoded)

	return base64Encoded, nil
}

func CreateGithubEnvironmentSecret(ctx context.Context, httpClient *http.Client, lambdaClient *lambda.Lambda, owner, repo, environmentName, secretName, secretValue string, stage string) error {
	// Step 1: Fetch the public key for the environment
	keyURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/environments/%s/secrets/public-key", owner, repo, environmentName)
	req, err := http.NewRequestWithContext(ctx, "GET", keyURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request to fetch public key: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+GetAccessTokenFromHttpClient(httpClient))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch public key: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to fetch public key: HTTP %d, response: %s", resp.StatusCode, string(body))
	}

	var keyResponse struct {
		Key   string `json:"key"`
		KeyID string `json:"key_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&keyResponse); err != nil {
		return fmt.Errorf("failed to decode public key response: %w", err)
	}
	if keyResponse.Key == "" || keyResponse.KeyID == "" {
		return fmt.Errorf("public key or key ID missing in GitHub response")
	}

	// Step 2: Encrypt the secret with Libsodium-compatible encryption
	encryptedValue, err := EncryptSecretValueWithLambda(secretValue, keyResponse.Key, lambdaClient, stage)
	if err != nil {
		return fmt.Errorf("failed to encrypt secret value: %w", err)
	}

	// Step 3: Validate Base64-encoded value
	if !isValidBase64(encryptedValue) {
		return fmt.Errorf("invalid Base64-encoded encrypted value: %s", encryptedValue)
	}

	// Step 4: Send the encrypted secret to GitHub
	secretURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/environments/%s/secrets/%s", owner, repo, environmentName, secretName)
	payload := map[string]string{
		"encrypted_value": encryptedValue,
		"key_id":          keyResponse.KeyID,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err = http.NewRequestWithContext(ctx, "PUT", secretURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request for secret creation: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+GetAccessTokenFromHttpClient(httpClient))
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err = httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create environment secret: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create environment secret: received HTTP status %d, response body: %s", resp.StatusCode, string(body))
	}

	log.Printf("Successfully created secret '%s' in environment '%s' for repository '%s/%s'.", secretName, environmentName, owner, repo)
	return nil
}

func EncryptSecretValueWithLambda(secretValue, publicKey string, lambdaClient *lambda.Lambda, stage string) (string, error) {
	// Prepare the payload for the Lambda function
	payload := map[string]string{
		"publicKey":  publicKey,
		"secretValue": secretValue,
	}

	// Marshal payload into JSON format
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload to JSON: %w", err)
	}

	// Log the payload for debugging
	log.Printf("Payload sent to Lambda: %s", string(payloadJSON))

	// Invoke the Lambda function
	result, err := lambdaClient.Invoke(&lambda.InvokeInput{
		FunctionName:   aws.String("goclassifieds-api-" + stage + "-EncryptSecretValue"),
		Payload:        payloadJSON,
		InvocationType: aws.String("RequestResponse"),
	})
	if err != nil {
		return "", fmt.Errorf("failed to invoke Lambda function: %w", err)
	}

	// Log the raw response for debugging
	log.Printf("Raw Lambda response payload: %s", string(result.Payload))

	// Check for Lambda function errors
	if result.FunctionError != nil {
		return "", fmt.Errorf("Lambda function error: %s", string(result.Payload))
	}

	// Parse the top-level Lambda response payload
	var lambdaResponse struct {
		StatusCode int    `json:"statusCode"`
		Body       string `json:"body"` // The Body is a string containing a JSON object
	}
	if err := json.Unmarshal(result.Payload, &lambdaResponse); err != nil {
		return "", fmt.Errorf("failed to parse Lambda response payload: %w", err)
	}

	// Check for invalid statusCode
	if lambdaResponse.StatusCode != 200 {
		return "", fmt.Errorf("unexpected Lambda statusCode: %d", lambdaResponse.StatusCode)
	}

	// Parse the nested "body" field to extract the encryptedValue
	var bodyResponse struct {
		EncryptedValue string `json:"encryptedValue"`
	}
	if err := json.Unmarshal([]byte(lambdaResponse.Body), &bodyResponse); err != nil {
		return "", fmt.Errorf("failed to parse nested body: %w", err)
	}

	// Ensure the encryptedValue is not empty
	if bodyResponse.EncryptedValue == "" {
		return "", fmt.Errorf("Lambda response contained an empty encrypted value")
	}

	// Log the extracted encrypted value
	log.Printf("Encrypted Value (extracted): %s", bodyResponse.EncryptedValue)

	// Validate the Base64-encoded string
	if !isValidBase64(bodyResponse.EncryptedValue) {
		return "", fmt.Errorf("invalid Base64-encoded encrypted value: %s", bodyResponse.EncryptedValue)
	}

	// Return the valid encrypted value
	return bodyResponse.EncryptedValue, nil
}

// Helper function to validate Base64 strings
func isValidBase64(s string) bool {
	matched, _ := regexp.MatchString(`^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=|[A-Za-z0-9+/]{4})$`, s)
	return matched
}