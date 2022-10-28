package jaws

import (
	"html/template"
)

type Ui interface {
	JawsUi(rq *Request, attrs ...string) template.HTML
}
