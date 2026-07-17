// jawstree.js is the local JaWS adapter for the vendored Quercus.js treeview
// (treeview.js). It builds one Quercus Treeview per rendered TreeView element and
// keeps its selection in sync with the server-authoritative jawstree.Tree.
//
// Server -> client uses two verbs, both element-scoped JsCalls:
//   jawstreeInit({key, jid, options, data})   build the widget once
//   jawstreeSelection({key, s:[idx...] | b:"<base64 bitmap>"})   absolute selection
// Client -> server sends one Input frame per interaction carrying either a delta
// {"d":{"add":[idx...],"remove":[idx...]}} of preorder node indices, or, when that
// would be large, an absolute bitmap {"b":"<base64>"}.
//
// Selection is reconciled with the public selectNodeById only; the widget is never
// rebuilt for a selection change, so the client's local expansion state survives.
// Node identity on the wire is the preorder index; the DOM data-id stays the
// positional path Quercus renders.

// jawstreeDeltaThreshold is the encoded-delta size (bytes) above which the adapter
// sends an absolute bitmap instead, keeping every frame within the jaws 32 KiB
// inbound WebSocket limit even for a select-all over a very large tree.
var jawstreeDeltaThreshold = 24 * 1024;

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

function jawstreeDecodeOptions(options) {
    /*jslint bitwise: true */
    return {
        searchEnabled: Boolean(options & (1 << 0)),
        initiallyExpanded: Boolean(options & (1 << 1)),
        multiSelectEnabled: Boolean(options & (1 << 2)),
        showSelectAllButton: Boolean(options & (1 << 3)),
        showInvertSelectionButton: Boolean(options & (1 << 4)),
        showExpandCollapseAllButtons: Boolean(options & (1 << 5)),
        nodeSelectionEnabled: !(options & (1 << 6)),
        cascadeSelectChildren: Boolean(options & (1 << 7)),
        checkboxSelectionEnabled: Boolean(options & (1 << 8))
    };
    /*jslint bitwise: false */
}

// jawstreeBuildIndex walks the wire tree in the same preorder the server uses,
// numbering root as 0 and each descendant in order, and returns the index<->id maps
// plus the total node count. The root carries no id and is never selectable.
function jawstreeBuildIndex(root) {
    var idByIndex = [];
    var indexById = {};
    var next = 0;
    (function walk(node) {
        idByIndex[next] = node.id;
        if (node.id !== undefined) {
            indexById[node.id] = next;
        }
        next++;
        if (node.children) {
            for (var i = 0; i < node.children.length; i++) {
                walk(node.children[i]);
            }
        }
    })(root);
    return { idByIndex: idByIndex, indexById: indexById, count: next };
}

// jawstreeSelectedIndexSet returns the currently selected nodes of t as a Set of
// preorder indices.
function jawstreeSelectedIndexSet(t) {
    var set = new Set();
    var nodes = t.getSelectedNodes();
    for (var i = 0; i < nodes.length; i++) {
        var idx = t.jawsIndexById[nodes[i].id];
        if (idx !== undefined) {
            set.add(idx);
        }
    }
    return set;
}

function jawstreeSetsEqual(a, b) {
    if (a.size !== b.size) {
        return false;
    }
    var equal = true;
    a.forEach(function (x) {
        if (!b.has(x)) {
            equal = false;
        }
    });
    return equal;
}

function jawstreeDepth(id) {
    if (!id) {
        return 0;
    }
    var depth = 0;
    var pos = id.indexOf('.');
    while (pos >= 0) {
        depth++;
        pos = id.indexOf('.', pos + 1);
    }
    return depth;
}

// jawstreeSortByDepth orders indices shallowest-first so that, in cascade mode,
// selecting or deselecting an ancestor subsumes its descendants before they are
// visited.
function jawstreeSortByDepth(t, indices) {
    indices.sort(function (a, b) {
        return jawstreeDepth(t.jawsIdByIndex[a]) - jawstreeDepth(t.jawsIdByIndex[b]);
    });
}

function jawstreeDomSelected(t, id) {
    var li = t.treeviewContainer.querySelector('[data-id="' + id + '"]');
    return Boolean(li) && li.classList.contains('selected');
}

function jawstreeBase64Encode(bytes) {
    var s = '';
    for (var i = 0; i < bytes.length; i++) {
        s += String.fromCharCode(bytes[i]);
    }
    return btoa(s);
}

function jawstreeEncodeBitmap(indexSet, count) {
    /*jslint bitwise: true */
    var bytes = new Uint8Array((count + 7) >> 3);
    indexSet.forEach(function (idx) {
        if (idx >= 0 && idx < count) {
            bytes[idx >> 3] |= (1 << (idx & 7));
        }
    });
    /*jslint bitwise: false */
    return jawstreeBase64Encode(bytes);
}

function jawstreeDecodeBitmap(b64, count) {
    /*jslint bitwise: true */
    var bin = atob(b64);
    var set = new Set();
    for (var i = 0; i < count; i++) {
        var byteIndex = i >> 3;
        var code = byteIndex < bin.length ? bin.charCodeAt(byteIndex) : 0;
        if (code & (1 << (i & 7))) {
            set.add(i);
        }
    }
    /*jslint bitwise: false */
    return set;
}

function jawstreeSend(jid, data) {
    if (typeof jaws === 'undefined' || !jaws) {
        return;
    }
    if (typeof jawsCanSend === 'function' && !jawsCanSend()) {
        return;
    }
    jaws.send("Input\t" + jid + "\t" + data + "\n");
}

// jawstreeReconcile drives the widget's DOM selection to the desired preorder-index
// Set using selectNodeById only, touching only mismatched nodes. It sets
// t.jawsReconciling so the resulting onSelectionChange callbacks do not echo back to
// the server.
function jawstreeReconcile(t, desired) {
    var current = jawstreeSelectedIndexSet(t);
    if (jawstreeSetsEqual(current, desired)) {
        t.lastServerSet = desired;
        return;
    }
    t.jawsReconciling = true;
    try {
        if (!t.jawsModes.multiSelectEnabled && !t.jawsModes.cascadeSelectChildren) {
            var target = -1;
            desired.forEach(function (idx) {
                target = idx;
            });
            if (target >= 0) {
                var id = t.jawsIdByIndex[target];
                if (id !== undefined && !current.has(target)) {
                    t.selectNodeById(id, true); // single-select: Quercus clears any previous
                }
                current.forEach(function (idx) {
                    if (idx !== target) {
                        var cid = t.jawsIdByIndex[idx];
                        if (cid !== undefined) {
                            t.selectNodeById(cid, false);
                        }
                    }
                });
            } else {
                current.forEach(function (idx) {
                    var cid = t.jawsIdByIndex[idx];
                    if (cid !== undefined) {
                        t.selectNodeById(cid, false);
                    }
                });
            }
        } else {
            var toDeselect = [];
            var toSelect = [];
            current.forEach(function (idx) {
                if (!desired.has(idx)) {
                    toDeselect.push(idx);
                }
            });
            desired.forEach(function (idx) {
                if (!current.has(idx)) {
                    toSelect.push(idx);
                }
            });
            jawstreeSortByDepth(t, toDeselect);
            jawstreeSortByDepth(t, toSelect);
            var k;
            for (k = 0; k < toDeselect.length; k++) {
                var did = t.jawsIdByIndex[toDeselect[k]];
                if (did !== undefined && jawstreeDomSelected(t, did)) {
                    t.selectNodeById(did, false);
                }
            }
            for (k = 0; k < toSelect.length; k++) {
                var sid = t.jawsIdByIndex[toSelect[k]];
                if (sid !== undefined && !jawstreeDomSelected(t, sid)) {
                    t.selectNodeById(sid, true);
                }
            }
        }
    } finally {
        t.jawsReconciling = false;
        t.lastServerSet = desired;
    }
}

// jawstreeOnSelectionChange reports a user selection change to the server as the
// delta versus the last applied server state, falling back to an absolute bitmap
// when the delta would be large.
function jawstreeOnSelectionChange(t, selectedNodesData) {
    var newSet = new Set();
    for (var i = 0; i < selectedNodesData.length; i++) {
        var idx = t.jawsIndexById[selectedNodesData[i].id];
        if (idx !== undefined) {
            newSet.add(idx);
        }
    }
    var add = [];
    var remove = [];
    newSet.forEach(function (idx) {
        if (!t.lastServerSet.has(idx)) {
            add.push(idx);
        }
    });
    t.lastServerSet.forEach(function (idx) {
        if (!newSet.has(idx)) {
            remove.push(idx);
        }
    });
    t.lastServerSet = newSet;
    if (add.length === 0 && remove.length === 0) {
        return;
    }
    var encoded = JSON.stringify({ d: { add: add, remove: remove } });
    if (encoded.length > jawstreeDeltaThreshold) {
        encoded = JSON.stringify({ b: jawstreeEncodeBitmap(newSet, t.jawsNodeCount) });
    }
    jawstreeSend(t.jawsJid, encoded);
}

function jawstreeInit(arg) {
    var container = document.getElementById(arg.jid);
    if (container) {
        container.hidden = false;
    }
    var modes = jawstreeDecodeOptions(arg.options);
    var index = jawstreeBuildIndex(arg.data);
    // applying suppresses the onSelectionChange that Quercus fires while applying the
    // initial selection from arg.data during construction, before window[...] is set.
    var applying = true;
    var t = new Treeview({
        containerId: arg.jid,
        data: jawstreeViewChildren(arg.data, modes.multiSelectEnabled, modes.cascadeSelectChildren),
        searchEnabled: modes.searchEnabled,
        initiallyExpanded: modes.initiallyExpanded,
        multiSelectEnabled: modes.multiSelectEnabled,
        showSelectAllButton: modes.showSelectAllButton,
        showInvertSelectionButton: modes.showInvertSelectionButton,
        showExpandCollapseAllButtons: modes.showExpandCollapseAllButtons,
        nodeSelectionEnabled: modes.nodeSelectionEnabled,
        cascadeSelectChildren: modes.cascadeSelectChildren,
        checkboxSelectionEnabled: modes.checkboxSelectionEnabled,
        onSelectionChange: function (selectedNodesData) {
            var tt = window["jawstree_" + arg.key];
            if (applying || !tt || tt.jawsReconciling) {
                return;
            }
            jawstreeOnSelectionChange(tt, selectedNodesData);
        }
    });
    t.jawsKey = arg.key;
    t.jawsJid = arg.jid;
    t.jawsModes = modes;
    t.jawsIdByIndex = index.idByIndex;
    t.jawsIndexById = index.indexById;
    t.jawsNodeCount = index.count;
    t.jawsReconciling = false;
    window["jawstree_" + arg.key] = t;
    // Baseline the outgoing-delta reference to the selection Quercus applied from
    // arg.data, then re-enable the callback for genuine user actions.
    t.lastServerSet = jawstreeSelectedIndexSet(t);
    applying = false;
    return t;
}

function jawstreeSelection(arg) {
    var t = window["jawstree_" + arg.key];
    if (!t) {
        return;
    }
    var desired;
    if (arg.b !== undefined) {
        desired = jawstreeDecodeBitmap(arg.b, t.jawsNodeCount);
    } else {
        desired = new Set(arg.s || []);
    }
    jawstreeReconcile(t, desired);
}
