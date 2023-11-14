package jaws

import (
	"fmt"

	"github.com/linkdata/jaws/what"
)

// Message contains the elements of a message to be sent to Requests.
type Message struct {
	Dest interface{} // destination (tag, html ID or *Element)
	What what.What   // what to change or do
	Data string      // data (e.g. inner HTML content or slice of tags)
}

// String returns the Message in a form suitable for debug output.
func (msg *Message) String() string {
	return fmt.Sprintf("{%q, %v, %q}",
		msg.Dest,
		msg.What,
		msg.Data,
	)
}
