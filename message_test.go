package jaws

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
	if msg.String() != "{\"Elem\", Update, \"Data\\nText\"}" {
		t.Fail()
	}
}
