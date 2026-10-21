package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/webrpc/ridlfmt/formatter"
)

func main() {
	flagSet := flag.NewFlagSet("ridlfmt", flag.ExitOnError)
	if err := runRidlfmt(flagSet, os.Args[1:]); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func runRidlfmt(flagSet *flag.FlagSet, args []string) error {
	flag.Usage = usage

	sortErrorsFlag := flagSet.Bool("s", false, "sort errors by code")
	writeFlag := flagSet.Bool("w", false, "write output to input file (overwrites the file)")
	helpFlag := flagSet.Bool("h", false, "show help")

	if err := flagSet.Parse(args); err != nil {
		return fmt.Errorf("parse args: %w", err)
	}

	if *helpFlag {
		flag.Usage()
		os.Exit(0)
	}

	fileArgs := flagSet.Args()

	if len(fileArgs) == 0 && !isInputFromPipe() {
		fmt.Fprintln(os.Stderr, "error: no input files specified")
		flag.Usage()
		os.Exit(1)
	}

	fileArgs, err := expandFileArgs(fileArgs)
	if err != nil {
		return err
	}

	if *writeFlag {
		for _, fileName := range fileArgs {
			err := formatAndWriteToFile(fileName, *sortErrorsFlag)
			if err != nil {
				log.Fatalf("Error processing file %s: %v", fileName, err)
			}
		}
	} else {
		if len(flagSet.Args()) == 0 && isInputFromPipe() {
			err := formatAndPrintFromPipe(*sortErrorsFlag)
			if err != nil {
				log.Fatalf("Error processing input from pipe: %v", err)
			}
		} else {
			for _, fileName := range fileArgs {
				err := formatAndPrintToStdout(fileName, *sortErrorsFlag)
				if err != nil {
					log.Fatalf("Error processing file %s: %v", fileName, err)
				}
			}
		}
	}

	return nil
}

// expandFileArgs expands directories and directory/... to regular .ridl files.
// WalkDir visits files in lexical order and does not follow symbolic links.
func expandFileArgs(args []string) ([]string, error) {
	var files []string
	seen := make(map[string]bool)
	addFile := func(path string) error {
		key, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if !seen[key] {
			seen[key] = true
			files = append(files, path)
		}
		return nil
	}
	for _, arg := range args {
		root := arg
		pattern := filepath.Base(arg) == "..."
		if pattern {
			root = filepath.Dir(arg)
		}
		info, err := os.Stat(root)
		if !pattern && (err != nil || !info.IsDir()) {
			if err := addFile(arg); err != nil {
				return nil, err
			}
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("expand path %s: %w", arg, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("expand path %s: %s is not a directory", arg, root)
		}
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type().IsRegular() && !strings.HasPrefix(entry.Name(), ".") && filepath.Ext(path) == ".ridl" {
				return addFile(path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("expand path %s: %w", arg, err)
		}
	}
	return files, nil
}

func isInputFromPipe() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}

func formatAndWriteToFile(fileName string, sortErrorsFlag bool) error {
	inputBytes, err := os.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("error opening input file %s: %w", fileName, err)
	}

	output, err := formatter.Format(bytes.NewReader(inputBytes), sortErrorsFlag)
	if err != nil {
		return fmt.Errorf("error formatting input file %s: %w", fileName, err)
	}

	err = os.WriteFile(fileName, []byte(output), 0644)
	if err != nil {
		return fmt.Errorf("error writing to output file %s: %w", fileName, err)
	}

	return nil
}

func formatAndPrintToStdout(fileName string, sortErrorsFlag bool) error {
	inputBytes, err := os.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("error opening input file %s: %w", fileName, err)
	}

	output, err := formatter.Format(bytes.NewReader(inputBytes), sortErrorsFlag)
	if err != nil {
		return fmt.Errorf("error formatting input file %s: %w", fileName, err)
	}

	fmt.Println(output)

	return nil
}

func formatAndPrintFromPipe(sortErrorsFlag bool) error {
	scanner := bufio.NewScanner(os.Stdin)
	var inputBuffer bytes.Buffer
	for scanner.Scan() {
		inputBuffer.WriteString(scanner.Text())
		inputBuffer.WriteByte('\n')
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading from pipe: %w", err)
	}

	output, err := formatter.Format(&inputBuffer, sortErrorsFlag)
	if err != nil {
		return fmt.Errorf("error formatting input from pipe: %w", err)
	}

	fmt.Println(output)

	return nil
}

func usage() {
	fmt.Fprintf(os.Stderr, `usage: ridlfmt [flags] [path...]

    Directories and directory/... recursively select .ridl files.

    -h    show help
    -s    sort errors by code
    -w    write result to (source) file instead of stdout 
`)
}
