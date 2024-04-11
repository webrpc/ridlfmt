package formatter

import (
	"fmt"
	"io"
)

func Format(inputFile io.Reader, sortErrors bool) (string, error) {
	f := form{
		sortErrors: sortErrors,
	}

	output, err := f.processLines(inputFile)
	if err != nil {
		return "", fmt.Errorf("process lines: %w", err)
	}

	output = f.removeDoubleLines(output)

	return output, nil
}
