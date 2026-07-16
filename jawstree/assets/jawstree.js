function jawstreeForEachNode(path, node, fn) {
    fn(path, node);
    if (node.children) {
        var i;
        for (i = 0; i < node.children.length; i++) {
            jawstreeForEachNode(path+'.children.'+i, node.children[i], fn);
        }
    }
}

function jawstreeTopSelectedChildren(children, parentSelected) {
    var result = [];
    for (var i = 0; i < children.length; i++) {
        var node = children[i];
        var copy = {};
        for (var key in node) {
            if (Object.hasOwn(node, key) && key != 'children') {
                copy[key] = node[key];
            }
        }
        if (parentSelected) {
            delete copy.selected;
        }
        if (node.children) {
            copy.children = jawstreeTopSelectedChildren(node.children, parentSelected || Boolean(node.selected));
        }
        result.push(copy);
    }
    return result;
}

function jawstreeViewChildren(data, multiSelectEnabled, cascadeSelectChildren) {
    var children = data.children || [];
    if (!multiSelectEnabled && cascadeSelectChildren) {
        return jawstreeTopSelectedChildren(children, false);
    }
    return children;
}

function jawstreeSetData(t, data) {
    t.setData(jawstreeViewChildren(data, t.options.multiSelectEnabled, t.options.cascadeSelectChildren));
}

function jawstreeInit(arg) {
    window["jawstree_"+arg.tree] = jawstreeNew(
        arg.tree,
        window["jawstreeroot_"+arg.tree],
        arg.options
    );
}

function jawstreeSet(arg) {
    var t = window["jawstree_"+arg.tree];
    var wasApplyingSet = t.jawsApplyingSet;
    window["jawstreeroot_"+arg.tree] = arg.data;
    t.jawsApplyingSet = true;
    try {
        jawstreeSetData(t, arg.data);
    } finally {
        t.jawsApplyingSet = wasApplyingSet;
    }
}

function jawstreeSetPath(arg) {
    var t = window["jawstree_"+arg.tree];
    var selectedNodes = t.getSelectedNodes();
    var isSelected = selectedNodes.some(function(element) {
        return element.id == arg.id;
    });
    if (!t.options.multiSelectEnabled && arg.set && isSelected) {
        return;
    }
    if (arg.set || t.options.multiSelectEnabled || isSelected) {
        // selectNodeById fires Treeview's onSelectionChange synchronously. Suppress it
        // the same way jawstreeNew and jawstreeSet do, so reflecting a peer's selection
        // never re-enters onSelectionChange and echoes a jawsVar write back to the
        // server. The shadow model has already been patched by the preceding what.Set
        // broadcast; guarding here makes that correctness independent of Set-before-Call
        // delivery ordering rather than relying on it.
        var wasApplyingSet = t.jawsApplyingSet;
        t.jawsApplyingSet = true;
        try {
            t.selectNodeById(arg.id,arg.set);
        } finally {
            t.jawsApplyingSet = wasApplyingSet;
        }
    }
}

function jawstreeNew(treename, rootnode, options) {
    /*jslint bitwise: true */
    var multiSelectEnabled = options & (1<<2);
    var cascadeSelectChildren = options & (1<<7);
    /*jslint bitwise: false */
    var applyingSet = true;
    var tree;
    try {
        tree = new Treeview({
            containerId: treename,
            data: jawstreeViewChildren(rootnode, multiSelectEnabled, cascadeSelectChildren),
            /*jslint bitwise: true */
            searchEnabled: options & (1<<0),
            initiallyExpanded: options & (1<<1),
            multiSelectEnabled: multiSelectEnabled,
            showSelectAllButton: options & (1<<3),
            showInvertSelectionButton: options & (1<<4),
            showExpandCollapseAllButtons: options & (1<<5),
            nodeSelectionEnabled: !(options & (1<<6)),
            cascadeSelectChildren: cascadeSelectChildren,
            checkboxSelectionEnabled: options & (1<<8),
            /*jslint bitwise: false */
            onSelectionChange: function(selectedNodesData) {
                var tree = window["jawstree_"+treename];
                if (applyingSet || (tree && tree.jawsApplyingSet)) {
                    return;
                }
                jawstreeForEachNode("jawstreeroot_"+treename, window["jawstreeroot_"+treename], function(path, node) {
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
    } finally {
        applyingSet = false;
    }
    return tree;
}
