package jawstree

import "testing"

// TestOptionBitPositions pins each Option constant to the exact bit position that
// jawstreeDecodeOptions in assets/jawstree.js tests with a literal (1<<n). The two
// sides are a hard cross-file coupling (see the comment in option.go); reordering or
// inserting a constant would silently mis-wire the script, so this test fails if the
// mapping drifts.
func TestOptionBitPositions(t *testing.T) {
	for _, tc := range []struct {
		name string
		opt  Option
		bit  uint
	}{
		{"SearchEnabled", SearchEnabled, 0},
		{"InitiallyExpanded", InitiallyExpanded, 1},
		{"MultiSelectEnabled", MultiSelectEnabled, 2},
		{"ShowSelectAllButton", ShowSelectAllButton, 3},
		{"ShowInvertSelectionButton", ShowInvertSelectionButton, 4},
		{"ShowExpandCollapseAllButtons", ShowExpandCollapseAllButtons, 5},
		{"NodeSelectionDisabled", NodeSelectionDisabled, 6},
		{"CascadeSelectChildren", CascadeSelectChildren, 7},
		{"CheckboxSelectionEnabled", CheckboxSelectionEnabled, 8},
	} {
		if want := Option(1 << tc.bit); tc.opt != want {
			t.Errorf("%s = %d, want 1<<%d = %d", tc.name, tc.opt, tc.bit, want)
		}
	}
}
