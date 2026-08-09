package ui

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Numeric is an integer or floating-point type supported by [Number] and [Range].
//
// Named types with one of these underlying types are included.
type Numeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64
}

type numericBinding struct {
	source   any
	ops      numericOps
	getValue func(*jaws.Element) any
	setValue func(*jaws.Element, any) error
}

func (nb numericBinding) writable() bool {
	return nb.setValue != nil
}

func (nb numericBinding) getText(elem *jaws.Element) (text string, err error) {
	text, err = nb.ops.format(nb.getValue(elem))
	return
}

func (nb numericBinding) acceptText(input *Input, elem *jaws.Element, text string) (accepted bool, err error) {
	var value any
	if value, accepted = nb.ops.parse(text); accepted {
		input.Last.Store(text)
		err = nb.setValue(elem, value)
	}
	return
}

func updateNumericInput(input *Input, binding numericBinding, elem *jaws.Element, name string) {
	if input.Last.Load() == nil {
		elem.Request.MustLog(fmt.Errorf("ui.%s.JawsUpdate called before successful rendering", name))
		return
	}
	text, err := binding.getText(elem)
	if err != nil {
		elem.Cancel(err)
		return
	}
	if prev := input.Last.Swap(text).(string); prev != text {
		elem.SetValue(text)
	}
}

func handleNumericInput(input *Input, binding numericBinding, elem *jaws.Element, text string) (err error) {
	if !binding.writable() {
		return
	}
	accepted, err := binding.acceptText(input, elem, text)
	if !accepted {
		// A formatter can never produce empty text, so empty forces the next
		// update to restore the canonical value.
		input.Last.Store("")
		// Exact dirtying serializes the correction with source-driven updates,
		// leaving a newer source value final.
		elem.Dirty(elem)
		return
	}
	if errors.Is(err, jaws.ErrValueUnchanged) {
		// The accepted text may still differ from the source's formatting.
		elem.Dirty(elem)
		err = nil
		return
	}
	err = input.maybeDirty(elem, err)
	return
}

func newNumericBinding[T Numeric](source bind.Getter[T]) (nb numericBinding) {
	nb.ops, _ = newNumericOps(reflect.TypeFor[T]())
	nb.source = source
	nb.getValue = func(elem *jaws.Element) any {
		return source.JawsGet(elem)
	}
	if setter, ok := any(source).(bind.Setter[T]); ok {
		nb.setValue = func(elem *jaws.Element, value any) error {
			return setter.JawsSet(elem, value.(T))
		}
	}
	return nb
}

// makeNumericBinding converts template Number and Range sources to the shared
// type-erased representation. It accepts reflected numeric Getter/Setter
// implementations and static built-in or named numeric values.
func makeNumericBinding(source any) (nb numericBinding, ok bool) {
	rv := reflect.ValueOf(source)
	if !rv.IsValid() {
		return numericBinding{}, false
	}
	if getter, ops, yes := reflectedNumericGetter(rv); yes {
		nb.ops = ops
		nb.source = source
		nb.getValue = func(elem *jaws.Element) any {
			out := getter.Call([]reflect.Value{reflect.ValueOf(elem)})
			return out[0].Interface()
		}
		if setter, yes := reflectedNumericSetter(rv, ops.typ); yes {
			nb.setValue = func(elem *jaws.Element, value any) (err error) {
				out := setter.Call([]reflect.Value{reflect.ValueOf(elem), reflect.ValueOf(value)})
				if !out[0].IsNil() {
					err = out[0].Interface().(error)
				}
				return
			}
		}
		return nb, true
	}
	if ops, yes := newNumericOps(rv.Type()); yes {
		nb.ops = ops
		nb.getValue = func(*jaws.Element) any {
			return source
		}
		return nb, true
	}
	return numericBinding{}, false
}

var (
	elementPointerType = reflect.TypeFor[*jaws.Element]()
	errorType          = reflect.TypeFor[error]()
)

func reflectedNumericGetter(source reflect.Value) (getter reflect.Value, ops numericOps, ok bool) {
	getter = source.MethodByName("JawsGet")
	if !getter.IsValid() {
		return reflect.Value{}, numericOps{}, false
	}
	typ := getter.Type()
	if typ.NumIn() != 1 || typ.In(0) != elementPointerType || typ.NumOut() != 1 {
		return reflect.Value{}, numericOps{}, false
	}
	if ops, ok = newNumericOps(typ.Out(0)); !ok {
		return reflect.Value{}, numericOps{}, false
	}
	return
}

func reflectedNumericSetter(source reflect.Value, valueType reflect.Type) (setter reflect.Value, ok bool) {
	setter = source.MethodByName("JawsSet")
	if !setter.IsValid() {
		return reflect.Value{}, false
	}
	typ := setter.Type()
	ok = typ.NumIn() == 2 && typ.In(0) == elementPointerType && typ.In(1) == valueType &&
		typ.NumOut() == 1 && typ.Out(0) == errorType
	if !ok {
		setter = reflect.Value{}
	}
	return
}

type numericOps struct {
	typ  reflect.Type
	kind reflect.Kind
	bits int
}

func newNumericOps(typ reflect.Type) (ops numericOps, ok bool) {
	ops.typ = typ
	ops.kind = typ.Kind()
	switch ops.kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		ops.bits = typ.Bits()
		ok = true
	}
	return
}

func (ops numericOps) format(value any) (text string, err error) {
	rv := reflect.ValueOf(value)
	switch ops.kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		text = strconv.FormatInt(rv.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		text = strconv.FormatUint(rv.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		value := rv.Float()
		if math.IsNaN(value) || math.IsInf(value, 0) {
			err = fmt.Errorf("%w: %g", jaws.ErrValueNotFinite, value)
			return
		}
		text = strconv.FormatFloat(value, 'f', -1, ops.bits)
	}
	return
}

func (ops numericOps) parse(text string) (value any, ok bool) {
	rv := reflect.New(ops.typ).Elem()
	switch ops.kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if n, err := strconv.ParseInt(text, 10, ops.bits); err == nil {
			rv.SetInt(n)
			value = rv.Interface()
			ok = true
			return
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if n, err := strconv.ParseUint(text, 10, ops.bits); err == nil {
			rv.SetUint(n)
			value = rv.Interface()
			ok = true
			return
		}
	case reflect.Float32, reflect.Float64:
		// Browsers sanitize number and range values before jaws.js reads them.
		// Forged finite Go spellings add no values and accepted input is canonicalized.
		if n, err := strconv.ParseFloat(text, ops.bits); err == nil && !math.IsNaN(n) && !math.IsInf(n, 0) {
			rv.SetFloat(n)
			value = rv.Interface()
			ok = true
			return
		}
	}
	// Numeric widgets restore rejected input without turning validation into a
	// handler error.
	return nil, false
}
