package bind

import (
	"testing"

	"github.com/linkdata/jaws/lib/tag"
)

// mustExpand expands v and fails the test on an expansion error.
//
// It replaces constructing a *jaws.Jaws purely to reach Jaws.MustTagExpand: these
// tests only need the expanded keys, not error logging.
func mustExpand(t *testing.T, v any) []any {
	t.Helper()
	expanded, err := tag.TagExpand(v)
	if err != nil {
		t.Fatalf("TagExpand(%#v) error = %v", v, err)
	}
	return expanded
}
