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
	idx := findCommentIndex(s)
	if idx < 0 {
		return nil
	}

	var hidden bool
	count := 1

	content := s[idx+1:]

	if strings.HasPrefix(content, "!") {
		hidden = true
		content = strings.SplitN(content, "!", 2)[1]
	} else if strings.HasPrefix(content, "#") {
		content, count = countHashes(content, count)
		sub, found := strings.CutPrefix(content, "!")
		if found {
			hidden = true
			content = sub
		}
	}

	if !strings.HasPrefix(content, " ") {
		content = " " + content
	}

	c := comment{
		content:   strings.TrimRight(content, " "),
		hidden:    hidden,
		hashCount: count,
		original:  s[idx+1:],
	}

	return &c
}

// wordBreak mirrors webrpc's ridl lexer wordBreak charset: characters that
// terminate an in-progress word token.
const wordBreak = "\x00 \t\r\n[]()<>{}=:¿?¡!,\""

func isWordBreak(r rune) bool {
	return strings.ContainsRune(wordBreak, r)
}

// isWordBeginning mirrors webrpc's ridl lexer wordBeginning charset: characters
// that can start (and, once started, continue) a word token.
func isWordBeginning(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

// findCommentIndex returns the byte index of the "#" that starts a comment,
// or -1 if none is found. It replicates webrpc's ridl lexer behavior: once a
// word token has started, "#" (like most punctuation) does not break it and
// is simply part of the word rather than a comment marker. For example, in
// "v0.0.1#version" the "#" is never lexed as a comment because the preceding
// characters are still part of the same word token.
func findCommentIndex(s string) int {
	inWord := false

	for i, r := range s {
		if isWordBreak(r) {
			inWord = false
			continue
		}

		if inWord {
			continue
		}

		if r == '#' {
			return i
		}

		if isWordBeginning(r) {
			inWord = true
		}
	}

	return -1
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
		s = fmt.Sprintf("%s!%s", strings.Repeat("#", c.hashCount), c.content)
	} else {
		s = fmt.Sprintf("%s%s", strings.Repeat("#", c.hashCount), c.content)
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
