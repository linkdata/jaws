package jawstree

import (
	"errors"
	"io/fs"
	"os"
	"path"
)

// Root builds a root node from an [os.Root]. If filterFn is not nil, it must return true
// for a directory entry to be included in the tree.
//
// Building the tree is best-effort: if one or more directories cannot be read,
// Root returns the tree built from the readable entries together with a non-nil
// error joining every read failure (see [errors.Join]). A subdirectory that
// fails to read is omitted from its parent, but its readable siblings are kept.
//
// The returned nodes have a nil Tree and unset path IDs; both are populated by
// [New]. The node tree must therefore be passed to New (as the JsVar value)
// before rendering or any path operation, which otherwise dereference the nil
// Tree and panic.
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
