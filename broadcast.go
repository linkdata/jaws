package jaws

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"slices"
	"strings"

	"github.com/linkdata/jaws/lib/key"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

// Broadcast sends a message to all [Request] values.
//
// It must not be called before the JaWS processing loop ([Jaws.Serve] or
// [Jaws.ServeWithTimeout]) is running. Otherwise this call may block once the
// internal broadcast channel fills.
//
// All convenience helpers on [Jaws] that call Broadcast inherit this requirement.
//
// A nil [wire.Message.Dest] targets every Request; a [key.Key] Dest targets the
// single Request with that identity key, and a zero key is dropped; a string Dest
// is an HTML id accepted by all Requests. Any other Dest is expanded into tags.
//
// A [wire.Message.Dest] that cannot be expanded into tags (an illegal tag type)
// is reported through [Jaws.MustLog], which panics when no [Jaws.Logger] is
// set; with a Logger the error is logged and the message is sent to the
// destinations that did expand.
func (jw *Jaws) Broadcast(msg wire.Message) {
	switch dest := msg.Dest.(type) {
	case nil: // send to all requests
	case key.Key: // send to the request with this identity key
		if dest == 0 {
			// A recycled producer captured a zeroed key; no live request can match
			// (keys are always non-zero), so drop it rather than fall through to
			// tag expansion.
			return
		}
	case string: // HTML id (accepted by all requests)
	default:
		expanded, err := tag.TagExpand(nil, msg.Dest)
		jw.MustLog(err)
		expanded = jw.dropNonComparableTags(expanded)
		switch len(expanded) {
		case 0:
			// no tags, so no requests will match
			return
		case 1:
			msg.Dest = expanded[0]
		default:
			msg.Dest = expanded
		}
	}
	select {
	case <-jw.Done():
	case jw.bcastCh <- msg:
	}
}

// dropNonComparableTags returns tags unchanged, or nil after reporting misuse via
// [Jaws.reportMisuse], if any tag is not comparable at runtime.
func (jw *Jaws) dropNonComparableTags(tags []any) []any {
	// Expanded tags become map keys in the processing loop (wantMessage's lookup in
	// Request.tagMap). TagExpand rejects the known runtime-non-comparable struct/array
	// case; this guard remains as a final defense before a bad Dest can panic the
	// Serve goroutine and crash the process.
	for _, tagValue := range tags {
		if cmperr := tag.NewErrNotComparable(tagValue); cmperr != nil {
			jw.reportMisuse(fmt.Errorf("jaws: Broadcast: %w", cmperr))
			return nil
		}
	}
	return tags
}

// setDirty marks all Elements that have one or more of the given tags as dirty.
func (jw *Jaws) setDirty(tags []any) {
	jw.mu.Lock()
	// Release the lock with defer so it is freed even if a map insert panics: a tag
	// that passed the static comparability check in ensureUsableTag can still be
	// non-comparable at runtime (a comparable struct holding e.g. a func in an
	// interface field) and panic when used as a map key here.
	defer jw.mu.Unlock()
	for _, tagValue := range tags {
		jw.dirtOrder++
		jw.dirty[tagValue] = jw.dirtOrder
	}
}

// Dirty marks all [Element] values that have one or more of the given tags as dirty.
//
// If any tag implements [tag.TagGetter] it is called with a nil [Request]; prefer
// [Request.Dirty], which avoids this. A tag that is not hashable panics the calling
// goroutine, but the panic is contained there and the [Jaws.Serve] loop is
// unaffected. [Request.Dirty] behaves the same here.
func (jw *Jaws) Dirty(dirtyTags ...any) {
	// A non-hashable tag panics here in the caller's goroutine rather than being
	// logged-and-dropped the way Broadcast handles a bad wire.Message.Dest: Broadcast
	// hashes the destination in the Serve goroutine, where a panic would crash the
	// process, whereas this hashing happens in setDirty under the caller's goroutine
	// with the lock released on panic via defer, so Serve is unaffected.
	//
	// Use TagExpand+MustLog rather than MustTagExpand: with a nil Context the latter
	// panics on an illegal tag even in production, unlike Request.Dirty and Broadcast.
	// Log and continue with the partial result.
	expanded, err := tag.TagExpand(nil, dirtyTags)
	jw.MustLog(err)
	jw.setDirty(expanded)
}

// dirtPair pairs a dirty tag with its insertion-order rank, used by sortedDirtTags
// to order tags without re-reading the order from the map on every comparison.
type dirtPair struct {
	tag any
	ord int
}

// sortedDirtTags returns the keys of dirty ordered by their insertion-order int value.
func sortedDirtTags(dirty map[any]int) []any {
	// Materialize {tag,ord} pairs and sort on the int, rather than sorting a []any
	// with a comparator that does two map lookups per comparison (~2*N*log2(N) hashed
	// any-map lookups); this runs under the exclusive jw.mu.
	pairs := make([]dirtPair, 0, len(dirty))
	for k, ord := range dirty {
		pairs = append(pairs, dirtPair{tag: k, ord: ord})
	}
	slices.SortFunc(pairs, func(a, b dirtPair) int { return cmp.Compare(a.ord, b.ord) })
	dirt := make([]any, len(pairs))
	for i := range pairs {
		dirt[i] = pairs[i].tag
	}
	return dirt
}

// distributeDirt drains the accumulated dirty tags and appends them to every live
// Request's pending-dirt list for the next update pass, returning the number of
// dirty tags distributed.
func (jw *Jaws) distributeDirt() int {
	var reqs []*Request
	var dirt []any

	// Snapshot the Request set under jw.mu, then append to each Request without it.
	jw.mu.Lock()
	if len(jw.dirty) > 0 {
		dirt = sortedDirtTags(jw.dirty)
		clear(jw.dirty)
		jw.dirtOrder = 0
		reqs = make([]*Request, 0, jw.requestCount)
		for _, rq := range jw.requests {
			if rq != nil {
				reqs = append(reqs, rq)
			}
		}
	}
	jw.mu.Unlock()

	// Appending outside jw.mu is deliberately safe:
	//   - The snapshot includes pending (not-yet-running) Requests; their todoDirt
	//     buffers until they connect or are recycled (bounded by the request timeout),
	//     so a value mutated in the render-to-connect window is reflected on the first
	//     update pass.
	//   - A Request can be recycled between snapshot and append. appendDirtyTags and
	//     clearLocked both take rq.mu, so there is no data race, and stale tags in a
	//     reborn Request resolve to nothing in Request.makeUpdateList against its
	//     freshly emptied tagMap (at worst a redundant re-render). Applying dirt is
	//     idempotent, so unlike destKey/cancelIfCurrent this path needs no
	//     key-identity guard.
	for _, rq := range reqs {
		rq.appendDirtyTags(dirt)
	}
	return len(dirt)
}

// Reload requests all [Request] values to reload their current page.
func (jw *Jaws) Reload() {
	jw.Broadcast(wire.Message{
		What: what.Reload,
	})
}

// isSafeRedirect reports whether rawurl is safe to hand to the browser's
// location.assign, and returns the normalized value to actually send.
//
// Leading and trailing ASCII whitespace and control characters are trimmed, as
// browsers strip them before navigating. Only same-document/relative paths and
// the http and https schemes are permitted; this blocks script-bearing schemes
// such as javascript: and data:, backslashes (which browsers treat as '/'), and
// protocol-relative URLs ("//host/path", "/\host") that would navigate to an
// arbitrary external origin.
func isSafeRedirect(rawurl string) (safe string, ok bool) {
	safe = strings.TrimFunc(rawurl, func(r rune) bool { return r <= ' ' })
	if strings.ContainsRune(safe, '\\') {
		return safe, false
	}
	if u, err := url.Parse(safe); err == nil {
		switch strings.ToLower(u.Scheme) {
		case "":
			if u.Host == "" && !strings.HasPrefix(safe, "//") {
				ok = true
			}
		case "http", "https":
			ok = true
		}
	}
	return
}

// redirectMessage validates url for the browser's location.assign and returns
// the wire.Message to broadcast (Data set to the normalized value); the caller
// sets msg.Dest. If url is unsafe it logs the refusal and returns ok=false. It
// is the single point where the redirect policy and rejection message live, so
// Jaws.Redirect and Request.Redirect cannot drift.
func (jw *Jaws) redirectMessage(url string) (msg wire.Message, ok bool) {
	var safe string
	if safe, ok = isSafeRedirect(url); ok {
		msg = wire.Message{What: what.Redirect, Data: safe}
	} else {
		_ = jw.Log(fmt.Errorf("jaws: refusing unsafe redirect to %q", url))
	}
	return
}

// Redirect requests all [Request] values to navigate to the given URL.
//
// The URL is validated to be a relative path or an http/https URL; script-bearing
// schemes such as javascript: and protocol-relative ("//host") URLs are refused
// and logged rather than sent to the browser.
func (jw *Jaws) Redirect(url string) {
	if msg, ok := jw.redirectMessage(url); ok {
		jw.Broadcast(msg)
	}
}

// Alert sends an alert to all [Request] values.
//
// The level argument should be one of Bootstrap's alert levels:
// primary, secondary, success, danger, warning, info, light or dark.
//
// The level and msg are HTML-escaped before being sent, so it is safe to pass
// untrusted text; do not pre-escape it.
func (jw *Jaws) Alert(level, msg string) {
	jw.Broadcast(wire.Message{
		What: what.Alert,
		Data: alertData(level, msg),
	})
}

// broadcastTo broadcasts a single wire command to all HTML elements matching
// target. It is the shared body of the public broadcast helpers below, which
// differ only in the What command and how they assemble data.
func (jw *Jaws) broadcastTo(target any, w what.What, data string) {
	jw.Broadcast(wire.Message{
		Dest: target,
		What: w,
		Data: data,
	})
}

// SetInner sends a request to replace the inner HTML of
// all HTML elements matching target.
func (jw *Jaws) SetInner(target any, innerHTML template.HTML) {
	jw.broadcastTo(target, what.Inner, string(innerHTML))
}

// SetAttr sends a request to replace the given attribute value in
// all HTML elements matching target.
//
// The value parameter must be the unescaped logical attribute value. It is sent
// to the browser DOM and used as the value argument to setAttribute().
func (jw *Jaws) SetAttr(target any, attr, value string) {
	jw.broadcastTo(target, what.SAttr, attr+"\n"+value)
}

// RemoveAttr sends a request to remove the given attribute from
// all HTML elements matching target.
func (jw *Jaws) RemoveAttr(target any, attr string) {
	jw.broadcastTo(target, what.RAttr, attr)
}

// SetClass sends a request to set the given class in
// all HTML elements matching target.
func (jw *Jaws) SetClass(target any, cls string) {
	jw.broadcastTo(target, what.SClass, cls)
}

// RemoveClass sends a request to remove the given class from
// all HTML elements matching target.
func (jw *Jaws) RemoveClass(target any, cls string) {
	jw.broadcastTo(target, what.RClass, cls)
}

// SetValue sends a request to set the current input value (in textual form) of
// all HTML elements matching target. It sets the live DOM value/state, not the
// HTML "value" attribute.
func (jw *Jaws) SetValue(target any, value string) {
	jw.broadcastTo(target, what.Value, value)
}

// Insert calls the JavaScript 'insertBefore()' method on
// all HTML elements matching target.
//
// The position parameter 'where' may be either an HTML ID, a child index or the text "null".
// html is trusted HTML, matching [Jaws.SetInner] and [Jaws.Append].
func (jw *Jaws) Insert(target any, where string, html template.HTML) {
	jw.broadcastTo(target, what.Insert, where+"\n"+string(html))
}

// Replace replaces HTML on all HTML elements matching target.
//
// html is trusted HTML, matching [Jaws.SetInner] and [Jaws.Append].
func (jw *Jaws) Replace(target any, html template.HTML) {
	jw.broadcastTo(target, what.Replace, string(html))
}

// Delete removes the HTML element(s) matching target.
func (jw *Jaws) Delete(target any) {
	jw.broadcastTo(target, what.Delete, "")
}

// Append calls the JavaScript appendChild method on all HTML elements matching target.
func (jw *Jaws) Append(target any, html template.HTML) {
	jw.broadcastTo(target, what.Append, string(html))
}

// maybeCompactJSON returns its input made safe to embed verbatim in a [what.Call]
// wire frame, which the client splits on '\n' (frames) and '\t' (order fields).
func maybeCompactJSON(in string) string {
	if strings.ContainsAny(in, "\n\t\r") {
		// For valid JSON, json.Compact strips all insignificant whitespace, including
		// any tabs, newlines and carriage returns between tokens. When the input is
		// not valid JSON (for example a raw control byte inside a string literal),
		// Compact fails; escape those control bytes rather than pass the raw bytes
		// through and corrupt the frame, so the payload is always frame-safe and
		// valid JSON.
		var b bytes.Buffer
		if err := json.Compact(&b, []byte(in)); err == nil {
			return b.String()
		}
		return jsonControlEscaper.Replace(in)
	}
	return in
}

// jsonControlEscaper turns control bytes into their JSON escape sequences for the
// fallback path. \n (frame terminator) and \t (field separator) are
// framing-significant; \r is escaped because a bare \r is illegal inside a JSON
// string literal, so the fallback output stays valid JSON.
var jsonControlEscaper = strings.NewReplacer("\t", `\t`, "\n", `\n`, "\r", `\r`)

// jsCallPathByteRemover strips bytes that would make the path=json Call payload
// ambiguous or invalid before it reaches jaws.js.
var jsCallPathByteRemover = strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "", "=", "")

func jsCallData(jsfunc, jsonstr string) string {
	return jsCallPathByteRemover.Replace(jsfunc) + "=" + maybeCompactJSON(jsonstr)
}

// JsCall calls a browser JavaScript function path for matching targets.
//
// target selects which requests or elements receive the Call message. In each
// receiving browser, jsfunc is resolved as a path from window and called with
// JSON.parse(jsonstr); the matched element is not passed as this or as an
// argument.
func (jw *Jaws) JsCall(target any, jsfunc, jsonstr string) {
	jw.broadcastTo(target, what.Call, jsCallData(jsfunc, jsonstr))
}
