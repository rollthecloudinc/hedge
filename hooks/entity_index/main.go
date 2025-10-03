package main

import (
	"context"
	"goclassifieds/lib/entity"
	"log"
	"os"
	"fmt"
	"strings"
	"encoding/json"
	"encoding/base64" // Import the encoding/base64 package

	"goclassifieds/lib/repo"

	"github.com/aws/aws-lambda-go/lambda"
	"golang.org/x/oauth2"
	"github.com/google/go-github/v46/github"
)

func handler(ctx context.Context, event entity.AfterSaveExecEntityRequest) (entity.AfterSaveExecEntityResponse, error) {

	var entityBase64 string
	var oldEntityBase64 string

	/**
	 * This is where all the code goes to create action SECRETS
	 * for a site. Both for repo and enviironment.
	 */
	log.Printf("Index entity %s in repo %s and owner %s", event.Contract, event.Repo, event.Owner)

	// Log the entire content of event.Entity
	entityJSON, err := json.Marshal(event.Entity) // Convert map[string]interface{} to JSON string
	if err != nil {
		log.Printf("Error marshalling event.Entity: %s", err)
	} else {
		// Base64 encode the JSON
		entityBase64 = base64.StdEncoding.EncodeToString(entityJSON)
		log.Printf("Entity content: %s", entityJSON)
		log.Printf("Base64 encoded entity content: %s", entityBase64)
	}

	// Log the entire content of event.OldEntity
	entityJSON, err = json.Marshal(event.OldEntity) // Convert map[string]interface{} to JSON string
	if err != nil {
		log.Printf("Error marshalling event.OldEntity: %s", err)
	} else {
		oldEntityBase64 = base64.StdEncoding.EncodeToString(entityJSON)
		log.Printf("Old Entity content: %s", entityJSON)
		log.Printf("Base64 encoded entity content: %s", oldEntityBase64)
	}

	if entityBase64 == oldEntityBase64 {
		log.Print("New and old match")
	} else {
		log.Print("Old does not match new")
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

	// repoName := "conversations10" // hardcode for now
	branch := "dev" // For now hard code.

	// Try out the index discovery experimental

	// Discover matching index entities for the given contract
	matchingIndexes, err := discoverIndexes(ctx, githubRestClient, event.Owner, event.Repo, branch, event.Contract)
	if err != nil {
		log.Printf("Failed to discover indexes: %v", err)
		return entity.AfterSaveExecEntityResponse{}, err
	}
	
	// Log the matching index entities
	log.Printf("Discovered %d matching index entities.", len(matchingIndexes))
	for _, indexEntity := range matchingIndexes {

		/*indexJSON, _ := json.Marshal(index)
		log.Printf("Match %d: %s", i+1, indexJSON)*/

		// Extract `repoName` from the indexEntity, fallback to hardcoded value if missing
		repoName, ok := indexEntity["repoName"].(string)
		if !ok || repoName == "" {
			log.Printf("ERROR: No 'repoName' field in index entity.")
			continue // for now ignore
		} else {
			log.Printf("Using repoName from index entity: %s", repoName)
		}

		// Extract and combine field values
		prefix, err := extractAndCombineFields(event.Entity, indexEntity)
		if err != nil {
			log.Printf("Failed to extract and combine fields: %v", err)
			continue // for now ignore
		} else {
			log.Printf("Extracted and combined into: %s", prefix)
		}

		// Handle renaming if OldEntity exists
		if event.OldEntity != nil {
			oldFilePath := fmt.Sprintf("%s/%s", prefix, oldEntityBase64)
			newFilePath := fmt.Sprintf("%s/%s", prefix, entityBase64)

			if (oldEntityBase64 != entityBase64) {
				log.Printf("Attempting to rename file from '%s' to '%s'", oldFilePath, newFilePath)
				if err := renameFile(ctx, githubRestClient, event.Owner, repoName, oldFilePath, newFilePath, branch); err != nil {
					log.Printf("Error renaming file: %s", err)
					continue // ignore for now
					// return entity.AfterSaveExecEntityResponse{}, err
				}
				log.Printf("File renamed successfully.")
			} else {
				log.Print("Old entity is the same as new entity bail out without indexing.")
			}

		} else {
			// No old entity, create the file directly
			filePath := fmt.Sprintf("%s/%s", prefix, entityBase64)
			if err := repo.CreateFileIfNotExists(ctx, githubRestClient, event.Owner, repoName, filePath, "", branch); err != nil {
				log.Printf("Error creating file: %s", err)
				continue // ignore for now
				// return entity.AfterSaveExecEntityResponse{}, err
			}
			log.Printf("File created successfully at path: %s", filePath)
		}

	}

	return entity.AfterSaveExecEntityResponse{}, nil
}


// Helper function to extract nested field values using dot notation
func getNestedValue(data map[string]interface{}, fieldPath string) (string, error) {
	parts := strings.Split(fieldPath, ".")
	var value interface{} = data

	for _, part := range parts {
		// Assert the current value is a map
		if m, ok := value.(map[string]interface{}); ok {
			value = m[part]
		} else {
			return "", fmt.Errorf("field '%s' does not exist in the provided data", fieldPath)
		}
	}

	// Convert final value to string
	if strValue, ok := value.(string); ok {
		return strValue, nil
	}

	return "", fmt.Errorf("field '%s' does not resolve to a string value", fieldPath)
}

// Main function to process and extract combined field values
func extractAndCombineFields(entityJSON map[string]interface{}, indexEntity map[string]interface{}) (string, error) {
	// Extract the `fields` array from the index entity
	fields, ok := indexEntity["fields"].([]interface{})
	if !ok {
		return "", fmt.Errorf("'fields' is missing or not an array in the index entity")
	}

	// Iterate through the fields and extract corresponding values from the entity JSON
	var extractedValues []string
	for _, field := range fields {
		fieldName, ok := field.(string)
		if !ok {
			return "", fmt.Errorf("field name '%v' is not a string", field)
		}

		value, err := getNestedValue(entityJSON, fieldName)
		if err != nil {
			return "", fmt.Errorf("error extracting field '%s': %v", fieldName, err)
		}

		extractedValues = append(extractedValues, value)
	}

	// Combine the extracted values into a single string separated by `:`
	return strings.Join(extractedValues, ":"), nil
}

func renameFile(ctx context.Context, client *github.Client, owner, repo, oldPath, newPath, branch string) error {
	// Retrieve the old file's SHA to delete it
	opts := &github.RepositoryContentGetOptions{Ref: branch}
	file, _, _, err := client.Repositories.GetContents(ctx, owner, repo, oldPath, opts)
	if err != nil {
		return fmt.Errorf("failed to retrieve old file at path '%s': %w", oldPath, err)
	}

	if file == nil || file.SHA == nil {
		return fmt.Errorf("old file at path '%s' is missing SHA", oldPath)
	}
	oldFileSHA := *file.SHA

	// Delete the old file
	deleteOpts := &github.RepositoryContentFileOptions{
		Message: github.String(fmt.Sprintf("Deleting old file at %s", oldPath)),
		SHA:     github.String(oldFileSHA),
		Branch:  github.String(branch),
	}
	if _, _, err := client.Repositories.DeleteFile(ctx, owner, repo, oldPath, deleteOpts); err != nil {
		return fmt.Errorf("failed to delete old file at path '%s': %w", oldPath, err)
	}
	log.Printf("Deleted old file at path: %s", oldPath)

	// Create the new file
	createOpts := &github.RepositoryContentFileOptions{
		Message: github.String(fmt.Sprintf("Creating new file at %s", newPath)),
		Content: []byte{},
		Branch:  github.String(branch),
	}
	if _, _, err := client.Repositories.CreateFile(ctx, owner, repo, newPath, createOpts); err != nil {
		return fmt.Errorf("failed to create new file at path '%s': %w", newPath, err)
	}
	log.Printf("Created new file at path: %s", newPath)

	return nil
}

func discoverIndexes(ctx context.Context, githubClient *github.Client, owner, repo, branch, contract string) ([]map[string]interface{}, error) {
    // Ensure the contract is prefixed with "contracts/"
    contractToFind := contract

    // Fetch directory contents
    _, dirContents, _, err := githubClient.Repositories.GetContents(ctx, owner, repo, "index", &github.RepositoryContentGetOptions{Ref: branch})
    if err != nil {
        return nil, fmt.Errorf("failed to list contents of the `index` directory: %w", err)
    }

    if dirContents == nil {
        return nil, fmt.Errorf("`index` directory is empty or not accessible")
    }

    // Prepare the slice to hold matching index entities
    var matchingEntities []map[string]interface{}

    // Iterate over each file in the directory
    for _, content := range dirContents {
        if content.GetType() != "file" {
            continue // Only process files
        }

        // Retrieve the content of the file
        file, _, _, err := githubClient.Repositories.GetContents(ctx, owner, repo, content.GetPath(), &github.RepositoryContentGetOptions{Ref: branch})
        if err != nil {
            log.Printf("Failed to retrieve file '%s': %v", content.GetPath(), err)
            continue // Skip to the next file
        }

        if file.Content == nil || *file.Content == "" {
            log.Printf("File '%s' has no content or is empty; skipping.", content.GetPath())
            continue
        }

        // Decode the file content, which is Base64 encoded
        var decodedContent []byte
        decodedContent, err = base64.StdEncoding.DecodeString(*file.Content)
        if err != nil {
            log.Printf("Failed to decode content of file '%s' as Base64: %v", content.GetPath(), err)
            log.Printf("Trying raw JSON...")

            // Attempt raw JSON parsing directly
            decodedContent = []byte(*file.Content)
        }

        // Parse the content into JSON
        var indexEntity map[string]interface{}
        err = json.Unmarshal(decodedContent, &indexEntity)
        if err != nil {
            log.Printf("Failed to parse JSON content for file '%s': %v", content.GetPath(), err)
            continue
        }

        // Check if the file's `entity` field matches the specified contract
        if entityField, exists := indexEntity["entity"].(string); exists && ("/contracts/" + entityField + ".json") == contractToFind {
            log.Printf("Found matching index entity in file: %s", content.GetPath())
            matchingEntities = append(matchingEntities, indexEntity)
        } else {
			log.Printf("No match to contract entity /contracts/%s.json != %s", entityField, contractToFind)
		}
    }

    return matchingEntities, nil
}

func main() {
	log.SetFlags(0)
	// Make the handler available for Remote Procedure Call by AWS Lambda
	lambda.Start(handler)
}