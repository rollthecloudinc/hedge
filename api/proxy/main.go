package main

import (
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func GetCities(country string, state string, city string) (string, error) {
	res, err := http.Get("http://api.zippopotam.us/" + country + "/" + state + "/" + city)
	if err != nil {
		return "", err
	}
	body, _ := ioutil.ReadAll(res.Body)
	return string(body), nil
}

func ProxyRequest(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	if strings.Index(req.Path, "cities") > -1 {
		body, err := GetCities(req.PathParameters["country"], req.PathParameters["state"], req.PathParameters["city"])
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 500}, err
		}
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: body, Headers: map[string]string{"Content-Type": "application/json"}}, nil
	}
	return events.APIGatewayProxyResponse{StatusCode: 400}, nil
}

func main() {
	lambda.Start(ProxyRequest)
}
