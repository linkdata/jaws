// Package jaws provides a mechanism to create dynamic
// webpages using Javascript and WebSockets.
//
// It integrates well with Go's html/template package,
// but can be used without it. It can be used with any
// router that supports the standard ServeHTTP interface.
package jaws

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/textproto"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/jid"
	"github.com/linkdata/jaws/what"
)

const (
	DefaultUpdateInterval = time.Millisecond * 100 // Default browser update interval
)

type Jid = jid.Jid // convenience alias

type Jaws struct {
	CookieName   string      // Name for session cookies, defaults to "jaws"
	Logger       *log.Logger // If not nil, send debug info and errors here
	Debug        bool        // set to true to enable debugging output
	doneCh       <-chan struct{}
	bcastCh      chan Message
	subCh        chan subscription
	unsubCh      chan chan Message
	updateTicker *time.Ticker
	headPrefix   string
	reqPool      sync.Pool
	mu           deadlock.RWMutex // protects following
	tmplookers   map[TemplateLookuper]struct{}
	kg           *bufio.Reader
	closeCh      chan struct{}
	requests     map[uint64]*Request
	sessions     map[uint64]*Session
	dirty        map[any]int
	dirtOrder    int
}

// NewWithDone returns a new JaWS object using the given completion channel.
// This is expected to be created once per HTTP server and handles
// publishing HTML changes across all connections.
func NewWithDone(doneCh <-chan struct{}) (jw *Jaws) {
	jw = &Jaws{
		CookieName:   DefaultCookieName,
		doneCh:       doneCh,
		bcastCh:      make(chan Message, 1),
		subCh:        make(chan subscription, 1),
		unsubCh:      make(chan chan Message, 1),
		updateTicker: time.NewTicker(DefaultUpdateInterval),
		headPrefix:   HeadHTML([]string{JavascriptPath}, nil),
		tmplookers:   make(map[TemplateLookuper]struct{}),
		kg:           bufio.NewReader(rand.Reader),
		requests:     make(map[uint64]*Request),
		sessions:     make(map[uint64]*Session),
		dirty:        make(map[any]int),
	}
	jw.reqPool.New = func() any {
		return (&Request{
			Jaws:   jw,
			tagMap: make(map[any][]*Element),
		}).clearLocked()
	}
	return
}

// New returns a new JaWS object that must be closed using Close().
// This is expected to be created once per HTTP server and handles
// publishing HTML changes across all connections.
func New() (jw *Jaws) {
	closeCh := make(chan struct{})
	jw = NewWithDone(closeCh)
	jw.closeCh = closeCh
	return
}

// Close frees resources associated with the JaWS object, and
// closes the completion channel if the JaWS was created with New().
// Once the completion channel is closed, broadcasts and sends may be discarded.
// Subsequent calls to Close() have no effect.
func (jw *Jaws) Close() {
	jw.mu.Lock()
	if jw.closeCh != nil {
		close(jw.closeCh)
		jw.closeCh = nil
	}
	jw.updateTicker.Stop()
	jw.mu.Unlock()
}

// Done returns the completion channel.
func (jw *Jaws) Done() <-chan struct{} {
	return jw.doneCh
}

// AddTemplateLookuper adds an object that can resolve
// strings to *template.Template.
func (jw *Jaws) AddTemplateLookuper(tl TemplateLookuper) {
	if tl != nil && tl != jw {
		jw.mu.Lock()
		jw.tmplookers[tl] = struct{}{}
		jw.mu.Unlock()
	}
}

// RemoveTemplateLookuper removes the given object from
// the map of TemplateLookupers.
func (jw *Jaws) RemoveTemplateLookuper(tl TemplateLookuper) {
	if tl != nil {
		jw.mu.Lock()
		delete(jw.tmplookers, tl)
		jw.mu.Unlock()
	}
}

// Lookup queries the known TemplateLookupers in random
// order for the given template name and returns the first found.
func (jw *Jaws) Lookup(name string) *template.Template {
	jw.mu.RLock()
	defer jw.mu.RUnlock()
	for tl := range jw.tmplookers {
		if t := tl.Lookup(name); t != nil {
			return t
		}
	}
	return nil
}

// RequestCount returns the number of Requests.
//
// The count includes all Requests, including those being rendered,
// those waiting for the WebSocket callback and those active.
func (jw *Jaws) RequestCount() (n int) {
	jw.mu.RLock()
	n = len(jw.requests)
	jw.mu.RUnlock()
	return
}

// Log sends an error to the Logger set in the Jaws.
// Has no effect if the err is nil or the Logger is nil.
// Returns err.
func (jw *Jaws) Log(err error) error {
	if err != nil && jw != nil && jw.Logger != nil {
		jw.Logger.Println(err.Error())
	}
	return err
}

// MustLog sends an error to the Logger set in the Jaws or
// panics with the given error if no Logger is set.
// Has no effect if the err is nil.
func (jw *Jaws) MustLog(err error) {
	if err != nil {
		if jw != nil && jw.Logger != nil {
			jw.Logger.Println(err.Error())
		} else {
			panic(err)
		}
	}
}

// NextID returns a uint64 unique within lifetime of the program.
func NextID() int64 {
	return atomic.AddInt64((*int64)(&nextJid), 1)
}

// AppendID appends the result of NextID() in text form to the given slice.
func AppendID(b []byte) []byte {
	return strconv.AppendInt(b, NextID(), 32)
}

// MakeID returns a string in the form 'jaws.X' where X is a unique string within lifetime of the program.
func MakeID() string {
	return string(AppendID([]byte("jaws.")))
}

// NewRequest returns a new pending JaWS request that times out after 10 seconds.
//
// Call this as soon as you start processing a HTML request, and store the
// returned Request pointer so it can be used while constructing the HTML
// response in order to register the JaWS id's you use in the response, and
// use it's Key attribute when sending the Javascript portion of the reply.
func (jw *Jaws) NewRequest(hr *http.Request) (rq *Request) {
	jw.mu.Lock()
	defer jw.mu.Unlock()
	for rq == nil {
		jawsKey := jw.nonZeroRandomLocked()
		if _, ok := jw.requests[jawsKey]; !ok {
			rq = jw.getRequestLocked(jawsKey, hr)
			jw.requests[jawsKey] = rq
		}
	}
	return
}

func (jw *Jaws) nonZeroRandomLocked() (val uint64) {
	random := make([]byte, 8)
	for val == 0 {
		if _, err := io.ReadFull(jw.kg, random); err != nil {
			panic(err)
		}
		val = binary.LittleEndian.Uint64(random)
	}
	return
}

// UseRequest extracts the JaWS request with the given key from the request
// map if it exists and the HTTP request remote IP matches.
//
// Call it when receiving the WebSocket connection on '/jaws/:key' to get the
// associated Request, and then call it's ServeHTTP method to process the
// WebSocket messages.
//
// Returns nil if the key was not found or the IP doesn't match, in which
// case you should return a HTTP "404 Not Found" status.
func (jw *Jaws) UseRequest(jawsKey uint64, hr *http.Request) (rq *Request) {
	if jawsKey != 0 {
		var err error
		jw.mu.Lock()
		if waitingRq, ok := jw.requests[jawsKey]; ok {
			if err = waitingRq.claim(hr); err == nil {
				rq = waitingRq
			}
		}
		jw.mu.Unlock()
		_ = jw.Log(err)
	}
	return
}

// SessionCount returns the number of active sessions.
func (jw *Jaws) SessionCount() (n int) {
	jw.mu.RLock()
	n = len(jw.sessions)
	jw.mu.RUnlock()
	return
}

// Sessions returns a list of all active sessions, which may be nil.
func (jw *Jaws) Sessions() (sl []*Session) {
	jw.mu.RLock()
	if n := len(jw.sessions); n > 0 {
		sl = make([]*Session, 0, n)
		for _, sess := range jw.sessions {
			sl = append(sl, sess)
		}
	}
	jw.mu.RUnlock()
	return
}

func (jw *Jaws) getSessionLocked(sessIds []uint64, remoteIP netip.Addr) *Session {
	for _, sessId := range sessIds {
		if sess, ok := jw.sessions[sessId]; ok && equalIP(remoteIP, sess.remoteIP) {
			return sess
		}
	}
	return nil
}

func cutString(s string, sep byte) (before, after string) {
	if i := strings.IndexByte(s, sep); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

func getCookieSessionsIds(h http.Header, wanted string) (cookies []uint64) {
	for _, line := range h["Cookie"] {
		if strings.Contains(line, wanted) {
			var part string
			line = textproto.TrimString(line)
			for len(line) > 0 {
				part, line = cutString(line, ';')
				if part = textproto.TrimString(part); part != "" {
					name, val := cutString(part, '=')
					name = textproto.TrimString(name)
					if name == wanted {
						if len(val) > 1 && val[0] == '"' && val[len(val)-1] == '"' {
							val = val[1 : len(val)-1]
						}
						if sessId := JawsKeyValue(val); sessId != 0 {
							cookies = append(cookies, sessId)
						}
					}
				}
			}
		}
	}
	return
}

// GetSession returns the Session associated with the given *http.Request, or nil.
func (jw *Jaws) GetSession(hr *http.Request) (sess *Session) {
	if sessIds := getCookieSessionsIds(hr.Header, jw.CookieName); len(sessIds) > 0 {
		remoteIP := parseIP(hr.RemoteAddr)
		jw.mu.RLock()
		sess = jw.getSessionLocked(sessIds, remoteIP)
		jw.mu.RUnlock()
	}
	return
}

// NewSession creates a new Session.
//
// Any pre-existing Session will be cleared and closed.
//
// Subsequent Requests created with `NewRequest()` that have the cookie set and
// originates from the same IP will be able to access the Session.
func (jw *Jaws) NewSession(w http.ResponseWriter, hr *http.Request) (sess *Session) {
	if oldSess := jw.GetSession(hr); oldSess != nil {
		oldSess.Clear()
		oldSess.Close()
	}

	jw.mu.Lock()
	defer jw.mu.Unlock()
	for sess == nil {
		sessionID := jw.nonZeroRandomLocked()
		if _, ok := jw.sessions[sessionID]; !ok {
			sess = newSession(jw, sessionID, parseIP(hr.RemoteAddr))
			jw.sessions[sessionID] = sess
			if w != nil {
				http.SetCookie(w, &sess.cookie)
			}
			hr.AddCookie(&sess.cookie)
		}
	}
	return
}

func (jw *Jaws) deleteSession(sessionID uint64) {
	jw.mu.Lock()
	delete(jw.sessions, sessionID)
	jw.mu.Unlock()
}

// GenerateHeadHTML (re-)generates the HTML code that goes in the HEAD section, ensuring
// that the provided scripts and stylesheets in `extra` are loaded.
//
// You only need to call this if you want to add your own scripts and stylesheets.
func (jw *Jaws) GenerateHeadHTML(extra ...string) error {
	var js, css []string
	addedJaws := false
	for _, e := range extra {
		if u, err := url.Parse(e); err == nil {
			if strings.HasSuffix(u.Path, ".js") {
				js = append(js, e)
				addedJaws = addedJaws || strings.HasSuffix(u.Path, JavascriptPath)
			} else if strings.HasSuffix(e, ".css") {
				css = append(css, e)
			} else {
				return fmt.Errorf("%q: not .js or .css", u.Path)
			}
		} else {
			return err
		}
	}
	if !addedJaws {
		js = append(js, JavascriptPath)
	}
	jw.headPrefix = HeadHTML(js, css) + `<script>var jawsKey="`
	return nil
}

// Broadcast sends a message to all Requests.
func (jw *Jaws) Broadcast(msg Message) {
	select {
	case <-jw.Done():
	case jw.bcastCh <- msg:
	}
}

// setDirty marks all Elements that have one or more of the given tags as dirty.
func (jw *Jaws) setDirty(tags []any) {
	jw.mu.Lock()
	defer jw.mu.Unlock()
	for _, tag := range tags {
		jw.dirtOrder++
		jw.dirty[tag] = jw.dirtOrder
	}
}

// Dirty marks all Elements that have one or more of the given tags as dirty.
//
// Note that if any of the tags are a TagGetter, it will be called with a nil Request.
// Prefer using Request.Dirty() which avoids this.
func (jw *Jaws) Dirty(tags ...any) {
	jw.setDirty(MustTagExpand(nil, tags))
}

func (jw *Jaws) distributeDirt() int {
	type orderedDirt struct {
		tag   any
		order int
	}

	jw.mu.Lock()
	dirt := make([]orderedDirt, 0, len(jw.dirty))
	for k, v := range jw.dirty {
		dirt = append(dirt, orderedDirt{tag: k, order: v})
		delete(jw.dirty, k)
	}
	jw.dirtOrder = 0

	var reqs []*Request
	if len(dirt) > 0 {
		reqs = make([]*Request, 0, len(jw.requests))
		for _, rq := range jw.requests {
			reqs = append(reqs, rq)
		}
	}
	jw.mu.Unlock()

	if len(dirt) > 0 {
		sort.Slice(dirt, func(i, j int) bool { return dirt[i].order < dirt[j].order })
		tags := make([]any, len(dirt))
		for i := range dirt {
			tags[i] = dirt[i].tag
		}
		for _, rq := range reqs {
			rq.appendDirtyTags(tags)
		}
	}
	return len(dirt)
}

// Reload requests all Requests to reload their current page.
func (jw *Jaws) Reload() {
	jw.Broadcast(Message{
		What: what.Reload,
	})
}

// Redirect requests all Requests to navigate to the given URL.
func (jw *Jaws) Redirect(url string) {
	jw.Broadcast(Message{
		What: what.Redirect,
		Data: url,
	})
}

// Alert sends an alert to all Requests. The lvl argument should be one of Bootstraps alert levels:
// primary, secondary, success, danger, warning, info, light or dark.
func (jw *Jaws) Alert(lvl, msg string) {
	jw.Broadcast(Message{
		What: what.Alert,
		Data: lvl + "\n" + msg,
	})
}

// Count returns the number of requests waiting for their WebSocket callbacks.
func (jw *Jaws) Pending() (n int) {
	jw.mu.RLock()
	defer jw.mu.RUnlock()
	for _, rq := range jw.requests {
		if !rq.claimed.Load() {
			n++
		}
	}
	return
}

// ServeWithTimeout begins processing requests with the given timeout.
// It is intended to run on it's own goroutine.
// It returns when the completion channel is closed.
func (jw *Jaws) ServeWithTimeout(requestTimeout time.Duration) {
	const minInterval = time.Millisecond * 10
	const maxInterval = time.Second
	maintenanceInterval := requestTimeout / 2
	if maintenanceInterval > maxInterval {
		maintenanceInterval = maxInterval
	}
	if maintenanceInterval < minInterval {
		maintenanceInterval = minInterval
	}
	t := time.NewTicker(maintenanceInterval)
	defer t.Stop()
	subs := map[chan Message]*Request{}

	killSub := func(msgCh chan Message) {
		if _, ok := subs[msgCh]; ok {
			delete(subs, msgCh)
			close(msgCh)
		}
	}

	// it's critical that we keep the broadcast
	// distribution loop running, so any Request
	// that fails to process it's messages quickly
	// enough must be terminated. the alternative
	// would be to drop some messages, but that
	// could mean nonreproducible and seemingly
	// random failures in processing logic.
	mustBroadcast := func(msg Message) {
		for msgCh, rq := range subs {
			if msg.Dest == nil || rq.wantMessage(&msg) {
				select {
				case msgCh <- msg:
				default:
					// the exception is Update messages, more will follow eventually
					if msg.What != what.Update {
						killSub(msgCh)
						rq.cancel(fmt.Errorf("%v: broadcast channel full sending %s", rq, msg.String()))
					}
				}
			}
		}
	}

	for {
		select {
		case <-jw.Done():
			return
		case <-jw.updateTicker.C:
			if jw.distributeDirt() > 0 {
				mustBroadcast(Message{What: what.Update})
			}
		case <-t.C:
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

// Serve calls ServeWithTimeout(time.Second * 10).
func (jw *Jaws) Serve() {
	jw.ServeWithTimeout(time.Second * 10)
}

func (jw *Jaws) subscribe(rq *Request, size int) chan Message {
	msgCh := make(chan Message, size)
	select {
	case <-jw.Done():
		close(msgCh)
		return nil
	case jw.subCh <- subscription{msgCh: msgCh, rq: rq}:
	}
	return msgCh
}

func (jw *Jaws) unsubscribe(msgCh chan Message) {
	select {
	case <-jw.Done():
	case jw.unsubCh <- msgCh:
	}
}

func (jw *Jaws) maintenance(requestTimeout time.Duration) {
	jw.mu.Lock()
	defer jw.mu.Unlock()
	now := time.Now()
	for _, rq := range jw.requests {
		if rq.maintenance(now, requestTimeout) {
			jw.recycleLocked(rq)
		}
	}
	for k, sess := range jw.sessions {
		if sess.isDead() {
			delete(jw.sessions, k)
		}
	}
}

func equalIP(a, b netip.Addr) bool {
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

func maybePanic(err error) {
	if err != nil {
		panic(err)
	}
}

// SetInner sends a request to replace the inner HTML of
// all HTML elements matching target.
func (jw *Jaws) SetInner(target any, innerHtml template.HTML) {
	jw.Broadcast(Message{
		Dest: target,
		What: what.Inner,
		Data: string(innerHtml),
	})
}

// SetAttr sends a request to replace the given attribute value in
// all HTML elements matching target.
func (jw *Jaws) SetAttr(target any, attr, val string) {
	jw.Broadcast(Message{
		Dest: target,
		What: what.SAttr,
		Data: attr + "\n" + val,
	})
}

// RemoveAttr sends a request to remove the given attribute from
// all HTML elements matching target.
func (jw *Jaws) RemoveAttr(target any, attr string) {
	jw.Broadcast(Message{
		Dest: target,
		What: what.RAttr,
		Data: attr,
	})
}

// SetClass sends a request to set the given class in
// all HTML elements matching target.
func (jw *Jaws) SetClass(target any, cls string) {
	jw.Broadcast(Message{
		Dest: target,
		What: what.SClass,
		Data: cls,
	})
}

// RemoveClass sends a request to remove the given class from
// all HTML elements matching target.
func (jw *Jaws) RemoveClass(target any, cls string) {
	jw.Broadcast(Message{
		Dest: target,
		What: what.RClass,
		Data: cls,
	})
}

// SetValue sends a request to set the HTML "value" attribute of
// all HTML elements matching target.
func (jw *Jaws) SetValue(target any, val string) {
	jw.Broadcast(Message{
		Dest: target,
		What: what.Value,
		Data: val,
	})
}

// Insert calls the Javascript 'insertBefore()' method on
// all HTML elements matching target.
//
// The position parameter 'where' may be either a HTML ID, an child index or the text 'null'.
func (jw *Jaws) Insert(target any, where, html string) {
	jw.Broadcast(Message{
		Dest: target,
		What: what.Insert,
		Data: where + "\n" + html,
	})
}

// Replace replaces the HTML content on
// all HTML elements matching target.
//
// The position parameter 'where' may be either a HTML ID or an index.
func (jw *Jaws) Replace(target any, where, html string) {
	jw.Broadcast(Message{
		Dest: target,
		What: what.Replace,
		Data: where + "\n" + html,
	})
}

// Delete removes the HTML element(s) matching target.
func (jw *Jaws) Delete(target any) {
	jw.Broadcast(Message{
		Dest: target,
		What: what.Delete,
	})
}

// Append calls the Javascript 'appendChild()' method on all HTML elements matching target.
func (jw *Jaws) Append(target any, html template.HTML) {
	jw.Broadcast(Message{
		Dest: target,
		What: what.Append,
		Data: string(html),
	})
}

func (jw *Jaws) getRequestLocked(jawsKey uint64, hr *http.Request) (rq *Request) {
	rq = jw.reqPool.Get().(*Request)
	rq.JawsKey = jawsKey
	rq.lastWrite = time.Now()
	rq.initial = hr
	rq.ctx, rq.cancelFn = context.WithCancelCause(context.Background())
	if hr != nil {
		rq.remoteIP = parseIP(hr.RemoteAddr)
		if sess := jw.getSessionLocked(getCookieSessionsIds(hr.Header, jw.CookieName), rq.remoteIP); sess != nil {
			sess.addRequest(rq)
			rq.session = sess
		}
	}
	return rq
}

func (jw *Jaws) recycleLocked(rq *Request) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	if rq.JawsKey != 0 {
		delete(jw.requests, rq.JawsKey)
		rq.clearLocked()
		jw.reqPool.Put(rq)
	}
}

func (jw *Jaws) recycle(rq *Request) {
	jw.mu.Lock()
	defer jw.mu.Unlock()
	jw.recycleLocked(rq)
}
