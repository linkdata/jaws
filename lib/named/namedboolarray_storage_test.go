package named

import (
	"html/template"
	"slices"
	"testing"
)

func TestBoolArrayWriteLockedReleasesReplacedStorage(t *testing.T) {
	tests := []struct {
		name         string
		selectValues func([]*Bool) []*Bool
		want         []int
	}{
		{
			name: "shorter prefix",
			selectValues: func(values []*Bool) []*Bool {
				return values[:1]
			},
			want: []int{0},
		},
		{
			name: "overlapping middle",
			selectValues: func(values []*Bool) []*Bool {
				return values[1:4]
			},
			want: []int{1, 2, 3},
		},
		{
			name: "nil filtering",
			selectValues: func(values []*Bool) []*Bool {
				return []*Bool{nil, values[3], nil, values[1], nil}
			},
			want: []int{3, 1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nba := NewBoolArray(false)
			for _, name := range []string{"a", "b", "c", "d", "e"} {
				nba.Add(name, template.HTML(name))
			}
			oldStorage := nba.data[:cap(nba.data)]
			want := make([]*Bool, len(tc.want))
			for i, wantIndex := range tc.want {
				want[i] = nba.data[wantIndex]
			}

			nba.WriteLocked(tc.selectValues)

			if !slices.Equal(nba.data, want) {
				t.Fatalf("data = %v, want %v", nba.data, want)
			}
			nonNil := func(value *Bool) bool { return value != nil }
			if i := slices.IndexFunc(oldStorage, nonNil); i >= 0 {
				t.Fatalf("old storage[%d] retains Bool %q", i, oldStorage[i].Name())
			}
			ownedStorage := nba.data[:cap(nba.data)]
			if i := slices.IndexFunc(ownedStorage[len(nba.data):], nonNil); i >= 0 {
				i += len(nba.data)
				t.Fatalf("owned storage[%d] retains Bool %q past len %d", i, ownedStorage[i].Name(), len(nba.data))
			}
		})
	}
}
