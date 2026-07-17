package jaws

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"strings"
	"sync/atomic"

	"github.com/linkdata/jaws/lib/jid"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

// Element is an instance of a [Request], a [UI] object and a [Jid].
//
// An Element pointer passed to a render, update or event handler is borrowed for
// that call. Do not retain it in application state or use it from a background
// goroutine. Its embedded Request may later be pooled and reused.
type Element struct {
	*Request // (read-only) the Request the Element belongs to
	// internals
	ui UI // the UI object
	// handlers is appended to only during render/registration (AddHandlers,
	// ApplyParams, ApplyGetter, all routed through appendHandlers) and read later
	// on the event goroutine without a lock. Request event dispatch reads handlers
	// only after frozen reports true. The frozen store after rendering publishes
	// the completed handler slice to that read, including for child Elements
	// rendered after the WebSocket connects; a preemptive event for an Element
	// still being rendered is ignored. Handlers must not be mutated once frozen.
	// All builds enforce this: appendHandlers drops late mutations (debug builds
	// panic).
	handlers []any
	jid      jid.Jid     // JaWS ID, unique to this Element within its Request
	deleted  atomic.Bool // true once the Element has been removed from its Request
	frozen   atomic.Bool // set when handlers are sealed (JawsRender returns or Freeze called); guards handler mutators in all builds
}

func (elem *Element) String() string {
	// Guard elem.Request like Request.String()/JawsKeyString guard a nil
	// receiver, so String() stays safe on a not-fully-constructed Element.
	var tags []any
	if elem.Request != nil {
		tags = elem.Request.TagsOf(elem)
	}
	return fmt.Sprintf("Element{%T, id=%q, Tags: %v}", elem.UI(), elem.Jid(), tags)
}

// appendHandlers is the single internal chokepoint for mutating elem.handlers.
//
// handlers is read lock-free on the event goroutine (via [CallEventHandlers], which
// calls the internal callEventHandlers), so it must only be appended to while the
// Element is being rendered. Request event dispatch ignores the Element until
// frozen reports true; that atomic publication makes the completed slice visible
// before the lock-free read. Once frozen, late mutations are a bug: reportMisuse
// panics in debug builds and logs in production, and the mutation is dropped.
func (elem *Element) appendHandlers(h ...any) {
	if len(h) == 0 {
		return
	}
	if elem.frozen.Load() {
		elem.Jaws.reportMisuse(errors.New("jaws: Element handlers mutated after JawsRender returned; handlers must be added during rendering, before events can fire"))
	} else if !elem.deleted.Load() {
		elem.handlers = append(elem.handlers, h...)
	}
}

// Freeze marks the [Element]'s handlers as final, as [Element.JawsRender] does on
// return. After Freeze, the handler-mutating methods (AddHandlers, ApplyParams,
// ApplyGetter) drop handlers; debug builds panic. Use this for elements registered
// for updates without being rendered.
func (elem *Element) Freeze() {
	elem.frozen.Store(true)
}

// AddHandlers adds the given handlers to the [Element].
//
// It must be called while the [Element] is being rendered, before any event can
// be processed for it; see the package "Locking" documentation. Handlers added
// after [Element.JawsRender] has returned (or [Element.Freeze] has been called)
// are dropped; debug builds panic.
func (elem *Element) AddHandlers(h ...any) {
	elem.appendHandlers(h...)
}

// Tag adds the given tags to the [Element].
func (elem *Element) Tag(tags ...any) {
	if !elem.deleted.Load() {
		elem.Request.Tag(elem, tags...)
	}
}

// HasTag returns true if this Element has the given tag.
func (elem *Element) HasTag(tagValue any) bool {
	return !elem.deleted.Load() && elem.Request.HasTag(elem, tagValue)
}

// Jid returns the JaWS ID for this [Element], unique within its [Request].
func (elem *Element) Jid() jid.Jid {
	return elem.jid
}

// UI returns the [UI] object.
func (elem *Element) UI() UI {
	return elem.ui
}

// Deleted reports whether the [Element] has been removed from its [Request].
//
// A deleted Element is inert: [Element.JawsRender], [Element.JawsUpdate] and the
// queue helpers are all no-ops on it. Deleted does not make a retained pointer
// safe to use outside the callback that supplied it.
func (elem *Element) Deleted() bool {
	return elem.deleted.Load()
}

func (elem *Element) renderDebug(w io.Writer) {
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "<!-- id=%q %T tags=[", elem.Jid(), elem.UI())
	if elem.mu.TryRLock() {
		defer elem.mu.RUnlock()
		for i, tagValue := range elem.tagsOfLocked(elem) {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(tag.TagString(tagValue))
		}
	} else {
		sb.WriteString("n/a")
	}
	sb.WriteByte(']')
	_, _ = w.Write([]byte(debugCommentSanitizer.Replace(sb.String()) + " -->"))
}

// debugCommentSanitizer neutralizes both the standard "-->" and the HTML5
// "--!>" comment-close sequences so tag text cannot escape the debug comment.
var debugCommentSanitizer = strings.NewReplacer("-->", "==>", "--!>", "==>")

// JawsRender calls [Renderer.JawsRender] for this [Element].
//
// Do not call this yourself unless it is from within another JawsRender implementation.
func (elem *Element) JawsRender(w io.Writer, params []any) (err error) {
	if !elem.deleted.Load() {
		if err = elem.UI().JawsRender(elem, w, params); err == nil {
			if elem.Jaws.Debug {
				elem.renderDebug(w)
			}
		}
	}
	// Render is complete: handlers are now frozen and read lock-free on the
	// event goroutine. Any later handler mutation is a bug and is dropped
	// (see appendHandlers).
	elem.frozen.Store(true)
	return
}

// JawsUpdate calls [Updater.JawsUpdate] for this [Element].
//
// Do not call this yourself unless it is from within another JawsUpdate implementation.
func (elem *Element) JawsUpdate() {
	if !elem.deleted.Load() {
		elem.UI().JawsUpdate(elem)
	}
}

// queue enqueues a wire message of the given type and data for this element on
// its Request, tagged with the element's Jid. It is a no-op once the element has
// been deleted.
//
// It is intended to be called while the element is rendering or updating; the
// message is appended to the Request's muQueue-guarded outbound queue and flushed
// the next time the processing loop runs a send pass. During rendering and updating
// that pass is imminent. Called from an event handler, however, the message is
// flushed only when the loop is next woken — by a broadcast, an incoming event, or a
// dirty-driven update — which on an otherwise-idle request is not guaranteed to
// happen promptly. The reliable event-driven path is therefore to mark the element
// dirty (see [Request.Dirty]), which schedules a [Updater.JawsUpdate] and the wakeup
// that delivers it.
func (elem *Element) queue(wht what.What, data string) (queued bool) {
	if !elem.deleted.Load() {
		elem.Request.queue(wire.WsMsg{
			Data: data,
			Jid:  elem.jid,
			What: wht,
		})
		queued = true
	}
	return
}

// SetAttr queues sending a new attribute value
// to the browser for the [Element].
//
// The value parameter must be the unescaped logical attribute value. It is sent
// to the browser DOM and used as the value argument to setAttribute().
//
// Call this while the [Element] is rendering or updating, when a send pass is
// imminent. To change the [Element] in response to a browser event, mark it dirty
// with [Request.Dirty] instead: a change queued directly from an event handler is
// flushed only when the processing loop is next woken, which on an otherwise-idle
// request is not guaranteed to be prompt (see [Element.queue]).
func (elem *Element) SetAttr(attr, value string) {
	elem.queue(what.SAttr, attr+"\n"+value)
}

// RemoveAttr queues sending a request to remove an attribute
// to the browser for the [Element].
//
// Call this while the [Element] is rendering or updating, when a send pass is
// imminent. To change the [Element] in response to a browser event, mark it dirty
// with [Request.Dirty] instead: a change queued directly from an event handler is
// flushed only when the processing loop is next woken, which on an otherwise-idle
// request is not guaranteed to be prompt (see [Element.queue]).
func (elem *Element) RemoveAttr(attr string) {
	elem.queue(what.RAttr, attr)
}

// SetClass queues sending a class
// to the browser for the [Element].
//
// Call this while the [Element] is rendering or updating, when a send pass is
// imminent. To change the [Element] in response to a browser event, mark it dirty
// with [Request.Dirty] instead: a change queued directly from an event handler is
// flushed only when the processing loop is next woken, which on an otherwise-idle
// request is not guaranteed to be prompt (see [Element.queue]).
func (elem *Element) SetClass(cls string) {
	elem.queue(what.SClass, cls)
}

// RemoveClass queues sending a request to remove a class
// to the browser for the [Element].
//
// Call this while the [Element] is rendering or updating, when a send pass is
// imminent. To change the [Element] in response to a browser event, mark it dirty
// with [Request.Dirty] instead: a change queued directly from an event handler is
// flushed only when the processing loop is next woken, which on an otherwise-idle
// request is not guaranteed to be prompt (see [Element.queue]).
func (elem *Element) RemoveClass(cls string) {
	elem.queue(what.RClass, cls)
}

// SetInner queues sending new inner HTML content
// to the browser for the [Element].
//
// Call this while the [Element] is rendering or updating, when a send pass is
// imminent. To change the [Element] in response to a browser event, mark it dirty
// with [Request.Dirty] instead: a change queued directly from an event handler is
// flushed only when the processing loop is next woken, which on an otherwise-idle
// request is not guaranteed to be prompt (see [Element.queue]).
func (elem *Element) SetInner(innerHTML template.HTML) {
	elem.queue(what.Inner, string(innerHTML))
}

// SetValue queues sending a new current input value in textual form
// to the browser for the [Element].
//
// Call this while the [Element] is rendering or updating, when a send pass is
// imminent. To change the [Element] in response to a browser event, mark it dirty
// with [Request.Dirty] instead: a change queued directly from an event handler is
// flushed only when the processing loop is next woken, which on an otherwise-idle
// request is not guaranteed to be prompt (see [Element.queue]).
func (elem *Element) SetValue(value string) {
	elem.queue(what.Value, value)
}

// JsCall queues a browser JavaScript function path call for the [Element].
//
// In the receiving browser, jsfunc is resolved as a path from window and called
// with JSON.parse(jsonstr); the Element is not passed as this or as an argument.
//
// Call this while the [Element] is rendering or updating, when a send pass is
// imminent; a call queued directly from an event handler is only flushed when the
// processing loop is next woken (see [Element.queue]). To call JavaScript for
// every element matching a tag, use [Jaws.JsCall].
func (elem *Element) JsCall(jsfunc, jsonstr string) {
	elem.queue(what.Call, jsCallData(jsfunc, jsonstr))
}

// Replace replaces the [Element]'s entire HTML DOM node with new HTML code.
//
// The trusted HTML should preserve the element identity by putting the element's
// own JaWS id on the replacement root element, normally as id="Jid.N". Replace is
// not an HTML validator: it performs only a lightweight textual guard for that
// expected id attribute. If the guard does not find it, the call is a programming
// error: debug builds panic and production builds report it via [Jaws.MustLog]
// and skip the replacement.
//
// Call this while the [Element] is rendering or updating, when a send pass is
// imminent. To change the [Element] in response to a browser event, mark it dirty
// with [Request.Dirty] instead: a change queued directly from an event handler is
// flushed only when the processing loop is next woken, which on an otherwise-idle
// request is not guaranteed to be prompt (see [Element.queue]).
func (elem *Element) Replace(htmlCode template.HTML) {
	if !elem.deleted.Load() {
		var b []byte
		b = append(b, "id="...)
		b = elem.Jid().AppendQuote(b)
		// string(htmlCode) is a no-op cast (template.HTML is a string), so this
		// avoids copying the whole payload into a fresh []byte just to search it.
		if !strings.Contains(string(htmlCode), string(b)) {
			elem.Jaws.reportMisuse(errors.New("jaws: Element.Replace(): expected HTML " + string(b)))
			return
		}
		elem.queue(what.Replace, string(htmlCode))
	}
}

// Append appends a new HTML element as a child to the current one.
//
// Call this while the [Element] is rendering or updating, when a send pass is
// imminent. To change the [Element] in response to a browser event, mark it dirty
// with [Request.Dirty] instead: a change queued directly from an event handler is
// flushed only when the processing loop is next woken, which on an otherwise-idle
// request is not guaranteed to be prompt (see [Element.queue]).
func (elem *Element) Append(htmlCode template.HTML) {
	elem.queue(what.Append, string(htmlCode))
}

// InsertBefore inserts new HTML immediately before child.
//
// child must be a live, distinct [Element] belonging to the same [Request] as
// elem. Violations are reported as [ErrInvalidChildElement], and no browser
// command is queued. The browser also verifies that child is a direct DOM child
// of elem before applying the insertion.
//
// Call this while elem is rendering or updating, when a send pass is imminent.
// To insert HTML at the same child index in every element matching a tag, use
// [Jaws.Insert].
func (elem *Element) InsertBefore(child *Element, htmlCode template.HTML) {
	if elem.validChildElement("InsertBefore", child) {
		elem.queue(what.Insert, child.Jid().String()+"\n"+string(htmlCode))
	}
}

// Order reorders the HTML elements.
//
// Call this while the [Element] is rendering or updating, when a send pass is
// imminent. To change the [Element] in response to a browser event, mark it dirty
// with [Request.Dirty] instead: a change queued directly from an event handler is
// flushed only when the processing loop is next woken, which on an otherwise-idle
// request is not guaranteed to be prompt (see [Element.queue]).
func (elem *Element) Order(jidList []jid.Jid) {
	if !elem.deleted.Load() && len(jidList) > 0 {
		var b []byte
		for i, jid := range jidList {
			if i > 0 {
				b = append(b, ' ')
			}
			b = jid.Append(b)
		}
		elem.queue(what.Order, string(b))
	}
}

// Remove removes child from the browser and its [Request] registry.
//
// child must be a live, distinct [Element] belonging to the same Request as
// elem. Violations are reported as [ErrInvalidChildElement], and neither the DOM
// nor the registry is changed. The caller is responsible for ensuring child is
// a direct DOM child of elem; the browser verifies that relationship before
// applying the removal.
//
// Call this while the [Element] is rendering or updating, when a send pass is
// imminent. To change the [Element] in response to a browser event, mark it dirty
// with [Request.Dirty] instead: a change queued directly from an event handler is
// flushed only when the processing loop is next woken, which on an otherwise-idle
// request is not guaranteed to be prompt (see [Element.queue]).
func (elem *Element) Remove(child *Element) {
	if elem.validChildElement("Remove", child) {
		if elem.queue(what.Remove, child.Jid().String()) {
			elem.Request.DeleteElement(child)
		}
	}
}

// validChildElement reports whether child can be used by a child DOM operation.
func (elem *Element) validChildElement(operation string, child *Element) (ok bool) {
	if elem.deleted.Load() {
		return false
	}
	var detail string
	switch {
	case child == nil:
		detail = "child is nil"
	case child == elem:
		detail = "child is the parent element"
	case child.Request != elem.Request:
		detail = "child belongs to another Request"
	case child.deleted.Load():
		detail = "child is deleted"
	case elem.Request.GetElementByJid(child.Jid()) != child:
		detail = "child is not registered"
	default:
		return true
	}
	elem.Jaws.reportMisuse(fmt.Errorf("jaws: Element.%s: %w: %s", operation, ErrInvalidChildElement, detail))
	return false
}

// ApplyParams parses the parameters passed to UI() when creating a new [Element],
// adding UI tags, adding any additional event handlers found.
//
// Returns the list of HTML attributes found, if any.
//
// Handlers found in params are added only while the [Element] is mutable; after
// it is frozen ([Element.JawsRender] returning or [Element.Freeze]) they are
// dropped (debug builds panic), though tags and HTML attributes are still
// processed.
func (elem *Element) ApplyParams(params []any) (attrs []template.HTMLAttr) {
	tags, handlers, rawAttrs := ParseParams(params)
	if !elem.deleted.Load() {
		elem.appendHandlers(handlers...)
		elem.Tag(tags...)
		for _, s := range rawAttrs {
			attr := template.HTMLAttr(s) // #nosec G203
			attrs = append(attrs, attr)
		}
	}
	return
}

// ApplyGetter examines getter and resolves its tag candidate.
//
// If getter implements [tag.TagGetter], the candidate is its returned value;
// otherwise the candidate is getter itself. TagGetter values, supported tag
// slices and runtime-comparable candidates are passed to [Element.Tag] for normal
// validation. Other non-comparable candidates are not automatically tagged,
// matching [ParseParams].
//
// If getter is an [InputHandler], [ClickHandler], [ContextMenuHandler] or
// [InitialHTMLAttrHandler], relevant values are added to the [Element].
//
// Finally, if getter is an [InitHandler], its JawsInit
// function is called.
//
// Returns the tag that was added (nil if none was added, whether because getter
// was nil or its candidate was not usable as a tag), any initial HTML attrs
// provided by InitialHTMLAttrHandler, and any error returned from JawsInit() if it
// was called.
//
// If the [Element] is already frozen and getter is an event handler, the handler
// is not added: in production with a [Jaws.Logger] configured this is logged and
// tag and init processing still occur, while debug builds and servers without a
// Logger panic via reportMisuse, aborting before tag and init processing. A
// non-event-handler getter never calls reportMisuse, so its tag and init
// processing always occur.
func (elem *Element) ApplyGetter(getter any) (tagValue any, attrs []template.HTMLAttr, err error) {
	if getter != nil {
		tagValue = getter
		if tagger, ok := getter.(tag.TagGetter); ok {
			tagValue = tagger.JawsGetTag(elem.Request)
		}
		if _, ok := getter.(InputHandler); ok {
			elem.appendHandlers(getter)
		} else if _, ok := getter.(ClickHandler); ok {
			elem.appendHandlers(getter)
		} else if _, ok := getter.(ContextMenuHandler); ok {
			elem.appendHandlers(getter)
		}
		if ah, ok := getter.(InitialHTMLAttrHandler); ok {
			if attr := ah.JawsInitialHTMLAttr(elem); attr != "" {
				attrs = append(attrs, attr)
			}
		}
		if usableAsTag(tagValue) {
			elem.Tag(tagValue)
		} else {
			tagValue = nil
		}
		if initer, ok := getter.(InitHandler); ok {
			err = initer.JawsInit(elem)
		}
	}
	return
}
