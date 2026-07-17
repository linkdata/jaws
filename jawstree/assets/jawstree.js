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

function jawstreeNodeID(path, node) {
    return typeof node.id == 'string' ? node.id : path;
}

function jawstreeSelectedIDs(data) {
    var selected = [];
    jawstreeForEachNode("", data, function(path, node) {
        if (node.selected) {
            selected.push(jawstreeNodeID(path, node));
        }
    });
    return selected;
}

function jawstreeApplySelection(data, selected) {
    jawstreeForEachNode("", data, function(path, node) {
        if (selected.indexOf(jawstreeNodeID(path, node)) != -1) {
            node.selected = true;
        } else {
            delete node.selected;
        }
    });
}

function jawstreeInit(arg) {
    var container = document.getElementById(arg.jid);
    if (container) {
        container.hidden = false;
    }
    window["jawstree_"+arg.key] = jawstreeNew(
        arg.key,
        arg.jid,
        window["jawstreeroot_"+arg.key],
        arg.options
    );
}

function jawstreeSet(arg) {
    var t = window["jawstree_"+arg.key];
    var currentVersion = t.jawsSelectionVersion || 0;
    var nextVersion = arg.selectionVersion || 0;
    var wasApplyingSet = t.jawsApplyingSet;
    if (nextVersion < currentVersion) {
        // A full update may have been rendered just before a newer browser
        // selection was accepted. Keep that newer selection while still applying
        // the full update's non-selection fields.
        jawstreeApplySelection(arg.data, jawstreeSelectedIDs(window["jawstreeroot_"+arg.key]));
    } else {
        t.jawsSelectionVersion = nextVersion;
    }
    window["jawstreeroot_"+arg.key] = arg.data;
    t.jawsApplyingSet = true;
    try {
        jawstreeSetData(t, arg.data);
    } finally {
        t.jawsApplyingSet = wasApplyingSet;
    }
}

function jawstreeSetSelection(arg) {
    var t = window["jawstree_"+arg.key];
    var currentVersion = t.jawsSelectionVersion || 0;
    if (arg.selectionVersion <= currentVersion) {
        return;
    }

    var selected = Array.isArray(arg.selected) ? arg.selected : [];
    var root = window["jawstreeroot_"+arg.key];
    jawstreeApplySelection(root, selected);

    // Quercus's ordinary single-select operation updates only selection classes
    // and checkboxes. Unlike setData, it preserves expansion and search state.
    var selectedID = null;
    for (var i = 0; i < selected.length; i++) {
        if (selected[i] !== "") {
            selectedID = selected[i];
        }
    }
    var selectedNodes = t.getSelectedNodes();
    var alreadySelected = selectedID !== null && selectedNodes.length == 1 && selectedNodes[0].id == selectedID;
    var wasApplyingSet = t.jawsApplyingSet;
    t.jawsSelectionVersion = arg.selectionVersion;
    t.jawsApplyingSet = true;
    try {
        if (selectedID === null) {
            selectedNodes.forEach(function(node) {
                t.selectNodeById(node.id, false);
            });
        } else if (!alreadySelected) {
            t.selectNodeById(selectedID, true);
        }
    } finally {
        t.jawsApplyingSet = wasApplyingSet;
    }
}

function jawstreeSetPath(arg) {
    var t = window["jawstree_"+arg.key];
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

function jawstreeNew(key, containerJid, rootnode, options) {
    /*jslint bitwise: true */
    var multiSelectEnabled = options & (1<<2);
    var cascadeSelectChildren = options & (1<<7);
    /*jslint bitwise: false */
    var applyingSet = true;
    var tree;
    try {
        tree = new Treeview({
            containerId: containerJid,
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
                var tree = window["jawstree_"+key];
                if (applyingSet || (tree && tree.jawsApplyingSet)) {
                    return;
                }
                jawstreeForEachNode("jawstreeroot_"+key, window["jawstreeroot_"+key], function(path, node) {
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
