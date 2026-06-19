package jawstree

import (
	"errors"
	"io/fs"
	"os"
	"path"
)

// Root builds a root node from an [os.Root].
//
// If filterFn is not nil, a directory entry is included only when filterFn returns
// true for it. Entries that are neither regular files nor directories (such as
// symbolic links) are always excluded, regardless of filterFn.
//
// Building the tree is best-effort: if one or more directories cannot be read, Root
// returns the tree built from the readable entries together with a non-nil error
// joining every read failure (see [errors.Join]). A subdirectory that fails to read
// is omitted from its parent, but its readable siblings are kept.
//
// The returned nodes have a nil Tree and filesystem-relative IDs; pass the tree to
// [New] before rendering or any path operation. New overwrites both fields with the
// owning Tree pointer and the canonical JSON path IDs; using a node before then
// dereferences the nil Tree and panics.
func Root(r *os.Root, filterFn func(dirpath string, ent fs.DirEntry) (include bool)) (rootnode *Node, err error) {
	rootnode = &Node{}
	err = getNodes(r.FS(), rootnode, ".", filterFn)
	return
}

func getNodes(rootfs fs.FS, parent *Node, dirpath string, filterFn func(dirpath string, ent fs.DirEntry) (include bool)) (err error) {
	var ents []fs.DirEntry
	if ents, err = fs.ReadDir(rootfs, dirpath); err == nil {
		for _, ent := range ents {
			if filterFn == nil || filterFn(dirpath, ent) {
				child := &Node{
					Tree:   parent.Tree,
					Parent: parent,
					ID:     path.Join(parent.ID, ent.Name()),
					Name:   ent.Name(),
				}
				if ent.Type().IsRegular() {
					parent.Children = append(parent.Children, child)
				} else if ent.IsDir() {
					// Append the directory only if its own subtree read
					// succeeded; accumulate any failure without letting it
					// suppress readable sibling directories.
					if cerr := getNodes(rootfs, child, path.Join(dirpath, ent.Name()), filterFn); cerr == nil {
						parent.Children = append(parent.Children, child)
					} else {
						err = errors.Join(err, cerr)
					}
				}
			}
		}
	}
	return
}
