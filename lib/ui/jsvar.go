package ui

import (
	"encoding"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"slices"
	"strconv"
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
	maxJsVarChanges   = 64
	maxJsVarLogBytes  = 16 * 1024
)

type jsVarChange struct {
	version uint64
	path    string
}

var jsVarNameRx = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*(\.[A-Za-z_$][A-Za-z0-9_$]*)*$`)

func validateJsVarName(name string) error {
	if len(name) > maxJsVarNameBytes {
		return fmt.Errorf("%w: too long", ErrIllegalJsVarName)
	}
	if !jsVarNameRx.MatchString(name) {
		return fmt.Errorf("%w: illegal syntax", ErrIllegalJsVarName)
	}
	for component := range strings.SplitSeq(name, ".") {
		if component == "__proto__" || component == "constructor" || component == "prototype" {
			return fmt.Errorf("%w: reserved", ErrIllegalJsVarName)
		}
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
// after jq tentatively applies a changed value. It must only inspect next;
// an error or panic rolls the proposal back. The source can authorize a user
// or session. Validate the complete value, including changes through parent
// and root paths.
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
// Use [JsVarStore.Bind] once per Request during initial page rendering.
// ClientCheck and ExtraTags must be configured before first use and remain
// unchanged while the store is active.
// Every binding sees the same JSON value; ClientCheck controls writes, not
// disclosure.
//
// All reads and writes of the bound value must use the supplied locker. Server
// mutations must use SetPath, DeletePath, or WriteLocked to publish changes.
// The JSON encoding must have unique object member names.
// For partial map patches, the bound JSON tree must not share mutable pointers,
// maps, or slices across different paths. Complex Go types use root patches.
// Custom JSON methods reached while locked must not re-enter the store or its
// locker.
//
// A JsVarStore must not be copied after first use.
type JsVarStore[T any] struct {
	ClientCheck JsVarCheck[T] // nil denies browser proposals
	ExtraTags   []any         // dirtied after each changed mutation

	jaws     *jaws.Jaws
	name     string
	locker   bind.RWLocker
	value    *T
	partial  bool // plain JSON tree with matching jq paths and safe partial patches
	version  uint64
	floor    uint64 // older bindings need a root patch
	changes  []jsVarChange
	logBytes int
}

// NewJsVarStore creates a store for a browser variable.
//
// The name is a dot-separated path from window using JavaScript identifiers
// with ASCII letters, digits, underscore, or dollar sign. No component may be
// "__proto__", "constructor", or "prototype"; the limit is 4096 bytes.
// Initial data and patches assign to that live path, including browser-owned
// properties and setters.
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
// Render each binding once during the Request's initial page render, outside
// regions that may later be replaced or removed. Browser names may neither
// duplicate nor contain one another within a Request.
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

// JsVarPathWriter applies path edits inside [JsVarStore.WriteLocked].
//
// A writer is valid only during the callback. Changed edits are published after
// the store unlocks, even if the callback returns an error or panics.
type JsVarPathWriter interface {
	// SetPath sets a canonical path in the locked value.
	// See [JsVarStore.SetPath] for path and no-op behavior.
	SetPath(path string, value any) (changed bool, err error)
	// DeletePath removes a string-keyed map entry from the locked value.
	// See [JsVarStore.DeletePath] for path and no-op behavior.
	DeletePath(path string) (changed bool, err error)
}

type jsVarPathWriter[T any] struct {
	store   *JsVarStore[T]
	changed bool
}

func (writer *jsVarPathWriter[T]) SetPath(path string, value any) (changed bool, err error) {
	if err = validateJsVarPath(path); err == nil {
		if changed, err = jq.Set(writer.store.value, path, value); err == nil && changed {
			writer.store.record(path)
			writer.changed = true
		}
	}
	return
}

func (writer *jsVarPathWriter[T]) DeletePath(path string) (changed bool, err error) {
	if err = validateJsVarPath(path); err == nil {
		if changed, err = writer.store.deletePathLocked(path); err == nil && changed {
			writer.store.record(path)
			writer.changed = true
		}
	}
	return
}

// WriteLocked groups server path edits under one application lock.
//
// The value is borrowed for reads and must not be retained or mutated directly.
// The writer is valid only during fn and must not be retained. The callback
// must use the writer for edits and must not call lock-taking store methods.
// Each successful changed edit remains applied and is published after unlocking,
// even if a later edit fails or fn panics. WriteLocked is a lock scope, not a
// rollback transaction.
func (store *JsVarStore[T]) WriteLocked(fn func(value *T, writer JsVarPathWriter) error) (err error) {
	writer := &jsVarPathWriter[T]{store: store}
	store.locker.Lock()
	defer func() {
		store.locker.Unlock()
		store.publish(writer.changed)
	}()
	err = fn(store.value, writer)
	return
}

// SetPath sets a Go value at a canonical path and publishes a changed result.
//
// Paths are dot-separated JSON names, with no empty or prototype-sensitive
// component, control byte, DEL, or '='; the maximum is 4096 UTF-8 bytes.
// The empty path replaces the root. A no-op returns changed=false. Server writes
// do not invoke ClientCheck. The resulting value must remain JSON encodable.
func (store *JsVarStore[T]) SetPath(path string, value any) (changed bool, err error) {
	err = store.WriteLocked(func(_ *T, writer JsVarPathWriter) error {
		changed, err = writer.SetPath(path, value)
		return err
	})
	return
}

// DeletePath removes a map entry by JSON path.
//
// The parent must be a map with built-in string keys. A missing key in that
// map returns changed=false; a missing or non-map parent returns
// [github.com/linkdata/jq.ErrPathNotFound]. JSON null is a value; deletion is a
// separate operation. The empty path returns [ErrIllegalJsVarPath].
func (store *JsVarStore[T]) DeletePath(path string) (changed bool, err error) {
	err = store.WriteLocked(func(_ *T, writer JsVarPathWriter) error {
		changed, err = writer.DeletePath(path)
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

func (store *JsVarStore[T]) publish(changed bool) {
	if changed {
		store.jaws.Dirty(append([]any{store}, store.ExtraTags...)...)
	}
}

// record runs under the store's write lock. A binding older than floor gets
// a root patch instead of retaining unbounded history.
func (store *JsVarStore[T]) record(path string) {
	store.version++
	if path == "" || !store.partial {
		store.changes = nil
		store.logBytes = 0
		store.floor = store.version
		return
	}
	for i, change := range store.changes {
		if change.path == path {
			store.changes = slices.Delete(store.changes, i, i+1)
			store.changes = append(store.changes, jsVarChange{store.version, path})
			return
		}
	}
	for len(store.changes) == maxJsVarChanges || store.logBytes+len(path) > maxJsVarLogBytes {
		store.floor = store.changes[0].version
		store.logBytes -= len(store.changes[0].path)
		store.changes = slices.Delete(store.changes, 0, 1)
	}
	store.changes = append(store.changes, jsVarChange{store.version, path})
	store.logBytes += len(path)
}

type (
	jsVarNameTag   struct{ name string }
	jsVarPrefixTag struct{ name string }
)

// JsVarBinding renders one browser route to a [JsVarStore].
//
// Bindings are one-use and request-scoped. [JsVarBinding.Deactivate] is safe to
// call concurrently with input dispatch.
type JsVarBinding[T any] struct {
	store         *JsVarStore[T]
	rendered      atomic.Bool
	active        atomic.Bool
	lastVersion   atomic.Uint64
	correctionMu  sync.Mutex
	correction    string
	hasCorrection bool
}

// Deactivate suppresses this binding's later proposal and patch work.
//
// Patch work already in progress may queue messages after Deactivate returns,
// and queued messages may still be delivered.
func (binding *JsVarBinding[T]) Deactivate() {
	binding.active.Store(false)
}

// JawsRender writes the hidden browser route and its initial JSON value.
//
// A Request owned by a different [jaws.Jaws] returns an error without output.
// Rendering twice returns [ErrJsVarBindingUsed]; a duplicate or overlapping
// browser name in the Request returns [ErrJsVarNameConflict].
func (binding *JsVarBinding[T]) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	if elem.Jaws != binding.store.jaws {
		// The store publishes through exactly one Jaws; a foreign Request could
		// render a route but would never receive its invalidations.
		return fmt.Errorf("jsvar: binding rendered on a different Jaws instance")
	}
	if !binding.rendered.CompareAndSwap(false, true) {
		return ErrJsVarBindingUsed
	}
	store := binding.store
	data, err := binding.renderSnapshot(elem)
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
		binding.active.Store(true)
	}
	return
}

func (binding *JsVarBinding[T]) renderSnapshot(elem *jaws.Element) (data []byte, err error) {
	store := binding.store
	store.locker.Lock()
	defer store.locker.Unlock()
	// Registration and snapshot share the store lock. A mutation sees either
	// the rendered value or the registered tag, including before WS connection.
	tags := []any{jsVarNameTag{store.name}, store}
	conflicts := []any{jsVarNameTag{store.name}, jsVarPrefixTag{store.name}}
	for i := 0; i < len(store.name); i++ {
		if store.name[i] == '.' {
			prefix := store.name[:i]
			tags = append(tags, jsVarPrefixTag{prefix})
			conflicts = append(conflicts, jsVarNameTag{prefix})
		}
	}
	elem.Tag(tags...)
	for _, tag := range conflicts {
		for _, other := range elem.Request.GetElements(tag) {
			if other != elem {
				err = ErrJsVarNameConflict
				return
			}
		}
	}
	data, _, err = marshalJsVar(store.value)
	if err == nil {
		binding.lastVersion.Store(store.version)
	}
	return
}

// JawsUpdate sends canonical patches for store changes and browser corrections.
//
// A failed encoding cancels the Request.
func (binding *JsVarBinding[T]) JawsUpdate(elem *jaws.Element) {
	if !binding.active.Load() {
		return
	}
	patches, err := binding.pendingPatches()
	if err != nil {
		elem.Request.Cancel(fmt.Errorf("jsvar: encode store %q: %w", binding.store.name, err))
		return
	}
	if binding.active.Load() {
		for _, patch := range patches {
			elem.JsVar(patch)
		}
	}
}

func (binding *JsVarBinding[T]) correct(elem *jaws.Element, path string) {
	binding.correctionMu.Lock()
	if binding.hasCorrection && binding.correction != path {
		binding.correction = ""
	} else {
		binding.correction = path
	}
	binding.hasCorrection = true
	binding.correctionMu.Unlock()
	binding.store.jaws.Dirty(elem)
}

func (binding *JsVarBinding[T]) takeCorrection() (path string, ok bool) {
	binding.correctionMu.Lock()
	path, ok = binding.correction, binding.hasCorrection
	binding.correction = ""
	binding.hasCorrection = false
	binding.correctionMu.Unlock()
	return
}

func (binding *JsVarBinding[T]) pendingPatches() (patches []string, err error) {
	store := binding.store
	correction, needsCorrection := binding.takeCorrection()
	var paths []string
	var data []byte
	var version uint64
	store.ReadLocked(func(value *T) {
		version = store.version
		lastVersion := binding.lastVersion.Load()
		if lastVersion < store.floor {
			paths = append(paths, "")
		} else {
			for _, change := range store.changes {
				if change.version > lastVersion {
					paths = append(paths, change.path)
				}
			}
		}
		if needsCorrection {
			paths = append(paths, correction)
		}
		if len(paths) > 0 {
			data, err = json.Marshal(value)
		}
	})
	if err == nil && len(paths) > 0 {
		patches, err = store.projectPatches(paths, data)
	}
	if err == nil {
		binding.lastVersion.Store(version)
	}
	return
}

func (store *JsVarStore[T]) projectPatches(paths []string, data []byte) (patches []string, err error) {
	var visible any
	visible, err = decodeJsVarJSON(data)
	if err == nil {
		rootPatch := "=" + string(data)
		seen := make(map[string]bool, len(paths))
		slices.Sort(paths)
		for _, path := range paths {
			covered := seen[path]
			for i := range len(path) {
				if path[i] == '.' && seen[path[:i]] {
					covered = true
					break
				}
			}
			if covered {
				continue
			}
			patch := store.projectVisiblePatch(path, data, visible)
			if patch == rootPatch {
				patches = []string{rootPatch}
				break
			}
			key, _, _ := strings.Cut(patch, "=")
			seen[key] = true
			patches = append(patches, patch)
		}
	}
	return
}

func (store *JsVarStore[T]) projectVisiblePatch(path string, data []byte, visible any) string {
	if path == "" || !store.partial || validateJsVarPath(path) != nil {
		return "=" + string(data)
	}
	current := visible
	var prefix string
	for component := range strings.SplitSeq(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			if prefix == "" {
				return "=" + string(data)
			}
			break
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
	encoded, err := json.Marshal(current)
	if err != nil {
		return "=" + string(data)
	}
	return prefix + "=" + string(encoded)
}

// JawsInput applies a browser proposal allowed by ClientCheck.
//
// A deactivated binding ignores proposals.
//
// A rejected, invalid, or unchanged proposal schedules a canonical correction
// for its source binding. A changed accepted proposal invalidates every binding.
// An unchanged proposal to a complex Go shape is rejected.
// A panicking check rolls back and schedules a root correction before the panic
// continues. [ErrJsVarTooLarge] cancels the source Request for reload recovery.
func (binding *JsVarBinding[T]) JawsInput(elem *jaws.Element, input string) (err error) {
	if !binding.active.Load() {
		return nil
	}
	defer func() {
		if panicValue := recover(); panicValue != nil {
			binding.correct(elem, "")
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
		binding.correct(elem, "")
		return errJsVarClientWrite{err}
	}
	var value any
	if err = json.Unmarshal([]byte(raw), &value); err != nil {
		binding.correct(elem, "")
		return errJsVarClientWrite{err}
	}
	changed, err := binding.applyProposal(elem, path, value)
	if changed {
		binding.store.publish(true)
	} else {
		binding.correct(elem, path)
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
	if store.partial {
		// Plain trees map decoded proposals to the same jq/JSON paths.
		// Check the changed subtree before accepting it.
		changed, err = jq.SetChecked(store.value, path, value, func() error {
			var next any = store.value
			if path != "" {
				var getErr error
				if next, getErr = jq.Get(store.value, path); getErr != nil {
					return getErr
				}
			}
			if _, encodeErr := json.Marshal(next); encodeErr != nil {
				return encodeErr
			}
			return store.ClientCheck(elem, store.value, path)
		})
		if changed {
			store.record(path)
		}
		return
	}
	_, before, err := marshalJsVar(store.value)
	if err != nil {
		return false, err
	}
	changed, err = jq.SetChecked(store.value, path, value, func() error {
		_, after, encodeErr := marshalJsVar(store.value)
		if encodeErr != nil {
			return encodeErr
		}
		if path != "" && !visibleJsVarChange(before, after, path) {
			return ErrIllegalJsVarPath
		}
		return store.ClientCheck(elem, store.value, path)
	})
	if err == nil && !changed {
		// jq skips the callback for equal Go values. With a custom encoder,
		// accepting that no-op could reveal a field absent from browser JSON.
		err = ErrIllegalJsVarPath
	}
	if changed {
		store.record(path)
	}
	return
}

// marshalJsVar rejects ambiguous JSON before it reaches a browser or commits.
func marshalJsVar(value any) (data []byte, visible any, err error) {
	if data, err = json.Marshal(value); err == nil {
		visible, err = decodeJsVarJSON(data)
	}
	return
}

func decodeJsVarJSON(data []byte) (visible any, err error) {
	err = jsonv2.Unmarshal(data, &visible, jsVarNumberOption)
	return
}

var jsVarNumberOption = jsonv2.WithUnmarshalers(jsonv2.UnmarshalFromFunc(func(decoder *jsontext.Decoder, value *any) error {
	if decoder.PeekKind() != '0' {
		return errors.ErrUnsupported
	}
	raw, err := decoder.ReadValue()
	if err == nil {
		*value = json.Number(raw)
	}
	return err
}))

func visibleJsVarChange(before, after any, path string) bool {
	if path == "" {
		return !reflect.DeepEqual(before, after)
	}
	component, rest, _ := strings.Cut(path, ".")
	switch current := before.(type) {
	case map[string]any:
		next, ok := after.(map[string]any)
		if !ok || len(current) != len(next) {
			return false
		}
		for key, value := range current {
			if key != component {
				other, found := next[key]
				if !found || !reflect.DeepEqual(value, other) {
					return false
				}
			}
		}
		value, found := current[component]
		other, exists := next[component]
		return found && exists && visibleJsVarChange(value, other, rest)
	case []any:
		next, ok := after.([]any)
		if !ok || len(current) != len(next) {
			return false
		}
		index, err := strconv.Atoi(component)
		if err != nil || index < 0 || index >= len(current) || strconv.Itoa(index) != component {
			return false
		}
		for i, value := range current {
			if i != index && !reflect.DeepEqual(value, next[i]) {
				return false
			}
		}
		return visibleJsVarChange(current[index], next[index], rest)
	}
	return false
}

var jsVarMarshalers = [...]reflect.Type{
	reflect.TypeFor[jsonv2.MarshalerTo](),
	reflect.TypeFor[json.Marshaler](),
	reflect.TypeFor[encoding.TextAppender](),
	reflect.TypeFor[encoding.TextMarshaler](),
}

// plainJsVarType is conservative about encoder paths and reference aliases.
// Map branches are partial under the documented JSON-tree/no-alias contract.
func plainJsVarType(t reflect.Type) bool {
	return plainJsVarTypeSeen(t, make(map[reflect.Type]bool))
}

func plainJsVarScalar(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String:
		return true
	}
	return false
}

func plainJsVarTypeSeen(t reflect.Type, seen map[reflect.Type]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true
	defer delete(seen, t)
	for _, marshaler := range jsVarMarshalers {
		if t.Implements(marshaler) || reflect.PointerTo(t).Implements(marshaler) {
			return false
		}
	}
	if plainJsVarScalar(t) {
		return true
	}
	switch t.Kind() {
	case reflect.Struct:
		names := make(map[string]bool, t.NumField())
		for i := range t.NumField() {
			field := t.Field(i)
			if field.Anonymous {
				return false
			}
			if !field.IsExported() || field.Tag.Get("json") == "-" {
				continue
			}
			name, options, _ := strings.Cut(field.Tag.Get("json"), ",")
			if name == "" {
				name = field.Name
			}
			if !jsVarNameRx.MatchString(name) || names[name] {
				return false
			}
			names[name] = true
			for option := range strings.SplitSeq(options, ",") {
				switch option {
				case "":
				case "string":
					if !plainJsVarScalar(field.Type) {
						return false
					}
				case "omitempty", "omitzero":
					return false
				default:
					return false
				}
			}
			if !plainJsVarTypeSeen(field.Type, seen) {
				return false
			}
		}
		return true
	case reflect.Map:
		return t.Key() == reflect.TypeFor[string]() && plainJsVarTypeSeen(t.Elem(), seen)
	case reflect.Array:
		return plainJsVarTypeSeen(t.Elem(), seen)
	}
	return false
}
