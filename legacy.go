package jaws

import "time"

type ClickFn = func(*Request, string) error
type InputTextFn = func(*Request, string, string) error
type InputBoolFn = func(*Request, string, bool) error
type InputFloatFn = func(*Request, string, float64) error
type InputDateFn = func(*Request, string, time.Time) error
