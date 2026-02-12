package core

import "html/template"

// TemplateLookuper resolves a name to a *template.Template.
type TemplateLookuper interface {
	Lookup(name string) *template.Template
}
