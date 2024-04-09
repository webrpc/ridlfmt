package formatter

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func (f *form) processLines(inputFile io.Reader) (string, error) {
	scanner := bufio.NewScanner(inputFile)

	var output strings.Builder
	var line string
	var prevSec section
	var err error

	for scanner.Scan() {
		line, err = f.formatLine(scanner.Text())
		if err != nil {
			return "", fmt.Errorf("format: %w", err)
		}

		if f.section == sectionUnknown {
			return "", fmt.Errorf("unknown section")
		}

		if f.section == sectionEmpty {
			output.WriteString(f.commentsPrint())
			if prevSec == sectionError {
				output.WriteString(f.errorsPrint() + "\n")
			}

			output.WriteString(line + "\n")
			prevSec = f.section

			continue
		}

		if f.section == sectionError && prevSec != sectionError {
			output.WriteString(f.commentsPrint())
		}

		if prevSec == sectionError && f.section != sectionError {
			output.WriteString(f.errorsPrint() + "\n")
			output.WriteString(f.commentsPrint())
		}

		if f.section != sectionComment && f.section != sectionError {
			output.WriteString(f.commentsPrint())
			output.WriteString(line + "\n")
		}

		prevSec = f.section
	}

	output.WriteString(f.errorsPrint() + "\n")
	output.WriteString(f.commentsPrint())

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading input file: %w", err)
	}

	return output.String(), nil
}

func (f *form) formatLine(line string) (string, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		f.section = sectionEmpty
		return line, nil
	}

	switch {
	case strings.HasPrefix(line, "webrpc"):
		f.section = sectionWebRPC
		f.topLvlSection = sectionWebRPC
	case strings.HasPrefix(line, "name"):
		f.section = sectionName
		f.topLvlSection = sectionName
	case strings.HasPrefix(line, "version"):
		f.section = sectionVersion
		f.topLvlSection = sectionVersion
	case strings.HasPrefix(line, "#"):
		f.section = sectionComment
	case strings.HasPrefix(line, "enum"):
		f.section = sectionEnum
		f.topLvlSection = sectionEnum
	case strings.HasPrefix(line, "struct"):
		f.section = sectionStruct
		f.topLvlSection = sectionStruct
	case strings.HasPrefix(line, "service"):
		f.section = sectionService
		f.topLvlSection = sectionService
	case strings.HasPrefix(line, "error"):
		f.section = sectionError
		f.topLvlSection = sectionError
	case strings.HasPrefix(line, "-"):
		f.section = sectionField
	case strings.HasPrefix(line, "+"):
		f.section = sectionTag
	default:
		f.section = sectionUnknown
	}

	switch f.section {
	case sectionComment:
		f.comments = append(f.comments, parseComment(line))
	case sectionEnum:
		f.padding = 0
		line = reduceSpaces(line)
		parts := strings.Split(line, ":")
		if len(parts) == 2 {
			p1 := strings.TrimSpace(parts[0])
			p2 := strings.TrimSpace(parts[1])
			line = fmt.Sprintf("%s: %s", p1, p2)
		}
	case sectionStruct:
		f.padding = 0
		line = reduceSpaces(line)
	case sectionService:
		f.padding = 0
		line = reduceSpaces(line)
	case sectionError:
		f.padding = 0
		errParts := strings.Split(line, `"`)

		e := ridlError{}
		if len(errParts) == 3 {
			errParts[0] = strings.TrimSpace(errParts[0])
			errParts[0] = reduceSpaces(errParts[0])

			parts := strings.Split(errParts[0], " ")
			code, err := strconv.Atoi(parts[1])
			if err != nil {
				return line, err
			}

			e.code = code
			e.name = parts[2]
			e.description = errParts[1]

			errorEnding := reduceSpaces(strings.TrimSpace(errParts[2]))
			parts = strings.Split(errorEnding, " ")
			httpCode, err := strconv.Atoi(parts[1])
			if err != nil {
				return line, err
			}

			e.inlineComment = parseComment(errorEnding)
			e.httpCode = httpCode
		}

		if f.section == sectionError {
			f.errors = append(f.errors, e)
		}
	case sectionField:
		f.padding = 2
		line = reduceSpaces(line)
		switch f.topLvlSection {
		case sectionStruct, sectionEnum:
			s, c := parseAndDivideInlineComment(line)
			parts := strings.Split(s, ":")
			if len(parts) == 2 {
				p1 := strings.TrimSpace(parts[0])
				p2 := strings.TrimSpace(parts[1])
				line = fmt.Sprintf("%s: %s", p1, p2)
			}

			line = fmt.Sprintf("%s%s", strings.Repeat(" ", f.padding), line)
			line = c.appendInlineComment(line)
		case sectionService:
			s, c := parseAndDivideInlineComment(line)
			parts := strings.Split(s, "=>")
			if len(parts) == 2 {
				p1 := strings.TrimSpace(parts[0])
				p2 := strings.TrimSpace(parts[1])
				line = fmt.Sprintf("%s => %s", p1, p2)
			}

			line = fmt.Sprintf("%s%s", strings.Repeat(" ", f.padding), line)
			line = c.appendInlineComment(line)
		default:
			return "", fmt.Errorf("wrong top level for field %s", line)
		}
	case sectionTag:
		f.padding = 4
		s, c := parseAndDivideInlineComment(line)
		parts := strings.Split(s, "=")
		if len(parts) == 2 {
			p1 := reduceSpaces(strings.TrimSpace(parts[0]))
			p2 := strings.TrimSpace(parts[1])
			line = fmt.Sprintf("%s = %s", p1, p2)
		}

		line = fmt.Sprintf("%s%s", strings.Repeat(" ", f.padding), line)
		line = c.appendInlineComment(line)
	default:
	}

	if f.section != sectionComment {
		s, c := parseAndDivideInlineComment(line)
		line = c.appendInlineComment(s)
	}

	return line, nil
}

func (f *form) removeDoubleLines(s string) string {
	var modifiedLines []string
	var emptyLine bool

	for _, line := range strings.Split(s, "\n") {
		if line != "" || !emptyLine {
			modifiedLines = append(modifiedLines, line)
			emptyLine = line == ""
		}
	}

	return strings.Join(modifiedLines, "\n")
}
