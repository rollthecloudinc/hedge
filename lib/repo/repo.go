package repo

import (
	"bytes"
	"context"
	"encoding/base64"
	"log"
	"strings"

	"github.com/shurcooL/githubv4"
)

type CommitParams struct {
	Repo   string
	Branch string
	Path   string
	Data   *[]byte
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
