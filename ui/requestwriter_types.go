package ui

import pkg "github.com/linkdata/jaws/jaws"

// RequestWriter is the template/request rendering context used by JaWS templates.
//
// It aliases the core type so methods and behavior remain identical.
type RequestWriter = pkg.RequestWriter

// With is the template execution context passed to Go html/template execution.
//
// It aliases the core type so fields and behavior remain identical.
type With = pkg.With
