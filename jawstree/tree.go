package jawstree

import (
	"encoding/base64"
	"fmt"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// wsInboundLimit mirrors the jaws inbound WebSocket message limit (jaws' internal
// webSocketReadLimit, 32 KiB): a single client-to-server frame must not exceed it,
// or the connection is dropped. [MaxTreeNodes] is derived from it.
const wsInboundLimit = 32 * 1024

// MaxTreeNodes is the largest node count [New] accepts.
//
// It bounds the largest possible selection message, whose size grows with the node
// count, to stay within the WebSocket size limit. [New] rejects a larger tree with
// [ErrInvalidTree]; TestSelectionFrameFitsInboundLimit pins the guarantee.
const MaxTreeNodes = 180000

// Tree is the shared, server-authoritative model behind a Quercus.js tree, and the
// [jaws.UI] that renders it.
//
// A Tree holds the node tree and the master selection state, guarded by the lock
// passed to [New] (which the application may share with other state). A single Tree
// is built once and rendered by every request that should show the same collaborative
// tree; it holds no per-request state, so sharing it is safe.
//
// The tree is structurally fixed once [New] returns: it derives each node's identity
// from its position. Mutating selection (through [Tree.SetSelected] or browser events)
// is safe under the lock, but adding, removing, or reordering Children afterward is
// unsupported.
type Tree struct {
	bind.RWLocker         // guards root selection state and is the concurrency contract for Node accessors
	key           string  // browser correlation key, unique per Tree
	options       Option  // feature flags, serialized to the browser initializer
	root          *Node   // authoritative node tree; positional-path IDs assigned by New
	byIndex       []*Node // preorder index -> node; index 0 is root, the compact wire alias
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

// New returns a shared tree model for root, guarded by l.
//
// It returns [ErrInvalidTree] for a nil root, a cyclic or shared-node graph, a
// negative or unknown [Option] bit, or more than [MaxTreeNodes] nodes, and
// [ErrInvalidSelection] when the initial Selected flags violate the selection policy
// (see [Tree.SetSelected]).
//
// New must run before rendering the Tree or using the name-path selection API. The
// Tree renders its own container, so no caller-provided HTML id is required.
func New(l sync.Locker, root *Node, options ...Option) (t *Tree, err error) {
	if root == nil {
		return nil, fmt.Errorf("%w: root must not be nil", ErrInvalidTree)
	}
	var opts Option
	for _, opt := range options {
		opts |= opt
	}
	if opts < 0 {
		return nil, fmt.Errorf("%w: options must be non-negative, got %d", ErrInvalidTree, int(opts))
	}
	if opts&^allOptions != 0 {
		return nil, fmt.Errorf("%w: unknown option bits %d", ErrInvalidTree, int(opts&^allOptions))
	}
	t = &Tree{
		RWLocker: bind.AsRWLocker(l),
		key:      makeTreeKey(),
		options:  opts,
		root:     root,
	}
	// The root is identified throughout by a nil Parent; index() only sets
	// descendants' Parent, so enforce the invariant for the root here (a node reused
	// as a new root could otherwise carry a stale Parent).
	root.Parent = nil
	if err = t.index(root, "", make(map[*Node]bool)); err != nil {
		return nil, err
	}
	if len(t.byIndex) > MaxTreeNodes {
		return nil, fmt.Errorf("%w: %d nodes exceeds MaxTreeNodes (%d)", ErrInvalidTree, len(t.byIndex), MaxTreeNodes)
	}
	// Validate the initial selection through the same policy the browser and server
	// mutators use, so construction can never produce a state the policy rejects.
	want := make(map[*Node]bool)
	for _, node := range t.byIndex {
		if node.Selected {
			want[node] = true
		}
	}
	if _, err = t.applySelection(want); err != nil {
		return nil, err
	}
	return t, nil
}

// index assigns node's positional-path ID, appends it to byIndex, compacts away nil
// children, sets child parent back-pointers, and rejects a cyclic or shared-node
// graph. The seen set guards the recursion, so a cycle terminates with an error
// rather than overflowing the stack. Compacting before descending keeps each node's
// slice index (its ID) matching its position in the wire array emitted by
// marshalJSON, which skips nils.
func (t *Tree) index(node *Node, jsPath string, seen map[*Node]bool) error {
	if seen[node] {
		return fmt.Errorf("%w: node %q is reachable more than once (cyclic or shared graph)", ErrInvalidTree, node.Name)
	}
	seen[node] = true
	node.ID = jsPath
	t.byIndex = append(t.byIndex, node)
	node.Children = slices.DeleteFunc(node.Children, func(c *Node) bool { return c == nil })
	if jsPath != "" {
		jsPath += "."
	}
	for i, child := range node.Children {
		child.Parent = node
		if err := t.index(child, jsPath+"children."+strconv.Itoa(i), seen); err != nil {
			return err
		}
	}
	return nil
}

// Dirty marks the Tree changed so every rendered view resynchronizes its client on
// the next update pass. Call it after mutating selection server-side.
func (t *Tree) Dirty(jw *jaws.Jaws) {
	jw.Dirty(t)
}

// strictSingle reports whether at most one node may be selected: neither
// multi-select nor cascade is enabled.
func (t *Tree) strictSingle() bool {
	return t.options&MultiSelectEnabled == 0 && t.options&CascadeSelectChildren == 0
}

// applySelection sets the selection to exactly want and returns the changed nodes.
//
// It is the single selection policy, shared by [New], [Tree.SetSelected], and
// browser input. It returns [ErrInvalidSelection] when want holds more than one node
// in a single-select tree, or any disabled or root node. Callers on a rendered Tree
// must hold the write lock.
func (t *Tree) applySelection(want map[*Node]bool) (changed []*Node, err error) {
	if t.strictSingle() && len(want) > 1 {
		return nil, fmt.Errorf("%w: single-select allows at most one node, got %d", ErrInvalidSelection, len(want))
	}
	for node := range want {
		if node == t.root {
			return nil, fmt.Errorf("%w: the root node cannot be selected", ErrInvalidSelection)
		}
		if node.Disabled {
			return nil, fmt.Errorf("%w: node %q is disabled", ErrInvalidSelection, node.ID)
		}
	}
	for _, node := range t.byIndex {
		if node.Selected != want[node] {
			node.Selected = want[node]
			changed = append(changed, node)
		}
	}
	return
}

// resolveIndex maps a wire index to a node, rejecting the root (index 0) and any
// out-of-range index with [ErrPathRejected].
func (t *Tree) resolveIndex(i int) (*Node, error) {
	if i <= 0 || i >= len(t.byIndex) {
		return nil, fmt.Errorf("%w: node index %d out of range", ErrPathRejected, i)
	}
	return t.byIndex[i], nil
}

// selectedSet returns the currently selected nodes as a set.
func (t *Tree) selectedSet() map[*Node]bool {
	sel := make(map[*Node]bool)
	for _, node := range t.byIndex {
		if node.Selected {
			sel[node] = true
		}
	}
	return sel
}

// applyClientDelta merges an add/remove index delta into the selection under the
// write lock, enforcing the policy, and reports whether anything changed.
//
// In a single-select tree the last valid add replaces the whole selection (the
// authoritative "deselect previous"); with no add, a remove that hits the current
// selection clears it and anything else is a no-op. Otherwise the delta merges, so
// concurrent multi-select edits from different clients compose rather than clobber.
func (t *Tree) applyClientDelta(add, remove []int) (changed bool, err error) {
	var want map[*Node]bool
	if t.strictSingle() {
		var target *Node
		for _, i := range add {
			var node *Node
			if node, err = t.resolveIndex(i); err != nil {
				return
			}
			if node.Disabled {
				return false, fmt.Errorf("%w: node %q is disabled", ErrPathRejected, node.ID)
			}
			target = node
		}
		want = make(map[*Node]bool)
		if target != nil {
			want[target] = true
		} else {
			cur := t.selectedSet()
			cleared := false
			for _, i := range remove {
				var node *Node
				if node, err = t.resolveIndex(i); err != nil {
					return
				}
				if cur[node] {
					cleared = true
				}
			}
			if !cleared {
				want = cur // remove of a non-selected node: no-op
			}
		}
	} else {
		want = t.selectedSet()
		for _, i := range remove {
			var node *Node
			if node, err = t.resolveIndex(i); err != nil {
				return
			}
			delete(want, node)
		}
		for _, i := range add {
			var node *Node
			if node, err = t.resolveIndex(i); err != nil {
				return
			}
			if node.Disabled {
				return false, fmt.Errorf("%w: node %q is disabled", ErrPathRejected, node.ID)
			}
			want[node] = true
		}
	}
	var chg []*Node
	if chg, err = t.applySelection(want); err == nil {
		changed = len(chg) > 0
	}
	return
}

// applyClientAbsolute replaces the selection with exactly the given indices (the
// bitmap fallback) under the write lock, enforcing the policy.
func (t *Tree) applyClientAbsolute(indices []int) (changed bool, err error) {
	want := make(map[*Node]bool, len(indices))
	for _, i := range indices {
		var node *Node
		if node, err = t.resolveIndex(i); err != nil {
			return
		}
		if node.Disabled {
			return false, fmt.Errorf("%w: node %q is disabled", ErrPathRejected, node.ID)
		}
		want[node] = true
	}
	if t.strictSingle() && len(want) > 1 {
		return false, fmt.Errorf("%w: single-select selection has %d nodes", ErrPathRejected, len(want))
	}
	var chg []*Node
	if chg, err = t.applySelection(want); err == nil {
		changed = len(chg) > 0
	}
	return
}

// GetSelected returns the name-paths of all selected nodes.
//
// It reads under the read lock. Selection is reported by name-path, which is lossy
// for duplicate sibling names; see [Node.GetSelected].
func (t *Tree) GetSelected() (nameLists [][]string) {
	t.RLock()
	defer t.RUnlock()
	nameLists = t.root.GetSelected()
	return
}

// SetSelected sets the selection to the nodes matching the given name-paths.
//
// It runs under the write lock and enforces the selection policy, returning
// [ErrInvalidSelection] when the match violates it (for example matching more than
// one node in a single-select tree, or a disabled node). Matching is by name-path
// and lossy for duplicate sibling names; see [Node.GetSelected].
func (t *Tree) SetSelected(nameLists [][]string) (err error) {
	t.Lock()
	defer t.Unlock()
	want := make(map[*Node]bool)
	for _, node := range t.byIndex {
		for _, names := range nameLists {
			if node.HasNames(names) {
				want[node] = true
				break
			}
		}
	}
	_, err = t.applySelection(want)
	return
}

// selectedIndexes returns the wire indices of the selected nodes under the read
// lock. Index 0 (the root) is never included.
func (t *Tree) selectedIndexes() (indexes []int) {
	t.RLock()
	defer t.RUnlock()
	for i := 1; i < len(t.byIndex); i++ {
		if t.byIndex[i].Selected {
			indexes = append(indexes, i)
		}
	}
	return
}

// Walk calls fn for the tree root and all descendants under the read lock; the
// callback must not call methods that acquire the same tree lock.
func (t *Tree) Walk(fn func(jsPath string, node *Node)) {
	t.RLock()
	defer t.RUnlock()
	t.root.Walk("", fn)
}

// initPayloadLocked builds the jawstreeInit JSON: the browser correlation key, the
// container Jid, the option flags, and the full node tree (with current selection).
// The caller must hold the read lock.
func (t *Tree) initPayloadLocked(jidStr string) string {
	var b []byte
	b = append(b, `{"key":`...)
	b = strconv.AppendQuote(b, t.key)
	b = append(b, `,"jid":`...)
	b = strconv.AppendQuote(b, jidStr)
	b = append(b, `,"options":`...)
	b = strconv.AppendInt(b, int64(t.options), 10)
	b = append(b, `,"data":`...)
	b = t.root.marshalJSON(b)
	b = append(b, '}')
	return string(b)
}

// selectionPayloadLocked builds the jawstreeSelection JSON carrying the absolute
// selected-index set, choosing the smaller of a sparse index list ("s") or a
// one-bit-per-node bitmap ("b"). The caller must hold the read lock.
func (t *Tree) selectionPayloadLocked() string {
	var idxs []int
	for i := 1; i < len(t.byIndex); i++ {
		if t.byIndex[i].Selected {
			idxs = append(idxs, i)
		}
	}
	// Each sparse index costs at most a few bytes; the bitmap is fixed at ~N/6 bytes.
	// Bound the sparse cost generously and pick the smaller wire form.
	bitmapLen := base64.StdEncoding.EncodedLen((len(t.byIndex) + 7) / 8)
	if len(idxs)*8 < bitmapLen {
		return t.sparsePayloadLocked(idxs)
	}
	return t.bitmapPayloadLocked()
}

func (t *Tree) sparsePayloadLocked(idxs []int) string {
	var b []byte
	b = append(b, `{"key":`...)
	b = strconv.AppendQuote(b, t.key)
	b = append(b, `,"s":[`...)
	for i, idx := range idxs {
		if i > 0 {
			b = append(b, ',')
		}
		b = strconv.AppendInt(b, int64(idx), 10)
	}
	b = append(b, `]}`...)
	return string(b)
}

func (t *Tree) bitmapPayloadLocked() string {
	buf := make([]byte, (len(t.byIndex)+7)/8)
	for i := 1; i < len(t.byIndex); i++ {
		if t.byIndex[i].Selected {
			buf[i/8] |= 1 << (uint(i) % 8)
		}
	}
	var b []byte
	b = append(b, `{"key":`...)
	b = strconv.AppendQuote(b, t.key)
	b = append(b, `,"b":`...)
	b = strconv.AppendQuote(b, base64.StdEncoding.EncodeToString(buf))
	b = append(b, '}')
	return string(b)
}

// decodeSelectionBitmap decodes a base64 one-bit-per-node bitmap into the set node
// indices, rejecting a malformed or wrong-length bitmap with [ErrPathRejected]. n is
// the total node count (len of byIndex).
func decodeSelectionBitmap(s string, n int) (indices []int, err error) {
	var buf []byte
	if buf, err = base64.StdEncoding.DecodeString(s); err != nil {
		return nil, fmt.Errorf("%w: malformed bitmap: %v", ErrPathRejected, err)
	}
	if want := (n + 7) / 8; len(buf) != want {
		return nil, fmt.Errorf("%w: bitmap length %d, want %d", ErrPathRejected, len(buf), want)
	}
	for i := 0; i < n; i++ {
		if buf[i/8]&(1<<(uint(i)%8)) != 0 {
			indices = append(indices, i)
		}
	}
	return
}
