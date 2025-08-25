package jawstree

import (
	"errors"
	"io/fs"
	"os"
	"path"
)

func Root(r *os.Root) (rootnode *Node, err error) {
	rootnode = &Node{}
	err = getNodes(r.FS(), rootnode, ".")
	return
}

func getNodes(rootfs fs.FS, parent *Node, dirpath string) (err error) {
	var ents []fs.DirEntry
	if ents, err = fs.ReadDir(rootfs, dirpath); err == nil {
		for _, ent := range ents {
			id := path.Join(parent.ID, ent.Name())
			child := &Node{Tree: parent.Tree, Parent: parent, ID: id, Name: ent.Name()}
			if ent.Type().IsRegular() {
				parent.Children = append(parent.Children, child)
			} else if ent.IsDir() {
				if err = errors.Join(err, getNodes(rootfs, child, path.Join(dirpath, child.Name))); err == nil {
					parent.Children = append(parent.Children, child)
				}
			}
		}
	}
	return
}
