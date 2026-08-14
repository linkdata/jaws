// Package jaws creates dynamic server-driven webpages over WebSockets.
//
// It provides the core engine, requests, sessions, and [UI] interfaces and
// integrates with [html/template] and routers that support [http.Handler].
// Standard widgets live in [github.com/linkdata/jaws/lib/ui], value binding in
// [github.com/linkdata/jaws/lib/bind], and dirty-target selection in
// [github.com/linkdata/jaws/lib/tag].
//
// Applications keep authoritative state on the server. Tags associate [Element]
// values with application data or logical signals for targeted dirtying,
// broadcasts, and lookup; see [github.com/linkdata/jaws/lib/tag].
//
// # Nil values
//
// Throughout this module, nil is unsupported for pointer receivers and values
// used as required operational collaborators, such as callbacks, handlers,
// providers, lockers, writers, file systems, contexts, and pointers to mutable
// values, unless an API documents a meaning for nil. Unsupported nil use is
// caller error and may panic. Nil slices, maps, data values, and results otherwise
// follow ordinary Go semantics and the relevant API. An interface containing a
// typed nil is non-nil; its behavior follows the receiving API and concrete type.
package jaws
