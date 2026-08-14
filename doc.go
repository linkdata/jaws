// Package jaws creates dynamic server-driven webpages over WebSockets.
//
// It provides the core engine, requests, sessions, and [UI] interfaces and
// integrates with [html/template] and routers that support [http.Handler].
// Standard widgets live in [github.com/linkdata/jaws/lib/ui], value binding in
// [github.com/linkdata/jaws/lib/bind], and dirty-target selection in
// [github.com/linkdata/jaws/lib/tag].
package jaws
