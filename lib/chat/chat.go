package chat

import (
	"bytes"
	"encoding/json"
	"log"
	"time"
)

type ChatConnection struct {
	ConnId    string    `form:"connId" json:"connId" binding:"required" validate:"required"`
	CreatedAt time.Time `form:"createdAt" json:"createdAt" binding:"required" validate:"required"`
	UserId    string    `form:"userId" json:"userId" binding:"required" validate:"required"`
}

func ToConnectionEntity(conn *ChatConnection) (map[string]interface{}, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(conn); err != nil {
		log.Fatalf("Error encoding query: %s", err)
	}
	jsonData, err := json.Marshal(conn)
	if err != nil {
		return nil, err
	}
	var entity map[string]interface{}
	err = json.Unmarshal(jsonData, &entity)
	return entity, nil
}
