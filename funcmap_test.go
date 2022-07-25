package jaws

import (
	"testing"

	"github.com/matryer/is"
)

func Test_FuncMap(t *testing.T) {
	is := is.New(t)
	is.True(FuncMap["int"] != nil)
	is.Equal(CastInt(1.1), 1)
	is.True(FuncMap["float64"] != nil)
	is.Equal(CastFloat64(2), 2.0)
}
