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

func TestFormat_BasepathAndRestRoutes(t *testing.T) {
	input := `webrpc = v1
name = example
version = v1
basepath   =   /api

service Users
  -   Get(GetUserRequest) => (GetUserResponse)
         +    GET    /{userId}   # by id
  - List(ListUsersRequest) => (ListUsersResponse)
  +QUERY /search
  - Ping()

struct GetUserRequest
  - userId: uint64
  - path: string
`
	expected := `webrpc = v1
name = example
version = v1
basepath = /api

service Users
  - Get(GetUserRequest) => (GetUserResponse)
    + GET /{userId} # by id
  - List(ListUsersRequest) => (ListUsersResponse)
    + QUERY /search
  - Ping()

struct GetUserRequest
  - userId: uint64
  - path: string
`

	output, err := Format(strings.NewReader(input), false)
	require.NoError(t, err)
	assert.Equal(t, expected, output)

	again, err := Format(strings.NewReader(output), false)
	require.NoError(t, err)
	assert.Equal(t, output, again)
}

func TestFormat_MultipleRoutesKeepOrder(t *testing.T) {
	input := `webrpc = v1

service Intents
  - CommitIntent(CommitIntentRequest) => (CommitIntentResponse)
      + POST /intents/{intentId}   # REST
      + POST /Intents/CommitIntent     # backward-compatible with pre-REST API route.
      + GET /intents/{intentId}
  - Ping()
`
	expected := `webrpc = v1

service Intents
  - CommitIntent(CommitIntentRequest) => (CommitIntentResponse)
    + POST /intents/{intentId} # REST
    + POST /Intents/CommitIntent # backward-compatible with pre-REST API route.
    + GET /intents/{intentId}
  - Ping()
`

	output, err := Format(strings.NewReader(input), false)
	require.NoError(t, err)
	assert.Equal(t, expected, output)

	again, err := Format(strings.NewReader(output), false)
	require.NoError(t, err)
	assert.Equal(t, output, again)
}

func TestFormat_RouteWithMetaLines(t *testing.T) {
	input := `webrpc = v1

error 1 Unauthorized "unauthorized" HTTP 401

service Users
  - Get(GetUserRequest) => (GetUserResponse)
    + json=id
    + go.field.name = ID
    +   GET   /users/{id}
    + json = id   # trailing
  - Ping()
`
	expected := `webrpc = v1

error 1 Unauthorized "unauthorized" HTTP 401

service Users
  - Get(GetUserRequest) => (GetUserResponse)
    + json = id
    + go.field.name = ID
    + GET /users/{id}
    + json = id # trailing
  - Ping()
`

	output, err := Format(strings.NewReader(input), false)
	require.NoError(t, err)
	assert.Equal(t, expected, output)

	again, err := Format(strings.NewReader(output), false)
	require.NoError(t, err)
	assert.Equal(t, output, again)
}

func TestFormat_RouteLikeLineOutsideService(t *testing.T) {
	output, err := Format(strings.NewReader("webrpc = v1\n+   GET   /users\n"), false)
	require.NoError(t, err)
	assert.Equal(t, "webrpc = v1\n    +   GET   /users\n", output)
}

func TestFormat_NotARoute(t *testing.T) {
	input := `webrpc = v1

service Users
  - Ping()
    + FOO /x
    + GET x
    + GET
    + json = /x
`
	expected := `webrpc = v1

service Users
  - Ping()
    + FOO /x
    + GET x
    + GET
    + json = /x
`

	output, err := Format(strings.NewReader(input), false)
	require.NoError(t, err)
	assert.Equal(t, expected, output)
}

func TestFormat_OldRouteSyntaxRejected(t *testing.T) {
	tests := map[string]string{
		"service path":    "webrpc = v1\n\nservice Users\n  path = /users\n",
		"bare route line": "webrpc = v1\n\nservice Users\n  - Ping()\n      GET /x\n",
	}

	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Format(strings.NewReader(input), false)
			require.Error(t, err)
			assert.ErrorContains(t, err, "unknown section")
		})
	}
}

func TestFormat_MethodAnnotationMigration(t *testing.T) {
	input := `webrpc = v1

service Users
  @internal @public @stringo:" a b " ## note
  - Get(GetUserRequest) => (GetUserResponse)
      + @auth:admin
      + GET /users/{id}
      + @success   200   # ok
  - Ping()
      + @deprecated:Use PingV2 instead.
`
	expected := `webrpc = v1

service Users
  - Get(GetUserRequest) => (GetUserResponse)
    + @internal
    + @public
    + @stringo:" a b " ## note
    + @auth:admin
    + GET /users/{id}
    + @success 200 # ok
  - Ping()
    + @deprecated:Use PingV2 instead.
`

	output, err := Format(strings.NewReader(input), false)
	require.NoError(t, err)
	assert.Equal(t, expected, output)

	again, err := Format(strings.NewReader(output), false)
	require.NoError(t, err)
	assert.Equal(t, output, again)
}

func TestFormat_AnnotationBeforeNonMethodFlushedUnchanged(t *testing.T) {
	input := `webrpc = v1

service Users
  @public
struct Foo
  - id: uint64
`
	expected := `webrpc = v1

service Users
    @public
struct Foo
  - id: uint64
`

	output, err := Format(strings.NewReader(input), false)
	require.NoError(t, err)
	assert.Equal(t, expected, output)
}

func TestFormat_AnnotationStashOrdering(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"comment after the annotation follows it",
			"webrpc = v1\n\nservice Users\n  @public\n  # after\nstruct Foo\n  - id: uint64\n",
			"webrpc = v1\n\nservice Users\n    @public\n    # after\nstruct Foo\n  - id: uint64\n",
		},
		{
			"comment before the annotation stays above it",
			"webrpc = v1\n\nservice Users\n  # before\n  @public\n\nstruct Foo\n  - id: uint64\n",
			"webrpc = v1\n\nservice Users\n    # before\n    @public\n\nstruct Foo\n  - id: uint64\n",
		},
		{
			"blank line after the annotation stays below it",
			"webrpc = v1\n\nservice Users\n  @public\n\nstruct Foo\n  - id: uint64\n",
			"webrpc = v1\n\nservice Users\n    @public\n\nstruct Foo\n  - id: uint64\n",
		},
		{
			"comment between the annotation and the method",
			"webrpc = v1\n\nservice Users\n  @public\n  # after\n  - Get()\n",
			"webrpc = v1\n\nservice Users\n  - Get()\n    + @public\n    # after\n",
		},
		{
			"annotation block at EOF",
			"webrpc = v1\n\nservice Users\n  @public\n  @auth:admin\n",
			"webrpc = v1\n\nservice Users\n    @public\n    @auth:admin\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := Format(strings.NewReader(tt.input), false)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, output)

			again, err := Format(strings.NewReader(output), false)
			require.NoError(t, err)
			assert.Equal(t, output, again)
		})
	}
}

func TestFormat_ErrorWithoutHTTPStatus(t *testing.T) {
	input := `webrpc = v1

error 1 Unauthorized "unauthorized" HTTP 401
error 200   UserNotFound   "user not found"   # defaults to HTTP 400
`
	expected := `webrpc = v1

error 1   Unauthorized "unauthorized"   HTTP 401
error 200 UserNotFound "user not found" # defaults to HTTP 400
`

	output, err := Format(strings.NewReader(input), false)
	require.NoError(t, err)
	assert.Equal(t, expected, output)
}
