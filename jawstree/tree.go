package jawstree

import (
	"crypto/sha256"
	"io"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/jid"
	"github.com/linkdata/jaws/lib/ui"
)

var (
	_ jaws.UI             = (*Tree)(nil)
	_ jaws.InputHandler   = (*Tree)(nil)
	_ jaws.ConnectUpdater = (*Tree)(nil)
)

const selectionSyncPrefix = "jawstree-selection-sync:"

// Tree renders and updates a shared Quercus.js tree bound to a [ui.JsVar].
//
// A Tree is shared UI state that may be rendered by multiple requests. It embeds
// a [ui.JsVar], which provides the lock and browser communication for the backing
// [Node] tree. Read or mutate that Node tree through Tree methods, or while
// holding the Tree lock.
//
// The tree is structurally fixed once [New] returns: it assigns each node's ID from its
// position, which must match the node's wire position. Mutating node fields (e.g. via
// [Tree.SetSelected]) under the lock is safe, but adding, removing, or reordering Children
// afterward breaks that mapping and is unsupported on a rendered Tree.
type Tree struct {
	key              string
	options          Option
	renderParams     [1]any
	selectionVersion uint64 // guarded by the embedded JsVar lock
	*ui.JsVar[Node]
}

type selectionSyncHandler struct {
	tree *Tree
}

func (h selectionSyncHandler) JawsInput(elem *jaws.Element, value string) (err error) {
	err = jaws.ErrEventUnhandled
	if h.tree.handleSelectionSync(elem, value) {
		err = nil
	}
	return
}

var nextTreeKey atomic.Uint64

func makeTreeKey() (key string) {
	for {
		current := nextTreeKey.Load()
		if current == ^uint64(0) {
			panic("jawstree.New: tree key space exhausted")
		}
		if nextTreeKey.CompareAndSwap(current, current+1) {
			key = strconv.FormatUint(current+1, 36)
			return
		}
	}
}

func (tree *Tree) isDefaultSingleSelect() bool {
	return tree.options&(MultiSelectEnabled|CascadeSelectChildren) == 0
}

// New returns a tree widget for jsvar.
//
// Both jsvar and jsvar.Ptr must be non-nil, and the combined options must be
// non-negative; New panics otherwise. Call New before serving or rendering the
// Tree.
//
// Tree renders its own Quercus.js container. No caller-provided HTML id or
// container element is required.
func New(jsvar *ui.JsVar[Node], options ...Option) (t *Tree) {
	if jsvar == nil {
		panic("jawstree.New: jsvar must not be nil")
	}
	if jsvar.Ptr == nil {
		panic("jawstree.New: jsvar.Ptr must not be nil")
	}
	t = &Tree{
		key:   makeTreeKey(),
		JsVar: jsvar,
	}
	for _, opt := range options {
		t.options |= opt
	}
	// options is serialized verbatim into the browser initializer. Panic here so a
	// bad Option surfaces as a clear construction error rather than a tree that
	// never renders.
	if t.options < 0 {
		panic("jawstree.New: options must be non-negative")
	}
	t.renderParams[0] = "jawstreeroot_" + t.key
	// Normalize away any nil children before assigning IDs so a node's slice
	// index (its ID) always matches its position in the compacted wire array
	// emitted by marshalJSON; otherwise a nil child desyncs the two and client
	// path resolution targets the wrong node. See [Node.stripNilChildren].
	jsvar.Ptr.stripNilChildren()
	// The root is identified throughout the name-path API by a nil Parent; the Walk
	// below only sets descendants' Parent, so enforce the invariant for the root here
	// (a node reused as a new root could otherwise carry a stale Parent).
	jsvar.Ptr.Parent = nil
	// Assign each node's JSON-path ID and its owning-Tree and parent back-pointers.
	// The name-path API (Node.HasNames, Node.GetNames, Tree.GetSelected,
	// Tree.SetSelected) requires the parent back-pointers.
	jsvar.Ptr.Walk("", func(jsPath string, node *Node) {
		node.ID = jsPath
		node.Tree = t
		for _, child := range node.Children {
			child.Parent = node // stripNilChildren guarantees no nil entries here
		}
	})
	return
}

func (tree *Tree) appendInitCallData(b []byte, containerJid jid.Jid, selectionVersion uint64) []byte {
	b = append(b, `{"key":`...)
	b = strconv.AppendQuote(b, tree.key)
	b = append(b, `,"jid":`...)
	b = containerJid.AppendQuote(b)
	b = append(b, `,"options":`...)
	b = strconv.AppendInt(b, int64(tree.options), 10)
	b = append(b, `,"selectionVersion":`...)
	b = strconv.AppendUint(b, selectionVersion, 10)
	b = append(b, '}')
	return b
}

func (tree *Tree) appendSelectionCallDataLocked(b []byte) []byte {
	b = append(b, `{"key":`...)
	b = strconv.AppendQuote(b, tree.key)
	b = append(b, `,"selectionVersion":`...)
	b = strconv.AppendUint(b, tree.selectionVersion, 10)
	b = append(b, `,"selected":[`...)
	first := true
	tree.Ptr.Walk("", func(_ string, node *Node) {
		if node.Selected {
			if !first {
				b = append(b, ',')
			}
			first = false
			b = strconv.AppendQuote(b, node.ID)
		}
	})
	b = append(b, `]}`...)
	return b
}

func (tree *Tree) appendSelectionCallData(b []byte) []byte {
	tree.RLock()
	b = tree.appendSelectionCallDataLocked(b)
	tree.RUnlock()
	return b
}

func (tree *Tree) appendSetCallData(b, data []byte, selectionVersion uint64) []byte {
	b = append(b, `{"key":`...)
	b = strconv.AppendQuote(b, tree.key)
	b = append(b, `,"selectionVersion":`...)
	b = strconv.AppendUint(b, selectionVersion, 10)
	b = append(b, `,"data":`...)
	b = append(b, data...)
	b = append(b, '}')
	return b
}

// JawsRender renders the tree state and schedules browser initialization.
func (tree *Tree) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	var selectionVersion uint64
	if len(params) == 0 {
		params = tree.renderParams[:]
	} else {
		params = append([]any{tree.renderParams[0]}, params...)
	}
	if err = tree.JsVar.JawsRenderSnapshot(elem, w, params, func(_ *Node) {
		selectionVersion = tree.selectionVersion
	}); err == nil {
		// JsVar renders caller-provided handlers before returning. Append the
		// protocol handler last so reverse-order event dispatch reaches it before a
		// general Input handler can consume the private synchronization message.
		elem.AddHandlers(selectionSyncHandler{tree})
		elem.JsCall("jawstreeInit", string(tree.appendInitCallData(nil, elem.Jid(), selectionVersion)))
	}
	return
}

func (tree *Tree) handleSelectionSync(elem *jaws.Element, value string) (handled bool) {
	versionText, isSelectionSync := strings.CutPrefix(value, selectionSyncPrefix)
	if isSelectionSync && tree.isDefaultSingleSelect() {
		if renderedVersion, parseErr := strconv.ParseUint(versionText, 10, 64); parseErr == nil && strconv.FormatUint(renderedVersion, 10) == versionText {
			var data []byte
			tree.RLock()
			if tree.selectionVersion > renderedVersion {
				data = tree.appendSelectionCallDataLocked(nil)
			}
			tree.RUnlock()
			if data != nil {
				elem.Jaws.JsCall(elem.Request.JawsKey, "jawstreeSetSelection", string(data))
			}
			handled = true
		}
	}
	return
}

// JawsInput handles selection reconciliation and JavaScript variable updates.
func (tree *Tree) JawsInput(elem *jaws.Element, value string) (err error) {
	if !tree.handleSelectionSync(elem, value) {
		err = tree.JsVar.JawsInput(elem, value)
	}
	return
}

// JawsConnectUpdate reconciles multi-select and cascading Trees after connection.
//
// Their clients do not use the default single-select selection handshake. When
// the shared Node data changed after the initial render but before the WebSocket
// subscription, JawsConnectUpdate queues one authoritative jawstreeSet call for
// this request. The call updates the browser-side variable and Quercus view as
// one snapshot. Default single-select Trees continue to reconcile through their
// versioned handshake, which preserves browser expansion and search state.
func (tree *Tree) JawsConnectUpdate(elem *jaws.Element, renderedState any) (err error) {
	owner, ownsElement := elem.UI().(*Tree)
	renderedDigest, hasRenderedDigest := renderedState.([sha256.Size]byte)
	if tree.isDefaultSingleSelect() || renderedState == nil || !ownsElement || owner != tree {
		return
	}

	var data []byte
	var selectionVersion uint64
	tree.RLock()
	if tree.Ptr != nil {
		data = tree.Ptr.marshalJSON(nil)
		selectionVersion = tree.selectionVersion
	}
	tree.RUnlock()
	if data != nil && (!hasRenderedDigest || sha256.Sum256(data) != renderedDigest) {
		elem.JsCall("jawstreeSet", string(tree.appendSetCallData(nil, data, selectionVersion)))
	}
	return
}

// JawsUpdate sends the latest tree JSON to the browser.
//
// It reads the shared Node tree under the Tree read lock, so it is safe to call
// concurrently with the JaWS event goroutines that mutate the tree under the
// write lock.
func (tree *Tree) JawsUpdate(elem *jaws.Element) {
	var b []byte
	b = append(b, `{"key":`...)
	b = strconv.AppendQuote(b, tree.key)
	// marshalJSON walks the shared Node tree, which a concurrent JawsInput on
	// another Request can mutate under the JsVar write lock. Read it under the
	// JsVar read lock so the selection version and data form one snapshot and the
	// walk never races; JawsRender is likewise locked.
	tree.RLock()
	b = append(b, `,"selectionVersion":`...)
	b = strconv.AppendUint(b, tree.selectionVersion, 10)
	b = append(b, `,"data":`...)
	b = tree.JsVar.Ptr.marshalJSON(b)
	tree.RUnlock()
	b = append(b, `}`...)
	elem.JsCall("jawstreeSet", string(b))
}

// Walk calls fn for the tree root and all descendants.
//
// It is called with the tree read lock held, so the callback must not call
// methods that acquire the same tree lock.
func (tree *Tree) Walk(fn func(jsPath string, node *Node)) {
	tree.RLock()
	defer tree.RUnlock()
	tree.Ptr.Walk("", fn)
}

// GetSelected returns the selected name-paths.
//
// It reads under the tree read lock.
func (tree *Tree) GetSelected() (nameLists [][]string) {
	tree.RLock()
	defer tree.RUnlock()
	nameLists = tree.Ptr.GetSelected()
	return
}

// SetSelected applies the selected name-paths and returns the changed [Node] values.
//
// It runs under the tree write lock. The returned [Node] pointers reference the
// lock-protected shared tree and the
// write lock is released on return, so on a rendered Tree they must only be read
// under the tree read lock (RLock) and mutated under the write lock (Lock), per
// the [Node] concurrency note. Dereferencing them without re-taking the lock
// races the JaWS event goroutines.
func (tree *Tree) SetSelected(nameLists [][]string) (changed []*Node) {
	tree.Lock()
	defer tree.Unlock()
	changed = tree.Ptr.SetSelected(nameLists)
	return
}
