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

function jawstreeSetData(t, data) {
    var children = data.children || [];
    if (!t.options.multiSelectEnabled && t.options.cascadeSelectChildren) {
        children = jawstreeTopSelectedChildren(children, false);
    }
    t.setData(children);
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
            if (window["jawstree_"+treename].jawsApplyingSet) {
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
}
