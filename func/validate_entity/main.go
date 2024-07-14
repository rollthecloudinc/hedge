package main

import (
	"context"
	"encoding/json"
	"errors"
	"goclassifieds/lib/ads"
	"goclassifieds/lib/cc"
	"goclassifieds/lib/chat"
	"goclassifieds/lib/entity"
	"goclassifieds/lib/profiles"
	"goclassifieds/lib/utils"
	"goclassifieds/lib/vocab"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/go-playground/validator/v10"
)

func handler(ctx context.Context, payload *entity.ValidateEntityRequest) (entity.ValidateEntityResponse, error) {

	utils.LogUsageForLambdaWithInput(payload.LogUsageLambdaInput)

	invalid := entity.ValidateEntityResponse{
		Entity:       payload.Entity,
		Valid:        false,
		Unauthorized: true,
	}

	if payload.EntityName != "shapeshifter" && payload.UserId == "" {
		log.Print("inside not lead and user id empty conditional")
		return invalid, errors.New("Unauthorized to create entity")
	}

	log.Printf("entity is %s", payload.EntityName)

	invalid.Unauthorized = false

	jsonData, err := json.Marshal(payload.Entity)
	if err != nil {
		return invalid, err
	}

	var newEntity map[string]interface{}
	if payload.EntityName == "ad" {
		newEntity, err = ValidateAd(jsonData, payload)
	} else if payload.EntityName == "vocabulary" {
		newEntity, err = ValidateVocabulary(jsonData, payload)
	} else if payload.EntityName == "profile" {
		newEntity, err = ValidateProfile(jsonData, payload)
	} else if payload.EntityName == "chatconnection" {
		newEntity, err = ValidateChatConnection(jsonData, payload)
	} else if payload.EntityName == "chatconversation" {
		newEntity, err = ValidateChatConversation(jsonData, payload)
	} else if payload.EntityName == "chatmessage" {
		newEntity, err = ValidateChatMessage(jsonData, payload)
	} else if payload.EntityName == "lead" {
		newEntity, err = ValidateLead(jsonData, payload)
	} else if payload.EntityName == "page" {
		newEntity, err = ValidatePage(jsonData, payload)
	} else if payload.EntityName == "panelpage" {
		newEntity, err = ValidatePanelPage(jsonData, payload)
	} else if payload.EntityName == "gridlayout" {
		newEntity, err = ValidateGridLayout(jsonData, payload)
	} else if payload.EntityName == "shapeshifter" {
		newEntity, err = ValidateShapeshifter(jsonData, payload)
	} else {
		newEntity = payload.Entity
		// This needs to be commented out to allow shapeshifters though for now.
		// return invalid, errors.New("Entity validation does exist")
	}

	log.Printf("after validation")

	if err != nil {
		return invalid, err
	}

	return entity.ValidateEntityResponse{
		Entity:       newEntity,
		Valid:        true,
		Unauthorized: false,
	}, nil
}

func ValidateAd(jsonData []byte, payload *entity.ValidateEntityRequest) (map[string]interface{}, error) {
	var deadObject map[string]interface{}

	log.Printf("Inside ValidateAd")

	var obj ads.Ad
	err := json.Unmarshal(jsonData, &obj)
	if err != nil {
		return deadObject, err
	}

	submitted := ads.Submitted

	obj.Id = utils.GenerateId()
	obj.Status = &submitted // @todo: Enums not being validated :(
	obj.UserId = payload.UserId

	validate := validator.New()
	err = validate.Struct(obj)

	if err != nil {
		msg, _ := json.Marshal(err.(validator.ValidationErrors))
		log.Printf("Validation Errors: %s", string(msg))
		return deadObject, err.(validator.ValidationErrors)
	}

	newEntity, _ := ads.ToEntity(&obj)
	return newEntity, nil
}

func ValidateVocabulary(jsonData []byte, payload *entity.ValidateEntityRequest) (map[string]interface{}, error) {
	var deadObject map[string]interface{}

	var obj vocab.Vocabulary
	err := json.Unmarshal(jsonData, &obj)
	if err != nil {
		return deadObject, err
	}

	obj.Id = utils.GenerateId()
	obj.UserId = payload.UserId

	validate := validator.New()
	err = validate.Struct(obj)
	if err != nil {
		return deadObject, err.(validator.ValidationErrors)
	}

	newEntity, _ := vocab.ToEntity(&obj)
	return newEntity, nil
}

func ValidateProfile(jsonData []byte, payload *entity.ValidateEntityRequest) (map[string]interface{}, error) {
	var deadObject map[string]interface{}

	var obj profiles.Profile
	err := json.Unmarshal(jsonData, &obj)
	if err != nil {
		return deadObject, err
	}

	submitted := profiles.Submitted

	obj.Id = utils.GenerateId()
	obj.Status = &submitted // @todo: Enums not being validated :(
	obj.UserId = payload.UserId
	obj.EntityPermissions = profiles.ProfilePermissions{
		ReadUserIds:   []string{obj.UserId},
		WriteUserIds:  []string{obj.UserId},
		DeleteUserIds: []string{obj.UserId},
	}

	validate := validator.New()
	err = validate.Struct(obj)
	if err != nil {
		return deadObject, err.(validator.ValidationErrors)
	}

	newEntity, _ := profiles.ToEntity(&obj)
	return newEntity, nil
}

func ValidateLead(jsonData []byte, payload *entity.ValidateEntityRequest) (map[string]interface{}, error) {
	var deadObject map[string]interface{}

	log.Printf("Inside ValidateLead")

	var obj ads.AdLead
	err := json.Unmarshal(jsonData, &obj)
	if err != nil {
		return deadObject, err
	}

	obj.CreatedAt = time.Now()
	// @todo: validate profile and ad exists - perhaps create custom validator.
	obj.SenderId = payload.UserId

	validate := validator.New()
	err = validate.Struct(obj)

	if err != nil {
		msg, _ := json.Marshal(err.(validator.ValidationErrors))
		log.Printf("Validation Errors: %s", string(msg))
		return deadObject, err.(validator.ValidationErrors)
	}

	newEntity, _ := ads.ToLeadEntity(&obj)
	return newEntity, nil
}

func ValidateChatConnection(jsonData []byte, payload *entity.ValidateEntityRequest) (map[string]interface{}, error) {
	var deadObject map[string]interface{}

	var obj chat.ChatConnection
	err := json.Unmarshal(jsonData, &obj)
	if err != nil {
		return deadObject, err
	}

	obj.CreatedAt = time.Now()
	obj.UserId = payload.UserId

	validate := validator.New()
	err = validate.Struct(obj)
	if err != nil {
		return deadObject, err.(validator.ValidationErrors)
	}

	newEntity, _ := chat.ToConnectionEntity(&obj)
	return newEntity, nil
}

func ValidateChatConversation(jsonData []byte, payload *entity.ValidateEntityRequest) (map[string]interface{}, error) {
	var deadObject map[string]interface{}

	var obj chat.ChatConversation
	err := json.Unmarshal(jsonData, &obj)
	if err != nil {
		return deadObject, err
	}

	obj.UserId = payload.UserId

	validate := validator.New()
	err = validate.Struct(obj)
	if err != nil {
		return deadObject, err.(validator.ValidationErrors)
	}

	newEntity, _ := chat.ToConversationEntity(&obj)
	return newEntity, nil
}

func ValidateChatMessage(jsonData []byte, payload *entity.ValidateEntityRequest) (map[string]interface{}, error) {
	var deadObject map[string]interface{}

	var obj chat.ChatMessage
	err := json.Unmarshal(jsonData, &obj)
	if err != nil {
		return deadObject, err
	}

	obj.SenderId = payload.UserId
	obj.CreatedAt = time.Now()

	validate := validator.New()
	err = validate.Struct(obj)
	if err != nil {
		return deadObject, err.(validator.ValidationErrors)
	}

	newEntity, _ := chat.ToMessageEntity(&obj)
	return newEntity, nil
}

func ValidatePage(jsonData []byte, payload *entity.ValidateEntityRequest) (map[string]interface{}, error) {
	var deadObject map[string]interface{}

	log.Printf("Inside ValidatePage")

	var obj cc.Page
	err := json.Unmarshal(jsonData, &obj)
	if err != nil {
		return deadObject, err
	}

	obj.CreatedAt = time.Now()

	validate := validator.New()
	err = validate.Struct(obj)

	if err != nil {
		msg, _ := json.Marshal(err.(validator.ValidationErrors))
		log.Printf("Validation Errors: %s", string(msg))
		return deadObject, err.(validator.ValidationErrors)
	}

	newEntity, _ := cc.ToPageEntity(&obj)
	return newEntity, nil
}

func ValidatePanelPage(jsonData []byte, payload *entity.ValidateEntityRequest) (map[string]interface{}, error) {
	var deadObject map[string]interface{}

	log.Printf("Inside ValidatePanelPage")

	var obj cc.PanelPage
	err := json.Unmarshal(jsonData, &obj)
	if err != nil {
		return deadObject, err
	}

	if obj.Id == "" {
		obj.Id = utils.GenerateId()
	}

	obj.UserId = payload.UserId

	readUserId := obj.UserId
	for _, userId := range obj.EntityPermissions.ReadUserIds {
		if userId == "*" {
			readUserId = userId
		}
	}

	obj.EntityPermissions = cc.PanelPagePermissions{
		ReadUserIds:   []string{readUserId},
		WriteUserIds:  []string{obj.UserId},
		DeleteUserIds: []string{obj.UserId},
	}

	validate := validator.New()
	err = validate.Struct(obj)

	if err != nil {
		msg, _ := json.Marshal(err.(validator.ValidationErrors))
		log.Printf("Validation Errors: %s", string(msg))
		return deadObject, err.(validator.ValidationErrors)
	}

	newEntity, _ := cc.ToPanelPageEntity(&obj)
	return newEntity, nil
}

func ValidateGridLayout(jsonData []byte, payload *entity.ValidateEntityRequest) (map[string]interface{}, error) {
	var deadObject map[string]interface{}

	log.Printf("Inside ValidateLayout")

	var obj cc.GridLayout
	err := json.Unmarshal(jsonData, &obj)
	if err != nil {
		return deadObject, err
	}

	obj.Id = utils.GenerateId()

	validate := validator.New()
	err = validate.Struct(obj)

	if err != nil {
		msg, _ := json.Marshal(err.(validator.ValidationErrors))
		log.Printf("Validation Errors: %s", string(msg))
		return deadObject, err.(validator.ValidationErrors)
	}

	newEntity, _ := cc.ToGridLayoutEntity(&obj)
	return newEntity, nil
}

func main() {
	log.SetFlags(0)
	// Make the handler available for Remote Procedure Call by AWS Lambda
	lambda.Start(handler)
}

func ValidateShapeshifter(jsonData []byte, payload *entity.ValidateEntityRequest) (map[string]interface{}, error) {
	var deadObject map[string]interface{}

	log.Printf("Inside ValidateShapeshifter")

	var obj map[string]interface{}
	err := json.Unmarshal(jsonData, &obj)
	if err != nil {
		return deadObject, err
	}

	/*if obj["id"] == "" {
		obj["id"] = utils.GenerateId()
	}*/

	obj["userId"] = payload.UserId

	/*readUserId := obj.UserId
	for _, userId := range obj.EntityPermissions.ReadUserIds {
		if userId == "*" {
			readUserId = userId
		}
	}*/

	/*obj.EntityPermissions = cc.PanelPagePermissions{
		ReadUserIds:   []string{readUserId},
		WriteUserIds:  []string{obj.UserId},
		DeleteUserIds: []string{obj.UserId},
	}*/

	/*validate := validator.New()
	err = validate.Struct(obj)*/

	/*if err != nil {
		msg, _ := json.Marshal(err.(validator.ValidationErrors))
		log.Printf("Validation Errors: %s", string(msg))
		return deadObject, err.(validator.ValidationErrors)
	}*/

	return obj, nil
}
