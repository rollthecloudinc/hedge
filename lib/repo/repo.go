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

	"goclassifieds/lib/utils"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/go-github/v46/github"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
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
			Name:  github.String(userInfo.Name),
			Email: github.String(userInfo.Email),
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

func CommitRestOptimized(c *github.Client, params *CommitParams) {
	userInfo := &GithubUserInfo{
		Name:/*"Vertigo"*/ params.UserName,
		Email: "vertigo@rollthecloud.com",
	}
	pieces := strings.Split(params.Repo, "/")
	opts := &github.RepositoryContentGetOptions{
		Ref: params.Branch,
	}
	file, _, res, err := c.Repositories.GetContents(context.Background(), pieces[0], pieces[1], params.Path, opts)
	if err != nil && res.StatusCode != 404 {
		log.Print("Github get content failure.")
		log.Panic(err)
	}
	if res.StatusCode == 404 {
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
			log.Print("Github create failure.")
			log.Panic(err)
		}
		log.Print("Created github file")
	} else {
		updateOpts := &github.RepositoryContentFileOptions{
			Branch:  github.String(params.Branch),
			Content: *params.Data,
			Message: github.String("Update file " + params.Path),
			Author: &github.CommitAuthor{
				Name:  github.String(userInfo.Name),
				Email: github.String(userInfo.Email),
			},
			SHA: file.SHA,
		}
		_, _, err := c.Repositories.UpdateFile(context.Background(), pieces[0], pieces[1], params.Path, updateOpts)
		if err != nil {
			log.Print("Github update failure.")
			log.Panic(err)
		}
		log.Print("Updated github file")
	}

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


func AppendToFile(ctx context.Context, client *github.Client, owner, repo, filePath, guid string) error {
	// Step 1: Fetch the existing file contents.
	fileContent, _, res, err := client.Repositories.GetContents(ctx, owner, repo, filePath, nil)
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

	// Step 2: Convert GUID string to binary and append it to the current content.
	binaryGUID, err := utils.EncodeStringToFixedBytes(guid, 16)
	if err != nil {
		return fmt.Errorf("failed to encode GUID: %w", err)
	}

	// Convert binary into a readable hex string and append it.
	newContent := existingContent + "\n" + hex.EncodeToString(binaryGUID)

	// Step 3: Update or create the file in the repository.
	options := &github.RepositoryContentFileOptions{
		Message: github.String("Appending content via go-github"),
		Content: []byte(newContent),
		SHA:     github.String(sha), // Use the SHA for updates (omit it for new files).
	}

	_, _, err = client.Repositories.UpdateFile(ctx, owner, repo, filePath, options)
	if err != nil {
		return fmt.Errorf("failed to update file in repository: %w", err)
	}

	fmt.Println("File successfully updated!")
	return nil
}
