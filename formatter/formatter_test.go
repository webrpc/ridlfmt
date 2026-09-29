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
    path =   /users   # REST prefix
  -   Get(GetUserRequest) => (GetUserResponse)
         GET    /{userId}   # by id
  - List(ListUsersRequest) => (ListUsersResponse)
  QUERY /search
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
  path = /users # REST prefix
  - Get(GetUserRequest) => (GetUserResponse)
      GET /{userId} # by id
  - List(ListUsersRequest) => (ListUsersResponse)
      QUERY /search
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

func TestFormat_RouteLikeLineOutsideService(t *testing.T) {
	_, err := Format(strings.NewReader("webrpc = v1\nGET /users\n"), false)
	require.Error(t, err)
	assert.ErrorContains(t, err, "unknown section")
}
