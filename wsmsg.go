package jaws

import (
	"html"
	"strconv"

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
	b = strconv.AppendInt(b, int64(m.Jid), 10)
	b = append(b, '\n')
	if m.What != 0 {
		b = append(b, m.What.String()...)
	}
	b = append(b, '\n')
	b = append(b, m.Data...)
	return b
}

func (m *wsMsg) String() string {
	return string(m.Append(nil))
}

func (m *wsMsg) FillAlert(err error) {
	m.Jid = 0
	m.What = what.Alert
	m.Data = "danger\n" + html.EscapeString(err.Error())
}
