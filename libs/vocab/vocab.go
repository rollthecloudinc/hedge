package vocab

type Vocabulary struct {
	Id          string `form:"id" binding:"required"`
	UserId      string `form:"userId" binding:"required"`
	MachineName string `form:"machineName" binding:"required"`
	HumanName   string `form:"humanName" binding:"required"`
	Terms       []Term `form:"terms[]" binding:"required"`
}

type Term struct {
	Id           string `form:"id" binding:"required"`
	VocabularyId string `form:"vocabularyId" binding:"required"`
	ParentId     string `form:"parentId"`
	MachineName  string `form:"machineName" binding:"required"`
	HumanName    string `form:"humanName" binding:"required"`
	Weight       int    `form:"weight" binding:"required"`
	Group        bool   `form:"group" binding:"required"`
	Selected     bool   `form:"selected" binding:"required"`
	Level        int    `form:"level" binding:"required"`
	Children     []Term `form:"children"`
}
