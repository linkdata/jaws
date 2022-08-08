package jaws

import (
	"testing"

	"github.com/matryer/is"
)

func Test_Message_Format(t *testing.T) {
	is := is.New(t)
	msg := &Message{
		Elem: "Elem",
		What: "What",
		Data: "Data\nText",
	}
	is.Equal(msg.Format(), "Elem\nWhat\nData\nText")
}

func Test_Message_String(t *testing.T) {
	is := is.New(t)
	msg := &Message{
		Elem: "Elem",
		What: "What",
		Data: "Data\nText",
	}
	is.Equal(msg.String(), "{\"Elem\", \"What\", \"Data\\nText\", Request<>}")
	msg.from = &Request{JawsKey: 0xcafe}
	is.Equal(msg.String(), "{\"Elem\", \"What\", \"Data\\nText\", Request<cafe>}")
	msg.from = &Request{JawsKey: 0}
	is.Equal(msg.String(), "{\"Elem\", \"What\", \"Data\\nText\", Request<>}")
}
