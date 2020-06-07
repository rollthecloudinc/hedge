package vocab

import (
	"bytes"
	"encoding/json"
	"log"
)

type Vocabulary struct {
	Id          string `form:"id" json:"id"`
	UserId      string `form:"userId" json:"userId"`
	MachineName string `form:"machineName" json:"machineName" binding:"required" validate:"required"`
	HumanName   string `form:"humanName" json:"humanName" binding:"required" validate:"required"`
	Terms       []Term `form:"terms[]" json:"terms" binding:"required" validate:"required"`
}

type Term struct {
	Id           string `form:"id" json:"id" binding:"required" validate:"required"`
	VocabularyId string `form:"vocabularyId" json:"vocabularyId" binding:"required" validate:"required"`
	ParentId     string `form:"parentId" json:"parentId"`
	MachineName  string `form:"machineName" json:"machineName" binding:"required" validate:"required"`
	HumanName    string `form:"humanName" json:"humanName" binding:"required" validate:"required"`
	Weight       int    `form:"weight" json:"weight" binding:"required" validate:"required"`
	Group        bool   `form:"group" json:"group" binding:"required" validate:"required"`
	Selected     bool   `form:"selected" json:"selected" binding:"required" validate:"required"`
	Level        int    `form:"level" json:"level" binding:"required" validate:"required"`
	Children     []Term `form:"children[]" json:"children"`
}

func ToEntity(vocab *Vocabulary) (map[string]interface{}, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(vocab); err != nil {
		log.Fatalf("Error encoding query: %s", err)
	}
	jsonData, err := json.Marshal(vocab)
	if err != nil {
		return nil, err
	}
	var entity map[string]interface{}
	err = json.Unmarshal(jsonData, &entity)
	return entity, nil
}

func FlattenTerm(term Term, selectedOnly bool) []Term {
	leafNodes := make([]Term, 0)
	if term.Children == nil || len(term.Children) == 0 {
		if !selectedOnly || term.Selected {
			leafNodes = append(leafNodes, term)
		}
	} else {
		for _, childTerm := range term.Children {
			flatChildren := FlattenTerm(childTerm, selectedOnly)
			for _, flatChild := range flatChildren {
				leafNodes = append(leafNodes, flatChild)
			}
		}
	}
	return leafNodes
}
