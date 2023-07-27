package jaws

type RadioGrouper interface {
	JawsRadioGroupData() *NamedBoolArray
	JawsRadioGroupHandler(rq *Request, jid string, boolName string) error
}

func (rq *Request) RadioGroup(grouper RadioGrouper) (rl []Radio) {
	return grouper.JawsRadioGroupData().radioList(rq, grouper.JawsRadioGroupHandler)
}
