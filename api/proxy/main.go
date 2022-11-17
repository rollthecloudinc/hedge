package main

import (
	"goclassifieds/lib/utils"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var aveDomain string
var aveApiKey string
var carbonAwareDomain string

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
	}
	return events.APIGatewayProxyResponse{StatusCode: 400}, nil
}

func main() {
	log.SetFlags(0)
	aveDomain = os.Getenv("PROXY_AVE_DOMAIN")
	aveApiKey = os.Getenv("PROXY_AVE_APIKEY")
	carbonAwareDomain = os.Getenv("PROXY_CARBONAWARE_DOMAIN")
	lambda.Start(ProxyRequest)
}
