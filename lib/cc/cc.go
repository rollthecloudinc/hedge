package cc

import (
	"encoding/json"
	"time"
)

type Page struct {
	Site    string    `form:"site" json:"site" binding:"required" validate:"required"`
	Path    string    `form:"path" json:"path" binding:"required" validate:"required"`
	Title    string    `form:"title" json:"title" binding:"required" validate:"required"`
	Body    string    `form:"body" json:"body" binding:"required" validate:"required"`
	Published    bool    `form:"published" json:"published" binding:"required" validate:"required"`
	CreatedAt    time.Time    `form:"createdat" json:"createdat" binding:"required" validate:"required"`
}

func ToPageEntity(page *Page) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(page)
	if err != nil {
		return nil, err
	}
	var entity map[string]interface{}
	err = json.Unmarshal(jsonData, &entity)
	return entity, nil
}