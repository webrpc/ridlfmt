package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestUsage(t *testing.T) {
	r, w, _ := os.Pipe()
	oldStderr := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = oldStderr }()

	usage()
	w.Close()

	var out bytes.Buffer
	_, err := io.Copy(&out, r)
	require.NoError(t, err)

	assert.Contains(t, out.String(), "usage: ridlfmt [flags] [path...]")
}

func TestIsInputFromPipe(t *testing.T) {
	t.Run("pipe", func(t *testing.T) {
		r, w, _ := os.Pipe()
		defer r.Close()
		w.Close()

		oldStdin := os.Stdin
		os.Stdin = r
		defer func() { os.Stdin = oldStdin }()

		assert.True(t, isInputFromPipe())
	})

	t.Run("char device", func(t *testing.T) {
		devNull, err := os.Open(os.DevNull)
		require.NoError(t, err)
		defer devNull.Close()

		oldStdin := os.Stdin
		os.Stdin = devNull
		defer func() { os.Stdin = oldStdin }()

		assert.False(t, isInputFromPipe())
	})
}

func TestFormatAndWriteToFileDirect(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "ridlfmt_test*.ridl")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())

		_, err = tempFile.WriteString(testInput)
		require.NoError(t, err)
		tempFile.Close()

		err = formatAndWriteToFile(tempFile.Name(), true)
		require.NoError(t, err)

		outputBytes, err := os.ReadFile(tempFile.Name())
		require.NoError(t, err)
		assert.Equal(t, expectedOutput, string(outputBytes))
	})

	t.Run("read error", func(t *testing.T) {
		err := formatAndWriteToFile("/nonexistent/path/to/file.ridl", false)
		require.Error(t, err)
		assert.ErrorContains(t, err, "error opening input file")
	})

	t.Run("format error", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "ridlfmt_test*.ridl")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())

		_, err = tempFile.WriteString("webrpc v1\n") // missing "=", causes a format error
		require.NoError(t, err)
		tempFile.Close()

		err = formatAndWriteToFile(tempFile.Name(), false)
		require.Error(t, err)
		assert.ErrorContains(t, err, "error formatting input file")
	})

	t.Run("write error", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("running as root can bypass file permissions")
		}

		tempFile, err := os.CreateTemp("", "ridlfmt_test*.ridl")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())

		_, err = tempFile.WriteString(testInput)
		require.NoError(t, err)
		tempFile.Close()

		require.NoError(t, os.Chmod(tempFile.Name(), 0o444))
		defer os.Chmod(tempFile.Name(), 0o644)

		err = formatAndWriteToFile(tempFile.Name(), false)
		require.Error(t, err)
		assert.ErrorContains(t, err, "error writing to output file")
	})
}

func TestFormatAndPrintToStdout(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "ridlfmt_test*.ridl")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())

		_, err = tempFile.WriteString(testInput)
		require.NoError(t, err)
		tempFile.Close()

		r, w, _ := os.Pipe()
		oldStdout := os.Stdout
		os.Stdout = w
		defer func() { os.Stdout = oldStdout }()

		err = formatAndPrintToStdout(tempFile.Name(), true)
		require.NoError(t, err)
		w.Close()

		var out bytes.Buffer
		_, err = io.Copy(&out, r)
		require.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expectedOutput), strings.TrimSpace(out.String()))
	})

	t.Run("read error", func(t *testing.T) {
		err := formatAndPrintToStdout("/nonexistent/path/to/file.ridl", false)
		require.Error(t, err)
		assert.ErrorContains(t, err, "error opening input file")
	})

	t.Run("format error", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "ridlfmt_test*.ridl")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())

		_, err = tempFile.WriteString("webrpc v1\n")
		require.NoError(t, err)
		tempFile.Close()

		err = formatAndPrintToStdout(tempFile.Name(), false)
		require.Error(t, err)
		assert.ErrorContains(t, err, "error formatting input file")
	})
}

func TestFormatAndPrintFromPipe_FormatError(t *testing.T) {
	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()
	os.Stdin = r

	w.Write([]byte("webrpc v1\n"))
	w.Close()

	err := formatAndPrintFromPipe(false)
	require.Error(t, err)
	assert.ErrorContains(t, err, "error formatting input from pipe")
}

func TestFormatAndPrintFromPipe_ScannerError(t *testing.T) {
	r, _, _ := os.Pipe()
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()
	os.Stdin = r
	r.Close() // reading from a closed pipe end forces a scanner error

	err := formatAndPrintFromPipe(false)
	require.Error(t, err)
	assert.ErrorContains(t, err, "error reading from pipe")
}

func TestRunRidlfmt_FlagParseError(t *testing.T) {
	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	err := runRidlfmt(flagSet, []string{"--not-a-real-flag"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "parse args")
}

func TestRunRidlfmt_PrintToStdoutFromFileArgs(t *testing.T) {
	devNull, err := os.Open(os.DevNull)
	require.NoError(t, err)
	defer devNull.Close()

	oldStdin := os.Stdin
	os.Stdin = devNull
	defer func() { os.Stdin = oldStdin }()

	tempFile, err := os.CreateTemp("", "ridlfmt_test*.ridl")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	_, err = tempFile.WriteString(testInput)
	require.NoError(t, err)
	tempFile.Close()

	rOut, wOut, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = wOut
	defer func() { os.Stdout = oldStdout }()

	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	err = runRidlfmt(flagSet, []string{tempFile.Name()})
	require.NoError(t, err)
	wOut.Close()

	var out bytes.Buffer
	_, err = io.Copy(&out, rOut)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "webrpc = v1")
}

func TestMain_Success(t *testing.T) {
	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()
	os.Stdin = r
	w.Write([]byte(testInput))
	w.Close()

	rOut, wOut, _ := os.Pipe()
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()
	os.Stdout = wOut

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"ridlfmt", "-s"}

	main()
	wOut.Close()

	var out bytes.Buffer
	_, err := io.Copy(&out, rOut)
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(expectedOutput), strings.TrimSpace(out.String()))
}

// The following tests exercise the process-exiting paths (-h, missing input,
// and the log.Fatalf wrappers around per-file errors) in a subprocess, since
// os.Exit inside the test binary itself would abort the whole test run (and
// discard its coverage profile). They verify real CLI behavior but, being a
// separate process, do not contribute to this package's coverage percentage.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	var args []string
	if raw := os.Getenv("HELPER_ARGS"); raw != "" {
		args = strings.Split(raw, "\x1f")
	}
	os.Args = append([]string{"ridlfmt"}, args...)

	main()
	os.Exit(0)
}

func runMainSubprocess(t *testing.T, args []string, stdin string, connectStdin bool) (stdout, stderr string, exitCode int) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "HELPER_ARGS="+strings.Join(args, "\x1f"))
	if connectStdin {
		cmd.Stdin = strings.NewReader(stdin)
	}

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("unexpected error running subprocess: %v", err)
	}

	return outBuf.String(), errBuf.String(), code
}

func TestSubprocess_HelpFlag(t *testing.T) {
	_, stderr, code := runMainSubprocess(t, []string{"-h"}, "", false)
	assert.Equal(t, 0, code)
	assert.Contains(t, stderr, "usage: ridlfmt [flags] [path...]")
	assert.Contains(t, stderr, "-v, -version")
}

func TestBuildVersion(t *testing.T) {
	oldVersion := version
	t.Cleanup(func() { version = oldVersion })
	for _, tt := range []struct {
		name     string
		override string
		info     *debug.BuildInfo
		want     string
	}{
		{"release module", "devel", &debug.BuildInfo{Main: debug.Module{Version: "v0.3.0"}}, "v0.3.0"},
		{"prerelease module", "devel", &debug.BuildInfo{Main: debug.Module{Version: "v0.4.0-rc.1"}}, "v0.4.0-rc.1"},
		{"pseudo version", "devel", &debug.BuildInfo{Main: debug.Module{Version: "v0.3.1-0.20261021070000-abcdef123456"}}, "v0.3.1-0.20261021070000-abcdef123456"},
		{"local build", "devel", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, "devel"},
		{"empty module version", "devel", &debug.BuildInfo{}, "devel"},
		{"missing build info", "devel", nil, "devel"},
		{"linker override", "v0.4.0", &debug.BuildInfo{Main: debug.Module{Version: "v0.3.0"}}, "v0.4.0"},
		{"linker override without build info", "v0.4.0", nil, "v0.4.0"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			version = tt.override
			assert.Equal(t, tt.want, buildVersion(tt.info))
		})
	}
}

func TestSubprocess_VersionFlags(t *testing.T) {
	for _, flag := range []string{"-version", "-v"} {
		for _, piped := range []bool{false, true} {
			name := flag + "/terminal"
			if piped {
				name = flag + "/pipe"
			}
			t.Run(name, func(t *testing.T) {
				stdout, stderr, code := runMainSubprocess(t, []string{flag, "missing.ridl"}, "invalid input", piped)
				assert.Equal(t, 0, code)
				assert.Equal(t, "ridlfmt devel\n", stdout)
				assert.Empty(t, stderr)
			})
		}
	}
}

func TestSubprocess_NoInputFiles(t *testing.T) {
	_, stderr, code := runMainSubprocess(t, nil, "", false)
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "no input files specified")
}

func TestSubprocess_WriteFlagFileError(t *testing.T) {
	_, stderr, code := runMainSubprocess(t, []string{"-w", "/nonexistent/path/to/file.ridl"}, "", false)
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "Error processing file")
}

func TestSubprocess_PrintToStdoutFileError(t *testing.T) {
	_, stderr, code := runMainSubprocess(t, []string{"/nonexistent/path/to/file.ridl"}, "", false)
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "Error processing file")
}

func TestSubprocess_PipeFormatError(t *testing.T) {
	_, stderr, code := runMainSubprocess(t, nil, "webrpc v1\n", true)
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "Error processing input from pipe")
}

func TestRecursivePaths(t *testing.T) {
	for _, path := range []string{".", "./...", "schemas", "schemas/..."} {
		for _, write := range []bool{false, true} {
			t.Run(fmtTestName(path, write), func(t *testing.T) {
				root := t.TempDir()
				t.Chdir(root)
				require.NoError(t, os.MkdirAll("schemas/nested", 0755))
				files := []string{"schemas/first.ridl", "schemas/nested/second.ridl"}
				for _, file := range files {
					require.NoError(t, os.WriteFile(file, []byte(testInput), 0644))
				}
				// Invalid content must never be passed to the formatter.
				require.NoError(t, os.WriteFile("schemas/notes.txt", []byte("webrpc v1\n"), 0644))
				require.NoError(t, os.WriteFile("schemas/.hidden.ridl", []byte("webrpc v1\n"), 0644))
				require.NoError(t, os.Mkdir("schemas/directory.ridl", 0755))

				args := []string{"-s"}
				if write {
					args = append(args, "-w")
				}
				args = append(args, path)
				// Explicit paths must win even when stdin is a pipe.
				stdout, stderr, code := runMainSubprocess(t, args, "webrpc v1\n", true)
				require.Equal(t, 0, code, stderr)
				if write {
					assert.Empty(t, stdout)
				} else {
					assert.Equal(t, strings.Repeat(expectedOutput+"\n", 2), stdout)
				}
				for _, file := range files {
					data, err := os.ReadFile(file)
					require.NoError(t, err)
					want := testInput
					if write {
						want = expectedOutput
					}
					assert.Equal(t, want, string(data))
				}
				data, err := os.ReadFile("schemas/notes.txt")
				require.NoError(t, err)
				assert.Equal(t, "webrpc v1\n", string(data))
			})
		}
	}
}

func fmtTestName(path string, write bool) string {
	if write {
		return path + "/write"
	}
	return path + "/stdout"
}

func TestExpandFileArgs(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	require.NoError(t, os.MkdirAll("schemas/nested", 0755))
	for _, file := range []string{"schemas/a.ridl", "schemas/nested/b.ridl", "explicit.txt"} {
		require.NoError(t, os.WriteFile(file, nil, 0644))
	}

	t.Run("mixed and overlapping paths", func(t *testing.T) {
		files, err := expandFileArgs([]string{"explicit.txt", "schemas/...", "schemas/a.ridl", filepath.Join(root, "schemas", "...")})
		require.NoError(t, err)
		assert.Equal(t, []string{"explicit.txt", "schemas/a.ridl", "schemas/nested/b.ridl"}, files)
	})
	t.Run("explicit hidden file", func(t *testing.T) {
		require.NoError(t, os.WriteFile(".hidden.ridl", nil, 0644))
		files, err := expandFileArgs([]string{".hidden.ridl"})
		require.NoError(t, err)
		assert.Equal(t, []string{".hidden.ridl"}, files)
	})
	t.Run("empty directory", func(t *testing.T) {
		require.NoError(t, os.Mkdir("empty", 0755))
		for _, path := range []string{"empty", "empty/..."} {
			files, err := expandFileArgs([]string{path})
			require.NoError(t, err)
			assert.Empty(t, files)
			stdout, stderr, code := runMainSubprocess(t, []string{path}, "webrpc v1\n", true)
			assert.Equal(t, 0, code, stderr)
			assert.Empty(t, stdout)
		}
	})
	t.Run("missing root", func(t *testing.T) {
		_, err := expandFileArgs([]string{"missing/..."})
		require.Error(t, err)
		assert.ErrorContains(t, err, "expand path")
		_, stderr, code := runMainSubprocess(t, []string{"missing/..."}, "", false)
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr, "expand path")
	})
	t.Run("root is a file", func(t *testing.T) {
		_, err := expandFileArgs([]string{"schemas/a.ridl/..."})
		require.ErrorContains(t, err, "not a directory")
	})
	t.Run("symbolic links are not followed", func(t *testing.T) {
		if err := os.Symlink("nested", "schemas/link"); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		require.NoError(t, os.Symlink("a.ridl", "schemas/link.ridl"))
		files, err := expandFileArgs([]string{"schemas/..."})
		require.NoError(t, err)
		assert.Equal(t, []string{"schemas/a.ridl", "schemas/nested/b.ridl"}, files)
	})
}

const testInput string = `
      webrpc    =    v1    # version of webrpc schema format (ridl or json)
   name    = 		example # name of your backend app
	version=v0.0.1 #version of your schema

# bar
enum Intent: string
  #! foo
   - openSession
  -       closeSession

enum           Kind:            uint32
      #   user   
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
error 20 SpaceshipNotFound "Spaceship not found"       HTTP 404 #comment
error 300 Unsomething "Un what?" HTTP                      444 #comment
error 1  IAmFirst "I am first" HTTP 101 # comment

error 20         UserNotFound     "User not found" HTTP 404
error 4         UserTooYoung     ""  HTTP   404 

service ExampleService # oof
  @   deprecated   :      Pong
  	@  auth   :   ApiKeyAuth @   who    dsa   :   J    W    T   ## dadsadadsa
- Ping()
 - Status() => (status: bool)
  	@     internal   @   public   @ stringo :  " string with spaces  "   ##    multiple hashes and spaces  
  - Version() => (version: Version)
@public
   - GetUser   (   header   :    map  <   string   ,   string   >   ,   userID   :    uint64   )   =>   (  code  :   uint32   ,   user  :   User  )
    - FindUser(s :SearchFilter) => (name: string, user: User) ###! last
	- Updat  eU ser (  User     )   =>   (  User )

    -    stream    Re cv   (req  :   string   )


  -     stream    Sen  d()    =>    (resp: string)

  -stream                        Se ndAndRecv(req: string) => stream (resp: string)
  -streamSe ndAndRecv(req: string) =>          stream (resp: string)



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
  #   user
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
    + go.tag.db = username #!       far away

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
  - Ping()
    + @deprecated = Pong
    + @auth = ApiKeyAuth
    + @whodsa = "J    W    T" ## dadsadadsa
  - Status() => (status: bool)
  - Version() => (version: Version)
    + @internal
    + @public
    + @stringo = " string with spaces  " ##    multiple hashes and spaces
  - GetUser(header: map<string,string>, userID: uint64) => (code: uint32, user: User)
    + @public
  - FindUser(s: SearchFilter) => (name: string, user: User) ###! last
  - UpdateUser(User) => (User)

  - stream Recv(req: string)

  - stream Send() => (resp: string)

  - stream SendAndRecv(req: string) => stream (resp: string)
  - streamSendAndRecv(req: string) => stream (resp: string)
`
