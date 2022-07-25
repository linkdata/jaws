package jaws

import (
	"testing"

	"github.com/matryer/is"
)

func Test_Keygen(t *testing.T) {
	is := is.New(t)
	kg := NewKeygen()
	is.Equal(false, kg.IsUsed())
	a := kg.Int63()
	is.Equal(true, kg.IsUsed())
	b := kg.Int63()
	is.True(a != b)
	kg.Reseed()
	is.Equal(false, kg.IsUsed())
	c := kg.Int63()
	is.Equal(true, kg.IsUsed())
	is.True(c != a)
	is.True(c != b)
}
