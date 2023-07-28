package jaws

import (
	"html"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type wsMsg struct {
	jid  int
	What what.What
	Data string
}

func (m *wsMsg) Jid() string {
	return jidToString(m.jid)
}

func (m *wsMsg) IsValid() bool {
	return m.What != 0
}

func (m *wsMsg) Append(b []byte) []byte {
	b = strconv.AppendInt(b, int64(m.jid), 10)
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
	m.jid = metaIds[" alert"]
	m.What = what.None
	m.Data = "danger\n" + html.EscapeString(err.Error())
}
