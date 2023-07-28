package jaws

type ValueProxy interface {
	JawsSet(val interface{}) (err error)
	JawsGet() (val interface{})
}

type defaultValueProxy struct{ v interface{} }

func (dvh *defaultValueProxy) JawsGet() interface{} {
	return dvh.v
}

func (dvh *defaultValueProxy) JawsSet(val interface{}) error {
	dvh.v = val
	return nil
}
