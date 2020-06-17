package ads

import (
	"bytes"
	"encoding/json"
	"goclassifieds/lib/attr"
	"goclassifieds/lib/vocab"
	"log"
	"time"
)

type AdStatuses int32

const (
	Submitted AdStatuses = iota
	Approved
	Rejected
	Expired
	Deleted
)

type AdListitemsRequest struct {
	TypeId       string   `form:"typeId" binding:"required"`
	SearchString string   `form:"searchString"`
	Location     string   `form:"location"`
	Features     []string `form:"features[]"`
	Page         int      `form:"page"`
}

type Ad struct {
	Id          string                `json:"id" validate:"required"`
	TypeId      string                `form:"typeId" json:"typeId" binding:"typeId" validate:"required"`
	Status      *AdStatuses           `form:"status" json:"status" validate:"required"`
	Title       string                `form:"title" json:"title" binding:"required" validate:"required"`
	Description string                `form:"description" json:"description" binding:"required" validate:"required"`
	Location    [2]float64            `form:"location[]" json:"location" binding:"required" validate:"required"`
	UserId      string                `form:"userId" json:"userId" validate:"required"`
	ProfileId   string                `form:"profileId" json:"profileId"`
	CityDisplay string                `form:"cityDisplay" json:"cityDisplay" binding:"required" validate:"required"`
	Images      []AdImage             `form:"images[]" json:"images" validate:"dive"`
	Attributes  []attr.AttributeValue `form:"attributes[]" json:"attributes" validate:"dive"`
	FeatureSets []vocab.Vocabulary    `form:"featureSets[]" json:"featureSets" validate:"dive"`
}

type AdImage struct {
	Id     string `form:"id" json:"id" binding:"required" validate:"required"`
	Path   string `form:"path" json:"path" binding:"required" validate:"required"`
	Weight int    `form:"weight" json:"weight" binding:"required" validate:"required"`
}

type AdLead struct {
	ProfileId string    `form:"profileId" json:"profileId" binding:"required" validate:"required"`
	AdId      string    `form:"adId" json:"adId" binding:"required" validate:"required"`
	SenderId  string    `form:"senderId" json:"senderId"`
	Email     string    `form:"email" json:"email" binding:"required" validate:"required,email"`
	Phone     string    `form:"phone" json:"phone" binding:"required" validate:"required"`
	Message   string    `form:"message" json:"message" binding:"required" validate:"required"`
	CreatedAt time.Time `form:"createdAt" json:"createdAt" binding:"required" validate:"required"`
}

func ToEntity(ad *Ad) (map[string]interface{}, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(ad); err != nil {
		log.Fatalf("Error encoding query: %s", err)
	}
	jsonData, err := json.Marshal(ad)
	if err != nil {
		return nil, err
	}
	var entity map[string]interface{}
	err = json.Unmarshal(jsonData, &entity)
	return entity, nil
}

func ToLeadEntity(lead *AdLead) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(lead)
	if err != nil {
		return nil, err
	}
	var entity map[string]interface{}
	err = json.Unmarshal(jsonData, &entity)
	return entity, nil
}
