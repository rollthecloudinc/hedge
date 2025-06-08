package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// GitHubToken is your GitHub Personal Access Token
var GitHubToken = os.Getenv("GITHUB_TOKEN")

// WebhookSecret is your GitHub webhook secret (optional for signature validation)
var WebhookSecret = os.Getenv("WEBHOOK_SECRET")

// BotUsername is the username of the bot (to avoid responding to itself)
var BotUsername = os.Getenv("VERSAIT_USERNAME")

// Open AI Api Key
var OpenAiApiKey = os.Getenv("OPENAI_API_KEY")

// CommentPayload represents the structure for posting a comment to GitHub API
type CommentPayload struct {
	Body string `json:"body"`
}

// WebhookPayload represents the incoming GitHub webhook payload
type WebhookPayload struct {
	Action     string `json:"action"`
	Comment    struct {
		Body string `json:"body"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"comment"`
	Issue struct {
		CommentsURL string `json:"comments_url"`
	} `json:"issue"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

// validateSignature validates the GitHub webhook signature.
func validateSignature(secret string, payload []byte, receivedSignature string) bool {
	// Ensure the received signature starts with "sha256=".
	if len(receivedSignature) <= 7 || receivedSignature[:7] != "sha256=" {
		return false //errors.New("invalid signature format")
	}

	// Extract the actual hash from the received signature (after "sha256=").
	expectedMAC := receivedSignature[7:]

	// Compute HMAC using SHA-256 with the secret and raw payload.
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	computedMAC := hex.EncodeToString(mac.Sum(nil))

	// Use hmac.Equal to securely compare the two signatures.
	if !hmac.Equal([]byte(computedMAC), []byte(expectedMAC)) {
		return false // errors.New("signature mismatch")
	}

	return true
}

/*func validateSignature(secret, payload []byte, signatureHeader string) bool {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	expectedMAC := mac.Sum(nil)
	expectedSignature := fmt.Sprintf("sha256=%x", expectedMAC)
	return hmac.Equal([]byte(expectedSignature), []byte(signatureHeader))
}*/

// postComment posts a "Hello World" comment to the GitHub API
func postComment(commentsURL string, message string) error {
	// Prepare the payload
	payload := CommentPayload{
		Body: message,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal comment payload: %v", err)
	}

	// Create the HTTP request
	req, err := http.NewRequest("POST", commentsURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	// Set headers for authentication and content type
	req.Header.Set("Authorization", "token "+GitHubToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	// Make the HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make HTTP request: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusCreated {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("failed to post comment: %v, response: %s", resp.StatusCode, string(body))
	}

	log.Println("Comment posted successfully!")
	return nil
}

func fetchComments(commentsURL string) ([]string, error) {
	// Create the HTTP request
	req, err := http.NewRequest("GET", commentsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %v", err)
	}

	// Set headers for authentication
	req.Header.Set("Authorization", "token "+GitHubToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	// Make the HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch comments: %v, response: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var comments []struct {
		Body string `json:"body"`
	}
	err = json.NewDecoder(resp.Body).Decode(&comments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse comments response: %v", err)
	}

	// Extract and return the comment bodies
	var commentBodies []string
	for _, comment := range comments {
		commentBodies = append(commentBodies, comment.Body)
	}
	return commentBodies, nil
}

func getGPT4Response(context []string, userComment string) (string, error) {
	// Prepare the OpenAI API request payload
	requestBody := map[string]interface{}{
		"model": "gpt-4o", // Update this to your specific GPT-4 deployment model name
		"messages": []map[string]string{
			{"role": "system", "content": "You are a helpful assistant."},
			{"role": "user", "content": "Here is the context of the discussion:\n" + strings.Join(context, "\n") + "\n\nNew Comment:\n" + userComment + "\n\nPlease provide an appropriate response."},
		},
	}

	// Convert the request body to JSON
	requestData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal GPT-4 request: %v", err)
	}

	// Create the HTTP request to the OpenAI API
	openAIAPIURL := "https://cent.openai.azure.com/openai/deployments/gpt-4o/chat/completions?api-version=2025-01-01-preview" // Replace with your GPT-4 deployment endpoint if it's different
	req, err := http.NewRequest("POST", openAIAPIURL, bytes.NewBuffer(requestData))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %v", err)
	}

	log.Printf("u = " + openAIAPIURL)
	log.Printf("k = " + OpenAiApiKey)

	// Set headers for authentication and content type
	req.Header.Set("Authorization", "Bearer " + OpenAiApiKey) // Ensure the OPENAI_API_KEY environment variable is set
	req.Header.Set("Content-Type", "application/json")

	// Make the HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make HTTP request to GPT-4: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get GPT-4 response: %v, response: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var responsePayload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	err = json.NewDecoder(resp.Body).Decode(&responsePayload)
	if err != nil {
		return "", fmt.Errorf("failed to parse GPT-4 response: %v", err)
	}

	// Extract the assistant's response
	if len(responsePayload.Choices) == 0 || responsePayload.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("GPT-4 returned an empty response")
	}

	return responsePayload.Choices[0].Message.Content, nil
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var body []byte
	var err error

	// Decode the Base64-encoded body if `isBase64Encoded` is true
	if request.IsBase64Encoded {
		body, err = base64.StdEncoding.DecodeString(request.Body)
		if err != nil {
			log.Printf("Failed to decode Base64 body: %v", err)
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusBadRequest,
				Body:       "Failed to decode Base64 body",
			}, nil
		}
	} else {
		body = []byte(request.Body)
	}

	// Parse the URL-encoded body to extract the payload
	values, err := url.ParseQuery(string(body))
	if err != nil {
		log.Printf("Failed to parse URL-encoded body: %v", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body: "Failed to parse URL-encoded body",
		}, nil
	}

	// Extract the JSON payload from the `payload` field
	payloadString := values.Get("payload")
	if payloadString == "" {
		log.Println("No payload found in the request body")
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "No payload found in the request body",
		}, nil
	}

	// Validate the webhook signature
	// 
	signature := request.Headers["X-Hub-Signature-256"]
	// 52f2982e-90ef-4241-8293-100518a62e02
	if WebhookSecret != "" && !validateSignature(WebhookSecret, []byte(payloadString), signature) {
		log.Println("Invalid signature")
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusForbidden,
			Body:       "Invalid signature",
		}, nil
	}

	// Parse the JSON payload into the WebhookPayload struct
	var payload WebhookPayload
	err = json.Unmarshal([]byte(payloadString), &payload)
	if err != nil {
		log.Printf("Failed to parse JSON payload: %v", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Failed to parse JSON payload",
		}, nil
	}

	// Process only "issue_comment" events with the "created" action
	if payload.Action == "created" {
		commenter := payload.Comment.User.Login

		// Avoid responding to the bot's own comments
		if commenter == BotUsername {
			log.Println("Ignoring bot's own comment")
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusOK,
				Body:       "Ignored bot's own comment",
			}, nil
		}

		// Fetch previous comments for context
		comments, err := fetchComments(payload.Issue.CommentsURL)
		if err != nil {
			log.Printf("Failed to fetch previous comments: %v", err)
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
				Body:       "Failed to fetch previous comments",
			}, nil
		}

		// Send the context and new comment to GPT-4 for a response
		gptResponse, err := getGPT4Response(comments, payload.Comment.Body)
		if err != nil {
			log.Printf("Failed to get GPT-4 response: %v", err)
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
				Body:       "Failed to get GPT-4 response",
			}, nil
		}

		// Post the GPT-4 response as a comment
		err = postComment(payload.Issue.CommentsURL, gptResponse)
		if err != nil {
			log.Printf("Failed to post GPT-4 response as a comment: %v", err)
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
				Body:       "Failed to post GPT-4 response as a comment",
			}, nil
		}

		log.Printf("Posted GPT-4 response to issue in repository %s", payload.Repository.FullName)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Body:       "GPT-4 response posted successfully",
		}, nil
	}

	// If the event type is not "created", ignore it
	log.Println("Event ignored")
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       "Event ignored",
	}, nil
}

func main() {
	log.SetFlags(0)
	lambda.Start(handler)
}
