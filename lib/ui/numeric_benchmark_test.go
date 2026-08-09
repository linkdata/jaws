package ui

import (
	"testing"

	"github.com/linkdata/jaws"
)

type numericBenchmarkSource[T comparable] struct {
	value T
}

func (source *numericBenchmarkSource[T]) JawsGet(*jaws.Element) T {
	return source.value
}

func (source *numericBenchmarkSource[T]) JawsSet(_ *jaws.Element, value T) error {
	source.value = value
	return nil
}

type (
	numericBenchmarkInt     int16
	numericBenchmarkUint    uint64
	numericBenchmarkFloat32 float32
)

var (
	numericBenchmarkBindingSink numericBinding
	numericBenchmarkTextSink    string
	numericBenchmarkBoolSink    bool
	numericBenchmarkErrorSink   error
)

func benchmarkNumericBindingGetText[T Numeric](b *testing.B, name string, value T) {
	b.Helper()
	b.Run(name, func(b *testing.B) {
		source := &numericBenchmarkSource[T]{value: value}
		binding := newNumericBinding[T](source)
		var text string
		var err error
		b.ReportAllocs()
		for b.Loop() {
			text, err = binding.getText(nil)
		}
		numericBenchmarkTextSink = text
		numericBenchmarkErrorSink = err
	})
}

func BenchmarkNumericBindingGetText(b *testing.B) {
	benchmarkNumericBindingGetText(b, "int16", numericBenchmarkInt(-12345))
	benchmarkNumericBindingGetText(b, "uint64", numericBenchmarkUint(12345678901234567890))
	benchmarkNumericBindingGetText(b, "float32", numericBenchmarkFloat32(0.1))
}

func benchmarkNumericBindingReflectedGetText[T Numeric](b *testing.B, name string, value T) {
	b.Helper()
	b.Run(name, func(b *testing.B) {
		source := &numericBenchmarkSource[T]{value: value}
		binding, ok := makeNumericBinding(source)
		if !ok {
			b.Fatalf("makeNumericBinding(%T) rejected numeric source", source)
		}
		var text string
		var err error
		b.ReportAllocs()
		for b.Loop() {
			text, err = binding.getText(nil)
		}
		numericBenchmarkTextSink = text
		numericBenchmarkErrorSink = err
	})
}

func BenchmarkNumericBindingReflectedGetText(b *testing.B) {
	benchmarkNumericBindingReflectedGetText(b, "int16", numericBenchmarkInt(-12345))
	benchmarkNumericBindingReflectedGetText(b, "uint64", numericBenchmarkUint(12345678901234567890))
	benchmarkNumericBindingReflectedGetText(b, "float32", numericBenchmarkFloat32(0.1))
}

func benchmarkNumericBindingAcceptText[T Numeric](b *testing.B, name, text string, value T) {
	b.Helper()
	b.Run(name, func(b *testing.B) {
		source := &numericBenchmarkSource[T]{value: value}
		binding := newNumericBinding[T](source)
		input := &Input{}
		var accepted bool
		var err error
		b.ReportAllocs()
		for b.Loop() {
			accepted, err = binding.acceptText(input, nil, text)
		}
		numericBenchmarkBoolSink = accepted
		numericBenchmarkErrorSink = err
	})
}

func BenchmarkNumericBindingAcceptText(b *testing.B) {
	benchmarkNumericBindingAcceptText(b, "int16", "-12345", numericBenchmarkInt(0))
	benchmarkNumericBindingAcceptText(b, "uint64", "12345678901234567890", numericBenchmarkUint(0))
	benchmarkNumericBindingAcceptText(b, "float32", "0.1", numericBenchmarkFloat32(0))
}

func benchmarkNumericBindingReflectedAcceptText[T Numeric](b *testing.B, name, text string, value T) {
	b.Helper()
	b.Run(name, func(b *testing.B) {
		source := &numericBenchmarkSource[T]{value: value}
		binding, ok := makeNumericBinding(source)
		if !ok {
			b.Fatalf("makeNumericBinding(%T) rejected numeric source", source)
		}
		input := &Input{}
		var accepted bool
		var err error
		b.ReportAllocs()
		for b.Loop() {
			accepted, err = binding.acceptText(input, nil, text)
		}
		numericBenchmarkBoolSink = accepted
		numericBenchmarkErrorSink = err
	})
}

func BenchmarkNumericBindingReflectedAcceptText(b *testing.B) {
	benchmarkNumericBindingReflectedAcceptText(b, "int16", "-12345", numericBenchmarkInt(0))
	benchmarkNumericBindingReflectedAcceptText(b, "uint64", "12345678901234567890", numericBenchmarkUint(0))
	benchmarkNumericBindingReflectedAcceptText(b, "float32", "0.1", numericBenchmarkFloat32(0))
}

func benchmarkNumericBindingConstruction[T Numeric](b *testing.B, name string, value T) {
	b.Helper()
	b.Run(name, func(b *testing.B) {
		source := &numericBenchmarkSource[T]{value: value}
		b.ReportAllocs()
		for b.Loop() {
			numericBenchmarkBindingSink = newNumericBinding[T](source)
		}
	})
}

func BenchmarkNumericBindingConstruction(b *testing.B) {
	benchmarkNumericBindingConstruction(b, "int16", numericBenchmarkInt(-12345))
	benchmarkNumericBindingConstruction(b, "uint64", numericBenchmarkUint(12345678901234567890))
	benchmarkNumericBindingConstruction(b, "float32", numericBenchmarkFloat32(0.1))
}

func benchmarkNumericBindingReflectedConstruction[T Numeric](b *testing.B, name string, value T) {
	b.Helper()
	b.Run(name, func(b *testing.B) {
		source := &numericBenchmarkSource[T]{value: value}
		var ok bool
		b.ReportAllocs()
		for b.Loop() {
			numericBenchmarkBindingSink, ok = makeNumericBinding(source)
		}
		numericBenchmarkBoolSink = ok
	})
}

func BenchmarkNumericBindingReflectedConstruction(b *testing.B) {
	benchmarkNumericBindingReflectedConstruction(b, "int16", numericBenchmarkInt(-12345))
	benchmarkNumericBindingReflectedConstruction(b, "uint64", numericBenchmarkUint(12345678901234567890))
	benchmarkNumericBindingReflectedConstruction(b, "float32", numericBenchmarkFloat32(0.1))
}
