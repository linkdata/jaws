package jaws

import (
	"fmt"
	"strconv"
)

// Message contains the elements of a message to be sent to Requests.
type Message struct {
	Elem string   // HTML 'jid' attribute or command (e.g. ' alert' or 'myButtonId')
	What string   // what to change or do, (e.g. 'inner')
	Data string   // data (e.g. inner HTML content)
	from *Request // source of the message (may be nil)
}

// Format returns the Message in the form it's expected by the Javascript.
func (msg *Message) Format() string {
	return msg.Elem + "\n" + msg.What + "\n" + msg.Data
}

// String returns the Message in a form suitable for debug output.
func (msg *Message) String() string {
	return fmt.Sprintf("{%s, %s, %s, %v}",
		strconv.QuoteToASCII(msg.Elem),
		strconv.QuoteToASCII(msg.What),
		strconv.QuoteToASCII(msg.Data),
		msg.from,
	)
}
