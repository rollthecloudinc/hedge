package main

import (
	"crypto/tls"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/gocql/gocql"
)

func Handler(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	return events.APIGatewayProxyResponse{StatusCode: 200}, nil
}

func init() {
	log.Printf("chat start")

	cluster := gocql.NewCluster("cassandra.us-east-1.amazonaws.com")
	cluster.Keyspace = "ClassifiedsDev"
	cluster.Port = 9142
	cluster.Consistency = gocql.LocalQuorum
	cluster.Authenticator = &gocql.PasswordAuthenticator{Username: "tzmijewski-at-989992233821", Password: "oALqeCqjS3BgyiBp2Ram8kTUbhttAYoyUoL70hmz+tY="}
	cluster.SslOpts = &gocql.SslOptions{Config: &tls.Config{ServerName: "cassandra.us-east-1.amazonaws.com"}, CaPath: "api/chat/AmazonRootCA1.pem", EnableHostVerification: true}
	cluster.PoolConfig = gocql.PoolConfig{HostSelectionPolicy: /*gocql.TokenAwareHostPolicy(*/ gocql.DCAwareRoundRobinPolicy("us-east-1") /*)*/}
	_, err := cluster.CreateSession()
	if err != nil {
		log.Fatal(err)
	}

	log.Print("chat started")
}

func main() {
	lambda.Start(Handler)
}
