package jaws

import "errors"

// ErrValueUnchanged reports a successful no-op set: there was no error, but the
// underlying value already equaled the desired value.
//
// Setter-style implementations (the JawsSet / JawsSetPath methods in
// github.com/linkdata/jaws/lib/ui and github.com/linkdata/jaws/jawstree) return it,
// and callers test for it with [errors.Is]. It lives in this package so all
// implementations share one error identity.
var ErrValueUnchanged = errors.New("value unchanged")
