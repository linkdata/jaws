package jaws

import (
	"html/template"
	"sort"
	"testing"
)

func Test_NamedBoolArray(t *testing.T) {
	is := newTestHelper(t)
	nba := NewNamedBoolArray()
	is.Equal(len(nba.data), 0)

	nba.Add("1", "one")
	is.Equal(len(nba.data), 1)
	is.Equal((nba.data)[0].Name(), "1")
	is.Equal((nba.data)[0].HTML(), template.HTML("one"))
	is.Equal((nba.data)[0].Checked(), false)
	is.Equal(nba.String(), `&NamedBoolArray{[&{"1","one",false}]}`)
	is.Equal(nba.Get(), "")
	is.Equal(nba.IsChecked("1"), false)
	is.Equal(nba.IsChecked("2"), false)

	nba.Set("1", true)
	is.Equal((nba.data)[0].Name(), "1")
	is.Equal((nba.data)[0].HTML(), template.HTML("one"))
	is.Equal((nba.data)[0].Checked(), true)
	is.Equal(nba.Get(), "1")
	is.Equal(nba.IsChecked("1"), true)
	is.Equal(nba.IsChecked("2"), false)

	nba.Add("2", "two")
	nba.Add("2", "also two")
	is.Equal(len(nba.data), 3)
	is.Equal((nba.data)[0].Name(), "1")
	is.Equal((nba.data)[0].HTML(), template.HTML("one"))
	is.Equal((nba.data)[0].Checked(), true)
	is.Equal((nba.data)[1].Name(), "2")
	is.Equal((nba.data)[1].HTML(), template.HTML("two"))
	is.Equal((nba.data)[1].Checked(), false)
	is.Equal((nba.data)[2].Name(), "2")
	is.Equal((nba.data)[2].HTML(), template.HTML("also two"))
	is.Equal((nba.data)[2].Checked(), false)
	is.Equal(nba.String(), `&NamedBoolArray{[&{"1","one",true},&{"2","two",false},&{"2","also two",false}]}`)

	nba.WriteLocked(func(nba []*NamedBool) []*NamedBool {
		sort.Slice(nba, func(i, j int) bool {
			return nba[i].Name() > nba[j].Name()
		})
		return nba
	})

	nba.ReadLocked(func(nba []*NamedBool) {
		is.Equal(nba[0].Name(), "2")
		is.Equal(nba[1].Name(), "2")
		is.Equal(nba[2].Name(), "1")
	})

	is.Equal((nba.data)[0].Checked(), false)
	is.Equal((nba.data)[1].Checked(), false)
	is.Equal((nba.data)[2].Checked(), true)

	nba.Multi = true
	nba.Set("2", true)
	is.Equal((nba.data)[0].Checked(), true)
	is.Equal((nba.data)[1].Checked(), true)
	is.Equal((nba.data)[2].Checked(), true)
	is.Equal(nba.Get(), "2")

	nba.Multi = false
	nba.Set("1", true)
	is.Equal((nba.data)[0].Checked(), false)
	is.Equal((nba.data)[1].Checked(), false)
	is.Equal((nba.data)[2].Checked(), true)
	is.Equal(nba.Get(), "1")

	is.Equal(nba.IsChecked("2"), false)
	(nba.data)[1].Set(true)
	is.Equal(nba.IsChecked("2"), true)

	rq := newTestRequest()
	e := rq.NewElement(NewUiSelect(nba))
	defer rq.Close()

	is.Equal(nba.JawsGet(e), "2")
	is.NoErr(nba.JawsSet(e, "1"))
	is.Equal(nba.JawsGet(e), "1")
	is.Equal(nba.JawsSet(e, "1"), ErrValueUnchanged)
}
