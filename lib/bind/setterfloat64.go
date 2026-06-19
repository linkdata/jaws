package bind

import (
	"errors"
	"fmt"
	"math"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
)

var (
	// ErrFloatNotFinite reports that a float value is NaN or infinite.
	//
	// Such values, typically from the untrusted browser, corrupt the bound value;
	// NaN in particular defeats the equality-based update dedup (NaN != NaN). They
	// are rejected at the binding boundary.
	ErrFloatNotFinite = errors.New("float value is not finite")
	// ErrFloatOutOfRange reports that a finite float does not fit the target type.
	//
	// The float-to-int conversion of an out-of-range value is implementation-defined
	// and silently wraps, so it is rejected rather than performed.
	ErrFloatOutOfRange = errors.New("float value out of range for target type")
)

type numeric interface {
	~float32 | ~float64 |
		~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

type setterFloat64[T numeric] struct {
	Setter[T]
}

// sanitizeFloatForT validates value before it is converted to T. It rejects
// non-finite values for every numeric T, and for integer T also rejects values
// outside the type's representable range. Bounds use an exclusive upper limit
// expressed as a power of two to avoid the float64 rounding pitfall at the top of
// the 64-bit ranges (float64(MaxInt64) rounds up to 2^63).
//
// The type switch matches predeclared types by exact type, so callers must
// instantiate T only with the predeclared numeric types. A named (defined) type
// such as "type Celsius float64" falls through to the float default branch and is
// range-checked as if it were its predeclared underlying type.
func sanitizeFloatForT[T numeric](value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return ErrFloatNotFinite
	}
	var lo, hiExcl float64
	switch any(T(0)).(type) {
	case int8:
		lo, hiExcl = math.MinInt8, math.MaxInt8+1
	case int16:
		lo, hiExcl = math.MinInt16, math.MaxInt16+1
	case int32:
		lo, hiExcl = math.MinInt32, math.MaxInt32+1
	case int, int64: // int is 64-bit on all supported platforms
		lo, hiExcl = math.MinInt64, -float64(math.MinInt64) // [-2^63, 2^63)
	case uint8:
		lo, hiExcl = 0, math.MaxUint8+1
	case uint16:
		lo, hiExcl = 0, math.MaxUint16+1
	case uint32:
		lo, hiExcl = 0, math.MaxUint32+1
	case uint, uint64: // uint is 64-bit on all supported platforms
		lo, hiExcl = 0, -2*float64(math.MinInt64) // [0, 2^64)
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
	if value < lo || value >= hiExcl {
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
		err = s.Setter.JawsSet(elem, T(value))
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
	panic(fmt.Errorf("expected bind.Setter[float64], bind.Getter[float64] or float64 not %T", value))
}
