package search

import (
	"context"
	"fmt"
	"log"
	"strings"
	"errors"
	"encoding/base64"
	"encoding/json"
	
	"github.com/google/go-github/v46/github" 
	// Assume other structs like GetIndexConfigurationInput, Query, etc. are defined here or imported.
)

// ====================================================================
// === CORE INTERFACES & ENGINE STRUCT (For Dynamic Data Sources) =====
// ====================================================================

// DocumentIterator defines the contract for streaming documents from any source.
type DocumentIterator interface {
	Next() (map[string]interface{}, bool)
	Error() error
	Close()
}

// DocumentLoader defines the contract for initiating the loading process.
type DocumentLoader interface {
	// Load starts the process of fetching documents based on the index configuration.
	// We still pass the client as the loader needs it for the GitHub API calls.
	Load(ctx context.Context, config *GetIndexConfigurationInput, queryComposite map[string]interface{}) (DocumentIterator, error)
}

// SearchEngine holds the single, swappable loader dependency.
type SearchEngine struct {
	Loader DocumentLoader 
}

// NewSearchEngine requires the concrete loader instance to be injected.
func NewSearchEngine(loader DocumentLoader) *SearchEngine {
	return &SearchEngine{
		Loader: loader,
	}
}

// ====================================================================
// === CONCRETE GITHUB LOADER IMPLEMENTATION ==========================
// ====================================================================

// GitHubLoader implements the DocumentLoader interface using the GitHub API.
type GitHubLoader struct {
	GitHubClient *github.Client // <-- NEW: Injected here
}

// NewGitHubLoader creates a GitHub-specific document loader.
func NewGitHubLoader(client *github.Client) *GitHubLoader {
	return &GitHubLoader{
		GitHubClient: client, // <-- Client is stored here
	}
}

// Load initiates the process of fetching documents from the GitHub repository.
// This function contains the logic previously in the inner loop of executeUnionQueries.
func (l *GitHubLoader) Load(
	ctx context.Context,
	config *GetIndexConfigurationInput,
	queryComposite map[string]interface{},
) (DocumentIterator, error) {

	// 1. Get Index Configuration
	indexObject, err := l.GetIndexById(config)
	if err != nil || indexObject == nil {
		return nil, fmt.Errorf("failed to retrieve index config for ID '%s'", config.Id)
	}

	var contentPath string
	fieldsInterface, fieldsOk := indexObject["fields"].([]interface{})
	if !fieldsOk {
		return nil, fmt.Errorf("index configuration missing 'fields'")
	}

	// 2. Build Composite Path (using the composite data from the query)
	if len(queryComposite) > 0 {
		compositePath := ""
		for idx, f := range fieldsInterface {
			fStr := f.(string)
			compositeVal, found := queryComposite[fStr]
			if found {
				compositePath += fmt.Sprintf("%v", compositeVal)
			}
			if idx < (len(fieldsInterface) - 1) {
				compositePath += ":"
			}
		}
		contentPath = compositePath
	} else {
		return nil, fmt.Errorf("query configuration missing 'Composite'")
	}

	repoToFetch, ok := indexObject["repoName"].(string)
	if !ok || repoToFetch == "" {
		return nil, fmt.Errorf("index configuration missing 'repoName'")
	}

	// 3. Fetch Directory Contents
	_, dirContents, _, err := l.GitHubClient.Repositories.GetContents(
		ctx, config.Owner, repoToFetch, contentPath,
		&github.RepositoryContentGetOptions{Ref: config.Branch},
	)

	if err != nil || dirContents == nil {
		return nil, fmt.Errorf("failed to list contents at path %s: %v", contentPath, err)
	}

	// 4. Return the concrete iterator implementation
	return NewGitHubFileIterator(dirContents), nil
}

// GetIndexById retrieves the index configuration JSON file from the GitHub repository.
func (l *GitHubLoader) GetIndexById(c *GetIndexConfigurationInput) (map[string]interface{}, error) {
	log.Printf("GetIndexById: Attempting to retrieve config for ID: %s", c.Id)
	var contract map[string]interface{}

	pieces := strings.Split(c.Repo, "/")
	opts := &github.RepositoryContentGetOptions{
		Ref: c.Branch,
	}
	// File path is assumed to be index/{ID}.json
	file, _, res, err := l.GitHubClient.Repositories.GetContents(context.Background(), pieces[0], pieces[1], "index/"+c.Id+".json", opts)
	if err != nil || res.StatusCode != 200 {
		log.Printf("GetIndexById: Failed to retrieve config for %s: Status %d, Error: %v", c.Id, res.StatusCode, err)
		return contract, nil
	}
	if file != nil && file.Content != nil {
		content, err := base64.StdEncoding.DecodeString(*file.Content)
		if err == nil {
			json.Unmarshal(content, &contract)
			log.Printf("GetIndexById: Successfully retrieved config for %s.", c.Id)
		} else {
			log.Printf("GetIndexById: Invalid index unable to parse content for %s: %v", c.Id, err)
			return contract, errors.New("Invalid index unable to parse.")
		}
	}
	return contract, nil
}

// ====================================================================
// === CONCRETE GITHUB ITERATOR IMPLEMENTATION ========================
// ====================================================================

// GitHubFileIterator implements the DocumentIterator interface.
type GitHubFileIterator struct {
	contents []*github.RepositoryContent
	index    int
	lastErr  error
}

// NewGitHubFileIterator creates a new iterator from the fetched GitHub directory contents.
func NewGitHubFileIterator(contents []*github.RepositoryContent) *GitHubFileIterator {
	files := make([]*github.RepositoryContent, 0, len(contents))
	for _, content := range contents {
		// Filter non-file items, assuming file name is base64 encoded JSON document
		if content.GetType() == "file" && content.GetName() != "" {
			files = append(files, content)
		}
	}
	return &GitHubFileIterator{contents: files}
}

// Next fetches, decodes, and unmarshals the next document file content.
func (i *GitHubFileIterator) Next() (map[string]interface{}, bool) {
	if i.index >= len(i.contents) {
		return nil, false
	}

	content := i.contents[i.index]
	i.index++

	decodedBytes, err := base64.StdEncoding.DecodeString(content.GetName())
	if err != nil {
		i.lastErr = fmt.Errorf("iterator failed to decode content '%s': %v", content.GetName(), err)
		return nil, true // Continue to next item
	}
	itemBody := string(decodedBytes)

	var itemData map[string]interface{}
	if err := json.Unmarshal([]byte(itemBody), &itemData); err != nil {
		i.lastErr = fmt.Errorf("iterator failed to unmarshal JSON from '%s': %v", content.GetName(), err)
		return nil, true // Continue to next item
	}
	
	return itemData, true
}

func (i *GitHubFileIterator) Error() error { return i.lastErr }
func (i *GitHubFileIterator) Close()       {}