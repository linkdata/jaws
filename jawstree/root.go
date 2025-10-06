package jawstree

import (
	"errors"
	"io/fs"
	"os"
	"path"
)

// Root builds a root node from a os.Root. If filterfn is not nil, it must return true
// for a directory entry to be included in the tree.
func Root(r *os.Root, filterfn func(dirpath string, ent fs.DirEntry) (include bool)) (rootnode *Node, err error) {
	rootnode = &Node{}
	err = getNodes(r.FS(), rootnode, ".", filterfn)
	return
}

func getNodes(rootfs fs.FS, parent *Node, dirpath string, filterfn func(dirpath string, ent fs.DirEntry) (include bool)) (err error) {
	var ents []fs.DirEntry
	if ents, err = fs.ReadDir(rootfs, dirpath); err == nil {
		for _, ent := range ents {
			ent.Name()
			if filterfn == nil || filterfn(dirpath, ent) {
				child := &Node{
					Tree:   parent.Tree,
					Parent: parent,
					ID:     path.Join(parent.ID, ent.Name()),
					Name:   ent.Name(),
				}
				if ent.Type().IsRegular() {
					parent.Children = append(parent.Children, child)
				} else if ent.IsDir() {
					if err = errors.Join(err, getNodes(rootfs, child, path.Join(dirpath, ent.Name()), filterfn)); err == nil {
						parent.Children = append(parent.Children, child)
					}
				}
			}
		}
	}
	return
}
