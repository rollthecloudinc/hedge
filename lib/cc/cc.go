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

type PanelPage struct {
	Id           string          `form:"id" json:"id" binding:"required" validate:"required"`
	Name         string          `form:"name" json:"name"`
	Title        string          `form:"title" json:"title"`
	Path         string          `form:"path" json:"path"`
	DisplayType  string          `form:"displayType" json:"displayType" binding:"required" validate:"required"`
	DerivativeId string          `form:"derivativeId" json:"derivativeId"`
	LayoutType   string          `form:"layoutType" json:"layoutType" binding:"required" validate:"required"`
	Contexts     []InlineContext `form:"contexts[]" json:"contexts" validate:"dive"`
	GridItems    []GridItem      `form:"gridItems[]" json:"gridItems" binding:"required" validate:"required,dive"`
	Panels       []Panel         `form:"panels[]" json:"panels" binding:"required" validate:"required,dive"`
}

type GridLayout struct {
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
	Name        string                `form:"name" json:"name"`
	Label       string                `form:"label" json:"label"`
	StylePlugin string                `form:"stylePlugin" json:"stylePlugin"`
	Settings    []attr.AttributeValue `form:"settings[]" json:"settings" validate:"dive"`
	Panes       []Pane                `form:"panes[]" json:"panes" validate:"dive"`
}

type Pane struct {
	Name          string                `form:"name" json:"name"`
	Label         string                `form:"label" json:"label"`
	Locked        *bool                 `form:"locked" json:"locked" binding:"required" validate:"required"`
	ContentPlugin string                `form:"contentPlugin" json:"contentPlugin" binding:"required" validate:"required"`
	LinkedPageId  string                `form:"linkedPageId" json:"linkedPageId"`
	Settings      []attr.AttributeValue `form:"settings[]" json:"settings" validate:"dive"`
	Rule          Rule                  `form:"rule" json:"rule" validate:"dive"`
}

type Rule struct {
	Condition string `form:"condition" json:"condition"`
	Field     string `form:"field" json:"field"`
	Value     string `form:"value" json:"value"`
	Operator  string `form:"operator" json:"operator"`
	Rules     []Rule `form:"rules" json:"rules" validate:"dive"`
}

type InlineContext struct {
	Name    string                  `form:"name" json:"name" binding:"required" validate:"required"`
	Adaptor string                  `form:"adaptor" json:"adaptor" binding:"required" validate:"required"`
	Plugin  string                  `form:"plugin" json:"plugin" binding:"required" validate:"required"`
	Rest    *Rest                   `form:"rest" json:"rest" binding:"omitempty" validate:"omitempty"`
	Snippet *Snippet                `form:"snippet" json:"snippet" binding:"omitempty" validate:"omitempty"`
	Data    *interface{}            `form:"data" json:"data" binding:"omitempty" validate:"omitempty"`
	Tokens  *map[string]interface{} `form:"tokens" json:"tokens" binding:"omitempty" validate:"omitempty"`
}

type Rest struct {
	Url string `form:"url" json:"url" binding:"required" validate:"required"`
	// Renderer RestRenderer `form:"renderer" json:"renderer" binding:"required" validate:"required"`
	Params []RestParam `form:"params[]" json:"params" binding:"required" validate:"required,dive"`
}

/*type RestRenderer struct {
	Type string  `form:"type" json:"type" binding:"required" validate:"required"`
	Data Snippet `form:"data" json:"data" validate:"dive"`
}*/

type RestParam struct {
	Mapping RestMapping `form:"mapping" json:"mapping" binding:"required" validate:"required,dive"`
	Flags   []RestFlag  `form:"flags[]" json:"flags" validate:"dive"`
}

type RestMapping struct {
	Type      string `form:"type" json:"type" binding:"required" validate:"required"`
	Value     string `form:"value" json:"value" binding:"required" validate:"required"`
	Context   string `form:"context" json:"context"`
	TestValue string `form:"testValue" json:"testValue"`
}

type RestFlag struct {
	Name    string `form:"name" json:"name" binding:"required" validate:"required"`
	Enabled *bool  `form:"enabled" json:"enabled" binding:"required" validate:"required"`
}

type Snippet struct {
	ContentType string `form:"contentType" json:"contentType" binding:"required" validate:"required"`
	Content     string `form:"content" json:"content" binding:"required" validate:"required"`
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

func ToPanelPageEntity(page *PanelPage) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(page)
	if err != nil {
		return nil, err
	}
	var entity map[string]interface{}
	err = json.Unmarshal(jsonData, &entity)
	return entity, nil
}

func ToGridLayoutEntity(layout *GridLayout) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(layout)
	if err != nil {
		return nil, err
	}
	var entity map[string]interface{}
	err = json.Unmarshal(jsonData, &entity)
	return entity, nil
}
