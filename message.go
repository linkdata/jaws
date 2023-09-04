package jaws

import (
	"fmt"

	"github.com/linkdata/jaws/what"
)

// Message contains the elements of a message to be sent to Requests.
type Message struct {
	Tag  interface{} // tag to affect
	What what.What   // what to change or do
	Data interface{} // data (e.g. inner HTML content)
	from interface{} // don't send to this
}

// String returns the Message in a form suitable for debug output.
func (msg *Message) String() string {
	return fmt.Sprintf("{%q, %v, %q, %v}",
		msg.Tag,
		msg.What,
		msg.Data,
		msg.from,
	)
}
