package jaws

type RadioGrouper interface {
	JawsRadioGroupData() *NamedBoolArray
	JawsRadioGroupHandler(rq *Request, jid string, boolName string) error
}
