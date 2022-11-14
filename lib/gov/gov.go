package gov

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
