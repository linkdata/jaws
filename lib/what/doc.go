// Package what defines the commands and events used by the JaWS wire protocol.
//
// [What] values include commands that do not require an Element,
// Element-associated commands, browser-originated events, and the test-only
// [Hook]. [What.IsCommand] identifies the first group, [Parse] maps exact wire
// names, and [Invalid] is the zero value. See
// [github.com/linkdata/jaws/lib/wire.WsMsg] for record framing.
package what
