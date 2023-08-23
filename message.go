package jaws

import (
	"fmt"
	"strconv"

	"github.com/linkdata/jaws/what"
)

// Message contains the elements of a message to be sent to Requests.
type Message struct {
	Tags []interface{} // tags to affect
	What what.What     // what to change or do
	Data string        // data (e.g. inner HTML content)
	from *Request      // source of the message (may be nil)
}

// String returns the Message in a form suitable for debug output.
func (msg *Message) String() string {
	return fmt.Sprintf("{%q, %v, %s, %v}",
		msg.Tags,
		msg.What,
		strconv.QuoteToASCII(msg.Data),
		msg.from,
	)
}
