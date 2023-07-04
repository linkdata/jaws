package jaws

import (
	"fmt"
	"strconv"

	"github.com/linkdata/jaws/what"
)

// Message contains the elements of a message to be sent to Requests.
type Message struct {
	Elem string    // HTML 'jid' attribute or command (e.g. ' alert' or 'myButtonId')
	What what.What // what to change or do
	Data string    // data (e.g. inner HTML content)
	from *Request  // source of the message (may be nil)
}

// Format returns the Message in the form it's expected by the Javascript.
func (msg *Message) Format() string {
	if msg.What == 0 {
		return msg.Elem + "\n\n" + msg.Data
	}
	return msg.Elem + "\n" + msg.What.String() + "\n" + msg.Data
}

// String returns the Message in a form suitable for debug output.
func (msg *Message) String() string {
	return fmt.Sprintf("{%s, %v, %s, %v}",
		strconv.QuoteToASCII(msg.Elem),
		msg.What,
		strconv.QuoteToASCII(msg.Data),
		msg.from,
	)
}
