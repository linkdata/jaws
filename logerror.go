package jaws

import (
	"fmt"
	"io"
	"strings"
)

type printlner interface {
	Println(v ...any)
}

type errorer interface {
	Error(msg string, args ...any)
}

// LogError is an adapter allowing logging an informational message
// to a log.Logger, slog.Logger or io.Writer.
func LogError(logger any, format string, args ...any) {
	msg := fmt.Sprintf(strings.TrimRight(format, "\n"), args...)
	switch x := logger.(type) {
	case printlner:
		x.Println(msg)
	case errorer:
		x.Error(msg)
	case io.Writer:
		_, _ = fmt.Fprintln(x, msg)
	}
}
