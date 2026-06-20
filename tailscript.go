package jaws

// This file implements the tail-script subsystem: the one-shot
// /jaws/.tail/<key> response that applies the HTML attribute and class updates
// queued during initial rendering, so the page reaches its correct state before
// the WebSocket connects without templates having to pre-render those values.
//
// [Request.TailHTML] emits the page-side <script src="/jaws/.tail/<key>"> tag,
// [Jaws.serveTailScript] handles the fetch, [Request.drainTailScript] builds the
// script body from the queued messages and [Request.writeTailResponse] writes it.
// appendJSQuote and jsInlineScriptEscaper keep interpolated values safe inside the
// inline <script>.

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/linkdata/jaws/lib/key"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

var headerContentTypeJavaScript = []string{"text/javascript"}

// appendJSQuote appends s as a JavaScript string literal safe to embed in an inline
// <script>.
//
// It JSON-quotes s with [wire.AppendJSONQuote] (whose output is valid JavaScript)
// and then escapes the characters JSON leaves literal that are hazardous inside a
// <script> element: '<' as '\x3c' (so '</script>' cannot close the block) and the
// U+2028/U+2029 line separators (illegal in a pre-ES2019 string literal). It is used
// instead of [strconv.AppendQuote], whose Go-only \UXXXXXXXX escapes for
// non-printable astral runes JavaScript silently mis-decodes (dropping the
// backslash and keeping the letters), corrupting the value.
func appendJSQuote(b []byte, s string) []byte {
	start := len(b)
	b = wire.AppendJSONQuote(b, s)
	// None of '<', U+2028 or U+2029 can appear inside an escape AppendJSONQuote
	// produces, so any occurrence in the appended region came from s. Most
	// attribute/class fragments contain none, so the common path returns with no copy.
	if !bytes.ContainsAny(b[start:], "<\u2028\u2029") {
		return b
	}
	rest := jsInlineScriptEscaper.Replace(string(b[start:]))
	return append(b[:start], rest...)
}

// jsInlineScriptEscaper escapes, in a JSON string that is already a valid JavaScript
// string literal, the characters that remain unsafe inside an inline <script>: '<'
// (so '</script>' cannot terminate the block) and the U+2028/U+2029 line separators
// (line terminators that break a pre-ES2019 string literal). The replacements are
// themselves valid JavaScript escapes.
var jsInlineScriptEscaper = strings.NewReplacer(
	"<", `\x3c`,
	"\u2028", `\u2028`,
	"\u2029", `\u2029`,
)

// drainTailScript builds the tail <script> body from the attribute and class messages
// queued during initial rendering, reporting sent=true the first time it runs for this
// Request (subsequent calls return sent=false so the response is 204).
func (rq *Request) drainTailScript() (b []byte, sent bool) {
	// Takes only muQueue and never touches the network. Jaws.ServeHTTP calls it while
	// holding jw.mu (read), which blocks recycling (which needs the jw.mu write lock),
	// so this Request cannot be recycled and reused under a different key mid-drain: the
	// bytes returned always belong to the identity the handler looked up. The slow
	// network write happens afterwards in writeTailResponse with no lock held, so a
	// stalled client cannot block recycling or the Serve loop. The data race on
	// wsQueue/tailsent is prevented because clearLocked also takes muQueue to reset them.
	rq.muQueue.Lock()
	defer rq.muQueue.Unlock()
	if !rq.tailsent {
		rq.tailsent = true
		sent = true
		n := 0
		for _, msg := range rq.wsQueue {
			var fn string
			switch msg.What {
			case what.SAttr:
				fn = "setAttribute"
			case what.RAttr:
				fn = "removeAttribute"
			case what.SClass:
				fn = "classList?.add"
			case what.RClass:
				fn = "classList?.remove"
			}
			if fn != "" {
				// Wrap each fixup so one that throws at runtime (an invalid class token
				// or attribute name reaches the throwing DOM call past the ?. element
				// guard) does not abandon the fixups that follow. The drain removes these
				// messages from wsQueue, making the tail script their sole applier, so an
				// unisolated throw would lose the rest permanently; this mirrors the
				// per-order isolation the WebSocket client applies in jawsMessage.
				b = append(b, "try{document.getElementById("...)
				b = msg.Jid.AppendQuote(b)
				b = append(b, ")?."...)
				b = append(b, fn...)
				b = append(b, "("...)
				attr, val, ok := strings.Cut(msg.Data, "\n")
				b = appendJSQuote(b, attr)
				if ok {
					b = append(b, ',')
					b = appendJSQuote(b, val)
				}
				b = append(b, ");}catch(e){console.error(e);}\n"...)
			} else {
				rq.wsQueue[n] = msg
				n++
			}
		}
		for i := n; i < len(rq.wsQueue); i++ {
			rq.wsQueue[i] = wire.WsMsg{}
		}
		rq.wsQueue = rq.wsQueue[:n]
	}
	return
}

// writeTailResponse writes the tail script response built by drainTailScript. It
// holds no locks, so the network write cannot stall recycling or the Serve loop.
//
// A sent=false drain (the tail was already fetched on an earlier request) responds
// 204 No Content. A first drain finding nothing queued reports sent=true with empty
// bytes and writes an empty 200 body.
func (*Request) writeTailResponse(w http.ResponseWriter, b []byte, sent bool) (err error) {
	hdr := w.Header()
	hdr["Cache-Control"] = headerCacheControlNoStore
	hdr["Content-Type"] = headerContentTypeJavaScript
	if !sent {
		w.WriteHeader(http.StatusNoContent)
	} else if len(b) > 0 {
		// b is built by drainTailScript, which JS-escapes every attribute and class
		// value via appendJSQuote (see TestRequest_writeTailScript_EscapesScriptClose),
		// so writing it verbatim to the response is safe.
		_, err = w.Write(b) // #nosec G705 -- tail bytes are JS-escaped by drainTailScript via appendJSQuote
	}
	return
}

// TailHTML writes optional HTML code at the end of the page's BODY section that
// will immediately apply HTML attribute and class updates made during initial
// rendering, which minimizes flicker without having to write the correct
// value in templates or during [Renderer.JawsRender].
//
// It also adds a <noscript> tag that warns of reduced functionality.
func (rq *Request) TailHTML(w io.Writer) (err error) {
	ks := rq.JawsKeyString()
	_, err = fmt.Fprintf(w, "\n"+`<noscript>`+
		`<div class="jaws-alert">This site requires Javascript for full functionality.</div>`+
		`<img src="/jaws/%s/noscript" alt="noscript"></noscript>`+"\n"+
		`<script src="/jaws/.tail/%s"></script>`+"\n", ks, ks)
	return
}

// serveTailScript handles a GET /jaws/.tail/<key> fetch, draining the one-shot
// attribute/class updates queued for the matching Request and writing them. It
// reports whether it produced a response; a false return means the path was not a
// handled tail fetch and [Jaws.ServeHTTP] should keep dispatching.
func (jw *Jaws) serveTailScript(w http.ResponseWriter, r *http.Request) (handled bool) {
	if jawsKeyString, ok := strings.CutPrefix(r.URL.Path, "/jaws/.tail/"); ok {
		if jawsKey, tail := key.Parse(jawsKeyString); tail == "" {
			remoteIP := jw.clientIP(r)
			// Hold jw.mu (read) across both the lookup and the drain: recycling needs
			// the jw.mu write lock, so rq cannot be recycled and reused under a different
			// key while we drain its queue. A stale key either misses the map (404) or
			// drains its own genuine content. The network write is done after releasing
			// jw.mu so a slow client cannot stall recycling or the Serve loop.
			jw.mu.RLock()
			rq := jw.requests[jawsKey]
			// Bind the tail fetch to the client like the WebSocket claim path
			// (Request.claim): the one-shot tail is drained only when the fetch comes from
			// the same client IP the initial request was issued to (loopback-aware, see
			// equalIP). rq.remoteIP is stable here because recycling requires the jw.mu
			// write lock. A mismatch is treated as not found, so a leaked key cannot drain
			// (and thereby deny) another client's tail. The WebSocket carries all live
			// data, so this only closes the cross-IP read of the already-rendered
			// attribute/class fragments and the cross-IP one-shot race.
			if rq != nil && !equalIP(remoteIP, rq.remoteIP) {
				rq = nil
			}
			var b []byte
			var sent bool
			if rq != nil {
				b, sent = rq.drainTailScript()
			}
			jw.mu.RUnlock()
			if rq != nil {
				if err := rq.writeTailResponse(w, b, sent); err != nil {
					jw.cancelIfCurrent(jawsKey, rq, err)
				}
				handled = true
			}
		}
	}
	return
}
