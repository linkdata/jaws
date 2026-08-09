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

type numericBinding interface {
	sourceValue() any
	writable() bool
	getText(*jaws.Element) (string, error)
	acceptText(*Input, *jaws.Element, string) (bool, error)
}

type numericTyped[T Numeric] struct {
	source bind.Getter[T]
	setter bind.Setter[T]
	bits   int
}

func (nb *numericTyped[T]) sourceValue() any {
	return nb.source
}

func (nb *numericTyped[T]) writable() bool {
	return nb.setter != nil
}

func (nb *numericTyped[T]) getText(elem *jaws.Element) (text string, err error) {
	return formatNumeric(nb.source.JawsGet(elem), nb.bits)
}

func (nb *numericTyped[T]) acceptText(input *Input, elem *jaws.Element, text string) (accepted bool, err error) {
	var value T
	if value, accepted = parseNumeric[T](text, nb.bits); accepted {
		input.Last.Store(text)
		err = nb.setter.JawsSet(elem, value)
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

func newNumericBinding[T Numeric](source bind.Getter[T]) numericBinding {
	nb := &numericTyped[T]{source: source, bits: reflect.TypeFor[T]().Bits()}
	if setter, ok := any(source).(bind.Setter[T]); ok {
		nb.setter = setter
	}
	return nb
}

type numericClass uint8

const (
	numericUnsigned numericClass = iota
	numericSigned
	numericFloat
)

func numericClassOf[T Numeric]() numericClass {
	// Float comes first because subtraction is negative for signed integers and
	// floats. Conversions to T make these non-constant expressions, so unsigned
	// subtraction wraps instead of causing a constant-overflow error.
	switch {
	case T(1)/T(2) != T(0):
		return numericFloat
	case T(0)-T(1) < T(0):
		return numericSigned
	default:
		return numericUnsigned
	}
}

func formatNumeric[T Numeric](value T, bits int) (text string, err error) {
	switch numericClassOf[T]() {
	case numericFloat:
		text, err = formatNumericFloat(float64(value), bits)
	case numericSigned:
		text = strconv.FormatInt(int64(value), 10)
	case numericUnsigned:
		text = strconv.FormatUint(uint64(value), 10)
	}
	return
}

func parseNumeric[T Numeric](text string, bits int) (value T, ok bool) {
	switch numericClassOf[T]() {
	case numericFloat:
		var parsed float64
		if parsed, ok = parseNumericFloat(text, bits); ok {
			value = T(parsed)
		}
	case numericSigned:
		var parsed int64
		if parsed, ok = parseNumericInt(text); ok {
			value = T(parsed)
			ok = int64(value) == parsed
		}
	case numericUnsigned:
		var parsed uint64
		if parsed, ok = parseNumericUint(text); ok {
			value = T(parsed)
			ok = uint64(value) == parsed
		}
	}
	return
}

// makeNumericBinding converts template Number and Range sources to the shared
// type-erased representation. It accepts reflected numeric Getter/Setter
// implementations and static built-in or named numeric values.
func makeNumericBinding(source any) (binding numericBinding, ok bool) {
	rv := reflect.ValueOf(source)
	if !rv.IsValid() {
		return
	}
	if getter, ops, yes := reflectedNumericGetter(rv); yes {
		reflected := &numericReflected{source: source, getter: getter, ops: ops}
		if setter, yes := reflectedNumericSetter(rv, ops.typ); yes {
			reflected.setter = setter
		}
		binding = reflected
		ok = true
		return
	}
	if ops, yes := newNumericOps(rv.Type()); yes {
		binding = &numericReflected{value: rv, ops: ops}
		ok = true
	}
	return
}

type numericReflected struct {
	source any
	value  reflect.Value
	getter reflect.Value
	setter reflect.Value
	ops    numericOps
}

func (nb *numericReflected) sourceValue() any {
	return nb.source
}

func (nb *numericReflected) writable() bool {
	return nb.setter.IsValid()
}

func (nb *numericReflected) getText(elem *jaws.Element) (text string, err error) {
	value := nb.value
	if nb.getter.IsValid() {
		value = nb.getter.Call([]reflect.Value{reflect.ValueOf(elem)})[0]
	}
	return nb.ops.formatValue(value)
}

func (nb *numericReflected) acceptText(input *Input, elem *jaws.Element, text string) (accepted bool, err error) {
	var value reflect.Value
	if value, accepted = nb.ops.parseValue(text); accepted {
		input.Last.Store(text)
		out := nb.setter.Call([]reflect.Value{reflect.ValueOf(elem), value})
		if !out[0].IsNil() {
			err = out[0].Interface().(error)
		}
	}
	return
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

func (ops numericOps) formatValue(value reflect.Value) (text string, err error) {
	switch ops.kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		text = strconv.FormatInt(value.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		text = strconv.FormatUint(value.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		text, err = formatNumericFloat(value.Float(), ops.bits)
	}
	return
}

func (ops numericOps) parseValue(text string) (value reflect.Value, ok bool) {
	value = reflect.New(ops.typ).Elem()
	switch ops.kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if parsed, yes := parseNumericInt(text); yes && !value.OverflowInt(parsed) {
			value.SetInt(parsed)
			ok = true
			return
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if parsed, yes := parseNumericUint(text); yes && !value.OverflowUint(parsed) {
			value.SetUint(parsed)
			ok = true
			return
		}
	case reflect.Float32, reflect.Float64:
		if parsed, yes := parseNumericFloat(text, ops.bits); yes {
			value.SetFloat(parsed)
			ok = true
			return
		}
	}
	// Numeric widgets restore rejected input without turning validation into a
	// handler error.
	return reflect.Value{}, false
}

func parseNumericInt(text string) (value int64, ok bool) {
	value, err := strconv.ParseInt(text, 10, 64)
	ok = err == nil
	return
}

func parseNumericUint(text string) (value uint64, ok bool) {
	value, err := strconv.ParseUint(text, 10, 64)
	ok = err == nil
	return
}

func formatNumericFloat(value float64, bits int) (text string, err error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		err = fmt.Errorf("%w: %g", jaws.ErrValueNotFinite, value)
		return
	}
	text = strconv.FormatFloat(value, 'f', -1, bits)
	return
}

func parseNumericFloat(text string, bits int) (value float64, ok bool) {
	// Browsers sanitize number and range values before jaws.js reads them.
	// Forged finite Go spellings add no values and accepted input is canonicalized.
	value, err := strconv.ParseFloat(text, bits)
	ok = err == nil && !math.IsNaN(value) && !math.IsInf(value, 0)
	return
}
