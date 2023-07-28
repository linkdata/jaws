package jaws

import (
	"fmt"
	"strconv"

	"github.com/linkdata/jaws/what"
)

// Message contains the elements of a message to be sent to Requests.
type Message struct {
	Tag  interface{} // tag to affect
	What what.What   // what to change or do
	Data string      // data (e.g. inner HTML content)
	from *Request    // source of the message (may be nil)
}

// Format returns the Message in the form it's expected by the Javascript.
func (msg *Message) Format() string {
	if msg.What == 0 {
		return fmt.Sprintf("%s\n\n%s", msg.Tag, msg.Data)
	}
	return fmt.Sprintf("%s\n%s\n%s", msg.Tag, msg.What.String(), msg.Data)
}

// String returns the Message in a form suitable for debug output.
func (msg *Message) String() string {
	return fmt.Sprintf("{%q, %v, %s, %v}",
		msg.Tag,
		msg.What,
		strconv.QuoteToASCII(msg.Data),
		msg.from,
	)
}
