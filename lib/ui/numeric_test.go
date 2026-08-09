package ui

import (
	"errors"
	"math"
	"reflect"
	"strconv"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

func TestScanHTMLNumber(t *testing.T) {
	tests := []struct {
		text string
		ok   bool
	}{
		{"0", true},
		{"-0", true},
		{"00", true},
		{"1", true},
		{"-1", true},
		{".5", true},
		{"-.5", true},
		{"1.0", true},
		{"1e3", true},
		{"1E+3", true},
		{"1e-3", true},
		{"-1.25e+3", true},
		{"", false},
		{"+1", false},
		{" 1", false},
		{"1 ", false},
		{".", false},
		{"-.", false},
		{"1.", false},
		{"1e", false},
		{"1e+", false},
		{"1e-", false},
		{"1_0", false},
		{"0x1p0", false},
		{"NaN", false},
		{"Inf", false},
		{"Infinity", false},
		{"１", false},
		{"−1", false},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			_, ok := scanHTMLNumber(tt.text)
			if ok != tt.ok {
				t.Fatalf("scanHTMLNumber(%q) ok = %v, want %v", tt.text, ok, tt.ok)
			}
		})
	}
}

func testNumericParse[T Numeric](t *testing.T, text string, want T, wantOK bool) {
	t.Helper()
	ops, ok := newNumericOps(reflect.TypeFor[T]())
	if !ok {
		t.Fatalf("newNumericOps(%T) rejected numeric type", want)
	}
	value, gotOK, err := ops.parse(text)
	if err != nil {
		t.Fatalf("parse(%q): %v", text, err)
	}
	if gotOK != wantOK {
		t.Fatalf("parse(%q) ok = %v, want %v", text, gotOK, wantOK)
	}
	if gotOK {
		if got := value.(T); got != want {
			t.Fatalf("parse(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestNumericParseExactIntegers(t *testing.T) {
	tests := []struct {
		text string
		want int64
		ok   bool
	}{
		{"1", 1, true},
		{"01", 1, true},
		{"1.0", 1, true},
		{"1e3", 1000, true},
		{"100e-2", 1, true},
		{"1.2300e2", 123, true},
		{".00100e3", 1, true},
		{"-0e999999999999999999999999", 0, true},
		{"1.2", 0, false},
		{"10e-2", 0, false},
		{"1e-999999999999999999999999", 0, false},
		{"1e999999999999999999999999", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			testNumericParse(t, tt.text, tt.want, tt.ok)
		})
	}
	testNumericParse(t, "-0", uint64(0), true)
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
	testNumericParse(t, "184467440737095516150e-1", uint64(math.MaxUint64), true)

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
	value, ok, err := ops32.parse("1e-46")
	if err != nil || !ok || value.(float32) != 0 {
		t.Fatalf("float32 underflow = (%v, %v, %v), want (0, true, nil)", value, ok, err)
	}
	if _, ok, err = ops32.parse("3.4028236e38"); err != nil || ok {
		t.Fatalf("float32 overflow = (_, %v, %v), want (_, false, nil)", ok, err)
	}
	if _, ok, err = ops32.parse("NaN"); err != nil || ok {
		t.Fatalf("NaN = (_, %v, %v), want (_, false, nil)", ok, err)
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
	value, ok, err = ops64.parse(text)
	if err != nil || !ok || !math.Signbit(value.(float64)) {
		t.Fatalf("negative zero round trip = (%v, %v, %v)", value, ok, err)
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

type numericTestCodec[T comparable] struct {
	format func(T) string
	parse  func(string) (T, bool)
}

func (codec numericTestCodec[T]) FormatNumber(value T) string {
	return codec.format(value)
}

func (codec numericTestCodec[T]) ParseNumber(text string) (T, bool) {
	return codec.parse(text)
}

func TestCustomNumericBindingValidation(t *testing.T) {
	source := &numericTestSource[int]{value: 7}
	parseCalls := 0
	codec := numericTestCodec[int]{
		format: strconv.Itoa,
		parse: func(text string) (int, bool) {
			parseCalls++
			value, err := strconv.Atoi(text)
			return value, err == nil
		},
	}
	nb := newCustomNumericBinding[int](source, codec)
	value, text, err := nb.get(nil)
	if err != nil || value != 7 || text != "7" {
		t.Fatalf("get = (%v, %q, %v), want (7, 7, nil)", value, text, err)
	}
	parsed, ok, err := nb.parse("8")
	if err != nil || !ok || parsed != 8 {
		t.Fatalf("parse = (%v, %v, %v), want (8, true, nil)", parsed, ok, err)
	}
	if err = nb.set(nil, parsed); err != nil || source.value != 8 {
		t.Fatalf("set = %v, value = %d", err, source.value)
	}
	parseCallsBeforeInvalid := parseCalls
	if _, ok, err = nb.parse("8 "); err != nil || ok {
		t.Fatalf("invalid HTML parse = (_, %v, %v), want (_, false, nil)", ok, err)
	}
	if parseCalls != parseCallsBeforeInvalid {
		t.Fatalf("invalid HTML called ParseNumber %d times", parseCalls-parseCallsBeforeInvalid)
	}
	rejectingCodec := numericTestCodec[int]{
		format: strconv.Itoa,
		parse: func(text string) (int, bool) {
			if text == "7" {
				return 7, true
			}
			return 0, false
		},
	}
	if _, ok, err = newCustomNumericBinding[int](source, rejectingCodec).parse("8"); err != nil || ok {
		t.Fatalf("codec-rejected parse = (_, %v, %v), want (_, false, nil)", ok, err)
	}

	badFormat := numericTestCodec[int]{
		format: func(int) string { return "+7" },
		parse:  codec.parse,
	}
	if _, _, err = newCustomNumericBinding[int](source, badFormat).get(nil); !errors.Is(err, ErrNumberFormat) {
		t.Fatalf("invalid format error = %v, want ErrNumberFormat", err)
	}
	badRoundTrip := numericTestCodec[int]{
		format: strconv.Itoa,
		parse:  func(string) (int, bool) { return 9, true },
	}
	if _, _, err = newCustomNumericBinding[int](source, badRoundTrip).get(nil); !errors.Is(err, ErrNumberFormat) {
		t.Fatalf("round-trip error = %v, want ErrNumberFormat", err)
	}
}

type numericLooseValue struct {
	Value any
}

type numericNanValue struct {
	Value float64
}

func TestCustomNumericBindingComparability(t *testing.T) {
	looseSource := &numericTestSource[numericLooseValue]{value: numericLooseValue{Value: 1}}
	looseCodec := numericTestCodec[numericLooseValue]{
		format: func(numericLooseValue) string { return "1" },
		parse:  func(string) (numericLooseValue, bool) { return numericLooseValue{Value: 1}, true },
	}
	if _, _, err := newCustomNumericBinding(looseSource, looseCodec).get(nil); !errors.Is(err, ErrNumberFormat) {
		t.Fatalf("non-strict type error = %v, want ErrNumberFormat", err)
	}

	nanSource := &numericTestSource[numericNanValue]{value: numericNanValue{Value: math.NaN()}}
	nanCodec := numericTestCodec[numericNanValue]{
		format: func(numericNanValue) string { return "0" },
		parse:  func(string) (numericNanValue, bool) { return numericNanValue{Value: math.NaN()}, true },
	}
	if _, _, err := newCustomNumericBinding(nanSource, nanCodec).get(nil); !errors.Is(err, ErrNumberFormat) {
		t.Fatalf("non-reflexive bound error = %v, want ErrNumberFormat", err)
	}

	reflexiveSource := &numericTestSource[numericNanValue]{value: numericNanValue{Value: 0}}
	if _, ok, err := newCustomNumericBinding(reflexiveSource, nanCodec).parse("1"); ok || !errors.Is(err, ErrNumberFormat) {
		t.Fatalf("non-reflexive parsed result = (_, %v, %v), want (_, false, ErrNumberFormat)", ok, err)
	}
}

func TestMakeNumericBinding(t *testing.T) {
	source := &numericTestSource[numericNamedUint]{value: 7}
	nb, ok := makeNumericBinding(source)
	if !ok || !nb.writable() || nb.source() != source {
		t.Fatalf("reflected binding = (%v, %v)", nb, ok)
	}
	value, text, err := nb.get(nil)
	if err != nil || value != numericNamedUint(7) || text != "7" {
		t.Fatalf("reflected get = (%v, %q, %v)", value, text, err)
	}
	parsed, ok, err := nb.parse("18446744073709551615")
	if err != nil || !ok || parsed != numericNamedUint(math.MaxUint64) {
		t.Fatalf("reflected parse = (%v, %v, %v)", parsed, ok, err)
	}
	if err = nb.set(nil, parsed); err != nil || source.value != numericNamedUint(math.MaxUint64) {
		t.Fatalf("reflected set = %v, value = %v", err, source.value)
	}

	static, ok := makeNumericBinding(numericNamedInt(-12))
	if !ok || static.writable() || static.source() != nil {
		t.Fatalf("static binding = (%v, %v)", static, ok)
	}
	if _, text, err = static.get(nil); err != nil || text != "-12" {
		t.Fatalf("static get text = %q, error = %v", text, err)
	}
	if err = static.set(nil, numericNamedInt(3)); !errors.Is(err, bind.ErrValueNotSettable) {
		t.Fatalf("static set error = %v", err)
	}

	codec := numericTestCodec[int]{
		format: strconv.Itoa,
		parse: func(text string) (int, bool) {
			value, err := strconv.Atoi(text)
			return value, err == nil
		},
	}
	custom, ok := makeNumericBinding(NewNumericBinding(&numericTestSource[int]{value: 2}, codec))
	if !ok || custom == nil {
		t.Fatal("custom NumericBinding was not unwrapped")
	}
	if _, ok = makeNumericBinding("not numeric"); ok {
		t.Fatal("string source was accepted")
	}
}

func FuzzScanHTMLNumber(f *testing.F) {
	for _, seed := range []string{"0", "-.5e+2", "1.", "1e999999", "NaN"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		_, _ = scanHTMLNumber(text)
	})
}
