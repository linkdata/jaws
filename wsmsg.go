package jaws

import (
	"html"

	"github.com/linkdata/jaws/what"
)

type wsMsg struct {
	Jid  string
	What what.What
	Data string
}

func (m *wsMsg) IsValid() bool {
	return m.What != 0
}

func (m *wsMsg) Append(b []byte) []byte {
	b = append(b, []byte(m.Jid)...)
	b = append(b, '\n')
	if m.What != 0 {
		b = append(b, []byte(m.What.String())...)
	}
	b = append(b, '\n')
	b = append(b, []byte(m.Data)...)
	return b
}

func (m *wsMsg) Format() string {
	return string(m.Append(nil))
}

func (m *wsMsg) FillAlert(err error) {
	m.Jid = " alert"
	m.What = what.None
	m.Data = "danger\n" + html.EscapeString(err.Error())
}
