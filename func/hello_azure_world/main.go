package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
)

type InvokeRequest struct {
	Data     map[string]json.RawMessage
	Metadata map[string]interface{}
}

type InvokeResponse struct {
	Outputs     map[string]interface{}
	Logs        []string
	ReturnValue interface{}
}

func EntityHandler(w http.ResponseWriter, r *http.Request) {
	var invokeRequest InvokeRequest

	awsRequest := events.APIGatewayProxyRequest{
		Path:       r.RequestURI,
		HTTPMethod: r.Method,
		Body:       "",
	}

	d := json.NewDecoder(r.Body)
	d.Decode(&invokeRequest)

	var reqData map[string]interface{}
	json.Unmarshal(invokeRequest.Data["req"], &reqData)

	outputs := make(map[string]interface{})
	outputs["message"] = reqData["Body"]

	resData := make(map[string]interface{})
	resData["body"] = "Order enqueued"
	outputs["res"] = resData
	invokeResponse := InvokeResponse{outputs, nil, nil}

	responseJson, _ := json.Marshal(invokeResponse)

	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJson)
}

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
