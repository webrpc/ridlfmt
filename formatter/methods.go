package formatter

import (
	"slices"
	"strings"
)

// orderMethodBlocks sorts routes by HTTP verb, followed by method options
// and annotations. Routes with the same verb retain their written order.
// It runs after formatting so migrated and existing annotations are ordered
// together, including methods whose signature contains a legacy errors clause.
func orderMethodBlocks(input string) string {
	var output, block []string
	var state form
	inMethod := false

	flush := func() {
		output = append(output, orderMethodOptions(block)...)
		block = nil
	}

	for _, line := range strings.Split(input, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			if inMethod {
				block = append(block, line)
			} else {
				output = append(output, line)
			}
			continue
		}

		state.parseSection(trimmed)
		method := state.section == sectionField && state.topLvlSection == sectionService
		meta := state.section == sectionRoute || state.section == sectionMethodOption ||
			state.section == sectionMethodAnnotation || state.section == sectionTag
		if inMethod && !meta {
			flush()
			inMethod = false
		}
		if method {
			inMethod = true
			output = append(output, line)
		} else if inMethod {
			block = append(block, line)
		} else {
			output = append(output, line)
		}
	}
	flush()
	return strings.Join(output, "\n")
}

type methodRoute struct {
	verb  string
	lines []string
}

func orderMethodOptions(block []string) []string {
	var routes []methodRoute
	var success, errors, options, annotations, comments []string
	for _, line := range block {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			comments = append(comments, line)
			continue
		}
		// Leading comments travel with their option or annotation. Trailing
		// comments and blank lines stay at the end, above the next declaration.
		if isRouteLine(trimmed) {
			verb := strings.Fields(strings.TrimPrefix(trimmed, "+"))[0]
			routes = append(routes, methodRoute{verb: verb, lines: append(comments, line)})
		} else if name, _, ok := parseMethodOption(trimmed); ok {
			if name == "success" {
				success = append(success, comments...)
				success = append(success, line)
			} else {
				errors = append(errors, comments...)
				errors = append(errors, line)
			}
		} else if isMethodAnnotationLine(trimmed) {
			annotations = append(annotations, comments...)
			annotations = append(annotations, line)
		} else {
			options = append(options, comments...)
			options = append(options, line)
		}
		comments = nil
	}
	slices.SortStableFunc(routes, func(a, b methodRoute) int {
		return slices.Index(routeVerbs, a.verb) - slices.Index(routeVerbs, b.verb)
	})
	var result []string
	for _, route := range routes {
		result = append(result, route.lines...)
	}
	result = append(result, success...)
	result = append(result, errors...)
	result = append(result, options...)
	result = append(result, annotations...)
	return append(result, comments...)
}
