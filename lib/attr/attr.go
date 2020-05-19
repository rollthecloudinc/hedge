package attr

type AttributeTypes int32

const (
	Number AttributeTypes = iota
	Text
	Complex
)

type AttributeValue struct {
	Name          string           `form:"name" json:"name" binding:"required"`
	DisplayName   string           `form:"displayName" json:"displayName" binding:"required"`
	Type          AttributeTypes   `form:"type" json:"type" binding:"required"`
	Value         string           `form:"value" json:"value" binding:"required"`
	ComputedValue string           `form:"computedValue" json:"computedValue" binding:"required"`
	Attributes    []AttributeValue `form:"attributes[]" json:"attributes"`
}
