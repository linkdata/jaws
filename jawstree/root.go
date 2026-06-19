package jawstree

import (
	"errors"
	"io/fs"
	"os"
	"path"
)

// Root builds a root node from an [os.Root]. It panics if r is nil.
//
// If filterFn is not nil, a directory entry is included only when filterFn returns
// true for it. Entries that are neither regular files nor directories (such as
// symbolic links) are always excluded, regardless of filterFn.
//
// Building the tree is best-effort: if one or more directories cannot be read, Root
// returns the tree built from the readable entries together with a non-nil error
// joining every read failure (see [errors.Join]). A directory whose own listing fails to
// read is omitted from its parent, but its readable siblings — and the readable entries
// of any directory with a deeper failure — are kept.
//
// The returned nodes have a nil Tree and filesystem-relative IDs; pass the tree to
// [New] before rendering or any path operation. New overwrites both fields with the
// owning Tree pointer and the canonical JSON path IDs. The name-path helpers work on the
// returned nodes as-is, but rendering or serving the tree reaches [Node.JawsPathSet],
// which dereferences Tree and panics until New has set it.
func Root(r *os.Root, filterFn func(dirpath string, ent fs.DirEntry) (include bool)) (rootnode *Node, err error) {
	if r == nil {
		panic("jawstree.Root: r must not be nil")
	}
	rootnode = &Node{}
	err = getNodes(r.FS(), rootnode, ".", filterFn)
	return
}

// getNodes reads dirpath from rootfs and appends its readable entries to parent.
func getNodes(rootfs fs.FS, parent *Node, dirpath string, filterFn func(dirpath string, ent fs.DirEntry) (include bool)) (err error) {
	var ents []fs.DirEntry
	if ents, err = fs.ReadDir(rootfs, dirpath); err == nil {
		err = addEntries(rootfs, parent, dirpath, ents, filterFn)
	}
	return
}

// addEntries appends the readable entries in ents (the already-read listing of dirpath)
// to parent; see [Root] for the best-effort inclusion rules.
func addEntries(rootfs fs.FS, parent *Node, dirpath string, ents []fs.DirEntry, filterFn func(dirpath string, ent fs.DirEntry) (include bool)) (err error) {
	for _, ent := range ents {
		if filterFn == nil || filterFn(dirpath, ent) {
			child := &Node{
				Tree:   parent.Tree,
				Parent: parent,
				ID:     path.Join(parent.ID, ent.Name()),
				Name:   ent.Name(),
			}
			switch {
			case ent.Type().IsRegular():
				parent.Children = append(parent.Children, child)
			case ent.IsDir():
				childpath := path.Join(dirpath, ent.Name())
				// Gate inclusion on this directory's own read, not on its
				// subtree: a deeper failure must not drop a readable directory.
				if subents, cerr := fs.ReadDir(rootfs, childpath); cerr == nil {
					parent.Children = append(parent.Children, child)
					err = errors.Join(err, addEntries(rootfs, child, childpath, subents, filterFn))
				} else {
					err = errors.Join(err, cerr)
				}
			}
		}
	}
	return
}
