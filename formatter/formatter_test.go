package formatter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormat_Success(t *testing.T) {
	input := "webrpc = v1\nname = example\nversion = v1\n"

	output, err := Format(strings.NewReader(input), false)
	require.NoError(t, err)
	assert.Contains(t, output, "webrpc = v1")
	assert.Contains(t, output, "name = example")
	assert.Contains(t, output, "version = v1")
}

func TestFormat_ProcessLinesError(t *testing.T) {
	input := "webrpc v1\n" // missing "=", causes formatLine to error out

	_, err := Format(strings.NewReader(input), false)
	require.Error(t, err)
	assert.ErrorContains(t, err, "process lines")
}
