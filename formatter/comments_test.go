package formatter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseComment_HashInsideWordIsNotAComment(t *testing.T) {
	// https://github.com/webrpc/ridlfmt/issues/17
	// webrpc's ridl lexer only starts a comment at a word boundary; a "#"
	// touching the previous character with no whitespace in between is part
	// of that word/value, not a comment marker.
	tests := []struct {
		name string
		in   string
	}{
		{"hash directly after value", "version=v0.0.1#version of your schema"},
		{"hash directly after http code", "HTTP 404#comment"},
		{"trailing hash with nothing after it", "name =foo-bar#"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Nil(t, parseComment(tt.in))
		})
	}
}

func TestParseComment_HashAfterWordBreakIsAComment(t *testing.T) {
	tests := []struct {
		name            string
		in              string
		expectedContent string
	}{
		{"hash after space", "version = v0.0.1 # version of your schema", " version of your schema"},
		{"hash after space following http code", "HTTP 404 # comment", " comment"},
		{"hash on its own", "# just a comment", " just a comment"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := parseComment(tt.in)
			require.NotNil(t, c)
			assert.Equal(t, tt.expectedContent, c.content)
		})
	}
}

func TestFindCommentIndex(t *testing.T) {
	tests := []struct {
		in       string
		expected int
	}{
		{"v0.0.1#version of your schema", -1},
		{"v0.0.1 #version of your schema", 7},
		{"foo-bar#", -1},
		{"# leading comment", 0},
		{"no comment here", -1},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, findCommentIndex(tt.in), "input=%q", tt.in)
	}
}
