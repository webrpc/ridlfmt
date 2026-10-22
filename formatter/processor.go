package formatter

import (
	"bufio"
	"fmt"
	"io"
	"slices"
	"sort"
	"strconv"
	"strings"
)

type form struct {
	padding            int
	comments           []*comment
	errors             ridlErrors
	sortErrors         bool
	section            section
	topLvlSection      section
	pending            []pendingAnnotation
	annotationComments []*comment
	// trailer holds comment and blank lines seen after a stashed annotation;
	// they are written out where the annotation block lands.
	trailer []string
}

// pendingAnnotation holds above-method annotations until the following line
// tells us whether they belong to a service method (migrated to `+ @...`
// lines below it) or not (flushed in the old form).
type pendingAnnotation struct {
	old   string
	lines []string
}

func newPendingAnnotation(tokens []string, c *comment, padding int) pendingAnnotation {
	indent := strings.Repeat(" ", padding)

	p := pendingAnnotation{
		old: c.appendInlineComment(indent + strings.Join(tokens, " ")),
	}
	for _, t := range tokens {
		p.lines = append(p.lines, fmt.Sprintf("%s+ %s", indent, formatAnnotationToken(t)))
	}
	// An inline comment on the source line belongs to its last annotation.
	p.lines[len(p.lines)-1] = c.appendInlineComment(p.lines[len(p.lines)-1])

	return p
}

func (f *form) processLines(inputFile io.Reader) (string, error) {
	scanner := bufio.NewScanner(inputFile)

	var output strings.Builder
	var line string
	var prevSec section
	var err error

	for scanner.Scan() {
		pendingBefore := len(f.pending) > 0

		line, err = f.formatLine(scanner.Text())
		if err != nil {
			return "", fmt.Errorf("format: %w", err)
		}

		if f.section == sectionUnknown {
			return "", fmt.Errorf("unknown section")
		}

		// formatLine moved the stashed annotations below the method line.
		migrated := pendingBefore && len(f.pending) == 0

		// Comments after a stashed annotation wait with it; the annotation may
		// still move below the method it belongs to.
		if len(f.pending) > 0 && f.section == sectionComment {
			f.holdComments()
		}

		if f.section == sectionEmpty {
			if len(f.pending) > 0 {
				f.holdBlank()
				prevSec = f.section

				continue
			}

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
			output.WriteString(f.pendingPrint())
		}

		if prevSec == sectionError && f.section != sectionError {
			output.WriteString(f.errorsPrint() + "\n")
			output.WriteString(f.commentsPrint())
			output.WriteString(f.pendingPrint())
		}

		// Annotations below a method line are written as-is; above-method ones
		// under a service are stashed until we see the method they belong to.
		stashed := f.section == sectionAnnotation && f.topLvlSection == sectionService
		if !stashed && f.section != sectionComment && f.section != sectionError {
			output.WriteString(f.commentsPrint())
			if migrated {
				output.WriteString(line + "\n")
				output.WriteString(f.trailerPrint())
			} else {
				output.WriteString(f.pendingPrint())

				output.WriteString(line + "\n")
			}
		}

		prevSec = f.section
	}

	// The blank line after the error block separates it from whatever follows.
	errors := f.errorsPrint()
	if errors != "" {
		errors += "\n"
	}

	output.WriteString(f.pendingPrint())
	output.WriteString(errors)
	output.WriteString(f.commentsPrint())

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading input file: %w", err)
	}

	return output.String(), nil
}

func (f *form) formatLine(line string) (string, error) {
	var prefix string
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
		fallthrough
	case sectionBasepath:
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
	case sectionTypeAlias:
		s, c := parseAndDivideInlineComment(line)
		name, typ, ok := strings.Cut(strings.TrimPrefix(s, "type"), ":")
		if !ok || strings.TrimSpace(name) == "" || strings.TrimSpace(typ) == "" {
			return "", fmt.Errorf("wrong type alias format: %s", line)
		}
		f.padding = 0
		line = c.appendInlineComment("type " + strings.TrimSpace(name) + ": " + removeSpaces(typ))
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
		s, c := parseAndDivideInlineComment(line)
		partsLine := strings.Split(s, `"`)

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
		httpPart := strings.TrimSpace(strings.Split(errorEnding, "#")[0])

		// The HTTP status is optional; webrpc defaults it to 400.
		var httpCode int
		if httpPart != "" {
			partsEnd := strings.Split(httpPart, " ")
			if len(partsEnd) != 2 {
				return "", fmt.Errorf("wrong format of end of an error =(%s)", errorEnding)
			}

			httpCode, err = strconv.Atoi(partsEnd[1])
			if err != nil {
				return "", fmt.Errorf("strconv http code: %w", err)
			}
		}

		e := ridlError{
			code:          code,
			name:          partsBegin[2],
			description:   partsLine[1],
			httpCode:      httpCode,
			inlineComment: c,
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
			s, methodErrors, err := splitMethodErrors(s)
			if err != nil {
				return "", err
			}
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

			if len(f.pending) > 0 {
				prefix = f.annotationCommentsPrint(2)
				line += "\n" + strings.Join(f.takePendingNew(), "\n")
			}
			if methodErrors != "" {
				line += "\n" + formatMethodOption("error", methodErrors, 4)
			}
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
	case sectionRoute:
		f.padding = 4
		s, c := parseAndDivideInlineComment(line)
		parts := strings.Fields(strings.TrimPrefix(s, "+"))
		line = fmt.Sprintf("%s+ %s %s", strings.Repeat(" ", f.padding), parts[0], parts[1])
		line = c.appendInlineComment(line)
	case sectionMethodOption:
		f.padding = 4
		s, c := parseAndDivideInlineComment(line)
		name, value, _ := parseMethodOption(s)
		line = c.appendInlineComment(formatMethodOption(name, value, f.padding))
	case sectionMethodAnnotation:
		f.padding = 4
		line = formatMethodAnnotation(line, f.padding)
	case sectionAnnotation:
		f.padding = 4
		s, c := parseAndDivideInlineComment(line)

		tokens := parseAnnotationTokens(s)

		if f.topLvlSection == sectionService {
			if len(f.pending) == 0 {
				f.annotationComments = append(f.annotationComments, f.comments...)
				f.comments = nil
			}
			f.pending = append(f.pending, newPendingAnnotation(tokens, c, f.padding))
			return "", nil
		}

		line = fmt.Sprintf("%s%s", strings.Repeat(" ", f.padding), strings.Join(tokens, " "))
		line = c.appendInlineComment(line)
	default:
	}

	if f.section != sectionComment {
		s, c := parseAndDivideInlineComment(line)
		line = c.appendInlineComment(s)
	}

	return prefix + line, nil
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
	case strings.HasPrefix(line, "basepath"):
		f.section = sectionBasepath
		f.topLvlSection = sectionBasepath
	case strings.HasPrefix(line, "import"):
		f.section = sectionImport
		f.topLvlSection = sectionImport
	case strings.HasPrefix(line, "#"):
		f.section = sectionComment
	case strings.HasPrefix(line, "enum"):
		f.section = sectionEnum
		f.topLvlSection = sectionEnum
	case strings.HasPrefix(line, "type ") || strings.HasPrefix(line, "type\t"):
		f.section = sectionTypeAlias
		f.topLvlSection = sectionTypeAlias
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
	case strings.HasPrefix(line, "+") && f.topLvlSection == sectionService && isRouteLine(line):
		f.section = sectionRoute
	case strings.HasPrefix(line, "+") && f.topLvlSection == sectionService && isMethodOptionLine(line):
		f.section = sectionMethodOption
	case strings.HasPrefix(line, "+") && f.topLvlSection == sectionService && isMethodAnnotationLine(line):
		f.section = sectionMethodAnnotation
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
		line := fmt.Sprintf("error %-*d %-*s \"%s\"",
			codeLen,
			err.code,
			nameLen,
			err.name,
			err.description,
		)
		if err.httpCode != 0 {
			line += fmt.Sprintf("%s HTTP %-*d", strings.Repeat(" ", descLen-len(err.description)), httpLen, err.httpCode)
		}
		lines += line

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

	for line := range strings.SplitSeq(s, "\n") {
		if line != "" || !emptyLine {
			modifiedLines = append(modifiedLines, line)
			emptyLine = line == ""
		}
	}

	return strings.Join(modifiedLines, "\n")
}

func reduceSpaces(input string) string {
	// Find the position of the comment-starting '#' character, if any.
	hashIndex := findCommentIndex(input)
	if hashIndex == -1 {
		// If there's no comment, just reduce spaces in the entire string
		return reduceSpacesInString(input)
	}

	// Separate the part before and after the '#'
	beforeHash := input[:hashIndex]
	afterHash := input[hashIndex:]

	// Reduce spaces in the part before the '#'
	reduced := reduceSpacesInString(beforeHash)
	if reduced != "" {
		reduced += " "
	}

	return reduced + afterHash
}

func reduceSpacesInString(s string) string {
	// Split the string and join using a single space
	parts := strings.Fields(s)
	return strings.Join(parts, " ")
}

func removeSpaces(input string) string {
	return strings.NewReplacer(" ", "", "\t", "").Replace(input)
}

func formatMethodArguments(s string) (string, error) {
	content, err := extractFromParenthesis(s)
	if err != nil {
		return "", fmt.Errorf("extract from parenthesis: %w", err)
	}

	args := splitArguments(content)
	for i, a := range args {
		p := strings.Split(a, ":")
		switch len(p) {
		case 1:
			args[i] = p[0]
		case 2:
			args[i] = fmt.Sprintf("%s: %s", p[0], p[1])
		default:
			return "", fmt.Errorf("wrong parameter values")
		}
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

func (f *form) pendingPrint() string {
	if len(f.pending) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(f.annotationCommentsPrint(4))
	for _, p := range f.pending {
		b.WriteString(p.old + "\n")
	}
	f.pending = nil

	return b.String() + f.trailerPrint()
}

// Comments preceding an annotation block stay above the destination method.
// If the block has no following method, they retain the old indentation.
func (f *form) annotationCommentsPrint(padding int) string {
	var b strings.Builder
	for _, c := range f.annotationComments {
		b.WriteString(strings.Repeat(" ", padding) + c.getString() + "\n")
	}
	f.annotationComments = nil
	return b.String()
}

func (f *form) takePendingNew() []string {
	var lines []string
	for _, p := range f.pending {
		lines = append(lines, p.lines...)
	}
	f.pending = nil

	return lines
}

// holdComments parks comments that follow a stashed annotation, so they keep
// the annotation's indentation and travel with it.
func (f *form) holdComments() {
	for _, c := range f.comments {
		f.trailer = append(f.trailer, strings.Repeat(" ", f.padding)+c.getString())
	}

	f.comments = nil
}

// holdBlank parks a blank line that follows a stashed annotation.
func (f *form) holdBlank() {
	f.trailer = append(f.trailer, "")
}

// trailerPrint writes the parked lines below the old-form annotation block.
func (f *form) trailerPrint() string {
	if len(f.trailer) == 0 {
		return ""
	}

	s := strings.Join(f.trailer, "\n") + "\n"
	f.trailer = nil

	return s
}

// parseAnnotationTokens splits the deprecated above-method form into one
// normalized `@name[:value]` token per annotation. The value stays raw so the
// formatter can quote it later; space-stripping here would corrupt multi-word
// values before the quoter sees them. The name/value split is on the first
// colon only, since values may contain colons.
func parseAnnotationTokens(s string) []string {
	parts := splitAboveMethodAnnotations(s)

	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		name, value, hasValue := strings.Cut(strings.TrimPrefix(part, "@"), ":")
		name = removeSpaces(name)

		if !hasValue {
			tokens = append(tokens, "@"+name)

			continue
		}

		tokens = append(tokens, fmt.Sprintf("@%s:%s", name, strings.TrimSpace(value)))
	}

	return tokens
}

// splitAboveMethodAnnotations respects quoted strings and word tokens, where
// '@' is part of the value (for example an email address), just like the RIDL
// lexer. Only an '@' at a token boundary starts another annotation.
func splitAboveMethodAnnotations(s string) []string {
	var parts []string
	start := -1
	inQuote, escaped, inWord := false, false, false
	for i, r := range s {
		if inQuote {
			if escaped {
				escaped = false
			} else if r == '\\' {
				escaped = true
			} else if r == '"' {
				inQuote = false
			}
			continue
		}
		if isWordBreak(r) {
			inWord = false
			inQuote = r == '"'
			continue
		}
		if inWord {
			continue
		}
		if r == '@' {
			if start >= 0 {
				parts = append(parts, strings.TrimSpace(s[start:i]))
			}
			start = i
		} else if isWordBeginning(r) {
			inWord = true
		}
	}
	if start >= 0 {
		parts = append(parts, strings.TrimSpace(s[start:]))
	}
	return parts
}

// formatMethodAnnotation normalizes a `+ @name[ = value]` meta-line. webrpc
// rejects more than one annotation per line, so `+ @a @b` is split.
func formatMethodAnnotation(line string, padding int) string {
	s, c := parseAndDivideInlineComment(line)
	body := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "+"))

	indent := strings.Repeat(" ", padding)
	lines := make([]string, 0, 1)
	for _, token := range splitMethodAnnotations(body) {
		lines = append(lines, indent+"+ "+formatAnnotationToken(token))
	}

	return c.appendInlineComment(strings.Join(lines, "\n"))
}

// splitMethodAnnotations splits a `+ @a @b` body into one token per
// annotation. A value after ':' or '=' runs to the end of the line, so only a
// value-less annotation can be followed by another '@'.
func splitMethodAnnotations(body string) []string {
	var tokens []string

	for len(body) > 0 {
		body = strings.TrimLeft(body, " \t")
		if !strings.HasPrefix(body, "@") {
			break
		}

		body = body[1:]

		i := strings.IndexAny(body, ": \t=")
		if i < 0 {
			tokens = append(tokens, "@"+body)

			break
		}

		name := body[:i]
		rest := strings.TrimLeft(body[i:], " \t")
		if rest != "" && (rest[0] == ':' || rest[0] == '=') {
			tokens = append(tokens, "@"+name+rest)

			break
		}

		if !strings.HasPrefix(rest, "@") {
			// Legacy `+ @success 200` accepts a space-separated value.
			tokens = append(tokens, "@"+name+" "+rest)

			break
		}

		tokens = append(tokens, "@"+name)
		body = rest
	}

	return tokens
}

// formatAnnotationToken formats one annotation token as `@name` or
// `@name = value`. A colon- or space-separated value is normalized to '=',
// except for legacy `@success`, which becomes a `success` option.
func formatAnnotationToken(token string) string {
	body := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(token), "@"))

	i := strings.IndexAny(body, ": \t=")
	if i < 0 {
		if body == "success" {
			return "success"
		}
		return "@" + removeSpaces(body)
	}

	name := removeSpaces(body[:i])
	rest := strings.TrimLeft(body[i:], " \t")
	if rest != "" && (rest[0] == ':' || rest[0] == '=') {
		rest = rest[1:]
	}

	// The former route option `+ @success` is now `+ success`.
	if name == "success" {
		rest = strings.TrimSpace(rest)
		if value, err := strconv.Unquote(rest); err == nil {
			rest = value
		}
		return strings.TrimSpace("success " + removeSpaces(rest))
	}

	// `+ @a =` is a parse error, so an empty value collapses to a bare name.
	value := strings.TrimSpace(rest)
	if value == "" {
		return "@" + name
	}

	return "@" + name + " = " + formatAnnotationValue(value)
}

// formatAnnotationValue renders an annotation value so webrpc re-reads it
// exactly. A bare value is a single word; a multi-word one must be quoted, but
// only when it holds no quote or backslash: webrpc cannot represent those in a
// quoted value, so such values are left for the parser to reject on its own.
func formatAnnotationValue(value string) string {
	if len(value) > 1 && strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		return value
	}

	if strings.ContainsAny(value, " \t") {
		if strings.ContainsAny(value, `"\`) {
			return value
		}

		return `"` + value + `"`
	}

	return value
}

var routeVerbs = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "QUERY"}

// isRouteLine matches a method's REST route tag, e.g. "+ GET /{userId}".
func isRouteLine(line string) bool {
	parts := strings.Fields(strings.TrimPrefix(line, "+"))
	return len(parts) >= 2 && slices.Contains(routeVerbs, parts[0]) && strings.HasPrefix(parts[1], "/")
}

// isMethodAnnotationLine matches a method's `+ @name[ = value]` meta-line.
func isMethodAnnotationLine(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(line, "+")), "@")
}

// parseMethodOption recognizes reserved method option names, not annotations
// such as @error or longer names such as successful.
func parseMethodOption(line string) (name, value string, ok bool) {
	body := strings.TrimSpace(strings.TrimPrefix(line, "+"))
	parts := strings.SplitN(reduceSpacesInString(body), " ", 2)
	name = parts[0]
	if name != "success" && name != "error" {
		return "", "", false
	}
	if len(parts) == 2 {
		value = parts[1]
	}
	return name, value, true
}

func isMethodOptionLine(line string) bool {
	_, _, ok := parseMethodOption(line)
	return ok
}

// Both lists use a compact bar separator. Keep names, duplicates and order so
// schema validation can report invalid declarations without losing information.
func formatMethodOption(name, value string, padding int) string {
	values := strings.Split(value, "|")
	for i := range values {
		values[i] = reduceSpacesInString(values[i])
	}
	return strings.Repeat(" ", padding) + strings.TrimSpace("+ "+name+" "+strings.Join(values, "|"))
}

// splitMethodErrors migrates the removed signature suffix to a + error line.
// Previously formatting silently discarded everything after the final ')'.
func splitMethodErrors(signature string) (string, string, error) {
	end := strings.LastIndex(signature, ")")
	if end < 0 {
		return signature, "", nil // Argument formatting reports missing parentheses.
	}
	suffix := strings.TrimSpace(signature[end+1:])
	if suffix == "" {
		return signature, "", nil
	}
	parts := strings.SplitN(reduceSpacesInString(suffix), " ", 2)
	if parts[0] != "errors" {
		return "", "", fmt.Errorf("unexpected method suffix: %s", suffix)
	}
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("missing error names after method errors clause")
	}
	return signature[:end+1], parts[1], nil
}
