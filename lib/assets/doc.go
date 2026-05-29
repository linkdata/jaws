// Package assets contains the embedded client assets and helpers used by JaWS
// setup code.
//
// The embedded JavaScript applies server-sent DOM updates and intentionally
// trusts the server to distinguish trusted HTML from escaped user text.
// Applications should route untrusted text through escaping helpers before it
// reaches template.HTML or raw string HTML paths.
package assets
