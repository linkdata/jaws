package bind

import (
	"errors"
	"fmt"
	"math"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
)

// ErrFloatOutOfRange reports that a finite float does not fit the target type.
var ErrFloatOutOfRange = errors.New("float value out of range for target type")

type numeric interface {
	~float32 | ~float64 |
		~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

type setterFloat64[T numeric] struct {
	Setter[T]
}

// sanitizeFloatForT validates value before it is converted to T. For integer T it
// rejects values whose truncation toward zero falls outside the type's
// representable range; for float32 it rejects a float64 that overflows to infinity
// on conversion.
//
// value is expected to be finite. The number and range input widgets terminate the
// Request on a non-finite value before it reaches the setter, so finiteness is not
// re-validated here: an infinity is incidentally rejected as out of range, but a NaN
// passed directly would slip through and convert to an implementation-defined value.
//
// The conversion T(value) truncates toward zero, so the lower bound is compared
// against math.Trunc(value): a fractional value like -128.5 truncates to the valid
// int8 -128 and is accepted, mirroring the high end where 127.5 truncates to 127.
//
// The exclusive upper bound MaxIntN+1 is an exact power of two: the +1 is
// untyped-constant (arbitrary-precision) arithmetic evaluated before the single
// float64 conversion, so it avoids the float64(MaxInt64) rounding pitfall (that
// conversion rounds up to 2^63). MinIntN is exactly representable for every width,
// so comparing the truncated value against it is safe. The int and uint bounds use
// math.MinInt, math.MaxInt and math.MaxUint, which track the target word size, so a
// 32-bit build rejects magnitudes that only fit a 64-bit word instead of letting
// T(value) wrap.
//
// The type switch matches predeclared types by exact type, so callers must
// instantiate T only with the predeclared numeric types. A named (defined) type
// such as "type Celsius float64" falls through to the float default branch and is
// range-checked as if it were its predeclared underlying type.
func sanitizeFloatForT[T numeric](value float64) error {
	var lo, hiExcl float64
	switch any(T(0)).(type) {
	case int8:
		lo, hiExcl = math.MinInt8, math.MaxInt8+1
	case int16:
		lo, hiExcl = math.MinInt16, math.MaxInt16+1
	case int32:
		lo, hiExcl = math.MinInt32, math.MaxInt32+1
	case int: // math.MinInt/math.MaxInt track the target word size
		lo, hiExcl = math.MinInt, math.MaxInt+1
	case int64:
		lo, hiExcl = math.MinInt64, math.MaxInt64+1
	case uint8:
		lo, hiExcl = 0, math.MaxUint8+1
	case uint16:
		lo, hiExcl = 0, math.MaxUint16+1
	case uint32:
		lo, hiExcl = 0, math.MaxUint32+1
	case uint: // math.MaxUint tracks the target word size
		lo, hiExcl = 0, math.MaxUint+1
	case uint64:
		lo, hiExcl = 0, math.MaxUint64+1
	default:
		// float32 or float64: reject a finite value that overflows the target type.
		// A float64 that exceeds the float32 range converts to ±Inf and silently
		// corrupts the bound value (reachable from browser input via NewNumber and
		// NewRange); float64 never overflows here, so this is a no-op for it.
		if math.IsInf(float64(T(value)), 0) {
			return ErrFloatOutOfRange
		}
		return nil
	}
	// The float-to-int conversion of an out-of-range value is implementation-defined and
	// silently wraps, so reject it rather than perform it.
	if math.Trunc(value) < lo || value >= hiExcl {
		return ErrFloatOutOfRange
	}
	return nil
}

func (s setterFloat64[T]) JawsGet(elem *jaws.Element) float64 {
	v := s.Setter.JawsGet(elem)
	return float64(v)
}

func (s setterFloat64[T]) JawsSet(elem *jaws.Element, value float64) (err error) {
	if err = sanitizeFloatForT[T](value); err == nil {
		converted := T(value)
		err = s.Setter.JawsSet(elem, converted)
		// ErrValueUnchanged is accurate for the underlying T setter, but not for
		// this float64 view when conversion changed the requested value. Report a
		// successful change so input widgets dirty themselves and reconcile the
		// browser with the canonical value returned by JawsGet.
		if err != nil && errors.Is(err, jaws.ErrValueUnchanged) && float64(converted) != value {
			err = nil
		}
	}
	return
}

func (s setterFloat64[T]) JawsGetTag(tag.Context) any {
	return s.Setter
}

type setterFloat64ReadOnly[T numeric] struct {
	Getter[T]
}

func (s setterFloat64ReadOnly[T]) JawsGet(elem *jaws.Element) float64 {
	v := s.Getter.JawsGet(elem)
	return float64(v)
}

func (setterFloat64ReadOnly[T]) JawsSet(elem *jaws.Element, value float64) error {
	return ErrValueNotSettable
}

func (s setterFloat64ReadOnly[T]) JawsGetTag(tag.Context) any {
	return s.Getter
}

type setterFloat64Static[T numeric] struct {
	v float64
}

func (setterFloat64Static[T]) JawsSet(elem *jaws.Element, value float64) error {
	return ErrValueNotSettable
}

func (s setterFloat64Static[T]) JawsGet(elem *jaws.Element) float64 {
	return s.v
}

func (s setterFloat64Static[T]) JawsGetTag(tag.Context) any {
	return nil
}

func makeSetterFloat64for[T numeric](s *Setter[float64], value any) bool {
	switch v := value.(type) {
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

// MakeSetterFloat64 returns v as a [Setter] for float64 values.
//
// v may be a float64 setter/getter/static value, or a setter/getter/static value
// of another supported numeric type (the predeclared signed and unsigned integer
// types and float32), which is bridged to float64 by ordinary Go conversion. That
// bridge can lose precision: int64/uint64 magnitudes beyond 2^53 are not exactly
// representable as float64.
//
// If conversion changes value but the converted value already matches the
// underlying setter's current value, the adapter reports a successful set
// instead of [jaws.ErrValueUnchanged]. This lets consumers reconcile their
// float64 view with the canonical converted value returned by [Getter.JawsGet].
//
// Only the predeclared numeric types are matched, by their exact type. Named
// (defined) numeric types such as "type Celsius float64" are NOT matched:
// neither the value itself nor a Setter[Celsius]/Getter[Celsius] over it is
// accepted (Setter[Celsius] is not a Setter[float64]), and passing one causes a
// panic. To bind such a value, expose it as a Setter[float64] / Getter[float64]
// (or a plain float64) — for example with a small adapter that converts to and
// from float64. Getter and static adapters are read-only and return
// [ErrValueNotSettable] from [Setter.JawsSet]. MakeSetterFloat64 panics for
// unsupported types.
func MakeSetterFloat64(value any) (s Setter[float64]) {
	switch v := value.(type) {
	case Setter[float64]:
		return v
	case Getter[float64]:
		return setterReadOnly[float64]{v}
	case float64:
		return setterStatic[float64]{v}
	default:
		ok := makeSetterFloat64for[int64](&s, v)
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
	panic(fmt.Errorf("expected bind.Setter[float64], bind.Getter[float64] or float64 not %T", value))
}
