package jaws

import (
	"html/template"
	"time"
)

// Deprecated: Will be removed in future
type Ui interface {
	JawsUi(rq *Request, attrs ...string) template.HTML
}

// Deprecated: Will be removed in future
type ClickFn = func(*Request, string) error

// Deprecated: Will be removed in future
type InputTextFn = func(*Request, string, string) error

// Deprecated: Will be removed in future
type InputBoolFn = func(*Request, string, bool) error

// Deprecated: Will be removed in future
type InputFloatFn = func(*Request, string, float64) error

// Deprecated: Will be removed in future
type InputDateFn = func(*Request, string, time.Time) error
