package wire

import (
	"fmt"

	"github.com/linkdata/jaws/lib/what"
)

// Message contains the elements of a message to be sent to requests.
type Message struct {
	// Dest selects recipients among active Requests: nil targets every active
	// Request, a nonzero request key targets the matching active Request, and any
	// other value is expanded into a tag or tag list. Plain strings and Jid values
	// are illegal destinations.
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
