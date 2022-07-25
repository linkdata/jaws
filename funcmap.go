package jaws

import (
	"html/template"
)

var FuncMap = template.FuncMap{
	// cast a float64 to an int
	"int": CastInt,
	// cast an int to a float64
	"float64": CastFloat64,
}

func CastInt(f float64) int {
	return int(f)
}

func CastFloat64(i int) float64 {
	return float64(i)
}
