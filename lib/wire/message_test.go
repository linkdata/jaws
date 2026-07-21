package wire

import (
	"fmt"
	"testing"

	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
)

func Test_Message_String(t *testing.T) {
	// Dest is rendered with tag.TagString, whose output depends on the build, so
	// build the expectation the same way rather than hardcoding one build's form.
	msg := Message{
		Dest: "Elem",
		What: what.Update,
		Data: "Data\nText",
	}
	want := fmt.Sprintf("{%s, Update, %q}", tag.TagString("Elem"), "Data\nText")
	if s := msg.String(); s != want {
		t.Errorf("Message.String() = %q, want %q", s, want)
	}
	// A nil Dest renders as <nil> in every build.
	msg = Message{What: what.Click}
	if s := msg.String(); s != "{<nil>, Click, \"\"}" {
		t.Error(s)
	}
}
