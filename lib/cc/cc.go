package cc

import (
	"encoding/json"
	"goclassifieds/lib/attr"
	"time"
)

type Page struct {
	Site      string    `form:"site" json:"site" binding:"required" validate:"required"`
	Path      string    `form:"path" json:"path" binding:"required" validate:"required"`
	Title     string    `form:"title" json:"title" binding:"required" validate:"required"`
	Body      string    `form:"body" json:"body" binding:"required" validate:"required"`
	Published bool      `form:"published" json:"published" binding:"required" validate:"required"`
	CreatedAt time.Time `form:"createdat" json:"createdat" binding:"required" validate:"required"`
}

type Layout struct {
	Id        string     `form:"id" json:"id" binding:"required" validate:"required"`
	Site      string     `form:"site" json:"site" binding:"required" validate:"required"`
	GridItems []GridItem `form:"gridItems[]" json:"gridItems" binding:"required" validate:"required,dive"`
}

type GridItem struct {
	Rows   *int `form:"rows" json:"rows" binding:"required" validate:"required"`
	Cols   *int `form:"cols" json:"cols" binding:"required" validate:"required"`
	X      *int `form:"x" json:"x" binding:"required" validate:"required"`
	Y      *int `form:"y" json:"y" binding:"required" validate:"required"`
	Weight *int `form:"weight" json:"weight" binding:"required" validate:"required"`
}

type Panel struct {
	Panes []Pane `form:"panes[]" json:"panes" validate:"dive"`
}

type Pane struct {
	ContentProvider string                `form:"contentProvider" json:"contentProvider" binding:"required" validate:"required"`
	Settings        []attr.AttributeValue `form:"settings[]" json:"settings" validate:"dive"`
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

func ToLayoutEntity(layout *Layout) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(layout)
	if err != nil {
		return nil, err
	}
	var entity map[string]interface{}
	err = json.Unmarshal(jsonData, &entity)
	return entity, nil
}
