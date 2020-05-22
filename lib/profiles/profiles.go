package profiles

import (
	"bytes"
	"encoding/json"
	"goclassifieds/lib/entity"
	"log"

	"github.com/aws/aws-sdk-go/aws/session"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
)

type ProfileStatuses int32

const (
	Submitted ProfileStatuses = iota
	Approved
	Rejected
	Deleted
)

type ProfileTypes int32

const (
	Person ProfileTypes = iota
	Company
	Shop
)

type ProfileSubtypes int32

const (
	Agent = iota
	Broker
	Dealer
	Seller
)

type AdTypes int32

const (
	General AdTypes = iota
	RealEstate
	Rental
	Auto
	Job
)

type PhoneNumberTypes int32

const (
	Email PhoneNumberTypes = iota
	Fax
)

type LocationTypes int32

const (
	Home LocationTypes = iota
	Office
)

type Profile struct {
	Id                string             `json:"id"`
	ParentId          string             `form:"parentId" json:"parentId"`
	UserId            string             `json:"userId"`
	Title             string             `form:"title" json:"title"`
	Status            ProfileStatuses    `form:"status" json:"status"`
	Type              ProfileTypes       `form:"type" json:"type"`
	Subtype           ProfileSubtypes    `form:"subtype" json:"subtype"`
	Adspace           AdTypes            `form:"adspace" json:"adspace"`
	FirstName         string             `form:"firstName" json:"firstName"`
	LastName          string             `form:"lastName" json:"lastName"`
	MiddleName        string             `form:"middleName" json:"middleName"`
	PreferredName     string             `form:"preferredName" json:"preferredName"`
	CompanyName       string             `form:"companyName" json:"companyName"`
	Email             string             `form:"email" json:"email"`
	Introduction      string             `form:"introduction" json:"introduction"`
	Logo              ProfileImage       `form:"logo" json:"logo"`
	Headshot          ProfileImage       `form:"headshot" json:"headshot"`
	PhoneNumbers      []PhoneNumber      `form:"phoneNumbers[]" json:"phoneNumbers"`
	Locations         []Location         `form:"locations[]" json:"locations"`
	EntityPermissions ProfilePermissions `json:"entityPermissions"`
}

type ProfileImage struct {
	Id     string `form:"id" json:"id" binding:"required"`
	Path   string `form:"path" json:"path" binding:"required"`
	Weight int    `form:"weight" json:"weight" binding:"required"`
}

type PhoneNumber struct {
	Type  PhoneNumberTypes `form:"type" json:"type" binding:"required"`
	Value string           `form:"value" json:"value" binding:"required"`
}

type Location struct {
	Title        string        `form:"title" json:"title" binding:"required"`
	Type         LocationTypes `form:"type" json:"type" binding:"required"`
	Address      Address       `form:"address" json:"address" binding:"required"`
	PhoneNumbers []PhoneNumber `form:"phoneNumbers[]" json:"phoneNumbers"`
}

type Address struct {
	Street1 string `form:"street1" json:"street1" binding:"required"`
	Street2 string `form:"street2" json:"street2"`
	Street3 string `form:"street3" json:"street3"`
	City    string `form:"city" json:"city" binding:"required"`
	State   string `form:"state" json:"state" binding:"required"`
	Zip     string `form:"zip" json:"zip" binding:"required"`
	Country string `form:"country" json:"country" binding:"required"`
}

type ProfilePermissions struct {
	ReadUserIds   []string `json:"readUserIds"`
	WriteUserIds  []string `json:"writeUserIds"`
	DeleteUserIds []string `json:"deleteUserIds"`
}

func CreateProfileManager(esClient *elasticsearch7.Client, session *session.Session) entity.EntityManager {
	return entity.EntityManager{
		Config: entity.EntityConfig{
			SingularName: "profile",
			PluralName:   "profiles",
			IdKey:        "id",
		},
		Loaders: map[string]entity.Loader{
			"s3": entity.S3LoaderAdaptor{
				Config: entity.S3AdaptorConfig{
					Session: session,
					Bucket:  "classifieds-ui-dev",
					Prefix:  "profiles/",
				},
			},
		},
		Storages: map[string]entity.Storage{
			"s3": entity.S3StorageAdaptor{
				Config: entity.S3AdaptorConfig{
					Session: session,
					Bucket:  "classifieds-ui-dev",
					Prefix:  "profiles/",
				},
			},
			"elastic": entity.ElasticStorageAdaptor{
				Config: entity.ElasticAdaptorConfig{
					Index:  "classified_profiles",
					Client: esClient,
				},
			},
		},
	}
}

func ToEntity(profile *Profile) (map[string]interface{}, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(profile); err != nil {
		log.Fatalf("Error encoding query: %s", err)
	}
	jsonData, err := json.Marshal(profile)
	if err != nil {
		return nil, err
	}
	var entity map[string]interface{}
	err = json.Unmarshal(jsonData, &entity)
	return entity, nil
}
