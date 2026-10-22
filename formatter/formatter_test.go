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
    + GET /intents/{intentId}
    + POST /intents/{intentId} # REST
    + POST /Intents/CommitIntent # backward-compatible with pre-REST API route.
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
    + GET /users/{id}
    + json = id
    + go.field.name = ID
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
    + GET /users/{id}
    + success 200 # ok
    + @internal
    + @public
    + @stringo = " a b " ## note
    + @auth = admin
  - Ping()
    + @deprecated = "Use PingV2 instead."
`

	output, err := Format(strings.NewReader(input), false)
	require.NoError(t, err)
	assert.Equal(t, expected, output)

	again, err := Format(strings.NewReader(output), false)
	require.NoError(t, err)
	assert.Equal(t, output, again)
}

func TestFormat_MethodAnnotationEqualsForm(t *testing.T) {
	input := `webrpc = v1

service Users
  - Get(GetUserRequest) => (GetUserResponse)
    + @auth:ApiKeyAuth
    + @deprecated = "Use GetUserV2 instead."
    + @path:C:\temp
    + @glob:C:\my "dir" here
    + @see:"https://a.com/#frag"
    + @success 200|201 # ok
  - Create(GetUserRequest) => (GetUserResponse)
    + @cache:60
    + @success = 201
  - Ping()
  - Status() => (status: bool)
    + @success:204
`
	expected := `webrpc = v1

service Users
  - Get(GetUserRequest) => (GetUserResponse)
    + success 200|201 # ok
    + @auth = ApiKeyAuth
    + @deprecated = "Use GetUserV2 instead."
    + @path = C:\temp
    + @glob = C:\my "dir" here
    + @see = "https://a.com/#frag"
  - Create(GetUserRequest) => (GetUserResponse)
    + success 201
    + @cache = 60
  - Ping()
  - Status() => (status: bool)
    + success 204
`

	output, err := Format(strings.NewReader(input), false)
	require.NoError(t, err)
	assert.Equal(t, expected, output)

	again, err := Format(strings.NewReader(output), false)
	require.NoError(t, err)
	assert.Equal(t, output, again)
}

func TestFormat_MethodAnnotationEscapedQuoteHash(t *testing.T) {
	// A `\"` inside a quoted value must not end the quote, so the `#` after it
	// stays part of the value and the line round-trips unchanged.
	input := `webrpc = v1

service Users
  - Get() => (ok: bool)
    + @note = "say \" hello # there \"" # trailing
`

	output, err := Format(strings.NewReader(input), false)
	require.NoError(t, err)
	assert.Equal(t, input, output)

	again, err := Format(strings.NewReader(output), false)
	require.NoError(t, err)
	assert.Equal(t, output, again)
}

func TestFormat_AboveMethodSuccessMigration(t *testing.T) {
	tests := []struct {
		name, annotations, options string
	}{
		{"success after annotation", "  @internal\n  @success:200\n", "    + GET /ping\n    + success 200\n    + @internal\n"},
		{"success before annotation", "  @success:200\n  @internal\n", "    + GET /ping\n    + success 200\n    + @internal\n"},
		{"success on same line", "  @internal @success:200 @public\n", "    + GET /ping\n    + success 200\n    + @internal\n    + @public\n"},
		{"quoted success codes", "  @success:\"200|201\" # statuses\n", "    + GET /ping\n    + success 200|201 # statuses\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := "webrpc = v1\n\nservice Users\n" + tt.annotations + "  - Ping()\n    + GET /ping\n"
			expected := "webrpc = v1\n\nservice Users\n  - Ping()\n" + tt.options
			output, err := Format(strings.NewReader(input), false)
			require.NoError(t, err)
			assert.Equal(t, expected, output)
			again, err := Format(strings.NewReader(output), false)
			require.NoError(t, err)
			assert.Equal(t, output, again)
		})
	}
}

func TestFormat_AboveMethodAnnotationValues(t *testing.T) {
	input := `webrpc = v1

service Users
  - Previous()

  # Get documentation
  @description:"Contact @support # for help" @internal
  @email:user@example.com @auth:ApiKeyAuth
  @note:"say \"@hello\" to Žan" # keep this comment
  - Get() # method comment
    + GET /users
    + @cache = enabled

  @public
  - Next()
`
	expected := `webrpc = v1

service Users
  - Previous()

  # Get documentation
  - Get() # method comment
    + GET /users
    + @description = "Contact @support # for help"
    + @internal
    + @email = user@example.com
    + @auth = ApiKeyAuth
    + @note = "say \"@hello\" to Žan" # keep this comment
    + @cache = enabled

  - Next()
    + @public
`
	output, err := Format(strings.NewReader(input), false)
	require.NoError(t, err)
	assert.Equal(t, expected, output)
	again, err := Format(strings.NewReader(output), false)
	require.NoError(t, err)
	assert.Equal(t, output, again)
}

func TestFormat_OrphanSuccessAnnotationStaysAbove(t *testing.T) {
	// Without a following method there is no destination for the annotation.
	for _, ending := range []string{"", "\nstruct User\n  - id: uint64\n"} {
		input := "service Users\n  @internal @success:200\n" + ending
		expected := "service Users\n    @internal @success:200\n" + ending
		output, err := Format(strings.NewReader(input), false)
		require.NoError(t, err)
		assert.Equal(t, expected, output)
	}
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

func TestFormat_MethodOptions(t *testing.T) {
	input := `webrpc = v1

service Users
  - Get(id: uint64) => (user: User)
      +   success   201 | 200 # first status is the default
      +error NotFound | Unauthorized
      + @auth=ApiKeyAuth
      + GET /users/{id}
      + error RateLimited
      + POST /Users/Get
  - Delete(id: uint64)
    + DELETE /users/{id}
    + success 204
`
	expected := `webrpc = v1

service Users
  - Get(id: uint64) => (user: User)
    + GET /users/{id}
    + POST /Users/Get
    + success 201|200 # first status is the default
    + error NotFound|Unauthorized
    + error RateLimited
    + @auth = ApiKeyAuth
  - Delete(id: uint64)
    + DELETE /users/{id}
    + success 204
`

	for _, sortErrors := range []bool{false, true} {
		output, err := Format(strings.NewReader(input), sortErrors)
		require.NoError(t, err)
		assert.Equal(t, expected, output)
		again, err := Format(strings.NewReader(output), sortErrors)
		require.NoError(t, err)
		assert.Equal(t, output, again)
	}
}

func TestFormat_AnnotationsAfterRoutesAndOptions(t *testing.T) {
	for _, annotations := range []string{
		"  @auth:ApiKeyAuth @internal\n  - GetUser(userID:uint64) => (user:User)\n",
		"  - GetUser(userID:uint64) => (user:User)\n    + @auth = ApiKeyAuth\n    + @internal\n",
	} {
		input := "service ExampleService\n" + annotations + `    + GET /users/{userID}
    + POST /ExampleService/GetUser # compatibility route
    + success 200 # applies to both routes
    + error UserNotFound|InvalidName
    # Description annotation
    + @description = "Get a user # including their profile" # keep annotation comment

  # Next method
  - Ping()

struct User
  - id: uint64
    + json = userID
`
		expected := `service ExampleService
  - GetUser(userID: uint64) => (user: User)
    + GET /users/{userID}
    + POST /ExampleService/GetUser # compatibility route
    + success 200 # applies to both routes
    + error UserNotFound|InvalidName
    + @auth = ApiKeyAuth
    + @internal
    # Description annotation
    + @description = "Get a user # including their profile" # keep annotation comment

  # Next method
  - Ping()

struct User
  - id: uint64
    + json = userID
`
		output, err := Format(strings.NewReader(input), false)
		require.NoError(t, err)
		assert.Equal(t, expected, output)
		again, err := Format(strings.NewReader(output), false)
		require.NoError(t, err)
		assert.Equal(t, output, again)
	}
}

func TestFormat_AnnotationsAfterEveryRouteVerb(t *testing.T) {
	for _, verb := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "QUERY"} {
		t.Run(verb, func(t *testing.T) {
			input := "service Users\n  - Get()\n    # Authentication\n    + @auth = ApiKeyAuth\n    # Route\n    + " + verb + " /users\n    + error NotFound\n    + @internal\n    + success 200\n"
			expected := "service Users\n  - Get()\n    # Route\n    + " + verb + " /users\n    + success 200\n    + error NotFound\n    # Authentication\n    + @auth = ApiKeyAuth\n    + @internal\n"
			output, err := Format(strings.NewReader(input), false)
			require.NoError(t, err)
			assert.Equal(t, expected, output)
			again, err := Format(strings.NewReader(output), false)
			require.NoError(t, err)
			assert.Equal(t, output, again)
		})
	}
}

func TestFormat_RoutesSortedByHTTPVerb(t *testing.T) {
	input := `service Users
  @auth:ApiKeyAuth
  - GetUser(userID:uint64) => (user:User)
    + QUERY /users/search
    + error UserNotFound|InvalidName
    + DELETE /users/{userID}
    + success 200
    + PATCH /users/{userID}
    + @internal
    + PUT /users/{userID}
    # Compatibility route
    + POST /ExampleService/GetUser # compatibility route
    + GET /users/{userID}
    + POST /users/{userID} # second POST stays second
    # Another GET
    + GET /users # second GET stays second

  # Separate method
  - Other()
    + POST /other
    + GET /other
    + @public
`
	expected := `service Users
  - GetUser(userID: uint64) => (user: User)
    + GET /users/{userID}
    # Another GET
    + GET /users # second GET stays second
    # Compatibility route
    + POST /ExampleService/GetUser # compatibility route
    + POST /users/{userID} # second POST stays second
    + PUT /users/{userID}
    + PATCH /users/{userID}
    + DELETE /users/{userID}
    + QUERY /users/search
    + success 200
    + error UserNotFound|InvalidName
    + @auth = ApiKeyAuth
    + @internal

  # Separate method
  - Other()
    + GET /other
    + POST /other
    + @public
`
	for _, sortErrors := range []bool{false, true} {
		output, err := Format(strings.NewReader(input), sortErrors)
		require.NoError(t, err)
		assert.Equal(t, expected, output)
		again, err := Format(strings.NewReader(output), sortErrors)
		require.NoError(t, err)
		assert.Equal(t, output, again)
	}
}

func TestFormat_LegacyMethodErrors(t *testing.T) {
	for _, signature := range []string{
		"Get(id:uint64) => (user:User)",
		"Get(id:uint64)",
		"stream Get(id:uint64) => stream (user:User)",
	} {
		t.Run(signature, func(t *testing.T) {
			input := "service Users\n  @internal\n  - " + signature + " errors NotFound | Unauthorized # scoped errors\n    + error RateLimited\n    + GET /users/{id}\n    + @success 200 | 201\n"
			output, err := Format(strings.NewReader(input), false)
			require.NoError(t, err)
			assert.NotContains(t, output, " errors ")
			assert.Contains(t, output, " # scoped errors\n    + GET /users/{id}\n    + success 200|201\n    + error NotFound|Unauthorized\n    + error RateLimited\n    + @internal\n")
			again, err := Format(strings.NewReader(output), false)
			require.NoError(t, err)
			assert.Equal(t, output, again)
		})
	}
}

func TestFormat_InvalidMethodSuffix(t *testing.T) {
	for _, suffix := range []string{"errors", "unexpected NotFound"} {
		_, err := Format(strings.NewReader("service Users\n  - Get() "+suffix+"\n"), false)
		require.Error(t, err)
	}
}

func TestFormat_MethodOptionBoundaries(t *testing.T) {
	input := `struct Options
  - value: string
    + success = yes
    + error = message

service Users
  - Get()
    + success 201|201
    + success
    + error A|A
    + error A
    + error
    + error A B
    + successful = yes
    + errors = custom
    + @error = custom
    + @successful = yes
`
	output, err := Format(strings.NewReader(input), false)
	require.NoError(t, err)
	assert.Equal(t, input, output)
}

func TestFormat_TypeAliasesEnumMetadataAndFiles(t *testing.T) {
	input := `webrpc = v1

type    Address :   string # identifier
     + go.type = Address
type Labels: [] Address
type Wallet: Address
type Lookup: map< string, Address >

enum Kind: uint32
  - User
      + json=USER

struct Upload
  - address: Wallet
  - labels: Labels
  - attachment: file
  - attachments: []file

service Files
  - Upload(Upload) => (data: file)
`
	expected := `webrpc = v1

type Address: string # identifier
    + go.type = Address
type Labels: []Address
type Wallet: Address
type Lookup: map<string,Address>

enum Kind: uint32
  - User
    + json = USER

struct Upload
  - address: Wallet
  - labels: Labels
  - attachment: file
  - attachments: []file

service Files
  - Upload(Upload) => (data: file)
`
	output, err := Format(strings.NewReader(input), false)
	require.NoError(t, err)
	assert.Equal(t, expected, output)
	again, err := Format(strings.NewReader(output), false)
	require.NoError(t, err)
	assert.Equal(t, output, again)
}

func TestFormat_ErrorWithQuotedComment(t *testing.T) {
	input := "error 1 Missing \"not found\" HTTP 404 # reserved for \"route not found\"\n"
	output, err := Format(strings.NewReader(input), false)
	require.NoError(t, err)
	assert.Equal(t, input, output)
}
