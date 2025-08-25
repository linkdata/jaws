function jawstreeForEachNode(path, node, fn) {
    fn(path, node);
    if (node.children) {
        var i;
        for (i = 0; i < node.children.length; i++) {
            jawstreeForEachNode(path+'.children.'+i, node.children[i], fn);
        }
    }
}

function jawstreeSet(arg) {
    window["jawstree_"+arg.tree].setData(arg.data);
}

function jawstreeSetPath(arg) {
    window["jawstree_"+arg.tree].selectNodeById(arg.id,arg.set);
}

function jawstreeNew(treename, rootnode) {
    return new Treeview({
        containerId: treename,
        data: rootnode.children,
        searchEnabled: true,
        initiallyExpanded: false,
        multiSelectEnabled: true,
        cascadeSelectChildren: false,
        onSelectionChange: function(selectedNodesData) {
            jawstreeForEachNode("jawstreeroot_"+treename, rootnode, function(path, node) {
                var selected = false;
                selectedNodesData.forEach(function(element) {
                    if (element.id == node.id) {
                        selected = true;
                    }
                });
                if (Boolean(node.selected) != selected) {
                    node.selected = selected;
                    jawsVar(path + ".selected", selected);
                }
            });
        }
    });
}
