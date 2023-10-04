package jaws

import (
	"fmt"
	"html"

	"github.com/linkdata/jaws/what"
)

// wsMsg is a message sent to or from a WebSocket.
type wsMsg struct {
	Data string    // data to send
	Jid  Jid       // Jid to send, or negative to not send
	What what.What // command
}

func (m *wsMsg) IsValid() bool {
	return m.What != what.None
}

func (m *wsMsg) Append(b []byte) []byte {
	if m.What != what.None {
		b = append(b, m.What.String()...)
		b = append(b, '\n')
	}
	if m.Jid >= 0 {
		b = m.Jid.Append(b)
		b = append(b, '\n')
	}
	b = append(b, m.Data...)
	return b
}

func (m *wsMsg) Format() string {
	return string(m.Append(nil))
}

func (m *wsMsg) String() string {
	return fmt.Sprintf("wsMsg{%s, %d, %q}", m.What, m.Jid, m.Data)
}

func (m *wsMsg) FillAlert(err error) {
	m.Jid = -1
	m.What = what.Alert
	m.Data = "danger\n" + html.EscapeString(err.Error())
}
