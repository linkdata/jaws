package jaws

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/textproto"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/linkdata/jaws/lib/jid"
	"github.com/linkdata/jaws/lib/key"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
	"github.com/linkdata/staticserve"
)

// The processing-loop subsystem distributes broadcasts to subscribed Requests
// and drives periodic maintenance.

// Pending returns the number of requests waiting for their WebSocket callbacks.
func (jw *Jaws) Pending() (n int) {
	jw.mu.RLock()
	defer jw.mu.RUnlock()
	n = jw.pendingRequestCountLocked()
	return
}

func (jw *Jaws) getWebSocketTimeout() (t time.Duration) {
	jw.mu.RLock()
	t = jw.webSocketTimeout
	jw.mu.RUnlock()
	return
}

// ServeWithTimeout begins processing requests.
//
// requestTimeout must be an exact multiple of [time.Second] from [time.Second]
// through 2,147,483,646 seconds. Other values have unspecified behavior.
//
// An overlapping [Jaws.Serve] or [Jaws.ServeWithTimeout] call reports
// [ErrServeAlreadyRunning] through [Jaws.MustLog]. It panics without a Logger
// and in debug or race builds; otherwise it returns without starting another
// processing loop.
//
// Before [Request.ServeHTTP] begins WebSocket processing, timeout-based Request
// retirement is periodic and approximate, not a hard deadline. [Jaws.NewRequest],
// a successful [Jaws.UseRequest], and [Request.MarkWritten] mark activity using
// whole-second samples from the epoch established by [New]. Retirement is checked
// only during maintenance passes, so it is not timed precisely from those events.
//
// requestTimeout also bounds each WebSocket keepalive ping and outbound write,
// independently of the maintenance schedule. See [Jaws.WebSocketPingInterval]
// for probe scheduling.
//
// It is intended to run on its own goroutine and returns when [Jaws.Close] is
// called. Errors reported through [Jaws.Log] are queued without waiting for
// Logger.Error. On a normal return after shutdown, ServeWithTimeout waits for
// every log entry accepted before [Jaws.Close] to finish. A blocked Logger.Error
// callback therefore delays that return.
func (jw *Jaws) ServeWithTimeout(requestTimeout time.Duration) {
	if !jw.serving.CompareAndSwap(false, true) {
		jw.reportMisuse(ErrServeAlreadyRunning)
		return
	}
	defer jw.serving.Store(false)

	const minInterval = time.Millisecond * 10
	const maxInterval = time.Second
	maintenanceInterval := min(requestTimeout/2, maxInterval)
	maintenanceInterval = max(maintenanceInterval, minInterval)

	subs := map[chan wire.Message]*Request{}
	t := jw.newMaintenanceTicker(maintenanceInterval)
	jw.mu.Lock()
	jw.webSocketTimeout = requestTimeout
	jw.maintenanceInterval = maintenanceInterval
	jw.mu.Unlock()
	// Seed the seconds counter so it is accurate from the first request, then keep
	// it fresh on every maintenance tick (see the case below).
	jw.refreshRuntimeSeconds()

	normalShutdown := false
	defer func() {
		t.Stop()
		for ch, rq := range subs {
			rq.cancel(nil)
			close(ch)
		}
		// Only the Done case below is a normal shutdown. A panic can race Close;
		// waiting here while it unwinds could hide that panic behind a blocked
		// Logger.Error callback. The flag preserves panic and Goexit semantics
		// without recover and re-panic.
		if normalShutdown && jw.loggerQueue != nil {
			<-jw.loggerQueue.doneCh
		}
	}()

	killSub := func(msgCh chan wire.Message) {
		if _, ok := subs[msgCh]; ok {
			delete(subs, msgCh)
			close(msgCh)
		}
	}

	// it is critical that we keep the broadcast
	// distribution loop running, so any Request
	// that fails to process its messages quickly
	// enough must be terminated. the alternative
	// would be to drop some messages, but that
	// could mean nonreproducible and seemingly
	// random failures in processing logic.
	mustBroadcast := func(msg wire.Message) {
		for msgCh, rq := range subs {
			if msg.Dest == nil || rq.wantMessage(&msg) {
				select {
				case msgCh <- msg:
				default:
					// Only the internal periodic dirty-render tick, a nil-destination
					// Update (see the updateTicker case below), is safe to drop.
					// distributeDirt has already moved the dirty selectors into Requests'
					// pending-dirt lists and cleared the global set, so the
					// tick carries no payload;
					// it only nudges the Request. The pending dirt is still rendered
					// without it: a Request already in its process loop is woken by the
					// message that filled the channel and drains todoDirt on the next pass,
					// and one still starting up (subscribed before onConnect) drains
					// todoDirt on its first pass without needing a wake. Every addressed
					// message is one-shot and must not be silently dropped — including a
					// tag-targeted Update and the key-targeted Update wake-up from
					// Session.Close — so an overloaded Request is failed-fast instead.
					if msg.What != what.Update || msg.Dest != nil {
						killSub(msgCh)
						rq.cancel(fmt.Errorf("%w: %v: broadcast channel full sending %s", ErrRequestOverloaded, rq, msg.String()))
					}
				}
			}
		}
	}

	for {
		select {
		case <-jw.Done():
			normalShutdown = true
			return
		case <-jw.updateTicker.C:
			if jw.distributeDirt() > 0 {
				mustBroadcast(wire.Message{What: what.Update})
			}
		case <-t.C:
			jw.refreshRuntimeSeconds()
			jw.maintenance(requestTimeout)
		case sub := <-jw.subCh:
			if sub.msgCh != nil {
				subs[sub.msgCh] = sub.rq
			}
		case msgCh := <-jw.unsubCh:
			killSub(msgCh)
		case msg, ok := <-jw.bcastCh:
			if ok {
				mustBroadcast(msg)
			}
		}
	}
}

// Serve calls [Jaws.ServeWithTimeout] with [DefaultWebSocketTimeout].
//
// See [Jaws.ServeWithTimeout] for lifecycle and panic behavior.
func (jw *Jaws) Serve() {
	jw.ServeWithTimeout(DefaultWebSocketTimeout)
}

func (jw *Jaws) subscribe(rq *Request, size int) chan wire.Message {
	msgCh := make(chan wire.Message, size)
	select {
	case <-jw.Done():
		close(msgCh)
		return nil
	case jw.subCh <- subscription{msgCh: msgCh, rq: rq}:
	}
	return msgCh
}

func (jw *Jaws) unsubscribe(msgCh chan wire.Message) {
	select {
	case <-jw.Done():
	case jw.unsubCh <- msgCh:
	}
}

func (jw *Jaws) maintenance(requestTimeout time.Duration) {
	jw.mu.Lock()
	nowSeconds := jw.runtimeSeconds.Load()
	for _, rq := range jw.requests {
		if rq == nil {
			continue
		}
		if expired, cause := rq.maintenance(nowSeconds, requestTimeout); expired {
			_ = jw.Log(cause)
			jw.retireNonRunningRequestLocked(rq)
		}
	}
	for _, sess := range jw.sessions {
		if sess.isDead() {
			jw.deleteSessionIfCurrentLocked(sess)
		}
	}
	jw.updateStatusLocked()
	jw.mu.Unlock()
}

// The client-IP subsystem resolves addresses for tail-fetch and WebSocket
// binding, honouring trusted forwarded headers when configured.

// equalIP reports whether a and b identify the same client for the purpose of
// session and request-key binding. Addresses are unmapped first so an
// IPv4-mapped IPv6 address (::ffff:a.b.c.d, as a proxy may write into a forwarded
// header) matches its plain IPv4 form. Two loopback addresses always compare equal
// so that a reverse proxy connecting to the backend over loopback does not break
// binding; the consequence is that when every request arrives from loopback (the
// typical proxied deployment without forwarded-IP binding) IP binding is a no-op.
// Enable [Jaws.TrustForwardedHeaders] to bind on the forwarded client IP instead
// (see the clientIP method).
func equalIP(a, b netip.Addr) bool {
	a, b = a.Unmap(), b.Unmap()
	return a.Compare(b) == 0 || (a.IsLoopback() && b.IsLoopback())
}

func parseIP(remoteAddr string) (ip netip.Addr) {
	if remoteAddr != "" {
		if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
			ip, _ = netip.ParseAddr(host)
		} else {
			ip, _ = netip.ParseAddr(remoteAddr)
		}
	}
	return
}

// clientIP returns the address used to bind sessions and request keys to a
// client. When [Jaws.TrustForwardedHeaders] is set it prefers the client IP from
// the proxy-supplied forwarded headers, so binding keeps working behind a reverse
// proxy that connects over loopback; otherwise (and as a fallback) it uses the
// transport peer address. TrustForwardedHeaders must only be enabled behind a
// single reverse proxy you control that sets these headers (see the field doc).
func (jw *Jaws) clientIP(r *http.Request) (ip netip.Addr) {
	if r != nil {
		if jw.TrustForwardedHeaders {
			if fip, ok := forwardedClientIP(r.Header); ok {
				return fip
			}
		}
		ip = parseIP(r.RemoteAddr)
	}
	return
}

// forwardedClientIP extracts the client IP from proxy-supplied headers. It uses
// the leftmost X-Forwarded-For entry (the original client as seen by a single
// trusted proxy), falling back to X-Real-IP. Callers must only trust these
// headers when behind a controlled proxy (see [Jaws.TrustForwardedHeaders]).
func forwardedClientIP(h http.Header) (netip.Addr, bool) {
	if xff := h.Get("X-Forwarded-For"); xff != "" {
		first, _, _ := strings.Cut(xff, ",")
		if ip, err := netip.ParseAddr(textproto.TrimString(first)); err == nil {
			return ip, true
		}
	}
	if xrip := textproto.TrimString(h.Get("X-Real-Ip")); xrip != "" {
		if ip, err := netip.ParseAddr(xrip); err == nil {
			return ip, true
		}
	}
	return netip.Addr{}, false
}

// The setup subsystem turns resource extras into handler registrations and head
// HTML URLs.

// HandleFunc matches the signature of [http.ServeMux.Handle].
type HandleFunc = func(pattern string, handler http.Handler)

// SetupFunc is called by [Jaws.Setup] and allows setting up addons for JaWS.
//
// When [Jaws.Setup] is called with a nil [HandleFunc], setup functions receive
// a no-op handler registration function.
//
// The URLs returned will be used in a call to [Jaws.GenerateHeadHTML].
type SetupFunc = func(jw *Jaws, handleFn HandleFunc, prefix string) (urls []*url.URL, err error)

// makeAbsPath returns a copy of u with prefix prepended to relative paths.
//
// When a non-empty prefix is applied, the joined path is slash-rooted. An empty
// prefix preserves a relative URL.
func makeAbsPath(prefix string, u *url.URL) *url.URL {
	if u != nil {
		copied := *u
		u = &copied
		if prefix != "" && u.Scheme == "" && u.Host == "" && !path.IsAbs(u.Path) {
			u.Path = staticserve.EnsurePrefixSlash(path.Join(prefix, u.Path))
		}
	}
	return u
}

// Setup configures [Jaws] with extra functionality and resources.
//
// The list of extras can be strings, [*url.URL], [*staticserve.StaticServe] or
// []*staticserve.StaticServe URL resources, or a [SetupFunc] such as
// jawsboot.Setup.
//
// A value of a defined function type must be converted to [SetupFunc] before it
// is passed as an extra.
//
// A nil [SetupFunc] extra is ignored.
//
// It calls [Jaws.GenerateHeadHTML] with the final list of URLs, with any
// relative URL paths prefixed with prefix.
//
// [staticserve.StaticServe] extras are local resources. Their generated URLs
// are slash-rooted so they match their registered handlers, including when
// prefix is empty. Other relative URL extras remain relative with an empty
// prefix. Each [staticserve.StaticServe.Name] is treated as a literal path,
// not as a pre-escaped URL; percent signs in a name are escaped as literal
// percent signs.
//
// If handleFn is nil, Setup generates head HTML from the configured resources
// without registering any handlers.
func (jw *Jaws) Setup(handleFn HandleFunc, prefix string, extras ...any) (err error) {
	var urls []*url.URL
	setupHandleFn := handleFn
	if setupHandleFn == nil {
		setupHandleFn = func(string, http.Handler) {}
	}

	handleStaticServe := func(ss *staticserve.StaticServe) {
		if ss != nil {
			assetPath := ss.Name
			if !path.IsAbs(assetPath) {
				assetPath = path.Join(prefix, assetPath)
			}
			u := &url.URL{Path: path.Join("/", assetPath)}
			urls = append(urls, u)
			if handleFn != nil {
				setupHandleFn(staticserve.NormalizeGET(u.String()), ss)
			}
		}
	}

	for _, extra := range extras {
		switch extra := extra.(type) {
		case []*staticserve.StaticServe:
			for _, ss := range extra {
				handleStaticServe(ss)
			}
		case string:
			u, urlErr := url.Parse(extra)
			err = errors.Join(err, urlErr)
			urls = append(urls, makeAbsPath(prefix, u))
		case *url.URL:
			urls = append(urls, makeAbsPath(prefix, extra))
		case *staticserve.StaticServe:
			handleStaticServe(extra)
		case SetupFunc:
			if extra != nil {
				setupURLs, setupErr := extra(jw, setupHandleFn, prefix)
				err = errors.Join(err, setupErr)
				for _, u := range setupURLs {
					urls = append(urls, makeAbsPath(prefix, u))
				}
			}
		default:
			err = errors.Join(err, fmt.Errorf("jaws.Setup: expected a string, *url.URL, *staticserve.StaticServe, []*staticserve.StaticServe or jaws.SetupFunc, not %T", extra))
		}
	}
	var extraFiles []string
	for _, u := range urls {
		if u != nil {
			extraFiles = append(extraFiles, u.String())
		}
	}
	err = errors.Join(err, jw.GenerateHeadHTML(extraFiles...))
	return
}

// The tail-script subsystem serves one-shot attribute and class updates queued
// during initial rendering before the WebSocket connects.

const headerContentTypeJavaScript = "text/javascript; charset=utf-8"

// tailScriptStart isolates initial DOM fixups and protects the id attribute.
const tailScriptStart = `{const X=f=>(i,d)=>{try{let e=document.getElementById("` + jid.Prefix + `"+i);e&&f(e,d)}catch(e){console.error(e)}},` +
	`I=(n,a)=>{if(n.toLowerCase()==="id")throw"jaws: refusing to "+a+" reserved attribute 'id'"},` +
	`A=X((e,d)=>{let i=d.indexOf("\n"),n=d.substring(0,i),v=d.substring(i+1);I(n,"change");e.getAttribute(n)===v||e.setAttribute(n,v)}),` +
	`R=X((e,n)=>{I(n,"remove");e.removeAttribute(n)}),` +
	`C=X((e,c)=>e.classList.add(c)),` +
	`D=X((e,c)=>e.classList.remove(c));`

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

func tailScriptAlias(msg wire.WsMsg) (fn byte) {
	if msg.Jid > 0 {
		switch msg.What {
		case what.SAttr:
			fn = 'A'
		case what.RAttr:
			fn = 'R'
		case what.SClass:
			fn = 'C'
		case what.RClass:
			fn = 'D'
		}
	}
	return
}

// drainTailScript builds the tail <script> body from the attribute and class messages
// queued during initial rendering, reporting sent=true the first time it runs for this
// Request (subsequent calls return sent=false so the response is 204).
func (rq *Request) drainTailScript() (b []byte, sent bool) {
	// Takes only muQueue and never touches the network. Jaws.ServeHTTP calls it while
	// holding jw.mu (read), which blocks finishing (which needs the jw.mu write lock),
	// so this Request cannot be unregistered and have its buffers released mid-drain:
	// the bytes returned always belong to the identity the handler looked up. The slow
	// network write happens afterwards in writeTailResponse with no lock held, so a
	// stalled client cannot block completion or the Serve loop. The data race on
	// wsQueue/tailsent is prevented because releaseBuffersLocked also takes muQueue to
	// reset them.
	rq.muQueue.Lock()
	defer rq.muQueue.Unlock()
	if !rq.tailsent {
		rq.tailsent = true
		sent = true
		tailOps := 0
		tailCap := len(tailScriptStart)
		for _, msg := range rq.wsQueue {
			if tailScriptAlias(msg) != 0 {
				tailOps++
				tailCap += len(msg.Data)
			}
		}
		if tailOps > 0 {
			// Reserve payload bytes plus a small per-fixup syntax estimate.
			b = make([]byte, 0, tailCap+tailOps*12)
			b = append(b, tailScriptStart...)
		}
		n := 0
		for _, msg := range rq.wsQueue {
			if fn := tailScriptAlias(msg); fn != 0 {
				b = append(b, fn, '(')
				b = msg.Jid.AppendInt(b)
				b = append(b, ',')
				// A splits SAttr at its first newline; other operations use Data unchanged.
				b = appendJSQuote(b, msg.Data)
				b = append(b, ')', ';')
			} else {
				rq.wsQueue[n] = msg
				n++
			}
		}
		clear(rq.wsQueue[n:])
		rq.wsQueue = rq.wsQueue[:n]
		if len(b) > 0 {
			b = append(b, '}', '\n')
		}
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
	hdr.Set("Cache-Control", headerCacheControlNoStore)
	hdr.Set("Content-Type", headerContentTypeJavaScript)
	if !sent {
		w.WriteHeader(http.StatusNoContent)
	} else if len(b) > 0 {
		// b is built by drainTailScript, which JS-escapes every attribute and class
		// payload via appendJSQuote (see TestRequest_writeTailScript_EscapesScriptClose),
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
			// Hold jw.mu (read) across both the lookup and the drain: finishing needs
			// the jw.mu write lock, so rq cannot be unregistered while we drain its
			// queue. A stale key either misses the map (404) or drains its own genuine
			// content. The network write is done after releasing jw.mu so a slow client
			// cannot stall completion or the Serve loop.
			jw.mu.RLock()
			rq := jw.requests[jawsKey]
			// Bind the tail fetch to the client like the WebSocket claim path
			// (Request.claim): the one-shot tail is drained only when the fetch comes from
			// the same client IP the initial request was issued to (loopback-aware, see
			// equalIP). rq.remoteIP is stable here because finishing requires the jw.mu
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
