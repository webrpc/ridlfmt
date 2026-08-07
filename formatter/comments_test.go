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

func TestParseComment_HiddenAndHashCount(t *testing.T) {
	tests := []struct {
		name              string
		in                string
		expectedContent   string
		expectedHidden    bool
		expectedHashCount int
	}{
		{"simple comment", "# hello", " hello", false, 1},
		{"hidden comment", "#! hello", " hello", true, 1},
		{"double hash comment", "## hello", " hello", false, 2},
		{"triple hash comment", "### hello", " hello", false, 3},
		{"double hash hidden comment", "##! hello", " hello", true, 2},
		{"no leading space in content", "#hello", " hello", false, 1},
		{"trailing spaces trimmed", "# hello   ", " hello", false, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := parseComment(tt.in)
			require.NotNil(t, c)
			assert.Equal(t, tt.expectedContent, c.content)
			assert.Equal(t, tt.expectedHidden, c.hidden)
			assert.Equal(t, tt.expectedHashCount, c.hashCount)
		})
	}
}

func TestParseAndDivideInlineComment(t *testing.T) {
	t.Run("with comment", func(t *testing.T) {
		s, c := parseAndDivideInlineComment("version = v1 # a comment")
		assert.Equal(t, "version = v1", s)
		require.NotNil(t, c)
		assert.Equal(t, " a comment", c.content)
	})

	t.Run("without comment", func(t *testing.T) {
		s, c := parseAndDivideInlineComment("version = v1")
		assert.Equal(t, "version = v1", s)
		assert.Nil(t, c)
	})
}

func TestCommentGetString(t *testing.T) {
	tests := []struct {
		name     string
		c        comment
		expected string
	}{
		{"visible", comment{content: " hello", hidden: false, hashCount: 1}, "# hello"},
		{"hidden", comment{content: " hello", hidden: true, hashCount: 1}, "#! hello"},
		{"double hash", comment{content: " hello", hidden: false, hashCount: 2}, "## hello"},
		{"hidden double hash", comment{content: " hello", hidden: true, hashCount: 2}, "##! hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.c.getString())
		})
	}
}

func TestCommentAppendInlineComment(t *testing.T) {
	t.Run("nil comment", func(t *testing.T) {
		var c *comment
		assert.Equal(t, "line", c.appendInlineComment("line"))
	})

	t.Run("non-nil comment", func(t *testing.T) {
		c := &comment{content: " hello", hidden: false, hashCount: 1}
		assert.Equal(t, "line # hello", c.appendInlineComment("line"))
	})

	t.Run("trims trailing spaces before appending", func(t *testing.T) {
		c := &comment{content: " hello", hidden: false, hashCount: 1}
		assert.Equal(t, "line # hello", c.appendInlineComment("line   "))
	})
}

func TestCountHashes(t *testing.T) {
	tests := []struct {
		in            string
		startCount    int
		expectedRest  string
		expectedCount int
	}{
		{"hello", 1, "hello", 1},
		{"#hello", 1, "hello", 2},
		{"##hello", 1, "hello", 3},
		{"", 1, "", 1},
	}

	for _, tt := range tests {
		rest, count := countHashes(tt.in, tt.startCount)
		assert.Equal(t, tt.expectedRest, rest)
		assert.Equal(t, tt.expectedCount, count)
	}
}
