package jaws

type SelectHandler interface {
	Container
	Setter[string]
}
