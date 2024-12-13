package jaws

type AnySetter interface {
	AnyGetter
	// JawsSetAny may return ErrValueUnchanged to indicate value was already set.
	// It may panic if the type of v cannot be handled.
	JawsSetAny(e *Element, v any) (err error)
}

func makeAnySetter(v any) AnySetter {
	switch v := v.(type) {
	case AnySetter:
		return v
	}
	return anyGetter{v}
}
