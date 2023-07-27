package jaws

type ValueSetter interface {
	JawsSet(val interface{}) (err error)
}

type ValueGetter interface {
	JawsGet() (val interface{})
}

type ValueHandler interface {
	ValueGetter
	ValueSetter
}

type defaultValueHandler struct{ v interface{} }

func (dvh *defaultValueHandler) JawsGet() interface{} {
	return dvh.v
}
