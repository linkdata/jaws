package jaws

import (
	"sort"
	"testing"

	"github.com/matryer/is"
)

func Test_NamedBoolArray(t *testing.T) {
	is := is.New(t)
	a := NewNamedBoolArray("foo")
	is.Equal(len(a.data), 0)
	is.Equal(a, a.JawsRadioGroupData())
	is.Equal(nil, a.JawsRadioGroupHandler(nil, ""))

	a.Add("1", "one")
	is.Equal(len(a.data), 1)
	is.Equal((a.data)[0].Name, "1")
	is.Equal((a.data)[0].Html, "one")
	is.Equal((a.data)[0].Checked, false)
	is.Equal(a.String(), `&NamedBoolArray{"foo",[&{"1","one",false}]}`)
	is.Equal(a.Get(), "")
	is.Equal(a.IsChecked("1"), false)
	is.Equal(a.IsChecked("2"), false)

	a.SetOnly("1")
	is.Equal((a.data)[0].Name, "1")
	is.Equal((a.data)[0].Html, "one")
	is.Equal((a.data)[0].Checked, true)
	is.Equal(a.Get(), "1")
	is.Equal(a.IsChecked("1"), true)
	is.Equal(a.IsChecked("2"), false)

	a.Add("2", "two")
	a.Add("2", "also two")
	is.Equal(len(a.data), 3)
	is.Equal((a.data)[0].Name, "1")
	is.Equal((a.data)[0].Html, "one")
	is.Equal((a.data)[0].Checked, true)
	is.Equal((a.data)[1].Name, "2")
	is.Equal((a.data)[1].Html, "two")
	is.Equal((a.data)[1].Checked, false)
	is.Equal((a.data)[2].Name, "2")
	is.Equal((a.data)[2].Html, "also two")
	is.Equal((a.data)[2].Checked, false)
	is.Equal(a.String(), `&NamedBoolArray{"foo",[&{"1","one",true},&{"2","two",false},&{"2","also two",false}]}`)

	a.WriteLocked(func(nba []*NamedBool) []*NamedBool {
		sort.Slice(nba, func(i, j int) bool {
			return nba[i].Name > nba[j].Name
		})
		return nba
	})

	a.ReadLocked(func(nba []*NamedBool) {
		is.Equal(nba[0].Name, "2")
		is.Equal(nba[1].Name, "2")
		is.Equal(nba[2].Name, "1")
	})

	is.Equal((a.data)[0].Checked, false)
	is.Equal((a.data)[1].Checked, false)
	is.Equal((a.data)[2].Checked, true)
	a.Set("2", true)
	is.Equal((a.data)[0].Checked, true)
	is.Equal((a.data)[1].Checked, true)
	is.Equal((a.data)[2].Checked, true)
	is.Equal(a.Get(), "2")

	a.SetOnly("1")
	is.Equal((a.data)[0].Checked, false)
	is.Equal((a.data)[1].Checked, false)
	is.Equal((a.data)[2].Checked, true)
	is.Equal(a.Get(), "1")

	is.Equal(a.IsChecked("2"), false)
	(a.data)[1].Checked = true
	is.Equal(a.IsChecked("2"), true)
}
