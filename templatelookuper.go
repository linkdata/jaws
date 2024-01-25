package jaws

import "html/template"

type TemplateLookuper interface {
	Lookup(name string) *template.Template
}
