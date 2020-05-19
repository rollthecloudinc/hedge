package vocab

type Vocabulary struct {
	Id          string `form:"id" json:"id" binding:"required"`
	UserId      string `form:"userId" json:"userId" binding:"required"`
	MachineName string `form:"machineName" json:"machineName" binding:"required"`
	HumanName   string `form:"humanName" json:"humanName" binding:"required"`
	Terms       []Term `form:"terms[]" json:"terms" binding:"required"`
}

type Term struct {
	Id           string `form:"id" json:"id" binding:"required"`
	VocabularyId string `form:"vocabularyId" json:"vocabularyId" binding:"required"`
	ParentId     string `form:"parentId" json:"parentId"`
	MachineName  string `form:"machineName" json:"machineName" binding:"required"`
	HumanName    string `form:"humanName" json:"humanName" binding:"required"`
	Weight       int    `form:"weight" json:"weight" binding:"required"`
	Group        bool   `form:"group" json:"group" binding:"required"`
	Selected     bool   `form:"selected" json:"selected" binding:"required"`
	Level        int    `form:"level" json:"level" binding:"required"`
	Children     []Term `form:"children" json:"children"`
}
