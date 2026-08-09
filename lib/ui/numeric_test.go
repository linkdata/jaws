package ui

import (
	"errors"
	"math"
	"reflect"
	"strconv"
	"testing"

	"github.com/linkdata/jaws"
)

func testNumericParse[T Numeric](t *testing.T, text string, want T, wantOK bool) {
	t.Helper()
	ops, ok := newNumericOps(reflect.TypeFor[T]())
	if !ok {
		t.Fatalf("newNumericOps(%T) rejected numeric type", want)
	}
	value, gotOK := ops.parse(text)
	if gotOK != wantOK {
		t.Fatalf("parse(%q) ok = %v, want %v", text, gotOK, wantOK)
	}
	if gotOK {
		if got := value.(T); got != want {
			t.Fatalf("parse(%q) = %v, want %v", text, got, want)
		}
	}
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
		testNumericParse(t, "18446744073709551615", maxUintptr, true)
	} else {
		testNumericParse(t, "-2147483648", minInt, true)
		testNumericParse(t, "2147483647", maxInt, true)
		testNumericParse(t, "2147483648", int(0), false)
		testNumericParse(t, "4294967295", maxUint, true)
		testNumericParse(t, "4294967295", maxUintptr, true)
	}
}

type (
	numericNamedInt   int16
	numericNamedUint  uint64
	numericNamedFloat float32
)

func TestNumericNamedTypes(t *testing.T) {
	testNumericParse(t, "-32768", numericNamedInt(-32768), true)
	testNumericParse(t, "18446744073709551615", numericNamedUint(math.MaxUint64), true)
	testNumericParse(t, "0.1", numericNamedFloat(0.1), true)
}

func TestNumericFloatParsingAndFormatting(t *testing.T) {
	ops32, _ := newNumericOps(reflect.TypeFor[float32]())
	text, err := ops32.format(float32(0.1))
	if err != nil {
		t.Fatal(err)
	}
	if text != "0.1" {
		t.Fatalf("float32(0.1) formatted as %q", text)
	}
	value, ok := ops32.parse("1e-46")
	if !ok || value.(float32) != 0 {
		t.Fatalf("float32 underflow = (%v, %v), want (0, true)", value, ok)
	}
	if _, ok = ops32.parse("3.4028236e38"); ok {
		t.Fatal("float32 overflow was accepted")
	}
	if _, ok = ops32.parse("NaN"); ok {
		t.Fatal("NaN was accepted")
	}
	if _, ok = ops32.parse("not a number"); ok {
		t.Fatal("malformed float was accepted")
	}
	if _, err = ops32.format(float32(math.Inf(1))); !errors.Is(err, jaws.ErrValueNotFinite) {
		t.Fatalf("format(+Inf) error = %v, want ErrValueNotFinite", err)
	}
	if _, err = ops32.format(float32(math.NaN())); !errors.Is(err, jaws.ErrValueNotFinite) {
		t.Fatalf("format(NaN) error = %v, want ErrValueNotFinite", err)
	}

	ops64, _ := newNumericOps(reflect.TypeFor[float64]())
	negativeZero := math.Copysign(0, -1)
	text, err = ops64.format(negativeZero)
	if err != nil {
		t.Fatal(err)
	}
	value, ok = ops64.parse(text)
	if !ok || !math.Signbit(value.(float64)) {
		t.Fatalf("negative zero round trip = (%v, %v)", value, ok)
	}
}

type numericTestSource[T comparable] struct {
	value T
}

func (source *numericTestSource[T]) JawsGet(*jaws.Element) T {
	return source.value
}

func (source *numericTestSource[T]) JawsSet(_ *jaws.Element, value T) error {
	source.value = value
	return nil
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
	err error
}

func (numericErrorSetter) JawsGet(*jaws.Element) int {
	return 1
}

func (source numericErrorSetter) JawsSet(*jaws.Element, int) error {
	return source.err
}

func TestMakeNumericBinding(t *testing.T) {
	source := &numericTestSource[numericNamedUint]{value: 7}
	nb, ok := makeNumericBinding(source)
	if !ok || !nb.writable() || nb.source != source {
		t.Fatalf("reflected binding = (%v, %v)", nb, ok)
	}
	text, err := nb.getText(nil)
	if err != nil || text != "7" {
		t.Fatalf("reflected get = (%q, %v), want (7, nil)", text, err)
	}
	value, accepted := nb.ops.parse("18446744073709551615")
	if !accepted {
		t.Fatal("reflected binding rejected uint64 maximum")
	}
	if err = nb.setValue(nil, value); err != nil || source.value != numericNamedUint(math.MaxUint64) {
		t.Fatalf("reflected set = %v, value = %v", err, source.value)
	}

	static, ok := makeNumericBinding(numericNamedInt(-12))
	if !ok || static.writable() || static.source != nil {
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
			if binding, accepted := makeNumericBinding(tt.source); accepted || binding.getValue != nil {
				t.Fatalf("makeNumericBinding(%T) = (%v, %v), want zero binding and false", tt.source, binding, accepted)
			}
		})
	}

	badSetter, ok := makeNumericBinding(numericBadSetter{})
	if !ok || badSetter.writable() {
		t.Fatalf("malformed setter binding = (%v, %v), want read-only binding", badSetter, ok)
	}

	setErr := errors.New("numeric setter error")
	errorSetter, ok := makeNumericBinding(numericErrorSetter{err: setErr})
	if !ok || !errorSetter.writable() {
		t.Fatalf("error setter binding = (%v, %v), want writable binding", errorSetter, ok)
	}
	if err = errorSetter.setValue(nil, 2); !errors.Is(err, setErr) {
		t.Fatalf("reflected setter error = %v, want %v", err, setErr)
	}
}
