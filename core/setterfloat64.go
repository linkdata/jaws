package core

import (
	"fmt"
)

type numeric interface {
	~float32 | ~float64 |
		~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

type SetterFloat64[T numeric] interface {
	Getter[T]
	// JawsSet may return ErrValueUnchanged to indicate value was already set.
	JawsSet(elem *Element, value T) (err error)
}

type setterFloat64[T numeric] struct {
	Setter[T]
}

func (s setterFloat64[T]) JawsGet(e *Element) float64 {
	v := s.Setter.JawsGet(e)
	return float64(v)
}

func (s setterFloat64[T]) JawsSet(e *Element, v float64) error {
	return s.Setter.JawsSet(e, T(v))
}

func (s setterFloat64[T]) JawsGetTag(*Request) any {
	return s.Setter
}

type setterFloat64ReadOnly[T numeric] struct {
	Getter[T]
}

func (s setterFloat64ReadOnly[T]) JawsGet(e *Element) float64 {
	v := s.Getter.JawsGet(e)
	return float64(v)
}

func (setterFloat64ReadOnly[T]) JawsSet(*Element, float64) error {
	return ErrValueNotSettable
}

func (s setterFloat64ReadOnly[T]) JawsGetTag(*Request) any {
	return s.Getter
}

type setterFloat64Static[T numeric] struct {
	v float64
}

func (setterFloat64Static[T]) JawsSet(*Element, float64) error {
	return ErrValueNotSettable
}

func (s setterFloat64Static[T]) JawsGet(*Element) float64 {
	return s.v
}

func (s setterFloat64Static[T]) JawsGetTag(*Request) any {
	return nil
}

func makeSetterFloat64for[T numeric](s *Setter[float64], v any) bool {
	switch v := v.(type) {
	case Setter[T]:
		*s = setterFloat64[T]{Setter: v}
		return true
	case Getter[T]:
		*s = setterFloat64ReadOnly[T]{Getter: v}
		return true
	case T:
		*s = setterFloat64Static[T]{float64(v)}
		return true
	}
	return false
}

func MakeSetterFloat64(v any) (s Setter[float64]) {
	switch v := v.(type) {
	case Setter[float64]:
		return v
	case Getter[float64]:
		return setterReadOnly[float64]{v}
	case float64:
		return setterStatic[float64]{v}
	default:
		var ok bool
		ok = ok || makeSetterFloat64for[int64](&s, v)
		ok = ok || makeSetterFloat64for[uint64](&s, v)
		ok = ok || makeSetterFloat64for[int](&s, v)
		ok = ok || makeSetterFloat64for[uint](&s, v)
		ok = ok || makeSetterFloat64for[int32](&s, v)
		ok = ok || makeSetterFloat64for[uint32](&s, v)
		ok = ok || makeSetterFloat64for[int8](&s, v)
		ok = ok || makeSetterFloat64for[int16](&s, v)
		ok = ok || makeSetterFloat64for[uint8](&s, v)
		ok = ok || makeSetterFloat64for[uint16](&s, v)
		ok = ok || makeSetterFloat64for[float32](&s, v)
		if ok {
			return
		}
	}
	panic(fmt.Errorf("expected jaws.Setter[float64], jaws.Getter[float64] or float64 not %T", v))
}
