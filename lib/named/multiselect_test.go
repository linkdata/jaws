package named

import (
	"errors"
	"slices"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/linkdata/jaws"
)

func TestBoolArray_JawsSetValues(t *testing.T) {
	for _, tt := range []struct {
		name      string
		names     []string
		want      []string
		unchanged bool
	}{
		{name: "replace", names: []string{"three", "two"}, want: []string{"two", "three"}},
		{name: "nil clears"},
		{name: "empty clears", names: []string{}},
		{name: "unknown clears", names: []string{"missing"}},
		{name: "unknown ignored", names: []string{"missing", "two"}, want: []string{"two"}},
		{name: "unchanged set", names: []string{"two", "one", "two", "missing"}, want: []string{"one", "two"}, unchanged: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			nba := NewBoolArray(true).Add("one", "One").Add("two", "Two").Add("three", "Three")
			nba.Set("one", true)
			nba.Set("two", true)
			before := nba.JawsGetValues(nil)
			if !slices.Equal(before, []string{"one", "two"}) {
				t.Fatalf("initial values = %v", before)
			}

			_, rq := newCoreRequest(t)
			err := nba.JawsSetValues(rq.NewElement(noopUI{}), tt.names)
			if tt.unchanged {
				if !errors.Is(err, jaws.ErrValueUnchanged) {
					t.Fatalf("error = %v, want ErrValueUnchanged", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if got := nba.JawsGetValues(nil); !slices.Equal(got, tt.want) {
				t.Fatalf("selected values = %v, want %v", got, tt.want)
			}
			if !slices.Equal(before, []string{"one", "two"}) {
				t.Fatalf("getter result changed: %v", before)
			}
		})
	}
}

func TestBoolArray_JawsSetValuesSingleSelect(t *testing.T) {
	nba := NewBoolArray(false).Add("one", "One").Add("two", "Two").Add("one", "Another one")
	nba.Set("two", true)
	_, rq := newCoreRequest(t)
	elem := rq.NewElement(noopUI{})
	if err := nba.JawsSetValues(elem, []string{"missing", "two", "one"}); err != nil {
		t.Fatal(err)
	}
	if got := nba.JawsGetValues(nil); !slices.Equal(got, []string{"one", "one"}) {
		t.Fatalf("selected values = %v, want both Bools named one", got)
	}
	if err := nba.JawsSetValues(elem, nil); err != nil {
		t.Fatal(err)
	}
	if got := nba.JawsGetValues(nil); got != nil {
		t.Fatalf("cleared values = %v, want nil", got)
	}
}

func TestBoolArray_JawsSetValuesDirtiesOnlyChangedBoolsAndArray(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		jw, rq := newTestRequest(t)
		defer closeBubbleRequest(jw, rq)
		nba := NewBoolArray(true).Add("one", "One").Add("two", "Two").Add("three", "Three")
		nba.Set("one", true)
		nba.Set("two", true)
		var oneHits, twoHits, threeHits, groupHits atomic.Int32
		registerDirtyProbe(rq, nba.data[0], &oneHits)
		registerDirtyProbe(rq, nba.data[1], &twoHits)
		registerDirtyProbe(rq, nba.data[2], &threeHits)
		registerDirtyProbe(rq, nba, &groupHits)
		elem := rq.NewElement(noopUI{})
		if err := nba.JawsSetValues(elem, []string{"two", "three"}); err != nil {
			t.Fatal(err)
		}
		waitForDirtyProbes(t, func() bool {
			return oneHits.Load() == 1 && twoHits.Load() == 0 && threeHits.Load() == 1 && groupHits.Load() == 1
		})
		if err := nba.JawsSetValues(elem, []string{"three", "two"}); !errors.Is(err, jaws.ErrValueUnchanged) {
			t.Fatalf("repeat error = %v, want ErrValueUnchanged", err)
		}
		waitForDirtyProbes(t, func() bool {
			return oneHits.Load() == 1 && twoHits.Load() == 0 && threeHits.Load() == 1 && groupHits.Load() == 1
		})
	})
}
