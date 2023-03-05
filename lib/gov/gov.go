package gov

import (
	"goclassifieds/lib/utils"
	"text/template"

	"github.com/gocql/gocql"
)

const (
	User ResourceUserTypes = iota
	Site
)

var UserTypeMap = map[string]ResourceUserTypes{
	"user": User,
	"site": Site,
}

type ResourceUserTypes int32

const (
	GithubRepo ResourceTypes = iota
	DruidSite
	CognitoUserPool
	CassandraTable
)

var ResourceTypeMap = map[string]ResourceTypes{
	"githubrepo":      GithubRepo,
	"druidsite":       DruidSite,
	"cognitouserpool": CognitoUserPool,
	"cassandratable":  CassandraTable,
}

type ResourceTypes int32

const (
	Read ResourceOperations = iota
	Write
	Delete
)

var OperationMap = map[string]ResourceOperations{
	"read":   Read,
	"write":  Write,
	"delete": Delete,
}

type ResourceOperations int32

type GrantAccessRequest struct {
	User                string
	Type                ResourceUserTypes
	Resource            ResourceTypes
	Asset               string
	Operation           ResourceOperations
	AdditionalResources []Resource
	LogUsageLambdaInput *utils.LogUsageLambdaInput
}

type Resource struct {
	User      string
	Type      ResourceUserTypes
	Resource  ResourceTypes
	Asset     string
	Operation ResourceOperations
}

type GrantAccessResponse struct {
	Grant bool `json:"grant"`
}

type ResourceManagerParams struct {
	Session  *gocql.Session
	Request  *GrantAccessRequest
	Template *template.Template
	// Resource  string
	// Operation string
}

func Query() string {
	return `
	{{ define "grant_access" }}
	SELECT
       op
	 FROM
			 resources2
	WHERE 
			 user = {{ bindValue (index .Metadata "user" ) }}
		 AND
		   type = {{ bindValue (index .Metadata "type" ) }}
		 AND
		   resource = {{ bindValue (index .Metadata "resource" ) }}
		 AND
		   asset = {{ bindValue (index .Metadata "asset" ) }}
		 AND
		   op = {{ bindValue (index .Metadata "op" ) }}
	{{end}}
	`
}
