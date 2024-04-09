package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/webrpc/ridlfmt/formatter"
)

func main() {
	flag.Usage = usage

	sortErrorsFlag := flag.Bool("s", false, "sort errors by code")
	writeFlag := flag.Bool("w", false, "write output to input file (overwrites the file)")
	helpFlag := flag.Bool("h", false, "show help")

	flag.Parse()

	args := flag.Args()

	if *helpFlag {
		flag.Usage()
		os.Exit(0)
	}

	if len(args) == 0 && !isInputFromPipe() {
		fmt.Fprintln(os.Stderr, "error: no input files specified")
		flag.Usage()
		os.Exit(1)
	}

	if *writeFlag {
		for _, fileName := range args {
			err := formatAndWriteToFile(fileName, *sortErrorsFlag)
			if err != nil {
				log.Fatalf("Error processing file %s: %v", fileName, err)
			}
		}
	} else {
		if isInputFromPipe() {
			err := formatAndPrintFromPipe(*sortErrorsFlag)
			if err != nil {
				log.Fatalf("Error processing input from pipe: %v", err)
			}
		} else {
			for _, fileName := range args {
				err := formatAndPrintToStdout(fileName, *sortErrorsFlag)
				if err != nil {
					log.Fatalf("Error processing file %s: %v", fileName, err)
				}
			}
		}
	}
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

    -h    show help
    -s    sort errors by code
    -w    write result to (source) file instead of stdout 
`)
}
