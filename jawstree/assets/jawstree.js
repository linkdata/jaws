// jawstree.js is the local JaWS adapter for the vendored Quercus.js treeview
// (treeview.js). It builds one Quercus Treeview per rendered TreeView element and
// keeps its selection in sync with the server-authoritative jawstree.Tree.
//
// Server -> client uses two verbs, both element-scoped JsCalls:
//   jawstreeInit({jid, options, data})   build the widget once
//   jawstreeSelection({jid, s:[idx...] | b:"<base64 bitmap>"})   absolute selection
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

// jawstreeSend delivers one Input frame to the server, returning whether it was
// actually sent (false when the socket is not open, e.g. before it connects).
function jawstreeSend(jid, data) {
    if (typeof jaws === 'undefined' || !jaws) {
        return false;
    }
    if (typeof jawsCanSend === 'function' && !jawsCanSend()) {
        return false;
    }
    jaws.send("Input\t" + jid + "\t" + data + "\n");
    return true;
}

// jawstreeReconcile drives the widget's DOM selection to the desired preorder-index
// Set using the public selectNodeById only. It re-reads the live selection before
// every step, because the vendored widget's selectNodeById has collateral effects a
// one-pass diff cannot predict: a single-select deselect clears the whole set, and a
// cascade select or deselect toggles every descendant. Each step therefore fixes one
// mismatch against the current DOM; the loop is bounded and stops as soon as it
// converges or a step makes no progress (e.g. a node the widget refuses to select).
// t.jawsReconciling is set so the resulting onSelectionChange callbacks do not echo
// back to the server.
function jawstreeReconcile(t, desired) {
    if (jawstreeSetsEqual(jawstreeSelectedIndexSet(t), desired)) {
        t.lastServerSet = desired;
        return;
    }
    t.jawsReconciling = true;
    try {
        var bound = 2 * t.jawsNodeCount + 2;
        for (var iter = 0; iter < bound; iter++) {
            var current = jawstreeSelectedIndexSet(t);
            var pick = -1;
            var select = false;
            // Prefer selecting a desired-but-missing node; otherwise deselect an
            // unwanted one. Re-reading current each pass accounts for the collateral.
            desired.forEach(function (idx) {
                if (pick < 0 && !current.has(idx)) {
                    pick = idx;
                    select = true;
                }
            });
            if (pick < 0) {
                current.forEach(function (idx) {
                    if (pick < 0 && !desired.has(idx)) {
                        pick = idx;
                    }
                });
            }
            if (pick < 0) {
                break; // converged
            }
            var id = t.jawsIdByIndex[pick];
            if (id === undefined) {
                break; // unmappable index (should not happen)
            }
            t.selectNodeById(id, select);
            if (jawstreeSetsEqual(current, jawstreeSelectedIndexSet(t))) {
                break; // no progress; avoid spinning on an unreachable target
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
    if (add.length === 0 && remove.length === 0) {
        t.lastServerSet = newSet;
        return;
    }
    var encoded = JSON.stringify({ d: { add: add, remove: remove } });
    if (encoded.length > jawstreeDeltaThreshold) {
        encoded = JSON.stringify({ b: jawstreeEncodeBitmap(newSet, t.jawsNodeCount) });
    }
    // Advance the baseline only when the frame was actually sent. If the socket is
    // not open yet the change is not lost: the next gesture re-diffs from the same
    // baseline and carries it, and a server push still reconciles the DOM.
    if (jawstreeSend(t.jawsJid, encoded)) {
        t.lastServerSet = newSet;
    }
}

// jawstreeGet resolves a widget only while its owning container is in the live DOM.
// Keeping the instance on that element lets the browser collect the complete
// Treeview graph when JaWS removes the container.
function jawstreeGet(jid) {
    var container = document.getElementById(jid);
    return container && container.jawsTreeview;
}

function jawstreeInit(arg) {
    var container = document.getElementById(arg.jid);
    if (!container) {
        return;
    }
    container.hidden = false;
    var modes = jawstreeDecodeOptions(arg.options);
    var index = jawstreeBuildIndex(arg.data);
    // applying suppresses the onSelectionChange that Quercus fires while applying the
    // initial selection from arg.data during construction, before the instance is set.
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
            var tt = jawstreeGet(arg.jid);
            if (applying || !tt || tt.jawsReconciling) {
                return;
            }
            jawstreeOnSelectionChange(tt, selectedNodesData);
        }
    });
    t.jawsJid = arg.jid;
    t.jawsModes = modes;
    t.jawsIdByIndex = index.idByIndex;
    t.jawsIndexById = index.indexById;
    t.jawsNodeCount = index.count;
    t.jawsReconciling = false;
    // One Tree may be rendered by several elements on a page. Each container owns
    // its widget, so removing the element also releases the only adapter reference.
    container.jawsTreeview = t;
    // Baseline the outgoing-delta reference to the selection Quercus applied from
    // arg.data, then re-enable the callback for genuine user actions.
    t.lastServerSet = jawstreeSelectedIndexSet(t);
    applying = false;
    return t;
}

function jawstreeSelection(arg) {
    var t = jawstreeGet(arg.jid);
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
