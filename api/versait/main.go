package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// GitHubToken is your GitHub Personal Access Token
var GitHubToken = os.Getenv("GITHUB_TOKEN")

// WebhookSecret is your GitHub webhook secret (optional for signature validation)
var WebhookSecret = os.Getenv("WEBHOOK_SECRET")

// BotUsername is the username of the bot (to avoid responding to itself)
var BotUsername = "classifieds-dev" // os.Getenv("BOT_USERNAME")

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

// validateSignature validates the webhook signature (optional)
func validateSignature(secret, payload []byte, signatureHeader string) bool {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	expectedMAC := mac.Sum(nil)
	expectedSignature := fmt.Sprintf("sha256=%x", expectedMAC)
	return hmac.Equal([]byte(expectedSignature), []byte(signatureHeader))
}

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
	signature := request.Headers["X-Hub-Signature-256"]
	if WebhookSecret != "" && !validateSignature([]byte(WebhookSecret), []byte(payloadString), signature) {
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

		// Post a "Hello World" comment
		err = postComment(payload.Issue.CommentsURL, "Hello World!")
		if err != nil {
			log.Printf("Failed to post comment: %v", err)
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
				Body:       "Failed to post comment",
			}, nil
		}

		log.Printf("Responded to comment by %s in repository %s", commenter, payload.Repository.FullName)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Body:       "Comment posted successfully",
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
