package tag

import (
	"fmt"
	"path"
	"runtime"
	"strings"
)

// ErrIllegalTagType is returned when a UI tag type is disallowed.
var ErrIllegalTagType errIllegalTagType

type errIllegalTagType struct {
	typeName string
	caller   string
}

func (e errIllegalTagType) Error() string {
	s := "illegal tag type"
	if e.typeName != "" {
		s += " " + e.typeName
	}
	if e.caller != "" {
		s += "; nearest external caller: " + e.caller
	}
	return s
}

func (errIllegalTagType) Is(target error) bool {
	return target == ErrIllegalTagType
}

func newErrIllegalTagType(tag any) errIllegalTagType {
	return errIllegalTagType{
		typeName: fmt.Sprintf("%T", tag),
		caller:   nearestExternalCaller(),
	}
}

func nearestExternalCaller() (caller string) {
	var pcs [64]uintptr
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if caller = formatExternalCaller(frame); caller != "" {
			return
		}
		if !more {
			return
		}
	}
}

func formatExternalCaller(frame runtime.Frame) string {
	if frame.Function == "" || frame.File == "" || frame.Line <= 0 || ignoredCaller(frame.Function) {
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
