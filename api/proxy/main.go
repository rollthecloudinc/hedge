package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"goclassifieds/lib/utils"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var aveDomain string
var aveApiKey string
var carbonAwareDomain string
var marvelPublicKey string
var marvelPrivateKey string
var comicvineApiKey string
var comicvineBaseURL = "https://comicvine.gamespot.com/api" // ComicVine API base URL

func GetCities(country string, state string, city string) (string, error) {
	res, err := http.Get("http://api.zippopotam.us/" + country + "/" + state + "/" + city)
	if err != nil {
		return "", err
	}
	body, _ := ioutil.ReadAll(res.Body)
	return string(body), nil
}

func GetRequest(domain string, req *events.APIGatewayProxyRequest) (string, error) {
	qs := make([]string, len(req.QueryStringParameters))
	i := 0
	for k, v := range req.QueryStringParameters {
		qs[i] = url.QueryEscape(k) + "=" + url.QueryEscape(v)
		i++
	}
	var res *http.Response
	var err error
	var uri string
	if strings.Index(req.Path, "carbonaware") > -1 {
		uri = "https://" + domain + "/" + req.PathParameters["proxy"] + "?" + strings.Join(qs, "&")
		log.Print(uri)
		res, err = http.Get(uri)
	} else {
		res, err = http.Get("https://" + domain + "/query?apikey=" + aveApiKey + "&" + strings.Join(qs, "&"))
	}
	if err != nil {
		return "", err
	}
	body, _ := ioutil.ReadAll(res.Body)
	return string(body), nil
}

func GetMarvelRequest(req *events.APIGatewayProxyRequest) (string, error) {
	baseURL := "https://gateway.marvel.com/v1/public"

	// Collect query string parameters from the request
	queryParams := url.Values{}
	for k, v := range req.QueryStringParameters {
		queryParams.Add(k, v)
	}

	// Generate authentication parameters
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	toHash := timestamp + marvelPrivateKey + marvelPublicKey
	hash := md5.Sum([]byte(toHash))
	hashString := hex.EncodeToString(hash[:])

	// Add authentication parameters to the query string
	queryParams.Set("ts", timestamp)
	queryParams.Set("apikey", marvelPublicKey) // Public Key
	queryParams.Set("hash", hashString)

	// Construct the full URI
	uri := fmt.Sprintf("%s/%s?%s", baseURL, req.PathParameters["proxy"], queryParams.Encode())

	// Log the request for debugging
	log.Printf("Marvel API Query String: %s", queryParams.Encode())
	log.Printf("Hash Input: ts=%s, privateKey=%s, publicKey=%s", timestamp, marvelPrivateKey, marvelPublicKey)
	log.Printf("Hash Result: %s", hashString)
	log.Printf("Constructed Marvel API URI: %s", uri)

	// Execute the GET request
	res, err := http.Get(uri)
	if err != nil {
		log.Printf("Error calling Marvel API: %v", err)
		return "", err
	}
	defer res.Body.Close()

	// Log the response status code
	log.Printf("Marvel API Response Status: %d", res.StatusCode)

	// Read and return the response body
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Printf("Error reading Marvel API response: %v", err)
		return "", err
	}

	// Log the response body (for debugging only; sensitive data may need redaction)
	log.Printf("Marvel API Response Body: %s", string(body))

	return string(body), nil
}

// Function to call ComicVine API
func GetComicVineRequest(req *events.APIGatewayProxyRequest) (string, error) {
	// Build the ComicVine API URL
	proxyPath := req.PathParameters["proxy"] // API endpoint to proxy (e.g., "characters")
	queryParams := url.Values{}

	// Apply query string parameters provided by the client
	for k, v := range req.QueryStringParameters {
		queryParams.Add(k, v)
	}

	// Add ComicVine-specific parameters
	queryParams.Set("api_key", comicvineApiKey) // Add ComicVine API key
	queryParams.Set("format", "json")          // Ensure JSON format for the response

	// Construct a full URL for the ComicVine API request
	apiURL := fmt.Sprintf("%s/%s?%s", comicvineBaseURL, proxyPath, queryParams.Encode())
	log.Printf("ComicVine API Request URL: %s", apiURL)

	// Make the request to ComicVine API
	res, err := http.Get(apiURL)
	if err != nil {
		log.Printf("Error while calling ComicVine API: %v", err)
		return "", err
	}
	defer res.Body.Close()

	// Read the response body and return it
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Printf("Error while reading response: %v", err)
		return "", err
	}
	return string(body), nil
}

func ProxyRequest(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	usageLog := &utils.LogUsageLambdaInput{
		// UserId: GetUserId(req),
		//Username:     GetUsername(req),
		UserId:       "null",
		Username:     "null",
		Resource:     req.Resource,
		Path:         req.Path,
		RequestId:    req.RequestContext.RequestID,
		Intensities:  "null",
		Regions:      "null",
		Region:       "null",
		Service:      "null",
		Repository:   "null",
		Organization: "null",
	}
	_, hedged := req.Headers["x-hedge-region"]
	if hedged {
		usageLog.Intensities = req.Headers["x-hedge-intensities"]
		usageLog.Regions = req.Headers["x-hedge-regions"]
		usageLog.Region = req.Headers["x-hedge-region"]
		usageLog.Service = req.Headers["x-hedge-service"]
	}

	utils.LogUsageForLambdaWithInput(usageLog)

	if strings.Index(req.Path, "cities") > -1 {
		body, err := GetCities(req.PathParameters["country"], req.PathParameters["state"], req.PathParameters["city"])
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 500}, err
		}
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: body, Headers: map[string]string{"Content-Type": "application/json"}}, nil
	} else if strings.Index(req.Path, "ave") > -1 {
		body, err := GetRequest(aveDomain, req)
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 500}, err
		}
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: body, Headers: map[string]string{"Content-Type": "application/json"}}, nil
	} else if strings.Index(req.Path, "carbonaware") > -1 {
		body, err := GetRequest(carbonAwareDomain, req)
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 500}, err
		}
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: body, Headers: map[string]string{"Content-Type": "application/json"}}, nil
	} else if strings.Index(req.Path, "marvel") > -1 {
		body, err := GetMarvelRequest(req)
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 500}, err
		}
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: body, Headers: map[string]string{"Content-Type": "application/json"}}, nil
	} else if strings.Index(req.Path, "comicvine") > -1 {
		body, err := GetComicVineRequest(req)
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 500}, err
		}
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: body, Headers: map[string]string{"Content-Type": "application/json"}}, nil
	}
	return events.APIGatewayProxyResponse{StatusCode: 400}, nil
}

func main() {
	log.SetFlags(0)
	aveDomain = os.Getenv("PROXY_AVE_DOMAIN")
	aveApiKey = os.Getenv("PROXY_AVE_APIKEY")
	carbonAwareDomain = os.Getenv("PROXY_CARBONAWARE_DOMAIN")
	marvelPublicKey = os.Getenv("MARVEL_API_PUBLIC_KEY")
	marvelPrivateKey = os.Getenv("MARVEL_API_PRIVATE_KEY")
	comicvineApiKey = os.Getenv("COMICVINE_API_KEY")
	lambda.Start(ProxyRequest)
}