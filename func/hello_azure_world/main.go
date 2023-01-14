package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"goclassifieds/lib/shapeshift"
	"log"
	"net/http"
	"os"

	"github.com/MicahParks/keyfunc"
	"github.com/aws/aws-lambda-go/events"
	"github.com/golang-jwt/jwt/v4"
)

type InvokeRequest struct {
	Data     map[string]json.RawMessage
	Metadata map[string]interface{}
}

type AuthenticateInput struct {
	Token string
}

type InvokeResponse struct {
	Outputs     map[string]interface{}
	Logs        []string
	ReturnValue interface{}
}

type AuthenticateOutput struct {
	Claims jwt.MapClaims
}

type AzureHttpTriggerRequest struct {
	Method  string              `json:"Method"`
	Body    string              `json:"Body"`
	Headers map[string][]string `json:"Headers"`
	Params  map[string]string   `json:"Params"`
}

func EntityHandler(w http.ResponseWriter, r *http.Request) {
	var invokeRequest InvokeRequest

	d := json.NewDecoder(r.Body)
	d.Decode(&invokeRequest)

	// var reqData map[string]interface{}
	var reqData AzureHttpTriggerRequest
	json.Unmarshal(invokeRequest.Data["req"], &reqData)

	log.Printf("%#v", reqData)
	b, _ := json.Marshal(reqData)
	log.Print(string(b))

	awsRequest, _ := AzureRequestToAws(&reqData)

	token, _ := awsRequest.Headers["Authorization"]
	authInput := &AuthenticateInput{
		Token: token,
	}

	_, err := Authenticate(authInput)
	if err != nil {

		w.WriteHeader(http.StatusUnauthorized)

	} else {

		/*for k, v := range authRes.Claims {
			log.Print("claim " + k + " = " + v.(string))
		}*/

		outputs := make(map[string]interface{})
		//outputs["message"] = reqData["Body"]
		outputs["message"] = reqData.Body

		resData := make(map[string]interface{})
		resData["body"] = "Order enqueued"
		outputs["res"] = resData
		invokeResponse := InvokeResponse{outputs, nil, nil}

		log.Printf("AWS Method = %s", awsRequest.HTTPMethod)
		log.Printf("AWS Host = %s", awsRequest.Headers["Host"])
		log.Printf("AWS Content-Type = %s", awsRequest.Headers["Content-Type"])
		log.Printf("AWS Accept = %s", awsRequest.Headers["Accept"])
		log.Printf("AWS Path = %s", awsRequest.Path)
		log.Printf("AWS Body = %s", awsRequest.Body)
		for param, value := range awsRequest.PathParameters {
			log.Printf("AWS Param Name = %s | Value = %s", param, value)
		}
		/*log.Printf("Method = %s", reqData.Method)
		log.Printf("Host = %s", reqData.Headers["Host"][0])
		log.Printf("Content-Type = %s", reqData.Headers["Content-Type"][0])
		log.Printf("Accept = %s", reqData.Headers["Accept"][0])
		log.Printf("Path = %s", reqData.Headers["X-Original-URL"][0])
		log.Printf("Body = %s", reqData.Body)*/
		/*log.Printf("Method = %s", reqData["Method"])
		log.Printf("Host = %s", reqData["Headers"].(map[string]interface{})["Host"].([]interface{})[0])
		log.Printf("Content-Type = %s", reqData["Headers"].(map[string]interface{})["Content-Type"].([]interface{})[0])
		log.Printf("Accept = %s", reqData["Headers"].(map[string]interface{})["Accept"].([]interface{})[0])
		log.Printf("Path = %s", reqData["Headers"].(map[string]interface{})["X-Original-URL"].([]interface{})[0])
		log.Printf("Body = %s", reqData["Body"])*/

		ac := shapeshift.ShapeshiftActionContext()
		handler := shapeshift.InitializeHandler(ac)
		awsRes, err := handler(awsRequest)
		if err != nil {
			log.Print(err.Error())
		} else {
			log.Printf("AWS Response Status Code: %d", awsRes.StatusCode)
		}

		responseJson, _ := json.Marshal(invokeResponse)

		w.Header().Set("Content-Type", "application/json")
		w.Write(responseJson)

	}
}

func AzureRequestToAws(req *AzureHttpTriggerRequest) (*events.APIGatewayProxyRequest, error) {
	headers := make(map[string]string)
	multiHeaders := make(map[string][]string)
	params := make(map[string]string)
	for header, values := range req.Headers {
		v := ""
		if values != nil && len(values) != 0 {
			v = values[0]
		}
		headers[header] = v
		multiHeaders[header] = values
	}
	for param, value := range req.Params {
		params[param] = value
	}
	awsRequest := &events.APIGatewayProxyRequest{
		Path:              headers["X-Original-URL"],
		HTTPMethod:        req.Method,
		Body:              req.Body,
		Headers:           headers,
		MultiValueHeaders: multiHeaders,
		PathParameters:    params,
	}
	return awsRequest, nil
}

func Authenticate(input *AuthenticateInput) (*AuthenticateOutput, error) {
	output := &AuthenticateOutput{}

	jwks, err := keyfunc.Get("https://cognito-idp.us-east-1.amazonaws.com/"+os.Getenv("USER_POOL_ID")+"/.well-known/jwks.json", keyfunc.Options{})
	if err != nil {
		// log.Fatalln("Unable to fetch keys")
		log.Print(err)
		return output, errors.New("Unable to parse token")
	}

	log.Print("fectehd keys")
	token := input.Token[7:]

	// Verify
	t, err := jwt.Parse(token, jwks.Keyfunc)
	if err != nil || !t.Valid {
		log.Print(err)
		return output, errors.New("Unable to parse token")
	}

	log.Print("authorized")

	output.Claims = t.Claims.(jwt.MapClaims)

	log.Print("got claims")

	//claims["cognito:groups"] = nil //claims["cognito:groups"]
	//claims["cognito:roles"] = nil  //claims["cognito:groups"]

	return output, nil
}

// func WriteAwsResponse

func main() {
	log.Print("Hello")
	customHandlerPort, exists := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT")
	if !exists {
		customHandlerPort = "8080"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", EntityHandler)
	fmt.Println("Go server Listening on: ", customHandlerPort)
	log.Fatal(http.ListenAndServe(":"+customHandlerPort, mux))
}
