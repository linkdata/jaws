// Package jaws creates dynamic server-driven webpages over WebSockets.
//
// It integrates with [html/template] and any router that supports [http.Handler].
//
// This package holds the core engine and the [UI] interfaces. The standard
// widgets (Span, Button, Select, Text, and so on) and the RequestWriter helper
// methods live in [github.com/linkdata/jaws/lib/ui], and value binding lives in
// [github.com/linkdata/jaws/lib/bind].
//
// # Nil values
//
// Throughout this module, including its subpackages, nil is unsupported for pointer
// receivers and for values used as required operational collaborators, such as
// callbacks, handlers, providers, lockers, writers, file systems, contexts, and
// pointers to mutable values, unless the relevant API documents a meaning for nil.
// Passing an unsupported nil is a caller error and may panic immediately or when
// used later. Nil slices, maps, data values, and results otherwise follow ordinary
// Go semantics and the relevant API. An interface containing a typed nil is non-nil;
// its behavior follows the relevant interface, API, and concrete type.
//
// # Tags
//
// Tags associate [Element] values with application data or logical signals for
// targeted dirtying, broadcasts, and lookup. See
// [github.com/linkdata/jaws/lib/tag] for tag selection, expansion, registration,
// and lifetime.
package jaws
