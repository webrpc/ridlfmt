package formatter

import (
	"fmt"
	"sort"
	"strings"
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

		if len(err.description) > descLen {
			descLen = len(err.description)
		}

		if len(fmt.Sprintf("%d", err.httpCode)) > httpLen {
			httpLen = len(fmt.Sprintf("%d", err.httpCode))
		}
	}

	return codeLen, nameLen, descLen, httpLen
}

func (f *form) errorsPrint() string {
	codeLen, nameLen, descLen, httpLen := f.errors.getLenghts()

	if f.sortErrors {
		sort.Sort(f.errors)
	}

	var lines string
	for i, err := range f.errors {
		lines += fmt.Sprintf("error %-*d %-*s \"%s\"%s HTTP %-*d",
			codeLen,
			err.code,
			nameLen,
			err.name,
			err.description,
			strings.Repeat(" ", descLen-len(err.description)),
			httpLen,
			err.httpCode,
		)

		if err.inlineComment != nil {
			lines += fmt.Sprintf(" %s", err.inlineComment.getString())
		}

		if i != len(f.errors)-1 {
			lines += "\n"
		}
	}

	f.errors = nil

	return lines
}
