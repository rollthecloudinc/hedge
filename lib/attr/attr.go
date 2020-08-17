package attr

import "strconv"

type AttributeTypes int32

const (
	Number AttributeTypes = iota
	Text
	Complex
	Float
	Array
	Bool
)

type AttributeValue struct {
	Name          string           `form:"name" json:"name" binding:"required"`
	DisplayName   string           `form:"displayName" json:"displayName" binding:"required"`
	Type          AttributeTypes   `form:"type" json:"type" binding:"required"`
	Value         string           `form:"value" json:"value" binding:"required"`
	IntValue      int32            `json:"intValue"`
	FloatValue    float64          `json:"floatValue"`
	ComputedValue string           `form:"computedValue" json:"computedValue" binding:"required"`
	Attributes    []AttributeValue `form:"attributes[]" json:"attributes"`
}

func FlattenAttributeValue(value AttributeValue) []AttributeValue {
	leafNodes := make([]AttributeValue, 0)
	if value.Attributes == nil || len(value.Attributes) == 0 {
		leafNodes = append(leafNodes, value)
	} else {
		for _, attr := range value.Attributes {
			flatChildren := FlattenAttributeValue(attr)
			for _, flatChild := range flatChildren {
				leafNodes = append(leafNodes, flatChild)
			}
		}
	}
	return leafNodes
}

func FinalizeAttributeValue(value *AttributeValue) {
	if value.Type == Number {
		computedValue, _ := strconv.ParseInt(value.ComputedValue, 10, 32)
		value.IntValue = int32(computedValue)
	} else if value.Type == Float {
		computedValue, _ := strconv.ParseFloat(value.ComputedValue, 64)
		value.FloatValue = computedValue
	} else {
		value.IntValue = 0
	}
}
