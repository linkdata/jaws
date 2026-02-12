package core

import (
	"testing"

	"github.com/linkdata/jaws/what"
)

func Test_Message_String(t *testing.T) {
	msg := Message{
		Dest: "Elem",
		What: what.Update,
		Data: "Data\nText",
	}
	if s := msg.String(); s != "{Elem, Update, \"Data\\nText\"}" {
		t.Error(s)
	}
	msg = Message{
		What: what.Click,
	}
	if s := msg.String(); s != "{<nil>, Click, \"\"}" {
		t.Error(s)
	}
}
