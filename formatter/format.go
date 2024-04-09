package formatter

import (
	"fmt"
	"io"
	"regexp"
)

type form struct {
	padding       int
	comments      []*comment
	errors        ridlErrors
	sortErrors    bool
	section       section
	topLvlSection section
}

func Format(inputFile io.Reader, sortErrors bool) (string, error) {
	f := form{
		sortErrors: sortErrors,
	}

	output, err := f.processLines(inputFile)
	if err != nil {
		return "", fmt.Errorf("process lines: %w", err)
	}

	output = f.removeDoubleLines(output)

	return output, nil
}

func reduceSpaces(input string) string {
	pattern := regexp.MustCompile(`\s+`)
	return pattern.ReplaceAllString(input, " ")
}
