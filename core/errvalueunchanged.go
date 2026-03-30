package jaws

import "errors"

// ErrValueUnchanged can be returned from JawsSet[Type] functions
// to indicate that while there was no error, the underlying value
// was already the desired value.
var ErrValueUnchanged = errors.New("value unchanged")
