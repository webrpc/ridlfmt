package formatter

type section int

const (
	sectionUnknown section = iota
	sectionEmpty
	sectionWebRPC
	sectionName
	sectionVersion
	sectionComment
	sectionEnum
	sectionStruct
	sectionService
	sectionField
	sectionTag
	sectionError
)
