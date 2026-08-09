package ui

import (
	"errors"
	"math"
	"reflect"
	"strconv"
	"testing"

	"github.com/linkdata/jaws"
)

func testNumericOpsFormat[T Numeric](ops numericOps, value T) (string, error) {
	return ops.formatValue(reflect.ValueOf(value))
}

func testNumericOpsParse[T Numeric](ops numericOps, text string) (value T, ok bool) {
	if parsed, yes := ops.parseValue(text); yes {
		return parsed.Interface().(T), true
	}
	return
}

func testNumericParse[T Numeric](t *testing.T, text string, want T, wantOK bool) {
	t.Helper()
	typ := reflect.TypeFor[T]()
	bits := typ.Bits()
	typed, typedOK := parseNumeric[T](text, bits)
	if typedOK != wantOK {
		t.Fatalf("parseNumeric[%v](%q) ok = %v, want %v", typ, text, typedOK, wantOK)
	}
	if typedOK && typed != want {
		t.Fatalf("parseNumeric[%v](%q) = %v, want %v", typ, text, typed, want)
	}

	ops, ok := newNumericOps(typ)
	if !ok {
		t.Fatalf("newNumericOps(%T) rejected numeric type", want)
	}
	value, gotOK := testNumericOpsParse[T](ops, text)
	if gotOK != wantOK {
		t.Fatalf("numericOps(%v).parse(%q) ok = %v, want %v", typ, text, gotOK, wantOK)
	}
	if gotOK && value != want {
		t.Fatalf("numericOps(%v).parse(%q) = %v, want %v", typ, text, value, want)
	}
}

func testNumericFormat[T Numeric](t *testing.T, name string, value T, wantText string, wantErr error) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		typ := reflect.TypeFor[T]()
		ops, ok := newNumericOps(typ)
		if !ok {
			t.Fatalf("newNumericOps(%v) rejected numeric type", typ)
		}
		typedText, typedErr := formatNumeric(value, typ.Bits())
		reflectedText, reflectedErr := testNumericOpsFormat(ops, value)
		for _, result := range []struct {
			name string
			text string
			err  error
		}{
			{"typed", typedText, typedErr},
			{"reflected", reflectedText, reflectedErr},
		} {
			if result.text != wantText || !errors.Is(result.err, wantErr) {
				t.Errorf("%s format = (%q, %v), want (%q, %v)", result.name, result.text, result.err, wantText, wantErr)
			}
		}
	})
}

func TestNumericParseIntegerSyntax(t *testing.T) {
	tests := []struct {
		text string
		want int64
		ok   bool
	}{
		{"1", 1, true},
		{"01", 1, true},
		{"-0", 0, true},
		{"+1", 1, true},
		{"1.0", 0, false},
		{"1e3", 0, false},
		{"100e-2", 0, false},
		{"1.2300e2", 0, false},
		{".00100e3", 0, false},
		{"1.2", 0, false},
		{"10e-2", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			testNumericParse(t, tt.text, tt.want, tt.ok)
		})
	}
	testNumericParse(t, "-0", uint64(0), false)
	testNumericParse(t, "-1", uint64(0), false)
}

func TestNumericParseIntegerBoundaries(t *testing.T) {
	testNumericParse(t, "-128", int8(-128), true)
	testNumericParse(t, "127", int8(127), true)
	testNumericParse(t, "-129", int8(0), false)
	testNumericParse(t, "128", int8(0), false)
	testNumericParse(t, "-32768", int16(-32768), true)
	testNumericParse(t, "32767", int16(32767), true)
	testNumericParse(t, "-32769", int16(0), false)
	testNumericParse(t, "32768", int16(0), false)
	testNumericParse(t, "-2147483648", int32(-2147483648), true)
	testNumericParse(t, "2147483647", int32(2147483647), true)
	testNumericParse(t, "-2147483649", int32(0), false)
	testNumericParse(t, "2147483648", int32(0), false)
	testNumericParse(t, "-9223372036854775808", int64(math.MinInt64), true)
	testNumericParse(t, "9223372036854775807", int64(math.MaxInt64), true)
	testNumericParse(t, "-9223372036854775809", int64(0), false)
	testNumericParse(t, "9223372036854775808", int64(0), false)

	testNumericParse(t, "255", uint8(math.MaxUint8), true)
	testNumericParse(t, "256", uint8(0), false)
	testNumericParse(t, "65535", uint16(math.MaxUint16), true)
	testNumericParse(t, "65536", uint16(0), false)
	testNumericParse(t, "4294967295", uint32(math.MaxUint32), true)
	testNumericParse(t, "4294967296", uint32(0), false)
	testNumericParse(t, "18446744073709551615", uint64(math.MaxUint64), true)
	testNumericParse(t, "18446744073709551616", uint64(0), false)

	maxUint := ^uint(0)
	maxInt := int(maxUint >> 1)
	minInt := -maxInt - 1
	maxUintptr := ^uintptr(0)
	if strconv.IntSize == 64 {
		testNumericParse(t, "-9223372036854775808", minInt, true)
		testNumericParse(t, "9223372036854775807", maxInt, true)
		testNumericParse(t, "9223372036854775808", int(0), false)
		testNumericParse(t, "18446744073709551615", maxUint, true)
		testNumericParse(t, "18446744073709551616", uint(0), false)
		testNumericParse(t, "18446744073709551615", maxUintptr, true)
		testNumericParse(t, "18446744073709551616", uintptr(0), false)
	} else {
		testNumericParse(t, "-2147483648", minInt, true)
		testNumericParse(t, "2147483647", maxInt, true)
		testNumericParse(t, "2147483648", int(0), false)
		testNumericParse(t, "4294967295", maxUint, true)
		testNumericParse(t, "4294967296", uint(0), false)
		testNumericParse(t, "4294967295", maxUintptr, true)
		testNumericParse(t, "4294967296", uintptr(0), false)
	}
}

type (
	numericNamedInt       int16
	numericNamedUint      uint64
	numericNamedSmallUint uint16
	numericNamedFloat     float32
)

func TestNumericNamedTypes(t *testing.T) {
	testNumericParse(t, "-32768", numericNamedInt(-32768), true)
	testNumericParse(t, "-32769", numericNamedInt(0), false)
	testNumericParse(t, "18446744073709551615", numericNamedUint(math.MaxUint64), true)
	testNumericParse(t, "65536", numericNamedSmallUint(0), false)
	testNumericParse(t, "0.1", numericNamedFloat(0.1), true)
	testNumericParse(t, "3.4028236e38", numericNamedFloat(0), false)
}

func TestNumericFormatting(t *testing.T) {
	testNumericFormat(t, "signed", numericNamedInt(-12345), "-12345", nil)
	testNumericFormat(t, "unsigned", numericNamedUint(math.MaxUint64), "18446744073709551615", nil)
	testNumericFormat(t, "float32", numericNamedFloat(0.1), "0.1", nil)
	testNumericFormat(t, "float64", float64(1.25), "1.25", nil)
	testNumericFormat(t, "negative zero", math.Copysign(0, -1), "-0", nil)
	testNumericFormat(t, "infinity", math.Inf(1), "", jaws.ErrValueNotFinite)
	testNumericFormat(t, "NaN", float32(math.NaN()), "", jaws.ErrValueNotFinite)
}

func TestNumericFloatParsing(t *testing.T) {
	testNumericParse(t, "1e-46", float32(0), true)
	testNumericParse(t, "3.4028236e38", float32(0), false)
	testNumericParse(t, "NaN", float32(0), false)
	testNumericParse(t, "not a number", float32(0), false)
	testNumericParse(t, "1.0000000596046448", math.Float32frombits(0x3f800001), true)

	ops64, _ := newNumericOps(reflect.TypeFor[float64]())
	typed, typedOK := parseNumeric[float64]("-0", 64)
	reflected, reflectedOK := testNumericOpsParse[float64](ops64, "-0")
	if !typedOK || !math.Signbit(typed) || !reflectedOK || !math.Signbit(reflected) {
		t.Fatalf("negative zero parse = typed(%v, %v), reflected(%v, %v)", typed, typedOK, reflected, reflectedOK)
	}
}

type numericTestSource[T comparable] struct {
	value    T
	input    *Input
	seenLast any
}

func (source *numericTestSource[T]) JawsGet(*jaws.Element) T {
	return source.value
}

func (source *numericTestSource[T]) JawsSet(_ *jaws.Element, value T) error {
	if source.input != nil {
		source.seenLast = source.input.Last.Load()
	}
	source.value = value
	return nil
}

func TestTypedNumericBindingStoresLastBeforeSet(t *testing.T) {
	// Observing Last at setter entry pins the ordering needed when a setter hook
	// dirties the Element and lets JawsUpdate run before JawsSet returns.
	input := &Input{}
	source := &numericTestSource[numericNamedInt]{input: input}
	binding := newNumericBinding[numericNamedInt](source)
	accepted, err := binding.acceptText(input, nil, "-123")
	if !accepted || err != nil || source.value != -123 {
		t.Fatalf("typed accept = (%v, %v), value = %v", accepted, err, source.value)
	}
	if source.seenLast != "-123" {
		t.Fatalf("Last observed by typed setter = %v, want -123", source.seenLast)
	}
}

type numericRejectedBinding struct {
	err error
}

func (binding numericRejectedBinding) sourceValue() any {
	return nil
}

func (binding numericRejectedBinding) writable() bool {
	return true
}

func (binding numericRejectedBinding) getText(*jaws.Element) (string, error) {
	return "", nil
}

func (binding numericRejectedBinding) acceptText(*Input, *jaws.Element, string) (bool, error) {
	return false, binding.err
}

func TestHandleNumericInputRejectsWithoutError(t *testing.T) {
	_, rq := newCoreRequest(t)
	elem := rq.NewElement(nil)
	input := &Input{}
	input.Last.Store("previous")
	err := handleNumericInput(input, numericRejectedBinding{err: errors.New("ignored")}, elem, "bad")
	if err != nil {
		t.Fatalf("handleNumericInput rejected error = %v, want nil", err)
	}
	if last := input.Last.Load(); last != "" {
		t.Fatalf("Last after rejected input = %v, want empty string", last)
	}
}

type numericBadGetterArity struct{}

func (numericBadGetterArity) JawsGet() int {
	return 1
}

type numericStringGetter struct{}

func (numericStringGetter) JawsGet(*jaws.Element) string {
	return "1"
}

type numericBadSetter struct{}

func (numericBadSetter) JawsGet(*jaws.Element) int {
	return 1
}

func (numericBadSetter) JawsSet(*jaws.Element, int) bool {
	return false
}

type numericErrorSetter struct {
	err      error
	input    *Input
	seenLast any
}

func (*numericErrorSetter) JawsGet(*jaws.Element) int {
	return 1
}

func (source *numericErrorSetter) JawsSet(*jaws.Element, int) error {
	source.seenLast = source.input.Last.Load()
	return source.err
}

func TestMakeNumericBinding(t *testing.T) {
	source := &numericTestSource[numericNamedUint]{value: 7}
	nb, ok := makeNumericBinding(source)
	if !ok || !nb.writable() || nb.sourceValue() != source {
		t.Fatalf("reflected binding = (%v, %v)", nb, ok)
	}
	text, err := nb.getText(nil)
	if err != nil || text != "7" {
		t.Fatalf("reflected get = (%q, %v), want (7, nil)", text, err)
	}
	input := &Input{}
	accepted, err := nb.acceptText(input, nil, "18446744073709551615")
	if !accepted || err != nil || source.value != numericNamedUint(math.MaxUint64) {
		t.Fatalf("reflected accept = (%v, %v), value = %v", accepted, err, source.value)
	}
	if last := input.Last.Load(); last != "18446744073709551615" {
		t.Fatalf("reflected accept stored Last = %v", last)
	}

	static, ok := makeNumericBinding(numericNamedInt(-12))
	if !ok || static.writable() || static.sourceValue() != nil {
		t.Fatalf("static binding = (%v, %v)", static, ok)
	}
	if text, err = static.getText(nil); err != nil || text != "-12" {
		t.Fatalf("static get text = %q, error = %v", text, err)
	}

	invalidSources := []struct {
		name   string
		source any
	}{
		{name: "nil", source: nil},
		{name: "string", source: "not numeric"},
		{name: "bad getter arity", source: numericBadGetterArity{}},
		{name: "string getter", source: numericStringGetter{}},
	}
	for _, tt := range invalidSources {
		t.Run(tt.name, func(t *testing.T) {
			if binding, accepted := makeNumericBinding(tt.source); accepted || binding != nil {
				t.Fatalf("makeNumericBinding(%T) = (%v, %v), want zero binding and false", tt.source, binding, accepted)
			}
		})
	}

	badSetter, ok := makeNumericBinding(numericBadSetter{})
	if !ok || badSetter.writable() {
		t.Fatalf("malformed setter binding = (%v, %v), want read-only binding", badSetter, ok)
	}

	setErr := errors.New("numeric setter error")
	errorInput := &Input{}
	errorSource := &numericErrorSetter{err: setErr, input: errorInput}
	errorSetter, ok := makeNumericBinding(errorSource)
	if !ok || !errorSetter.writable() {
		t.Fatalf("error setter binding = (%v, %v), want writable binding", errorSetter, ok)
	}
	accepted, err = errorSetter.acceptText(errorInput, nil, "2")
	if !accepted || !errors.Is(err, setErr) {
		t.Fatalf("reflected accept = (%v, %v), want (true, %v)", accepted, err, setErr)
	}
	if errorSource.seenLast != "2" {
		t.Fatalf("Last observed by setter = %v, want 2", errorSource.seenLast)
	}
}
