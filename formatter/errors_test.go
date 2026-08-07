package formatter

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRidlErrors_LenLessSwap(t *testing.T) {
	errs := ridlErrors{
		{code: 20, name: "B"},
		{code: 1, name: "A"},
	}

	assert.Equal(t, 2, errs.Len())
	assert.True(t, errs.Less(1, 0))
	assert.False(t, errs.Less(0, 1))

	errs.Swap(0, 1)
	assert.Equal(t, "A", errs[0].name)
	assert.Equal(t, "B", errs[1].name)
}

func TestRidlErrors_Sort(t *testing.T) {
	errs := ridlErrors{
		{code: 300, name: "C"},
		{code: 1, name: "A"},
		{code: 20, name: "B"},
	}

	sort.Sort(errs)

	assert.Equal(t, []int{1, 20, 300}, []int{errs[0].code, errs[1].code, errs[2].code})
}

func TestRidlErrors_GetLenghts(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		var errs ridlErrors
		codeLen, nameLen, descLen, httpLen := errs.getLenghts()
		assert.Equal(t, 0, codeLen)
		assert.Equal(t, 0, nameLen)
		assert.Equal(t, 0, descLen)
		assert.Equal(t, 0, httpLen)
	})

	t.Run("multiple errors", func(t *testing.T) {
		errs := ridlErrors{
			{code: 1, name: "Short", description: "d", httpCode: 404},
			{code: 300, name: "LongerName", description: "a longer description", httpCode: 1},
		}

		codeLen, nameLen, descLen, httpLen := errs.getLenghts()
		assert.Equal(t, 3, codeLen)
		assert.Equal(t, len("LongerName"), nameLen)
		assert.Equal(t, len("a longer description"), descLen)
		assert.Equal(t, 3, httpLen)
	})
}
