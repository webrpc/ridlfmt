package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatAndPrintFromPipe(t *testing.T) {
	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	defer func() {
		os.Stdin = oldStdin
	}()

	os.Stdin = r
	w.Write([]byte(testInput))
	w.Close()

	rOut, wOut, _ := os.Pipe()
	oldStdout := os.Stdout
	defer func() {
		os.Stdout = oldStdout
	}()
	os.Stdout = wOut

	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)

	args := []string{"-s"}
	err := runRidlfmt(flagSet, args)
	require.NoError(t, err)
	wOut.Close()

	var out bytes.Buffer
	_, err = io.Copy(&out, rOut)
	require.NoError(t, err)

	require.Equal(t, strings.TrimSpace(expectedOutput), strings.TrimSpace(out.String()))
}

func TestFormatAndWriteToFile(t *testing.T) {
	tempFile, err := os.CreateTemp("", "ridlfmt_test*.ridl")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	_, err = tempFile.WriteString(testInput)
	require.NoError(t, err)
	tempFile.Close()

	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)

	args := []string{"-w", "-s", tempFile.Name()}
	err = runRidlfmt(flagSet, args)
	require.NoError(t, err)

	// Read the output from the temp file
	outputBytes, err := os.ReadFile(tempFile.Name())
	require.NoError(t, err)

	require.Equal(t, expectedOutput, string(outputBytes))
}

func testHelpFlag(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-h")

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil && err.Error() != "exit status 2" { // exit code 2 is expected for help flag
		t.Fatalf("Error running command: %v", err)
	}

	expectedHelpText := `usage: ridlfmt [flags] [path...]`
	if !strings.Contains(out.String(), expectedHelpText) {
		t.Errorf("Expected help message not found. Got: %s", out.String())
	}
}

const testInput string = `
      webrpc    =    v1    #    version of webrpc schema format (ridl or json)
   name    = 		example # name of your backend app
	version=v0.0.1#version of your schema

# bar
enum Intent: string
  #! foo
   - openSession
  -       closeSession

enum           Kind:            uint32
  - USER
# admin
  - ADMIN

struct             Empty

      # struct comment
struct     User
  - id: uint64
    + json = id
    + go.field.name = ID # dsadsa
    + go.tag.db = id

  - username: string
    + json = USERNAME
         +      go.tag.db      =        username       #!       far away

#! role?
               #! role!
  -           role:              string
    + go.tag.db = -

  - kind: Kind
    + json = kind

  - intent: Intent
    + json = intent ###! dsadasdasds
    + go.tag.db = -

struct Version
  - webrpcVersion: string
  - schemaVersion: string
  - schemaHash: string

struct ComplexType # dsdas
      # https://www.example.com/?first=1&second=12#help
  -      meta: map<string,any>
  - metaNestedExample: map<string,map<string,uint32>>
  - namesList: []string
  - numsList: []int64
  - doubleArray: [][]string
  - listOfMaps:        []map<string,uint32> # dsadasdasdas
  - listOfUsers:                 []User
  - mapOfUsers: map<string,User>
  - user: User

#!
#! Errors
#!
error      2      UserNotFound "User not found" HTTP 404
error 20 SpaceshipNotFound "Spaceship not found"       HTTP 404#comment
error 300 Unsomething "Un what?" HTTP                      444 #comment
error 1  IAmFirst "I am first" HTTP 101 # comment

error 20         UserNotFound     "User not found" HTTP 404
error 4         UserTooYoung     ""  HTTP   404 

service ExampleService # oof
  @   deprecated   :      Pong
  	@  auth   :   ApiKeyAuth @   who    dsa   :   J    W    T   ## dadsadadsa
- Ping()
 - Status() => (status: bool)
  	@                        internal                         @      public                ##      dsada s dsa
  - Version() => (version: Version)
@public
   - GetUser   (   header   :    map  <   string   ,   string   >   ,   userID   :    uint64   )   =>   (  code  :   uint32   ,   user  :   User  )
    - FindUser(s :SearchFilter) => (name: string, user: User) ###! last




`

const expectedOutput string = `
webrpc = v1 # version of webrpc schema format (ridl or json)
name = example # name of your backend app
version = v0.0.1 # version of your schema

# bar
enum Intent: string
  #! foo
  - openSession
  - closeSession

enum Kind: uint32
  - USER
  # admin
  - ADMIN

struct Empty

# struct comment
struct User
  - id: uint64
    + json = id
    + go.field.name = ID # dsadsa
    + go.tag.db = id

  - username: string
    + json = USERNAME
    + go.tag.db = username #! far away

  #! role?
  #! role!
  - role: string
    + go.tag.db = -

  - kind: Kind
    + json = kind

  - intent: Intent
    + json = intent ###! dsadasdasds
    + go.tag.db = -

struct Version
  - webrpcVersion: string
  - schemaVersion: string
  - schemaHash: string

struct ComplexType # dsdas
  # https://www.example.com/?first=1&second=12#help
  - meta: map<string,any>
  - metaNestedExample: map<string,map<string,uint32>>
  - namesList: []string
  - numsList: []int64
  - doubleArray: [][]string
  - listOfMaps: []map<string,uint32> # dsadasdasdas
  - listOfUsers: []User
  - mapOfUsers: map<string,User>
  - user: User

#!
#! Errors
#!
error 1   IAmFirst          "I am first"          HTTP 101 # comment
error 2   UserNotFound      "User not found"      HTTP 404
error 20  SpaceshipNotFound "Spaceship not found" HTTP 404 # comment
error 300 Unsomething       "Un what?"            HTTP 444 # comment

error 4  UserTooYoung ""               HTTP 404
error 20 UserNotFound "User not found" HTTP 404

service ExampleService # oof
    @deprecated:Pong
    @auth:ApiKeyAuth @whodsa:JWT ## dadsadadsa
  - Ping()
  - Status() => (status: bool)
    @internal @public ## dsada s dsa
  - Version() => (version: Version)
    @public
  - GetUser(header: map<string,string>, userID: uint64) => (code: uint32, user: User)
  - FindUser(s: SearchFilter) => (name: string, user: User) ###! last
`
