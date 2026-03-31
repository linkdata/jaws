package jaws

import "github.com/linkdata/jaws/core/internal/testutil"

func newTestSetter[T comparable](val T) *testutil.Setter[T, Element] {
	return testutil.NewSetter[T, Element](val, ErrValueUnchanged)
}
