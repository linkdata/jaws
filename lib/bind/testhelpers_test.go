package bind

type selfTagger struct{}

func (st *selfTagger) JawsGetTag() any {
	return st
}
