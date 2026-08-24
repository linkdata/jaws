package tag

import (
	"fmt"
	"path"
	"runtime"
	"strings"
)

// ErrIllegalTagType is returned when a UI tag type is disallowed.
//
// Its origin hints are best-effort, and its text may vary with the call site
// and build. Match this error with [errors.Is].
var ErrIllegalTagType errIllegalTagType

type errIllegalTagType struct {
	typeName      string
	tagGetterType string
	caller        string
}

func (e errIllegalTagType) Error() string {
	s := "illegal tag type"
	if e.typeName != "" {
		s += " " + e.typeName
	}
	if e.tagGetterType != "" {
		s += " returned by JawsGetTag on " + e.tagGetterType
	}
	if e.caller != "" {
		s += "; nearest external caller: " + e.caller
	}
	return s
}

func (errIllegalTagType) Is(target error) bool {
	return target == ErrIllegalTagType
}

func newErrIllegalTagType(tag any, active []any) (err errIllegalTagType) {
	err.typeName = fmt.Sprintf("%T", tag)
	for i := len(active) - 1; i >= 0; i-- {
		if _, ok := active[i].(TagGetter); ok {
			err.tagGetterType = fmt.Sprintf("%T", active[i])
			break
		}
	}
	err.caller = nearestExternalCaller()
	return
}

func nearestExternalCaller() string {
	return scanCallers(ignoredCaller)
}

func scanCallers(ignored func(string) bool) (caller string) {
	var pcs [64]uintptr
	// Skip runtime.Callers and scanCallers; the predicate handles later frames.
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if caller = formatExternalCaller(frame, ignored); caller != "" {
			return
		}
		if !more {
			return
		}
	}
}

func formatExternalCaller(frame runtime.Frame, ignored func(string) bool) string {
	if frame.Function == "" || frame.File == "" || frame.Line <= 0 || ignored(frame.Function) {
		return ""
	}
	return fmt.Sprintf("%s (%s:%d)", frame.Function, path.Base(frame.File), frame.Line)
}

func ignoredCaller(function string) bool {
	for _, pkg := range [...]string{
		"github.com/linkdata/jaws",
		"html/template",
		"net/http",
		"reflect",
		"runtime",
		"testing",
		"text/template",
	} {
		if strings.HasPrefix(function, pkg+".") || strings.HasPrefix(function, pkg+"/") {
			return true
		}
	}
	return false
}
