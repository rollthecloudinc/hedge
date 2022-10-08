package repo

import (
	"bytes"
	"context"
	"encoding/base64"
	"log"
	"strings"

	"github.com/google/go-github/v46/github"
	"github.com/shurcooL/githubv4"
)

type CommitParams struct {
	Repo   string
	Branch string
	Path   string
	Data   *[]byte
}

type GithubUserInfo struct {
	Name  string
	Email string
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
			Headline: "add file",
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
	userInfo := &GithubUserInfo{
		Name:  "Vertigo",
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
