package jaws

import (
	"fmt"
	"html"

	"github.com/linkdata/jaws/what"
)

// wsMsg is a message sent to or from a WebSocket.
type wsMsg struct {
	Jid  Jid
	What what.What
	Data string
}

func (m *wsMsg) IsValid() bool {
	return m.What != what.None
}

func (m *wsMsg) Append(b []byte) []byte {
	b = m.Jid.AppendInt(b)
	b = append(b, '\n')
	if m.What != 0 {
		b = append(b, m.What.String()...)
	}
	b = append(b, '\n')
	b = append(b, m.Data...)
	return b
}

func (m *wsMsg) Format() string {
	return string(m.Append(nil))
}

func (m *wsMsg) String() string {
	return fmt.Sprintf("wsMsg{%d, %s, %q}", m.Jid, m.What, m.Data)
}

func (m *wsMsg) FillAlert(err error) {
	m.Jid = 0
	m.What = what.Alert
	m.Data = "danger\n" + html.EscapeString(err.Error())
}
