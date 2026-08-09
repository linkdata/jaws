package ui

import (
	"fmt"
	"math"
	"reflect"
	"strconv"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Numeric is a built-in or named real numeric type supported by Number and Range.
type Numeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64
}

// NumberCodec converts application values to and from HTML number text.
//
// T must be strictly comparable, and bound and parsed values must equal
// themselves. FormatNumber must be total and deterministic. Its result must be a
// valid HTML floating-point number that ParseNumber maps back to the original
// value. ParseNumber returns false for text it does not accept and is called for
// browser input only after JaWS validates that grammar. A codec may be shared by
// concurrent requests and must be safe for concurrent use.
type NumberCodec[T comparable] interface {
	FormatNumber(value T) string
	ParseNumber(text string) (value T, ok bool)
}

// NumericBinding supplies a custom Number codec to the Number template helper.
//
// Construct NumericBinding values with NewNumericBinding. Its zero value is not
// usable.
type NumericBinding struct {
	binding *numericBinding
}

// NewNumericBinding returns a template Number binding over source using codec.
//
// Source capability, tag, and codec requirements are the same as for
// [NewNumberWith].
func NewNumericBinding[T comparable](source bind.Getter[T], codec NumberCodec[T]) *NumericBinding {
	return &NumericBinding{binding: newCustomNumericBinding(source, codec)}
}

// numericBinding erases a source's concrete type while retaining exact typed
// getter, setter, parser, formatter, and comparison operations.
type numericBinding struct {
	sourceValue any
	getValue    func(*jaws.Element) any
	setValue    func(*jaws.Element, any) error
	parseValue  func(string) (any, bool, error)
	formatValue func(any) (string, error)
	equalValue  func(any, any) bool
	configErr   error
}

func (nb *numericBinding) source() any {
	return nb.sourceValue
}

func (nb *numericBinding) writable() bool {
	return nb.setValue != nil
}

func (nb *numericBinding) get(elem *jaws.Element) (value any, text string, err error) {
	if err = nb.configErr; err == nil {
		value = nb.getValue(elem)
		text, err = nb.formatValue(value)
	}
	return
}

func (nb *numericBinding) parse(text string) (value any, ok bool, err error) {
	if err = nb.configErr; err == nil {
		value, ok, err = nb.parseValue(text)
	}
	return
}

func (nb *numericBinding) set(elem *jaws.Element, value any) (err error) {
	if nb.setValue == nil {
		err = bind.ErrValueNotSettable
	} else {
		err = nb.setValue(elem, value)
	}
	return
}

func (nb *numericBinding) equal(a, b any) bool {
	return nb.equalValue(a, b)
}

func newBuiltinNumericBinding[T Numeric](source bind.Getter[T]) *numericBinding {
	typ := reflect.TypeFor[T]()
	ops, _ := newNumericOps(typ)
	nb := &numericBinding{
		sourceValue: source,
		getValue: func(elem *jaws.Element) any {
			return source.JawsGet(elem)
		},
		parseValue:  ops.parse,
		formatValue: ops.format,
		equalValue: func(a, b any) bool {
			return a.(T) == b.(T)
		},
	}
	if setter, ok := any(source).(bind.Setter[T]); ok {
		nb.setValue = func(elem *jaws.Element, value any) error {
			return setter.JawsSet(elem, value.(T))
		}
	}
	return nb
}

func newCustomNumericBinding[T comparable](source bind.Getter[T], codec NumberCodec[T]) *numericBinding {
	typ := reflect.TypeFor[T]()
	nb := &numericBinding{
		sourceValue: source,
		getValue: func(elem *jaws.Element) any {
			return source.JawsGet(elem)
		},
	}
	if !strictlyComparable(typ) {
		nb.configErr = newErrNumberFormat(typ.String() + " is not strictly comparable")
	}
	if isNilValue(codec) {
		nb.configErr = newErrNumberFormat("nil codec")
	}
	nb.equalValue = func(a, b any) bool {
		return a.(T) == b.(T)
	}
	nb.formatValue = func(value any) (text string, err error) {
		if err = nb.configErr; err != nil {
			return
		}
		v := value.(T)
		if v != v {
			err = newErrNumberFormat("bound value is not reflexive")
			return
		}
		text = codec.FormatNumber(v)
		if _, ok := scanHTMLNumber(text); !ok {
			err = newErrNumberFormat("FormatNumber returned invalid HTML number text")
			return
		}
		var roundTrip T
		var ok bool
		if roundTrip, ok = codec.ParseNumber(text); !ok {
			err = newErrNumberFormat("ParseNumber rejected formatted output")
			return
		}
		if roundTrip != roundTrip {
			err = newErrNumberFormat("formatted output parsed to a non-reflexive value")
			return
		}
		if roundTrip != v {
			err = newErrNumberFormat("formatted output did not round-trip")
		}
		return
	}
	nb.parseValue = func(text string) (value any, ok bool, err error) {
		if err = nb.configErr; err != nil {
			return
		}
		if _, valid := scanHTMLNumber(text); !valid {
			return
		}
		var parsed T
		if parsed, ok = codec.ParseNumber(text); !ok {
			return nil, false, nil
		}
		if parsed != parsed {
			err = newErrNumberFormat("ParseNumber returned a non-reflexive value")
			ok = false
			return
		}
		if _, err = nb.formatValue(parsed); err != nil {
			ok = false
			return
		}
		value = parsed
		return
	}
	if setter, ok := any(source).(bind.Setter[T]); ok {
		nb.setValue = func(elem *jaws.Element, value any) error {
			return setter.JawsSet(elem, value.(T))
		}
	}
	return nb
}

// makeNumericBinding converts template Number and Range sources to the shared
// type-erased representation. It accepts NumericBinding, reflected numeric
// Getter/Setter implementations, and static built-in or named numeric values.
func makeNumericBinding(source any) (nb *numericBinding, ok bool) {
	if custom, yes := source.(*NumericBinding); yes {
		if custom != nil && custom.binding != nil {
			return custom.binding, true
		}
		return nil, false
	}
	rv := reflect.ValueOf(source)
	if !rv.IsValid() {
		return nil, false
	}
	if getter, valueType, yes := reflectedNumericGetter(rv); yes {
		ops, _ := newNumericOps(valueType)
		nb = &numericBinding{
			sourceValue: source,
			getValue: func(elem *jaws.Element) any {
				out := getter.Call([]reflect.Value{reflect.ValueOf(elem)})
				return out[0].Interface()
			},
			parseValue:  ops.parse,
			formatValue: ops.format,
			equalValue: func(a, b any) bool {
				return a == b
			},
		}
		if setter, yes := reflectedNumericSetter(rv, valueType); yes {
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
		nb = &numericBinding{
			getValue: func(*jaws.Element) any {
				return rv.Interface()
			},
			parseValue:  ops.parse,
			formatValue: ops.format,
			equalValue: func(a, b any) bool {
				return a == b
			},
		}
		return nb, true
	}
	return nil, false
}

var (
	elementPointerType = reflect.TypeFor[*jaws.Element]()
	errorType          = reflect.TypeFor[error]()
)

func reflectedNumericGetter(source reflect.Value) (getter reflect.Value, valueType reflect.Type, ok bool) {
	getter = source.MethodByName("JawsGet")
	if !getter.IsValid() {
		return reflect.Value{}, nil, false
	}
	typ := getter.Type()
	if typ.NumIn() != 1 || typ.In(0) != elementPointerType || typ.NumOut() != 1 {
		return reflect.Value{}, nil, false
	}
	valueType = typ.Out(0)
	if _, ok = newNumericOps(valueType); !ok {
		return reflect.Value{}, nil, false
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

func isNilValue(value any) bool {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return true
	}
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	}
	return false
}

func strictlyComparable(typ reflect.Type) bool {
	switch typ.Kind() {
	case reflect.Interface:
		return false
	case reflect.Array:
		return strictlyComparable(typ.Elem())
	case reflect.Struct:
		for i := 0; i < typ.NumField(); i++ {
			if !strictlyComparable(typ.Field(i).Type) {
				return false
			}
		}
		return true
	default:
		return typ.Comparable()
	}
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
	if !rv.IsValid() || rv.Type() != ops.typ {
		err = newErrNumberFormat("numeric value has the wrong type")
		return
	}
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

func (ops numericOps) parse(text string) (value any, ok bool, err error) {
	parsed, valid := scanHTMLNumber(text)
	if !valid {
		return nil, false, nil
	}
	rv := reflect.New(ops.typ).Elem()
	switch ops.kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var integerText string
		if integerText, ok = parsed.integerText(true); ok {
			var n int64
			if n, err = strconv.ParseInt(integerText, 10, ops.bits); err == nil {
				rv.SetInt(n)
				value = rv.Interface()
				return value, true, nil
			}
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		var integerText string
		if integerText, ok = parsed.integerText(false); ok {
			var n uint64
			if n, err = strconv.ParseUint(integerText, 10, ops.bits); err == nil {
				rv.SetUint(n)
				value = rv.Interface()
				return value, true, nil
			}
		}
	case reflect.Float32, reflect.Float64:
		var n float64
		if n, err = strconv.ParseFloat(text, ops.bits); err == nil && !math.IsNaN(n) && !math.IsInf(n, 0) {
			rv.SetFloat(n)
			value = rv.Interface()
			return value, true, nil
		}
	}
	// Browser parse rejection is deliberately not exposed as a Go error. Numeric
	// widgets restore the canonical source value without turning validation into
	// a handler error.
	return nil, false, nil
}

// htmlNumber is a validated WHATWG valid floating-point number split into
// zero-copy decimal components.
type htmlNumber struct {
	negative       bool
	integerDigits  string
	fractionDigits string
	exponentDigits string
	exponentNeg    bool
}

func scanHTMLNumber(text string) (number htmlNumber, ok bool) {
	if text == "" {
		return
	}
	i := 0
	if text[i] == '-' {
		number.negative = true
		i++
		if i == len(text) {
			return number, false
		}
	}
	start := i
	for i < len(text) && isASCIIDigit(text[i]) {
		i++
	}
	number.integerDigits = text[start:i]
	if i < len(text) && text[i] == '.' {
		i++
		start = i
		for i < len(text) && isASCIIDigit(text[i]) {
			i++
		}
		number.fractionDigits = text[start:i]
		if number.fractionDigits == "" {
			return number, false
		}
	}
	if number.integerDigits == "" && number.fractionDigits == "" {
		return number, false
	}
	if i < len(text) && (text[i] == 'e' || text[i] == 'E') {
		i++
		if i < len(text) && (text[i] == '+' || text[i] == '-') {
			number.exponentNeg = text[i] == '-'
			i++
		}
		start = i
		for i < len(text) && isASCIIDigit(text[i]) {
			i++
		}
		number.exponentDigits = text[start:i]
		if number.exponentDigits == "" {
			return number, false
		}
	}
	ok = i == len(text)
	return
}

func isASCIIDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func (number htmlNumber) significandLen() int {
	return len(number.integerDigits) + len(number.fractionDigits)
}

func (number htmlNumber) significandDigit(index int) byte {
	if index < len(number.integerDigits) {
		return number.integerDigits[index]
	}
	return number.fractionDigits[index-len(number.integerDigits)]
}

func (number htmlNumber) exponent(limit int) int {
	value := 0
	for i := range len(number.exponentDigits) {
		digit := int(number.exponentDigits[i] - '0')
		if value > (limit-digit)/10 {
			value = limit
			break
		}
		value = value*10 + digit
	}
	if number.exponentNeg {
		return -value
	}
	return value
}

func (number htmlNumber) integerText(signed bool) (text string, ok bool) {
	digitCount := number.significandLen()
	first := 0
	for first < digitCount && number.significandDigit(first) == '0' {
		first++
	}
	if first == digitCount {
		return "0", true
	}
	if number.negative && !signed {
		return "", false
	}
	last := digitCount - 1
	for number.significandDigit(last) == '0' {
		last--
	}
	trailingZeros := digitCount - last - 1
	// No Go integer has more than 20 decimal digits. Saturating relative to
	// the input length avoids integer overflow and exponent-sized work.
	limit := digitCount + 21
	scale := number.exponent(limit) - len(number.fractionDigits) + trailingZeros
	if scale < 0 {
		return "", false
	}
	resultDigits := last - first + 1 + scale
	if resultDigits > 20 {
		return "", false
	}
	result := make([]byte, 0, resultDigits+1)
	if number.negative {
		result = append(result, '-')
	}
	for i := first; i <= last; i++ {
		result = append(result, number.significandDigit(i))
	}
	for range scale {
		result = append(result, '0')
	}
	return string(result), true
}
