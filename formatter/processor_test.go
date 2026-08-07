package formatter

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -- low level helpers --

func TestReduceSpaces(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		expected string
	}{
		{"no comment, multiple spaces", "a    b   c", "a b c"},
		{"comment after real word boundary", "a   b # a  comment", "a b # a  comment"},
		{"comment glued to word is not split off", "a0#comment more", "a0#comment more"},
		{"only a comment", "# a  comment", "# a  comment"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, reduceSpaces(tt.in))
		})
	}
}

func TestReduceSpacesInString(t *testing.T) {
	assert.Equal(t, "a b c", reduceSpacesInString("  a   b   c  "))
	assert.Equal(t, "", reduceSpacesInString("   "))
}

func TestRemoveSpaces(t *testing.T) {
	assert.Equal(t, "abc", removeSpaces(" a b c "))
}

func TestExtractFromParenthesis(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		content, err := extractFromParenthesis("Ping( a, b )")
		require.NoError(t, err)
		assert.Equal(t, "a, b", content)
	})

	t.Run("missing open paren", func(t *testing.T) {
		_, err := extractFromParenthesis("Ping a, b)")
		require.Error(t, err)
	})

	t.Run("missing close paren", func(t *testing.T) {
		_, err := extractFromParenthesis("Ping(a, b")
		require.Error(t, err)
	})
}

func TestSplitArguments(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		expected []string
	}{
		{"empty", "", nil},
		{"single arg", "a:string", []string{"a:string"}},
		{"multiple args", "a:string,b:uint64", []string{"a:string", "b:uint64"}},
		{"with map type", "a:map<string,string>,b:uint64", []string{"a:map<string,string>", "b:uint64"}},
		{"trailing map type", "a:uint64,b:map<string,uint32>", []string{"a:uint64", "b:map<string,uint32>"}},
		{"only map type", "a:map<string,string>", []string{"a:map<string,string>"}},
		{
			"nested map type followed by another arg",
			"a:map<string,map<string,string>>,b:uint64",
			[]string{"a:map<string,map<string,string>>", "b:uint64"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, splitArguments(tt.in))
		})
	}
}

func TestFormatMethodArguments(t *testing.T) {
	t.Run("no args", func(t *testing.T) {
		out, err := formatMethodArguments("Ping()")
		require.NoError(t, err)
		assert.Equal(t, "", out)
	})

	t.Run("single named arg", func(t *testing.T) {
		out, err := formatMethodArguments("Ping(a:string)")
		require.NoError(t, err)
		assert.Equal(t, "a: string", out)
	})

	t.Run("bare type arg", func(t *testing.T) {
		out, err := formatMethodArguments("Ping(User)")
		require.NoError(t, err)
		assert.Equal(t, "User", out)
	})

	t.Run("multiple args", func(t *testing.T) {
		out, err := formatMethodArguments("Ping(a:string,b:uint64)")
		require.NoError(t, err)
		assert.Equal(t, "a: string, b: uint64", out)
	})

	t.Run("missing parenthesis error", func(t *testing.T) {
		_, err := formatMethodArguments("Ping a:string")
		require.Error(t, err)
	})

	t.Run("wrong parameter values error", func(t *testing.T) {
		_, err := formatMethodArguments("Ping(a:b:c)")
		require.Error(t, err)
	})
}

func TestRemoveDoubleLines(t *testing.T) {
	f := &form{}

	in := "a\n\n\nb\n\nc\n"
	out := f.removeDoubleLines(in)
	assert.Equal(t, "a\n\nb\n\nc\n", out)
}

// -- parseSection --

func TestParseSection(t *testing.T) {
	tests := []struct {
		line     string
		expected section
	}{
		{"webrpc = v1", sectionWebRPC},
		{"name = foo", sectionName},
		{"version = v1", sectionVersion},
		{"import foo", sectionImport},
		{"# comment", sectionComment},
		{"enum Foo: string", sectionEnum},
		{"struct Foo", sectionStruct},
		{"service Foo", sectionService},
		{"error 1 Foo \"bar\" HTTP 404", sectionError},
		{"- field", sectionField},
		{"+ tag", sectionTag},
		{"@ann", sectionAnnotation},
		{"totally unknown", sectionUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			f := &form{}
			f.parseSection(tt.line)
			assert.Equal(t, tt.expected, f.section)
		})
	}
}

// -- formatLine --

func TestFormatLine_Empty(t *testing.T) {
	f := &form{}
	line, err := f.formatLine("   ")
	require.NoError(t, err)
	assert.Equal(t, "", line)
	assert.Equal(t, sectionEmpty, f.section)
}

func TestFormatLine_WebRPCNameVersion(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		f := &form{}
		line, err := f.formatLine("webrpc   =   v1   # a comment")
		require.NoError(t, err)
		assert.Equal(t, "webrpc = v1 # a comment", line)
	})

	t.Run("missing equal sign errors", func(t *testing.T) {
		f := &form{}
		_, err := f.formatLine("webrpc v1")
		require.Error(t, err)
	})
}

func TestFormatLine_Comment(t *testing.T) {
	f := &form{}
	_, err := f.formatLine("# hello")
	require.NoError(t, err)
	require.Len(t, f.comments, 1)
	assert.Equal(t, " hello", f.comments[0].content)
}

func TestFormatLine_Enum(t *testing.T) {
	t.Run("with colon", func(t *testing.T) {
		f := &form{}
		line, err := f.formatLine("enum    Foo   :   string")
		require.NoError(t, err)
		assert.Equal(t, "enum Foo: string", line)
	})

	t.Run("without colon", func(t *testing.T) {
		f := &form{}
		line, err := f.formatLine("enum Foo")
		require.NoError(t, err)
		assert.Equal(t, "enum Foo", line)
	})
}

func TestFormatLine_Struct(t *testing.T) {
	f := &form{}
	line, err := f.formatLine("struct    Foo")
	require.NoError(t, err)
	assert.Equal(t, "struct Foo", line)
}

func TestFormatLine_Service(t *testing.T) {
	f := &form{}
	line, err := f.formatLine("service    Foo")
	require.NoError(t, err)
	assert.Equal(t, "service Foo", line)
}

func TestFormatLine_Import(t *testing.T) {
	f := &form{}
	line, err := f.formatLine("import    foo")
	require.NoError(t, err)
	assert.Equal(t, "import foo", line)
}

func TestFormatLine_Error(t *testing.T) {
	t.Run("success with comment", func(t *testing.T) {
		f := &form{}
		_, err := f.formatLine(`error 1 Foo "bar" HTTP 404 # a comment`)
		require.NoError(t, err)
		require.Len(t, f.errors, 1)
		assert.Equal(t, 1, f.errors[0].code)
		assert.Equal(t, "Foo", f.errors[0].name)
		assert.Equal(t, "bar", f.errors[0].description)
		assert.Equal(t, 404, f.errors[0].httpCode)
		require.NotNil(t, f.errors[0].inlineComment)
	})

	t.Run("success without comment", func(t *testing.T) {
		f := &form{}
		_, err := f.formatLine(`error 1 Foo "bar" HTTP 404`)
		require.NoError(t, err)
		require.Len(t, f.errors, 1)
		assert.Nil(t, f.errors[0].inlineComment)
	})

	t.Run("wrong quote count errors", func(t *testing.T) {
		f := &form{}
		_, err := f.formatLine(`error 1 Foo bar HTTP 404`)
		require.Error(t, err)
	})

	t.Run("wrong begin token count errors", func(t *testing.T) {
		f := &form{}
		_, err := f.formatLine(`error 1 Foo extra "bar" HTTP 404`)
		require.Error(t, err)
	})

	t.Run("bad error code errors", func(t *testing.T) {
		f := &form{}
		_, err := f.formatLine(`error abc Foo "bar" HTTP 404`)
		require.Error(t, err)
	})

	t.Run("wrong ending token count errors", func(t *testing.T) {
		f := &form{}
		_, err := f.formatLine(`error 1 Foo "bar" HTTP`)
		require.Error(t, err)
	})

	t.Run("bad http code errors", func(t *testing.T) {
		f := &form{}
		_, err := f.formatLine(`error 1 Foo "bar" HTTP abc`)
		require.Error(t, err)
	})
}

func TestFormatLine_Field(t *testing.T) {
	t.Run("enum field", func(t *testing.T) {
		f := &form{topLvlSection: sectionEnum}
		line, err := f.formatLine("- USER # a comment")
		require.NoError(t, err)
		assert.Equal(t, "  - USER # a comment", line)
	})

	t.Run("struct field", func(t *testing.T) {
		f := &form{topLvlSection: sectionStruct}
		line, err := f.formatLine("- id : uint64 # a comment")
		require.NoError(t, err)
		assert.Equal(t, "  - id: uint64 # a comment", line)
	})

	t.Run("service field, no stream, no output", func(t *testing.T) {
		f := &form{topLvlSection: sectionService}
		line, err := f.formatLine("- Ping()")
		require.NoError(t, err)
		assert.Equal(t, "  - Ping()", line)
	})

	t.Run("service field, stream input, with output", func(t *testing.T) {
		f := &form{topLvlSection: sectionService}
		line, err := f.formatLine("- stream Send(req: string) => (resp: string)")
		require.NoError(t, err)
		assert.Equal(t, "  - stream Send(req: string) => (resp: string)", line)
	})

	t.Run("service field, stream output", func(t *testing.T) {
		f := &form{topLvlSection: sectionService}
		line, err := f.formatLine("- Recv() => stream (resp: string)")
		require.NoError(t, err)
		assert.Equal(t, "  - Recv() => stream (resp: string)", line)
	})

	t.Run("service field, bad input args errors", func(t *testing.T) {
		f := &form{topLvlSection: sectionService}
		_, err := f.formatLine("- Ping(a:b:c)")
		require.Error(t, err)
	})

	t.Run("service field, bad output args errors", func(t *testing.T) {
		f := &form{topLvlSection: sectionService}
		_, err := f.formatLine("- Ping() => (a:b:c)")
		require.Error(t, err)
	})

	t.Run("import field with colon", func(t *testing.T) {
		f := &form{topLvlSection: sectionImport}
		line, err := f.formatLine("- foo : bar")
		require.NoError(t, err)
		assert.Equal(t, "  - foo: bar", line)
	})

	t.Run("import field without colon", func(t *testing.T) {
		f := &form{topLvlSection: sectionImport}
		line, err := f.formatLine("- foo")
		require.NoError(t, err)
		assert.Equal(t, "  - foo", line)
	})

	t.Run("wrong top level section errors", func(t *testing.T) {
		f := &form{topLvlSection: sectionError}
		_, err := f.formatLine("- foo")
		require.Error(t, err)
	})
}

func TestFormatLine_Tag(t *testing.T) {
	t.Run("with equal sign", func(t *testing.T) {
		f := &form{}
		line, err := f.formatLine("+ json   =   id # a comment")
		require.NoError(t, err)
		assert.Equal(t, "    + json = id # a comment", line)
	})

	t.Run("without equal sign", func(t *testing.T) {
		f := &form{}
		line, err := f.formatLine("+ go.tag.db = -")
		require.NoError(t, err)
		assert.Equal(t, "    + go.tag.db = -", line)
	})
}

func TestFormatLine_Annotation(t *testing.T) {
	t.Run("single flag annotation", func(t *testing.T) {
		f := &form{}
		line, err := f.formatLine("@public")
		require.NoError(t, err)
		assert.Equal(t, "    @public", line)
	})

	t.Run("key value annotation", func(t *testing.T) {
		f := &form{}
		line, err := f.formatLine("@deprecated : Pong")
		require.NoError(t, err)
		assert.Equal(t, "    @deprecated:Pong", line)
	})

	t.Run("quoted value annotation is not space-stripped", func(t *testing.T) {
		f := &form{}
		line, err := f.formatLine(`@stringo: " a b c "`)
		require.NoError(t, err)
		assert.Equal(t, `    @stringo:" a b c "`, line)
	})

	t.Run("multiple annotations", func(t *testing.T) {
		f := &form{}
		line, err := f.formatLine("@internal @public @stringo: \"a b\"")
		require.NoError(t, err)
		assert.Equal(t, `    @internal @public @stringo:"a b"`, line)
	})

	t.Run("wrong annotation part count errors", func(t *testing.T) {
		f := &form{}
		_, err := f.formatLine("@foo:bar:baz")
		require.Error(t, err)
	})
}

// -- processLines --

func TestProcessLines_Success(t *testing.T) {
	input := `
webrpc = v1
name = example
version = v1

struct Foo
  - id: uint64

#!
#! Errors
#!
error 1 First "first" HTTP 400
error 2 Second "second" HTTP 404

struct Bar
  - id: uint64
`

	f := &form{}
	out, err := f.processLines(fixedReader(input))
	require.NoError(t, err)
	assert.Contains(t, out, "webrpc = v1")
	assert.Contains(t, out, "struct Foo")
	assert.Contains(t, out, "error 1 First")
	assert.Contains(t, out, "struct Bar")
}

func TestProcessLines_ErrorsFlushedWithoutBlankLine(t *testing.T) {
	// error block immediately followed by another top-level section, with no
	// blank line in between, must still flush the accumulated errors.
	input := "error 1 First \"first\" HTTP 400\nstruct Foo\n"

	f := &form{}
	out, err := f.processLines(fixedReader(input))
	require.NoError(t, err)
	assert.Contains(t, out, "error 1 First")
	assert.Contains(t, out, "struct Foo")
}

func TestProcessLines_UnknownSectionErrors(t *testing.T) {
	f := &form{}
	_, err := f.processLines(fixedReader("totally unknown line\n"))
	require.Error(t, err)
	assert.ErrorContains(t, err, "unknown section")
}

func TestProcessLines_FormatLineErrorPropagates(t *testing.T) {
	f := &form{}
	_, err := f.processLines(fixedReader("webrpc v1\n"))
	require.Error(t, err)
	assert.ErrorContains(t, err, "format:")
}

type erroringReader struct{}

func (erroringReader) Read(p []byte) (int, error) {
	return 0, errors.New("boom")
}

func TestProcessLines_ScannerError(t *testing.T) {
	f := &form{}
	_, err := f.processLines(erroringReader{})
	require.Error(t, err)
	assert.ErrorContains(t, err, "reading input file")
}

func fixedReader(s string) *strings.Reader {
	return strings.NewReader(s)
}

// -- commentsPrint / errorsPrint --

func TestCommentsPrint(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		f := &form{}
		assert.Equal(t, "", f.commentsPrint())
	})

	t.Run("with comments and padding", func(t *testing.T) {
		f := &form{padding: 2}
		f.comments = []*comment{
			{content: " a", hashCount: 1},
			{content: " b", hashCount: 1},
		}

		out := f.commentsPrint()
		assert.Equal(t, "  # a\n  # b\n", out)
		assert.Nil(t, f.comments)
	})
}

func TestErrorsPrint(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		f := &form{}
		assert.Equal(t, "", f.errorsPrint())
	})

	t.Run("multiple errors with sorting and comment", func(t *testing.T) {
		f := &form{
			sortErrors: true,
			errors: ridlErrors{
				{code: 20, name: "Second", description: "second desc", httpCode: 404},
				{code: 1, name: "First", description: "d", httpCode: 400, inlineComment: &comment{content: " note", hashCount: 1}},
			},
		}

		out := f.errorsPrint()
		lines := splitLines(out)
		require.Len(t, lines, 2)
		assert.Contains(t, lines[0], "error 1")
		assert.Contains(t, lines[0], "# note")
		assert.Contains(t, lines[1], "error 20")
		assert.Nil(t, f.errors)
	})

	t.Run("without sorting keeps original order", func(t *testing.T) {
		f := &form{
			sortErrors: false,
			errors: ridlErrors{
				{code: 20, name: "Second", description: "d", httpCode: 404},
				{code: 1, name: "First", description: "d", httpCode: 400},
			},
		}

		out := f.errorsPrint()
		lines := splitLines(out)
		require.Len(t, lines, 2)
		assert.Contains(t, lines[0], "error 20")
		assert.Contains(t, lines[1], "error 1")
	})
}

func splitLines(s string) []string {
	var lines []string
	cur := ""
	for _, r := range s {
		if r == '\n' {
			lines = append(lines, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}
