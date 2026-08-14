// Package assets provides the embedded JaWS client assets and page-resource
// helpers.
//
// The embedded client forwards browser events and applies server-sent DOM
// commands; authoritative application state remains on the server. It inserts
// server-sent HTML without escaping, so applications must escape untrusted text
// before converting it to [html/template.HTML] or passing it as a plain string
// to an HTML-producing widget; see
// [github.com/linkdata/jaws/lib/bind.MakeHTMLGetter].
package assets
