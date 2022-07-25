package jaws

import (
	"testing"

	"github.com/matryer/is"
)

func Test_NamedBoolArray(t *testing.T) {
	is := is.New(t)
	a := NewNamedBoolArray()
	is.Equal(len(*a), 0)
	a.Add("1", "one")
	is.Equal(len(*a), 1)
	is.Equal((*a)[0].Value, "1")
	is.Equal((*a)[0].Text, "one")
	is.Equal((*a)[0].Checked, false)
	is.Equal(a.String(), `&NamedBoolArray{&{"1","one",false}}`)

	a.Check("1")
	is.Equal((*a)[0].Value, "1")
	is.Equal((*a)[0].Text, "one")
	is.Equal((*a)[0].Checked, true)
	a.Add("2", "two")
	a.Add("2", "also two")
	is.Equal(len(*a), 3)
	is.Equal((*a)[0].Value, "1")
	is.Equal((*a)[0].Text, "one")
	is.Equal((*a)[0].Checked, true)
	is.Equal((*a)[1].Value, "2")
	is.Equal((*a)[1].Text, "two")
	is.Equal((*a)[1].Checked, false)
	is.Equal((*a)[2].Value, "2")
	is.Equal((*a)[2].Text, "also two")
	is.Equal((*a)[2].Checked, false)
	is.Equal(a.String(), `&NamedBoolArray{&{"1","one",true},&{"2","two",false},&{"2","also two",false}}`)

	a.Check("2")
	is.Equal((*a)[0].Checked, true)
	is.Equal((*a)[1].Checked, true)
	is.Equal((*a)[2].Checked, true)
	a.Set("2", false)
	is.Equal((*a)[0].Checked, true)
	is.Equal((*a)[1].Checked, false)
	is.Equal((*a)[2].Checked, false)
}
