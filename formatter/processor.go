package formatter

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type form struct {
	padding       int
	comments      []*comment
	errors        ridlErrors
	sortErrors    bool
	section       section
	topLvlSection section
}

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

	f.parseSection(line)

	switch f.section {
	case sectionWebRPC:
		fallthrough
	case sectionName:
		fallthrough
	case sectionVersion:
		f.padding = 0
		line = reduceSpaces(line)
		s, c := parseAndDivideInlineComment(line)
		parts := strings.Split(s, "=")
		if len(parts) != 2 {
			return "", fmt.Errorf("unexpected amount of parts=(%d) %s", len(parts), line)
		}

		line = fmt.Sprintf("%s = %s", removeSpaces(parts[0]), removeSpaces(parts[1]))

		line = c.appendInlineComment(line)

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
	case sectionImport:
		f.padding = 0
		line = reduceSpaces(line)
	case sectionError:
		f.padding = 0
		partsLine := strings.Split(line, `"`)

		if len(partsLine) != 3 {
			return "", fmt.Errorf("wrong error format line=(%s)", line)
		}

		partsLine[0] = strings.TrimSpace(partsLine[0])
		partsLine[0] = reduceSpaces(partsLine[0])

		partsBegin := strings.Split(partsLine[0], " ")
		if len(partsBegin) != 3 {
			return "", fmt.Errorf("wrong error format line=(%s)", line)
		}

		code, err := strconv.Atoi(partsBegin[1])
		if err != nil {
			return "", fmt.Errorf("strconv error code: %w", err)
		}

		errorEnding := reduceSpaces(strings.TrimSpace(partsLine[2]))
		partsEnd := strings.Split(strings.TrimSpace(strings.Split(errorEnding, "#")[0]), " ")
		if len(partsEnd) != 2 {
			return "", fmt.Errorf("wrong format of end of an error =(%s)", errorEnding)
		}

		httpCode, err := strconv.Atoi(strings.Split(partsEnd[1], "#")[0])
		if err != nil {
			return "", fmt.Errorf("strconv http code: %w", err)
		}

		e := ridlError{
			code:          code,
			name:          partsBegin[2],
			description:   partsLine[1],
			httpCode:      httpCode,
			inlineComment: parseComment(errorEnding),
		}

		f.errors = append(f.errors, e)
	case sectionField:
		f.padding = 2
		line = reduceSpaces(line)
		switch f.topLvlSection {
		case sectionEnum:
			s, c := parseAndDivideInlineComment(line)
			p := strings.TrimSpace(strings.Split(strings.TrimSpace(s), "-")[1])
			line = fmt.Sprintf("%s- %s", strings.Repeat(" ", f.padding), p)
			line = c.appendInlineComment(line)
		case sectionStruct:
			s, c := parseAndDivideInlineComment(line)
			parts := strings.Split(s, ":")
			p1 := strings.TrimSpace(strings.Split(strings.TrimSpace(parts[0]), "-")[1])
			p2 := strings.TrimSpace(parts[1])
			line = fmt.Sprintf("%s- %s: %s", strings.Repeat(" ", f.padding), p1, p2)
			line = c.appendInlineComment(line)
		case sectionService:
			s, c := parseAndDivideInlineComment(line)
			parts := strings.Split(s, "=>")
			p1 := strings.TrimSpace(parts[0])

			methodName := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.Split(p1, "(")[0]), "-"))
			methodName, isStreamInput := strings.CutPrefix(methodName, "stream ")
			methodName = removeSpaces(methodName)

			if isStreamInput {
				methodName = "stream " + methodName
			}

			inArgs, err := formatMethodArguments(p1)
			if err != nil {
				return line, err
			}

			line = fmt.Sprintf("%s- %s(%s)", strings.Repeat(" ", f.padding), methodName, inArgs)

			if len(parts) == 2 {
				p2 := strings.TrimSpace(parts[1])
				_, stream := strings.CutPrefix(p2, "stream")
				outArgs, err := formatMethodArguments(p2)
				if err != nil {
					return line, err
				}

				line += " => "
				if stream {
					line += "stream "
				}

				line += "(" + outArgs + ")"
			}

			line = c.appendInlineComment(line)
		case sectionImport:
			s, c := parseAndDivideInlineComment(line)
			parts := strings.Split(s, ":")
			if len(parts) == 2 {
				p1 := strings.TrimSpace(parts[0])
				p2 := strings.TrimSpace(parts[1])
				s = fmt.Sprintf("%s: %s", p1, p2)
			}

			line = fmt.Sprintf("%s%s", strings.Repeat(" ", f.padding), s)
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
	case sectionAnnotation:
		f.padding = 4
		s, c := parseAndDivideInlineComment(line)

		parts := strings.Split(s, "@")
		if len(parts) < 2 {
			return "", fmt.Errorf("unexpected amount of parts=(%d)", len(parts))
		}

		var as string
		for i := 1; i < len(parts); i++ {
			ap := strings.Split(parts[i], ":")
			if i > 1 {
				as += " "
			}

			switch len(ap) {
			case 1:
				as = fmt.Sprintf("%s@%s", as, removeSpaces(ap[0]))
			case 2:
				as = fmt.Sprintf("%s@%s:%s", as, removeSpaces(ap[0]), removeSpaces(ap[1]))
			default:
				return "", fmt.Errorf("unexpected amount of parts for one anotation parts=(%d) %s", len(ap), line)
			}
		}

		line = fmt.Sprintf("%s%s", strings.Repeat(" ", f.padding), as)
		line = c.appendInlineComment(line)
	default:
	}

	if f.section != sectionComment {
		s, c := parseAndDivideInlineComment(line)
		line = c.appendInlineComment(s)
	}

	return line, nil
}

func (f *form) parseSection(line string) {
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
	case strings.HasPrefix(line, "import"):
		f.section = sectionImport
		f.topLvlSection = sectionImport
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
	case strings.HasPrefix(line, "@"):
		f.section = sectionAnnotation
	default:
		f.section = sectionUnknown
	}
}

func (f *form) commentsPrint() string {
	var lines string
	for _, c := range f.comments {
		lines += fmt.Sprintf("%s%s\n", strings.Repeat(" ", f.padding), c.getString())
	}

	f.comments = nil

	return lines
}

func (f *form) errorsPrint() string {
	codeLen, nameLen, descLen, httpLen := f.errors.getLenghts()

	if f.sortErrors {
		sort.Sort(f.errors)
	}

	var lines string
	for i, err := range f.errors {
		lines += fmt.Sprintf("error %-*d %-*s \"%s\"%s HTTP %-*d",
			codeLen,
			err.code,
			nameLen,
			err.name,
			err.description,
			strings.Repeat(" ", descLen-len(err.description)),
			httpLen,
			err.httpCode,
		)

		if err.inlineComment != nil {
			lines += fmt.Sprintf(" %s", err.inlineComment.getString())
		}

		if i != len(f.errors)-1 {
			lines += "\n"
		}
	}

	f.errors = nil

	return lines
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

func reduceSpaces(input string) string {
	pattern := regexp.MustCompile(`\s+`)
	return pattern.ReplaceAllString(input, " ")
}

func removeSpaces(input string) string {
	return strings.ReplaceAll(input, " ", "")
}

func formatMethodArguments(s string) (string, error) {
	content, err := extractFromParenthesis(s)
	if err != nil {
		return "", fmt.Errorf("extract from parenthesis: %w", err)
	}

	args := splitArguments(content)
	for i, a := range args {
		p := strings.Split(a, ":")
		if len(p) != 2 {
			return "", fmt.Errorf("missing ':' in arguments for method")
		}

		args[i] = fmt.Sprintf("%s: %s", p[0], p[1])
	}

	return strings.Join(args, ", "), nil
}

func extractFromParenthesis(s string) (string, error) {
	start := strings.Index(s, "(")
	end := strings.LastIndex(s, ")")
	if start == -1 || end == -1 {
		return "", fmt.Errorf("missing '(' or ')'")
	}

	return strings.TrimSpace(s[start+1 : end]), nil
}

func splitArguments(s string) []string {
	s = removeSpaces(strings.TrimSpace(s))
	var parts []string
	var ic, im int

	for len(s) != 0 {
		ic = strings.Index(s, ",")
		im = strings.Index(s, "map<")

		if ic < im || im == -1 {
			p := strings.SplitN(s, ",", 2)
			if len(p) == 2 {
				s = p[1]
				parts = append(parts, p[0])
			} else {
				parts = append(parts, s)
				break
			}
		} else {
			s = removeSpaces(s)
			c, more := findComma(ic, s)
			if more {
				parts = append(parts, s[:c])
				s = s[c+1:]
			} else {
				parts = append(parts, s)
				s = ""
			}
		}
	}

	return parts
}

func findComma(ic int, s string) (int, bool) {
	c := strings.Index(s[ic+1:], ",")
	m := strings.Index(s[ic+1:], "map<")
	if c < m || m == -1 {
		if c == -1 {
			return c + ic + 1, false
		}

		return c + ic + 1, true
	}

	return findComma(ic+c+1, s)
}
