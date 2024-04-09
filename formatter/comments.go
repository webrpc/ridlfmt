package formatter

import (
	"fmt"
	"strings"
)

type comment struct {
	content   string
	hidden    bool
	hashCount int
	original  string
}

func parseComment(s string) *comment {
	parts := strings.SplitN(s, "#", 2)
	if len(parts) > 1 {
		var hidden bool
		count := 1

		content := parts[1]
		if strings.HasPrefix(content, " ") {
			content = strings.TrimSpace(strings.SplitN(parts[1], " ", 2)[1])
		} else if strings.HasPrefix(content, "!") {
			hidden = true
			content = strings.TrimSpace(strings.SplitN(parts[1], "!", 2)[1])
		} else if strings.HasPrefix(content, "#") {
			content, count = countHashes(content, count)
			sub, found := strings.CutPrefix(content, "!")
			if found {
				hidden = true
				content = strings.TrimSpace(sub)
			}
		}

		c := comment{
			content:   strings.TrimSpace(content),
			hidden:    hidden,
			hashCount: count,
			original:  parts[1],
		}

		return &c
	}

	return nil
}

func parseAndDivideInlineComment(s string) (string, *comment) {
	c := parseComment(s)
	if c != nil {
		s = strings.TrimSuffix(s, c.original)
		s = s[:len(s)-1]
		s = strings.TrimRight(s, " ")
	}

	return s, c
}

func (c comment) getString() string {
	var s string
	if c.hidden {
		s = fmt.Sprintf("%s! %s", strings.Repeat("#", c.hashCount), c.content)
	} else {
		s = fmt.Sprintf("%s %s", strings.Repeat("#", c.hashCount), c.content)
	}

	return strings.TrimSpace(s)
}

func (c *comment) appendInlineComment(s string) string {
	if c != nil {
		return fmt.Sprintf("%s %s", strings.TrimRight(s, " "), c.getString())
	}

	return s
}

func countHashes(s string, count int) (string, int) {
	sub, found := strings.CutPrefix(s, "#")
	if found {
		sub, count = countHashes(sub, count+1)
	}

	return sub, count
}

func (f *form) commentsPrint() string {
	var lines string
	for _, c := range f.comments {
		lines += fmt.Sprintf("%s%s\n", strings.Repeat(" ", f.padding), c.getString())
	}

	f.comments = nil

	return lines
}
