package profiles

import (
	"bytes"
	"encoding/json"
	"log"
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
	UserId            string             `json:"userId" validate:"required"`
	Title             string             `form:"title" json:"title" validate:"required"`
	Status            *ProfileStatuses   `form:"status" json:"status" validate:"required"`
	Type              *ProfileTypes      `form:"type" json:"type" validate:"required"`
	Subtype           *ProfileSubtypes   `form:"subtype" json:"subtype" validate:"required"`
	Adspace           *AdTypes           `form:"adspace" json:"adspace" validate:"required"`
	FirstName         string             `form:"firstName" json:"firstName"`
	LastName          string             `form:"lastName" json:"lastName"`
	MiddleName        string             `form:"middleName" json:"middleName"`
	PreferredName     string             `form:"preferredName" json:"preferredName"`
	CompanyName       string             `form:"companyName" json:"companyName"`
	Email             string             `form:"email" json:"email"`
	Introduction      string             `form:"introduction" json:"introduction"`
	Logo              *ProfileImage      `form:"logo,omitempty" json:"logo,omitempty" binding:"omitempty" validate:"omitempty"`
	Headshot          *ProfileImage      `form:"headshot,omitempty" json:"headshot,omitempty" binding:"omitempty" validate:"omitempty"`
	PhoneNumbers      []PhoneNumber      `form:"phoneNumbers[]" json:"phoneNumbers" validate:"dive"`
	Locations         []Location         `form:"locations[]" json:"locations" validate:"dive"`
	EntityPermissions ProfilePermissions `json:"entityPermissions" validate:"required, dive"`
}

type ProfileImage struct {
	Id     string `form:"id" json:"id" binding:"required" validate:"required"`
	Path   string `form:"path" json:"path" binding:"required" validate:"required"`
	Weight int    `form:"weight" json:"weight" binding:"required" validate:"required"`
}

type PhoneNumber struct {
	Type  *PhoneNumberTypes `form:"type" json:"type" binding:"required" validate:"required"`
	Value string            `form:"value" json:"value" binding:"required" validate:"required"`
}

type Location struct {
	Title        string         `form:"title" json:"title" binding:"required" validate:"required"`
	Type         *LocationTypes `form:"type" json:"type" binding:"required" validate:"required"`
	Address      Address        `form:"address" json:"address" binding:"required" validate:"required"`
	PhoneNumbers []PhoneNumber  `form:"phoneNumbers[]" json:"phoneNumbers" validate:"required"`
}

type Address struct {
	Street1 string `form:"street1" json:"street1" binding:"required" validate:"required"`
	Street2 string `form:"street2" json:"street2"`
	Street3 string `form:"street3" json:"street3"`
	City    string `form:"city" json:"city" binding:"required" validate:"required"`
	State   string `form:"state" json:"state" binding:"required" validate:"required"`
	Zip     string `form:"zip" json:"zip" binding:"required" validate:"required"`
	Country string `form:"country" json:"country" binding:"required" validate:"required"`
}

type ProfileNavItem struct {
	Id       string `json:"id" binding:"required"`
	ParentId string `json:"parentId"`
	Title    string `json:"title" binding:"required"`
}

type ProfilePermissions struct {
	ReadUserIds   []string `json:"readUserIds"`
	WriteUserIds  []string `json:"writeUserIds"`
	DeleteUserIds []string `json:"deleteUserIds"`
}

type ProfileListItemsQuery struct {
	ParentId string
	UserId   string
}

type ProfileNavItemsQuery1 struct {
	UserId string
}

type ProfileNavItemsQuery2 struct {
	Ids  []string
	Last int
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
