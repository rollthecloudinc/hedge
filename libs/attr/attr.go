package attr

type AttributeTypes int32

const (
	Number AttributeTypes = iota
	Text
	Complex	
)

type AttributeValue struct {
	Name 				string `form:"name" binding:"required"`
	DisplayName string `form:"displayName" binding:"required"`
	Type AttributeTypes `form:"type" binding:"required"`
	Value string `form:"value" binding:"required"`
	ComputedValue string `form:"computedValue" binding:"required"`
	Attributes []AttributeValue `form:"attributes[]"`
}