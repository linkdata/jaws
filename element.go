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
// An Element pointer supplied to a render, update or event handler is borrowed
// for that call. A request-scoped widget may retain child Elements it creates
// between its render and update calls within the same Request lifecycle, but
// should access them only from those calls. Do not retain an Element in
// longer-lived application state or pass it to background work: once the embedded
// Request finishes it is unregistered, so the Element receives no further broadcasts
// or updates, though its fields are left intact and its methods still operate on the
// now finished Request. Request identities are never reused, so a retained Element
// can never come to represent an unrelated connection.
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
	// data is the widget state slot, claimed by the widget rendering this Element and
	// reached through [ElementState] and [SetElementState]. It is guarded by
	// Request.mu; the stored value's own synchronization guards its contents.
	data    any
	jid     jid.Jid     // JaWS ID, unique to this Element within its Request
	deleted atomic.Bool // true once the Element has been removed from its Request
	frozen  atomic.Bool // set when handlers are sealed (JawsRender returns or Freeze called); guards handler mutators in all builds
}

// String returns a debug representation of elem: its UI type, Jid, and tags.
//
// The tags render through [tag.TagsString], so in the default build they show
// their types (and a pointer's address when readable; see [tag.TagStringRelease]),
// while debug and -race builds render them in full — more informative, but able
// to crash on a self-referential or oversized tag. String tolerates an Element
// whose Request is not yet set.
func (elem *Element) String() string {
	// Guard elem.Request like Request.String()/JawsKeyString guard a nil
	// receiver, so String() stays safe on a not-fully-constructed Element.
	var tags []any
	if elem.Request != nil {
		tags = elem.Request.TagsOf(elem)
	}
	return fmt.Sprintf("Element{%T, id=%q, Tags: %s}", elem.UI(), elem.Jid(), tag.TagsString(tags))
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
// be processed for it. Handlers added after [Element.JawsRender] has returned
// (or [Element.Freeze] has been called) are dropped; debug builds panic.
//
// Input callback functions used directly by signature are recognized according
// to the dynamic-type rules documented by [InputFn].
func (elem *Element) AddHandlers(h ...any) {
	elem.appendHandlers(h...)
}

// Tag associates elem with tags.
//
// It is shorthand for [Request.Tag]. A deleted Element ignores the call without
// expanding tags.
func (elem *Element) Tag(tags ...any) {
	if !elem.deleted.Load() {
		elem.Request.Tag(elem, tags...)
	}
}

// HasTag reports whether this Element has tagValue.
//
// It reports false for a deleted Element. This is the advanced exact-key lookup
// described by [Request.HasTag]: tagValue is not expanded or validated, and an
// invalid value may panic.
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
// [Element.JawsRender], [Element.JawsUpdate] and the queue helpers are no-ops on
// a deleted Element. A request-scoped widget that retains child Elements it
// creates between render and update calls within one Request lifecycle can use
// Deleted to detect and discard children removed out-of-band before reuse.
//
// An event accepted while the Element is live may still invoke its handler after
// the Element is later removed from the Request.
//
// Deleted is not a lifetime check: it does not report whether the embedded
// Request still represents the owning connection or make that Request safe to
// use after its lifecycle.
func (elem *Element) Deleted() bool {
	return elem.deleted.Load()
}

func (elem *Element) renderDebug(w io.Writer) (err error) {
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "<!-- id=%q %T tags=[", elem.Jid(), elem.UI())
	if tags, ok := elem.tryTagsOf(elem); ok {
		for i, tagValue := range tags {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(tag.TagString(tagValue))
		}
	} else {
		sb.WriteString("n/a")
	}
	sb.WriteByte(']')
	_, err = w.Write([]byte(debugCommentSanitizer.Replace(sb.String()) + " -->"))
	return
}

// debugCommentSanitizer neutralizes both the standard "-->" and the HTML5
// "--!>" comment-close sequences so tag text cannot escape the debug comment.
var debugCommentSanitizer = strings.NewReplacer("-->", "==>", "--!>", "==>")

// JawsRender calls [Renderer.JawsRender] for this [Element].
//
// Do not call this yourself unless it is from within another JawsRender implementation.
//
// A nil [UI] interface renders as a no-op; this arises only from [Request.NewElement]
// given a nil interface. A typed nil (a non-nil interface holding a nil pointer) is
// still dispatched to its [Renderer], so the call panics unless that concrete type
// documents nil-receiver tolerance; see [UI].
func (elem *Element) JawsRender(w io.Writer, params []any) (err error) {
	if ui := elem.UI(); ui != nil && !elem.deleted.Load() {
		if err = ui.JawsRender(elem, w, params); err == nil {
			if elem.Jaws.Debug {
				err = elem.renderDebug(w)
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
//
// A nil [UI] interface is a no-op; a typed nil dispatches to its [Updater] (see
// [Element.JawsRender]).
func (elem *Element) JawsUpdate() {
	if ui := elem.UI(); ui != nil && !elem.deleted.Load() {
		ui.JawsUpdate(elem)
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

// isReservedAttr reports whether attr names a framework-owned attribute that must
// not be set or removed through the public helpers. The match is ASCII
// case-insensitive because browsers lower-case HTML attribute names in setAttribute.
func isReservedAttr(attr string) bool {
	return strings.EqualFold(attr, "id")
}

// SetAttr queues sending a new attribute value
// to the browser for the [Element].
//
// The value parameter must be the unescaped logical attribute value. It is sent
// to the browser DOM and used as the value argument to setAttribute().
//
// The framework-owned "id" attribute is rejected (ASCII case-insensitively):
// attempting to set it is reported as [ErrReservedAttribute] via reportMisuse and
// nothing is sent, since it carries the [Element]'s JaWS identity.
//
// Call this while the [Element] is rendering or updating, when a send pass is
// imminent. To change the [Element] in response to a browser event, mark it dirty
// with [Request.Dirty] instead: a change queued directly from an event handler is
// flushed only when the processing loop is next woken, which on an otherwise-idle
// request is not guaranteed to be prompt (see [Element.queue]).
func (elem *Element) SetAttr(attr, value string) {
	if isReservedAttr(attr) {
		elem.Jaws.reportMisuse(fmt.Errorf("jaws: Element.SetAttr: %q: %w", attr, ErrReservedAttribute))
		return
	}
	elem.queue(what.SAttr, attr+"\n"+value)
}

// RemoveAttr queues sending a request to remove an attribute
// to the browser for the [Element].
//
// The framework-owned "id" attribute is rejected (ASCII case-insensitively):
// attempting to remove it is reported as [ErrReservedAttribute] via reportMisuse and
// nothing is sent, since it carries the [Element]'s JaWS identity.
//
// Call this while the [Element] is rendering or updating, when a send pass is
// imminent. To change the [Element] in response to a browser event, mark it dirty
// with [Request.Dirty] instead: a change queued directly from an event handler is
// flushed only when the processing loop is next woken, which on an otherwise-idle
// request is not guaranteed to be prompt (see [Element.queue]).
func (elem *Element) RemoveAttr(attr string) {
	if isReservedAttr(attr) {
		elem.Jaws.reportMisuse(fmt.Errorf("jaws: Element.RemoveAttr: %q: %w", attr, ErrReservedAttribute))
		return
	}
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

// SetInner queues new inner HTML content for the [Element].
//
// When innerHTML exactly matches the browser's current serialized inner HTML,
// JaWS leaves the existing descendants and their live state unchanged. Use
// [Element.Replace] when matching markup must still create new nodes.
//
// Call this while the [Element] is rendering or updating, when a send pass is
// imminent. To change the [Element] in response to a browser event, mark it dirty
// with [Request.Dirty] instead: a change queued directly from an event handler is
// flushed only when the processing loop is next woken, which on an otherwise-idle
// request is not guaranteed to be prompt.
func (elem *Element) SetInner(innerHTML template.HTML) {
	elem.queue(what.Inner, string(innerHTML))
}

// SetValue queues sending a new current input value in textual form
// to the browser for the [Element].
//
// Call this while the [Element] is rendering or updating, when a send pass is
// imminent. To reconcile only this Element after a browser event, call
// elem.Dirty(elem); this schedules JawsUpdate on the Request loop and serializes the
// correction with other updates. Dirty a source tag instead when shared application
// state changed. Calling SetValue directly from an event handler may not flush
// promptly and bypasses that update ordering.
func (elem *Element) SetValue(value string) {
	elem.queue(what.Value, value)
}

// JsCall queues a browser JavaScript function path call for the [Element].
//
// In the receiving browser, jsfunc is resolved as a path from window and called
// with JSON.parse(jsonstr); the Element is not passed as this or as an argument.
// jsfunc must be an application-controlled dot path. The browser rejects an
// exact "__proto__" component; put user data in jsonstr, not jsfunc.
//
// Call this while the [Element] is rendering or updating, when a send pass is
// imminent; a call queued directly from an event handler is only flushed when the
// processing loop is next woken. To call JavaScript for every element matching a
// tag, use [Jaws.JsCall].
func (elem *Element) JsCall(jsfunc, jsonstr string) {
	elem.queue(what.Call, jsCallData(jsfunc, jsonstr))
}

// Replace replaces the [Element]'s entire HTML DOM node with new HTML code.
//
// A valid call recreates the target subtree even when htmlCode matches its
// current serialization. Browser-only state on replaced nodes, including focus,
// selection, scroll position, live form-control properties, programmatic
// listeners, expando properties, and custom-element instances, is not preserved.
// JaWS reattaches its own managed browser behavior.
//
// A replacement node bearing an existing JaWS ID keeps that server-side
// [Element] registration and UI state; omitted descendant IDs are unregistered.
// Retained Elements are not rerendered separately, so their markup must match that
// state and each reused ID must identify the same logical Element.
//
// The trusted HTML should preserve the element identity by putting the element's
// own JaWS ID on the replacement root element, normally as id="Jid.N". Replace is
// not an HTML validator: it performs only a lightweight textual guard for that
// expected id attribute. If the guard does not find it, the call is a programming
// error: debug builds panic and production builds report it via [Jaws.MustLog]
// and skip the replacement.
//
// Call this while the [Element] is rendering or updating, when a send pass is
// imminent. To change the [Element] in response to a browser event, mark it dirty
// with [Request.Dirty] instead: a change queued directly from an event handler is
// flushed only when the processing loop is next woken, which on an otherwise-idle
// request is not guaranteed to be prompt.
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

// ApplyParams applies UI-helper parameters to elem.
//
// For a live Element, it registers tags and event handlers and returns any HTML
// attributes found by [ParseParams]. A deleted Element applies nothing and returns nil.
//
// On a live, frozen Element, handler params are queued for logging and dropped
// in production when [Jaws.Logger] is configured, after which tags and attributes
// are processed. Debug builds and servers without a Logger panic first. Params
// without handlers continue normally.
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

// ApplyGetter applies getter's tag and event-handler interfaces to elem.
//
// If getter implements [tag.TagGetter], the candidate is the value returned by
// [tag.TagGetter.JawsGetTag]; otherwise the candidate is getter itself. Eligible
// candidates — TagGetter values, supported tag slices and runtime-comparable
// values — are passed to [Element.Tag] for normal expansion and validation.
// That expansion may invoke JawsGetTag again when the candidate is itself a
// TagGetter or a []any containing one. Other non-comparable candidates are not
// automatically tagged.
//
// If getter implements [InputHandler], [ClickHandler], or [ContextMenuHandler], it is
// added as an event handler. ApplyGetter does not invoke [InitialHTMLAttrHandler];
// call [Element.ApplyInitialHTMLAttr] separately.
//
// The returned tagValue does not confirm registration. It is nil if getter or its
// candidate is a nil interface, or if the candidate is ineligible for expansion. A
// successfully expandable tagValue may be retained for later dirtying only while it
// expands to the same keys. Retained slices must not be mutated concurrently with
// expansion; candidates derived from a [tag.TagGetter] rely on its stable-identity
// contract.
//
// If the [Element] is already frozen and getter is an event handler, the handler
// is not added: in production with a [Jaws.Logger] configured this is queued for
// logging and tag processing still occurs, while debug builds and servers without a
// Logger panic before that processing. For a non-event-handler getter, tag processing
// still occurs after freezing.
func (elem *Element) ApplyGetter(getter any) (tagValue any) {
	if getter != nil {
		tagValue = getter
		if tagger, ok := getter.(tag.TagGetter); ok {
			tagValue = tagger.JawsGetTag()
		}
		if _, ok := getter.(InputHandler); ok {
			elem.appendHandlers(getter)
		} else if _, ok := getter.(ClickHandler); ok {
			elem.appendHandlers(getter)
		} else if _, ok := getter.(ContextMenuHandler); ok {
			elem.appendHandlers(getter)
		}
		if eligibleAsTag(tagValue, tag.NewErrNotComparable) {
			elem.Tag(tagValue)
		} else {
			tagValue = nil
		}
	}
	return
}

// ApplyInitialHTMLAttr returns getter's initial HTML attributes.
//
// It returns nil unless getter implements [InitialHTMLAttrHandler] and returns a
// non-empty value; otherwise that value is the sole slice element. It does not apply
// tags or event handlers.
//
// Callers must not hold a lock protecting getter or its source.
func (elem *Element) ApplyInitialHTMLAttr(getter any) (attrs []template.HTMLAttr) {
	if ah, ok := getter.(InitialHTMLAttrHandler); ok {
		if attr := ah.JawsInitialHTMLAttr(elem); attr != "" {
			attrs = append(attrs, attr)
		}
	}
	return
}

// ErrElementStateClaimed is returned by [SetElementState] when the [Element] already
// has widget state, including state of the same type.
var ErrElementStateClaimed = errors.New("jaws: element state already claimed")

// ErrElementStateNil is returned by [SetElementState] when the state to store is a nil
// interface, which cannot be distinguished from an unclaimed slot.
var ErrElementStateNil = errors.New("jaws: element state must not be nil")

// ElementState returns the widget state stored for elem, or nil if none was claimed.
//
// The state belongs to the [Element], not to the widget value that claimed it.
// ElementState therefore does not identify its caller, but that storage detail does not
// permit a different UI widget to update the Element; see [Updater]. Loading never
// claims: a widget that finds no state did not claim this Element. Most widgets never
// claim one, so a nil return says nothing about whether the Element was rendered.
//
// Safe for concurrent use; it takes the [Request] lock. Only the slot itself is
// synchronized: whatever the stored value contains is guarded by that value's own
// synchronization, not by this call.
//
// elem must be obtained from [Request.NewElement]. ElementState does not verify
// that provenance.
func ElementState(elem *Element) (state any) {
	rq := elem.Request
	rq.mu.RLock()
	state = elem.data
	rq.mu.RUnlock()
	return
}

// SetElementState claims elem's widget state slot, which a widget does while rendering
// the Element so its updates and cleanup can find that state again.
//
// There is one slot per Element and it cannot be replaced, only claimed: a second claim
// returns [ErrElementStateClaimed] and leaves the stored state untouched, even when the
// new state has the same type. At most one widget may claim a given Element, so a widget
// that renders an Element and delegates to another renderer on that same Element must
// decide which of them claims it.
//
// A nil state returns [ErrElementStateNil] and stores nothing, since a nil interface is
// how an unclaimed slot is represented; that check comes first, so a nil state is
// rejected whatever the slot holds, and before elem is examined at all. A typed nil is a
// non-nil interface and does claim the slot.
//
// Safe for concurrent use: concurrent claims on one Element are serialized by the
// [Request] lock and exactly one wins, the rest reporting [ErrElementStateClaimed]. Only
// the claim is synchronized; mutating the stored value afterwards is guarded by that
// value's own synchronization, not by this call.
//
// When state is non-nil, elem must be obtained from [Request.NewElement].
// SetElementState does not verify that provenance.
func SetElementState(elem *Element, state any) error {
	// The two functions are package-level rather than methods on Element because
	// ui.With embeds both *Element and ui.RequestWriter, so any method returning a value
	// is reachable from a template: {{$.Element.SetElementState $.Dot}} would let a
	// template claim the slot out from under the renderer. html/template cannot call a
	// package-level function.
	if state == nil {
		return ErrElementStateNil
	}
	rq := elem.Request
	rq.mu.Lock()
	defer rq.mu.Unlock()
	if elem.data != nil {
		return ErrElementStateClaimed
	}
	elem.data = state
	return nil
}
