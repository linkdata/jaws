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
    window["jawstree_"+arg.tree].setData(arg.data.children);
}

function jawstreeSetPath(arg) {
    var t = window["jawstree_"+arg.tree];
    if (arg.set || t.options.multiSelectEnabled) {
        t.selectNodeById(arg.id,arg.set);
    }
}

function jawstreeNew(treename, rootnode, options) {
    return new Treeview({
        containerId: treename,
        data: rootnode.children,
        /*jslint bitwise: true */
        searchEnabled: options & (1<<0),
        initiallyExpanded: options & (1<<1),
        multiSelectEnabled: options & (1<<2),
        showSelectAllButton: options & (1<<3),
        showInvertSelectionButton: options & (1<<4),
        showExpandCollapseAllButtons: options & (1<<5),
        nodeSelectionEnabled: !(options & (1<<6)),
        cascadeSelectChildren: options & (1<<7),
        checkboxSelectionEnabled: options & (1<<8),
        /*jslint bitwise: false */
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
