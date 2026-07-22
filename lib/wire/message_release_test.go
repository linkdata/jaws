//go:build !debug && !race

package wire

import (
	"testing"

	"github.com/linkdata/jaws/lib/what"
)

// Test_Message_String_CyclicDestBounded is a unit test for Message.String: a
// self-referential Dest must render to a bounded string in the default build
// rather than overflow the stack.
//
// The broadcast-overload path in serve.go formats a Message's Dest through this
// same Message.String, so a bounded Message.String is what keeps that path safe;
// this test exercises Message.String directly, not that path.
func Test_Message_String_CyclicDestBounded(t *testing.T) {
	dest := []any{nil}
	dest[0] = dest
	msg := Message{Dest: dest, What: what.Inner}
	if s := msg.String(); len(s) > 1<<13 {
		t.Errorf("Message.String() = %d bytes, want bounded", len(s))
	}
}
