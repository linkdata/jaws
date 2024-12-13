package jaws

type AnyGetter interface {
	JawsGetAny(elem *Element) (value any)
}

type anyGetter struct{ v any }

func (g anyGetter) JawsGetAny(e *Element) any {
	return g.v
}

func (g anyGetter) JawsSetAny(*Element, any) error {
	return ErrValueNotSettable
}

func (g anyGetter) JawsGetTag(rq *Request) any {
	return nil
}
