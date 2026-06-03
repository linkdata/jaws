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
type Element struct {
	*Request // (read-only) the Request the Element belongs to
	// internals
	ui UI // the UI object
	// handlers is appended to only during initial render/init (AddHandlers,
	// ApplyParams, ApplyGetter, all routed through appendHandlers) and read later
	// on the event goroutine without a lock. This is safe solely because rendering
	// an Element fully populates its handlers before any event for it can be
	// processed; handlers must not be mutated once events may fire. All builds
	// enforce this: once the frozen flag below is set, appendHandlers drops late
	// mutations (debug builds panic).
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
// handlers is read lock-free on the event goroutine (see callEventHandlers), so
// it must only be appended to while the Element is being rendered, before any
// event for it can fire. Once frozen, late mutations are a bug: reportMisuse
// panics in debug builds and logs in production, and the mutation is dropped
// rather than racing the lock-free read.
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
// ApplyGetter) no longer add handlers (debug builds panic). Use this for elements
// registered for updates without being rendered.
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
	_, _ = w.Write([]byte(strings.ReplaceAll(sb.String(), "-->", "==>") + " -->"))
}

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
// been deleted. Call only during JawsRender or JawsUpdate processing.
func (elem *Element) queue(wht what.What, data string) {
	if !elem.deleted.Load() {
		elem.Request.queue(wire.WsMsg{
			Data: data,
			Jid:  elem.jid,
			What: wht,
		})
	}
}

// SetAttr queues sending a new attribute value
// to the browser for the [Element].
//
// The value parameter must be the unescaped logical attribute value. It is sent
// to the browser DOM and used as the value argument to setAttribute().
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (elem *Element) SetAttr(attr, value string) {
	elem.queue(what.SAttr, attr+"\n"+value)
}

// RemoveAttr queues sending a request to remove an attribute
// to the browser for the [Element].
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (elem *Element) RemoveAttr(attr string) {
	elem.queue(what.RAttr, attr)
}

// SetClass queues sending a class
// to the browser for the [Element].
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (elem *Element) SetClass(cls string) {
	elem.queue(what.SClass, cls)
}

// RemoveClass queues sending a request to remove a class
// to the browser for the [Element].
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (elem *Element) RemoveClass(cls string) {
	elem.queue(what.RClass, cls)
}

// SetInner queues sending new inner HTML content
// to the browser for the [Element].
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (elem *Element) SetInner(innerHTML template.HTML) {
	elem.queue(what.Inner, string(innerHTML))
}

// SetValue queues sending a new current input value in textual form
// to the browser for the [Element].
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (elem *Element) SetValue(value string) {
	elem.queue(what.Value, value)
}

// Replace replaces the [Element]'s entire HTML DOM node with new HTML code.
//
// The HTML code must contain the element's own HTML id; if it does not, the call
// is a programming error: debug builds panic and production builds report it via
// [Jaws.MustLog] and skip the replacement.
//
// Call this only during JawsRender() or JawsUpdate() processing.
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
// Call this only during JawsRender() or JawsUpdate() processing.
func (elem *Element) Append(htmlCode template.HTML) {
	elem.queue(what.Append, string(htmlCode))
}

// Order reorders the HTML elements.
//
// Call this only during JawsRender() or JawsUpdate() processing.
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

// Remove requests that the HTML child with the given HTML ID of this [Element]
// is removed from the [Request] and its HTML element from the browser.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (elem *Element) Remove(htmlID string) {
	elem.queue(what.Remove, htmlID)
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

// ApplyGetter examines getter, and if it is not nil, either adds it
// as a tag, or, if it is a [tag.TagGetter], adds the result of that as a tag.
//
// If getter is an [InputHandler], [ClickHandler], [ContextMenuHandler] or
// [InitialHTMLAttrHandler], relevant values are added to the [Element].
//
// Finally, if getter is an [InitHandler], its JawsInit
// function is called.
//
// Returns the Tag(s) added (or nil if getter was nil), any initial HTML attrs
// provided by InitialHTMLAttrHandler, and any error returned from JawsInit()
// if it was called.
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
		elem.Tag(tagValue)
		if initer, ok := getter.(InitHandler); ok {
			err = initer.JawsInit(elem)
		}
	}
	return
}
