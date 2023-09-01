package jaws

type ClickHandler interface {
	JawsClick(e *Element, name string) (err error)
}
