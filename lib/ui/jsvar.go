package ui

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/htmlio"
	"github.com/linkdata/jq"
)

const (
	maxJsVarPathBytes = 4096
	maxJsVarNameBytes = 4096
)

var jsVarNameRx = regexp.MustCompile("^[A-Za-z_$][A-Za-z0-9_$]*$")

func validateJsVarName(name string) error {
	if len(name) > maxJsVarNameBytes {
		return errIllegalJsVarName("too long")
	}
	if !jsVarNameRx.MatchString(name) {
		return errIllegalJsVarName("illegal syntax")
	}
	if name == "__proto__" || name == "constructor" || name == "prototype" {
		return errIllegalJsVarName("reserved")
	}
	return nil
}

func validateJsVarPath(path string) error {
	if len(path) > maxJsVarPathBytes || !utf8.ValidString(path) {
		return ErrIllegalJsVarPath
	}
	for i := range len(path) {
		if path[i] < 0x20 || path[i] == 0x7f || path[i] == '=' {
			return ErrIllegalJsVarPath
		}
	}
	if path == "" {
		return nil
	}
	for component := range strings.SplitSeq(path, ".") {
		switch component {
		case "", "__proto__", "constructor", "prototype":
			return ErrIllegalJsVarPath
		}
	}
	return nil
}

// JsVarCheck validates the complete tentative state of a browser proposal.
//
// A nil check denies browser writes. The check runs under the store's write lock
// after jq tentatively applies a changed value. It must only inspect next: jq
// rolls the proposal back if the check returns an error or panics. The source
// may be used to authorize a user or session; every binding sees the same value.
type JsVarCheck[T any] func(source *jaws.Element, next *T, path string) error

// JSONSizeCheck limits the encoded size of a tentative JsVar store value.
//
// A non-positive limit returns nil, which denies browser writes if used as the
// store's sole ClientCheck. The check bounds JSON bytes, not Go heap capacity.
func JSONSizeCheck[T any](maxBytes int) (check JsVarCheck[T]) {
	if maxBytes > 0 {
		check = func(_ *jaws.Element, value *T, _ string) (err error) {
			var data []byte
			if data, err = json.Marshal(value); err == nil {
				if len(data) > maxBytes {
					err = fmt.Errorf("%w: serialized size %d exceeds maximum %d", ErrJsVarTooLarge, len(data), maxBytes)
				}
			} else {
				err = fmt.Errorf("%w: cannot serialize value: %w", ErrJsVarTooLarge, err)
			}
			return
		}
	}
	return
}

// JsVarStore owns one authoritative Go value and its browser name.
//
// Use [JsVarStore.Bind] for each rendered binding. The store never retains a
// Request or Element. ClientCheck and ExtraTags are configured before first use
// and must not be changed while the store is active. Every binding sees the same
// JSON value; ClientCheck controls writes, not disclosure.
//
// All reads and writes of the bound value must use the supplied locker. Server
// mutations must use SetPath, DeletePath, or WriteLocked to publish changes.
// For partial map patches, the bound JSON tree must not share mutable pointers,
// maps, or slices across different paths: changing one alias can change another
// browser path. Complex Go types fall back to a root patch. Custom JSON methods
// reached while locked must not re-enter the store or its locker.
//
// A JsVarStore must not be copied after first use.
type JsVarStore[T any] struct {
	ClientCheck JsVarCheck[T] // nil denies browser proposals
	ExtraTags   []any         // dirtied after each changed mutation

	jaws    *jaws.Jaws
	name    string
	locker  bind.RWLocker
	value   *T
	partial bool
}

// NewJsVarStore creates a store with an application-owned browser name.
//
// The name is one JavaScript identifier using ASCII letters, digits,
// underscore, or dollar sign; it cannot start with a digit or equal
// "__proto__", "constructor", or "prototype", and is at most 4096 bytes.
//
// The value and locker must remain valid for the store's lifetime. An invalid
// name returns [ErrIllegalJsVarName], and a nil value returns
// [github.com/linkdata/jq.ErrInvalidReceiver].
func NewJsVarStore[T any](jw *jaws.Jaws, name string, locker sync.Locker, value *T) (store *JsVarStore[T], err error) {
	if err = validateJsVarName(name); err == nil {
		if value == nil {
			err = jq.ErrInvalidReceiver
		} else {
			store = &JsVarStore[T]{
				jaws:    jw,
				name:    name,
				locker:  bind.AsRWLocker(locker),
				value:   value,
				partial: plainJsVarType(reflect.TypeOf(value).Elem()),
			}
		}
	}
	return
}

// Bind creates a one-use browser binding for this store.
//
// Render each binding once through [RequestWriter.NewUI]. A binding can be
// deactivated during a connection redirect to suppress provisional traffic.
func (store *JsVarStore[T]) Bind() *JsVarBinding[T] {
	return &JsVarBinding[T]{store: store}
}

// ReadLocked inspects the authoritative value while its read lock is held.
//
// The callback must not retain value or a mutable value reachable from it, and
// must not call a lock-taking store method. The lock is released after a panic.
func (store *JsVarStore[T]) ReadLocked(fn func(value *T)) {
	store.locker.RLock()
	defer store.locker.RUnlock()
	fn(store.value)
}

// WriteLocked groups server path writes under one application lock.
//
// The supplied path functions are valid only during fn. They must not escape or
// call lock-taking store methods. Values returned by get are borrowed and must
// not be retained or mutated directly. Each successful changed write remains applied
// and is published after unlocking, even if a later write fails or fn panics.
// WriteLocked is a lock scope, not a rollback transaction.
func (store *JsVarStore[T]) WriteLocked(fn func(get func(string) (any, error), set func(string, any) (bool, error), deletePath func(string) (bool, error)) error) (err error) {
	var changedPaths []string
	store.locker.Lock()
	defer func() {
		store.locker.Unlock()
		store.publish(changedPaths)
	}()
	get := func(path string) (value any, err error) {
		if err = validateJsVarPath(path); err == nil {
			value, err = jq.Get(store.value, path)
		}
		return
	}
	set := func(path string, value any) (changed bool, err error) {
		if err = validateJsVarPath(path); err == nil {
			if changed, err = jq.Set(store.value, path, value); err == nil && changed {
				changedPaths = append(changedPaths, path)
			}
		}
		return
	}
	deletePath := func(path string) (changed bool, err error) {
		if err = validateJsVarPath(path); err == nil {
			if changed, err = store.deletePathLocked(path); err == nil && changed {
				changedPaths = append(changedPaths, path)
			}
		}
		return
	}
	err = fn(get, set, deletePath)
	return
}

// SetPath sets a Go value at a canonical path and publishes a changed result.
//
// Paths are dot-separated JSON names, with no empty or prototype-sensitive
// component, control byte, DEL, or '='; the maximum is 4096 UTF-8 bytes.
// The empty path replaces the root. A no-op returns changed=false. Server writes
// do not invoke ClientCheck. The resulting value must remain JSON encodable.
func (store *JsVarStore[T]) SetPath(path string, value any) (changed bool, err error) {
	err = store.WriteLocked(func(_ func(string) (any, error), set func(string, any) (bool, error), _ func(string) (bool, error)) error {
		changed, err = set(path, value)
		return err
	})
	return
}

// DeletePath removes a map entry by JSON path.
//
// The parent must be a map with built-in string keys. A missing key in that
// map returns changed=false; a missing or non-map parent returns
// [github.com/linkdata/jq.ErrPathNotFound]. JSON null is a value; deletion is a
// separate operation. The empty root path is invalid.
func (store *JsVarStore[T]) DeletePath(path string) (changed bool, err error) {
	err = store.WriteLocked(func(_ func(string) (any, error), _ func(string, any) (bool, error), deletePath func(string) (bool, error)) error {
		changed, err = deletePath(path)
		return err
	})
	return
}

func (store *JsVarStore[T]) deletePathLocked(path string) (changed bool, err error) {
	if path == "" {
		return false, ErrIllegalJsVarPath
	}
	parentPath, key := "", path
	if dot := strings.LastIndexByte(path, '.'); dot >= 0 {
		parentPath, key = path[:dot], path[dot+1:]
	}
	var parent any = store.value
	if parentPath != "" {
		if parent, err = jq.Get(store.value, parentPath); err != nil {
			return
		}
	}
	rv := reflect.ValueOf(parent)
	for rv.IsValid() && (rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface) {
		if rv.IsNil() {
			return false, jq.ErrPathNotFound
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() || rv.Kind() != reflect.Map || rv.Type().Key() != reflect.TypeFor[string]() {
		return false, jq.ErrPathNotFound
	}
	if rv.IsNil() {
		return false, nil
	}
	mapKey := reflect.ValueOf(key)
	if rv.MapIndex(mapKey).IsValid() {
		rv.SetMapIndex(mapKey, reflect.Value{})
		changed = true
	}
	return
}

func (store *JsVarStore[T]) publish(paths []string) {
	for _, path := range paths {
		store.jaws.DirtyPath(store, path)
	}
	if len(paths) > 0 && len(store.ExtraTags) > 0 {
		// A pending page may register a dependency tag after this dirty pass.
		store.jaws.Dirty(store.ExtraTags...)
	}
}

type jsVarNameTag struct{ name string }

type jsVarBindingRoute interface {
	jsVarStore() any
	Deactivate()
}

// JsVarBinding renders one browser route to a [JsVarStore].
//
// Bindings are one-use and request-scoped. Deactivate makes a provisional route
// inert before a redirect; it is safe to call concurrently with input dispatch.
type JsVarBinding[T any] struct {
	store    *JsVarStore[T]
	rendered atomic.Bool
	active   atomic.Bool
}

func (binding *JsVarBinding[T]) jsVarStore() any { return binding.store }

// Deactivate prevents this binding from handling proposals or sending patches.
func (binding *JsVarBinding[T]) Deactivate() { binding.active.Store(false) }

// JawsRender writes the hidden browser route and its initial JSON value.
func (binding *JsVarBinding[T]) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	if !binding.rendered.CompareAndSwap(false, true) {
		return ErrJsVarBindingUsed
	}
	store := binding.store
	data, previous, err := binding.renderSnapshot(elem)
	if err != nil {
		return
	}
	b := []byte("\n<div id=")
	b = elem.Jid().AppendQuote(b)
	b = htmlio.AppendAttr(b, "data-jawsstore", store.name)
	b = htmlio.AppendAttr(b, "data-jawsdata", string(data))
	b = htmlio.AppendAttrs(b, elem.ApplyParams(params))
	b = append(b, " hidden></div>"...)
	_, err = w.Write(b)
	if err == nil {
		binding.activate(previous)
		if len(previous) > 0 {
			store.jaws.DirtyPath(elem, "")
		}
	}
	return
}

func (binding *JsVarBinding[T]) renderSnapshot(elem *jaws.Element) (data []byte, previous []*jaws.Element, err error) {
	store := binding.store
	store.locker.Lock()
	defer store.locker.Unlock()
	// Registration and snapshot share the store lock. A mutation sees either
	// the rendered value or the registered tag, including before WS connection.
	elem.Tag(jsVarNameTag{store.name}, store)
	for _, other := range elem.Request.GetElements(jsVarNameTag{store.name}) {
		if other == elem {
			continue
		}
		route, ok := other.UI().(jsVarBindingRoute)
		if !ok || route.jsVarStore() != store {
			err = ErrJsVarNameConflict
			break
		}
		previous = append(previous, other)
	}
	if err == nil {
		data, err = json.Marshal(store.value)
	}
	return
}

func (binding *JsVarBinding[T]) activate(previous []*jaws.Element) {
	store := binding.store
	store.locker.Lock()
	defer store.locker.Unlock()
	for _, other := range previous {
		if route, ok := other.UI().(jsVarBindingRoute); ok {
			route.Deactivate()
		}
	}
	binding.active.Store(true)
}

// JawsUpdate does not change a browser route without a path invalidation.
func (binding *JsVarBinding[T]) JawsUpdate(*jaws.Element) {}

// JawsUpdatePaths sends current canonical state for invalidated paths.
//
// Complex JSON shapes use a root patch. Plain value trees use partial patches
// extracted from one full root encoding, preserving JSON tags and exact numbers.
// A failed encoding cancels the Request.
func (binding *JsVarBinding[T]) JawsUpdatePaths(elem *jaws.Element, paths []string) {
	if !binding.active.Load() {
		return
	}
	patches, err := binding.snapshotPatches(paths)
	if err != nil {
		elem.Request.Cancel(fmt.Errorf("jsvar: encode store %q: %w", binding.store.name, err))
		return
	}
	if binding.active.Load() {
		for _, patch := range patches {
			elem.Patch(patch)
		}
	}
}

func (binding *JsVarBinding[T]) snapshotPatches(paths []string) (patches []string, err error) {
	store := binding.store
	store.locker.RLock()
	defer store.locker.RUnlock()
	var data []byte
	data, err = json.Marshal(store.value)
	if err == nil {
		rootPatch := "=" + string(data)
		for _, path := range paths {
			patch := store.projectPatch(path, data)
			if patch == rootPatch {
				patches = []string{rootPatch}
				break
			}
			patches = append(patches, patch)
		}
	}
	return
}

func (store *JsVarStore[T]) projectPatch(path string, data []byte) string {
	if path == "" || !store.partial {
		return "=" + string(data)
	}
	current := json.RawMessage(data)
	var prefix string
	for component := range strings.SplitSeq(path, ".") {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(current, &object); err != nil || object == nil {
			if prefix == "" {
				return "=" + string(data)
			}
			return prefix + "=" + string(current)
		}
		if prefix != "" {
			prefix += "."
		}
		prefix += component
		next, found := object[component]
		if !found {
			return prefix + "="
		}
		current = next
	}
	return path + "=" + string(current)
}

// JawsInput applies a browser proposal after checking the whole tentative value.
//
// A rejected, invalid, or unchanged proposal schedules a canonical correction
// for its source binding. A changed accepted proposal invalidates every binding.
// A panicking check rolls back and schedules a root correction before the panic
// continues. [ErrJsVarTooLarge] cancels the source Request for reload recovery.
func (binding *JsVarBinding[T]) JawsInput(elem *jaws.Element, input string) (err error) {
	if !binding.active.Load() {
		return nil
	}
	defer func() {
		if panicValue := recover(); panicValue != nil {
			binding.store.jaws.DirtyPath(elem, "")
			panic(panicValue)
		}
	}()
	path, raw, found := strings.Cut(input, "=")
	if !found {
		err = ErrIllegalJsVarPath
	} else {
		err = validateJsVarPath(path)
	}
	if err != nil {
		binding.store.jaws.DirtyPath(elem, "")
		return errJsVarClientWrite{err}
	}
	var value any
	if err = json.Unmarshal([]byte(raw), &value); err != nil {
		binding.store.jaws.DirtyPath(elem, "")
		return errJsVarClientWrite{err}
	}
	changed, err := binding.applyProposal(elem, path, value)
	if changed {
		binding.store.publish([]string{path})
	} else {
		binding.store.jaws.DirtyPath(elem, path)
	}
	if err != nil {
		if errors.Is(err, ErrJsVarTooLarge) {
			elem.Request.Cancel(err)
			return ErrJsVarTooLarge
		}
		return errJsVarClientWrite{err}
	}
	return
}

func (binding *JsVarBinding[T]) applyProposal(elem *jaws.Element, path string, value any) (changed bool, err error) {
	store := binding.store
	store.locker.Lock()
	defer store.locker.Unlock()
	if !binding.active.Load() {
		return
	}
	if store.ClientCheck == nil {
		return false, ErrJsVarReadOnly
	}
	// jq permits one append at the current slice length. Browser proposals only
	// replace existing paths, so repeated inputs cannot grow a slice unchecked.
	if path != "" {
		if _, err = jq.Get(store.value, path); err != nil {
			return
		}
	}
	changed, err = jq.SetChecked(store.value, path, value, func() error {
		return store.ClientCheck(elem, store.value, path)
	})
	return
}

var (
	jsonMarshalerType = reflect.TypeFor[json.Marshaler]()
	textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()
)

// plainJsVarType is conservative about custom encoding and reference aliases.
// Map branches are partial under the documented JSON-tree/no-alias contract.
func plainJsVarType(t reflect.Type) bool {
	if t.Implements(jsonMarshalerType) || t.Implements(textMarshalerType) {
		return false
	}
	if t.Kind() != reflect.Pointer &&
		(reflect.PointerTo(t).Implements(jsonMarshalerType) || reflect.PointerTo(t).Implements(textMarshalerType)) {
		return false
	}
	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String:
		return true
	case reflect.Struct:
		for i := range t.NumField() {
			field := t.Field(i)
			if field.Anonymous {
				return false
			}
			if field.IsExported() && field.Tag.Get("json") != "-" && !plainJsVarType(field.Type) {
				return false
			}
		}
		return true
	case reflect.Map:
		return t.Key() == reflect.TypeFor[string]() && plainJsVarType(t.Elem())
	case reflect.Array:
		return plainJsVarType(t.Elem())
	}
	return false
}
