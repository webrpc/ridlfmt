package formatter

import (
	"fmt"
)

type ridlError struct {
	code          int
	name          string
	description   string
	httpCode      int
	inlineComment *comment
}

type ridlErrors []ridlError

func (e ridlErrors) Len() int           { return len(e) }
func (e ridlErrors) Less(i, j int) bool { return e[i].code < e[j].code }
func (e ridlErrors) Swap(i, j int)      { e[i], e[j] = e[j], e[i] }

func (e ridlErrors) getLenghts() (codeLen, nameLen, descLen, httpLen int) {
	for _, err := range e {
		if len(fmt.Sprintf("%d", err.code)) > codeLen {
			codeLen = len(fmt.Sprintf("%d", err.code))
		}

		if len(err.name) > nameLen {
			nameLen = len(err.name)
		}

		if len([]rune(err.description)) > descLen {
			descLen = len([]rune(err.description))
		}

		if len(fmt.Sprintf("%d", err.httpCode)) > httpLen {
			httpLen = len(fmt.Sprintf("%d", err.httpCode))
		}
	}

	return codeLen, nameLen, descLen, httpLen
}
