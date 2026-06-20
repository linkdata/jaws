package wire

import (
	"fmt"

	"github.com/linkdata/jaws/lib/what"
)

// Message contains the elements of a message to be sent to requests.
type Message struct {
	// Dest selects recipients: nil targets every Request, a request key targets a
	// single Request, an HTML id string targets matching elements in all Requests,
	// and any other value is expanded into a tag or tag list.
	Dest any
	What what.What // command to perform
	Data string    // payload: inner HTML content or a tag list
}

// String returns the Message in a form suitable for debug output.
func (msg *Message) String() string {
	return fmt.Sprintf(
		"{%v, %v, %q}",
		msg.Dest,
		msg.What,
		msg.Data,
	)
}
