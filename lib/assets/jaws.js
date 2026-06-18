// https://github.com/linkdata/jaws
//
// This script trusts the server to HTML escape user
// provided data before sending it. The script must not
// itself HTML-escape strings from the server, as the
// server needs to be able to inject arbitrary HTML.
//
// The script needs 'jawsKey' to be defined in a HTML
// meta tag. This is a per-request randomly generated
// key used to associate the WebSocket callback with
// the initial HTTP request.

var jaws = null;
var jawsIdPrefix = 'Jid.';
var jawsDebug = false;

function jawsContains(a, v) {
	return a.includes(String(v).trim().toLowerCase());
}

function jawsIsCheckable(v) {
	return jawsContains(['checkbox', 'radio'], v);
}

function jawsHasSelection(v) {
	return jawsContains(['text', 'search', 'url', 'tel', 'password'], v);
}

function jawsIsInputTag(v) {
	return jawsContains(['input', 'select', 'textarea'], v);
}

function jawsIsTrue(v) {
	return jawsContains(['true', 't', 'on', '1', 'yes', 'y', 'selected'], v);
}

function jawsShouldSet(currentValue, newValue) {
	if (typeof currentValue !== 'function') {
		if (currentValue !== newValue) {
			if (typeof currentValue === typeof newValue) {
				if (currentValue && newValue && typeof currentValue === 'object') {
					try {
						return JSON.stringify(currentValue) !== JSON.stringify(newValue);
					} catch {
						// ignored
					}
				}
			}
			return true;
		}
	}
	return false;
}

function jawsGetName(e) {
	let elem = e;
	while (elem != null) {
		let name = null;
		if (typeof elem.getAttribute === 'function') {
			name = elem.getAttribute('name');
		}
		if (name == null && String(elem.tagName || "").toLowerCase() === 'button') {
			name = elem.textContent;
		}
		if (name != null) {
			return name.replaceAll('\t', ' ');
		}
		elem = elem.parentElement || null;
	}
	return String(e?.id || "");
}

function jawsGetKeyState(e) {
	return (e.shiftKey ? 1 : 0) + (e.ctrlKey ? 2 : 0) + (e.altKey ? 4 : 0);
}

function jawsBuildClickData(elem, e) {
	let val = e.clientX +
		" " + e.clientY +
		" " + jawsGetKeyState(e) +
		" " + jawsGetName(elem);
	while (elem != null) {
		const elemId = String(elem.id || "");
		if (elemId.startsWith(jawsIdPrefix) && !jawsIsInputTag(elem.tagName)) {
			val += "\t" + elemId;
		}
		elem = elem.parentElement || null;
	}
	return val;
}

function jawsSendClickLike(what, e) {
	jaws.send(what + "\t\t" + JSON.stringify(jawsBuildClickData(e.target, e)) + "\n");
}

function jawsClickHandler(e) {
	if (jaws instanceof WebSocket && e instanceof Event) {
		e.stopPropagation();
		jawsSendClickLike("Click", e);
	}
}

function jawsIsInputOrigin(elem) {
	while (elem != null) {
		const tagName = String(elem.tagName || "").toLowerCase();
		if (tagName === 'option' || jawsIsInputTag(tagName)) {
			return true;
		}
		elem = elem.parentElement;
	}
	return false;
}

function jawsContextMenuHandler(e) {
	if (jaws instanceof WebSocket && e instanceof Event) {
		if (jawsIsInputOrigin(e.target)) {
			return;
		}
		e.stopPropagation();
		e.preventDefault();
		jawsSendClickLike("ContextMenu", e);
	}
}

function jawsInputHandler(e) {
	if (jaws instanceof WebSocket && e instanceof Event) {
		e.stopPropagation();
		let val;
		const elem = e.currentTarget;
		if (jawsIsCheckable(elem.getAttribute('type'))) {
			val = elem.checked;
		} else if (elem.tagName.toLowerCase() === 'option') {
			val = elem.selected;
		} else {
			val = elem.value;
		}
		jaws.send("Input\t" + elem.id + "\t" + JSON.stringify(val) + "\n");
	}
}

function jawsRemoving(topElem) {
	const elements = topElem.querySelectorAll('[id^="' + jawsIdPrefix + '"]');
	if (elements.length === 0) {
		return;
	}
	let val = '';
	for (let i = 0; i < elements.length; i++) {
		if (i > 0) {
			val += '\t';
		}
		val += elements[i].id;
	}
	jaws.send("Remove\t" + topElem.id + "\t" + JSON.stringify(val) + "\n");
}

function jawsAttach(elem) {
	if (elem.hasAttribute("data-jawsname")) {
		const name = elem.dataset.jawsname;
		window.jawsNames[name] = elem.id;
		if (elem.hasAttribute("data-jawsdata")) {
			jawsVar(name, JSON.parse(elem.dataset.jawsdata), 'Set');
		}
		return;
	}
	if (jawsIsInputTag(elem.tagName)) {
		elem.addEventListener('input', jawsInputHandler, false);
		return;
	}
	elem.addEventListener('click', jawsClickHandler, false);
	elem.addEventListener('contextmenu', jawsContextMenuHandler, false);
}

function jawsAttachChildren(topElem) {
	topElem.querySelectorAll('[data-jawsonchangesubmit]').forEach(elem => {
		elem.addEventListener('change', function() { this.form.submit(); });
	});
	topElem.querySelectorAll('[id^="' + jawsIdPrefix + '"]').forEach(jawsAttach);
	return topElem;
}

function jawsAlert(data) {
	const lines = data.split('\n');
	const type = lines.shift();
	const message = lines.join('\n');
	if (typeof bootstrap !== 'undefined') {
		const alertsElem = document.getElementById('jaws-alerts');
		if (alertsElem) {
			const wrapper = document.createElement('div');
			wrapper.innerHTML = '<div class="alert alert-' + type + ' alert-dismissible" role="alert">' + message +
				'<button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button></div>';
			alertsElem.append(wrapper);
			return;
		}
	}
	console.log("jaws: " + type + ": " + message);
}

function jawsList(idlist) {
	const elements = [];
	const idstrings = idlist.split(' ');
	for (let i = 0; i < idstrings.length; i++) {
		const elem = document.getElementById(idstrings[i]);
		if (elem) {
			elements.push(elem);
		}
	}
	return elements;
}

function jawsOrder(idlist) {
	const elements = jawsList(idlist);
	for (let i = 0; i < elements.length; i++) {
		elements[i].parentElement.appendChild(elements[i]);
	}
}

function jawsSetValue(elem, str) {
	const elemtype = elem.getAttribute('type');
	const tagName = elem.tagName.toLowerCase();
	if (jawsIsCheckable(elemtype)) {
		const checked = jawsIsTrue(str);
		if (elem.checked !== checked) {
			elem.checked = checked;
		}
		return;
	}
	if (tagName === 'option') {
		const selected = jawsIsTrue(str);
		if (elem.selected !== selected) {
			elem.selected = selected;
		}
		return;
	}
	if (elem.value === str) {
		return;
	}
	if (jawsHasSelection(elemtype) || tagName === 'textarea') {
		const ss = elem.selectionStart;
		const se = elem.selectionEnd;
		const oldVal = elem.value;
		let delta = str.indexOf(oldVal);
		elem.value = str;
		if (delta === -1) {
			delta = oldVal.indexOf(str);
			if (delta === -1) {
				return;
			}
			delta = -delta;
		}
		const valueLength = elem.value.length;
		elem.selectionStart = Math.max(0, Math.min(valueLength, ss + delta));
		elem.selectionEnd = Math.max(0, Math.min(valueLength, se + delta));
		return;
	}
	elem.value = str;
}

function jawsLost() {
	let delay = 1;
	let innerHTML = 'Server connection lost';
	if (jaws instanceof Date) {
		let elapsed = Math.floor((Date.now() - jaws) / 1000);
		if (elapsed > 0) {
			let units = ' second';
			delay = elapsed;
			if (elapsed >= 60) {
				delay = 60;
				units = ' minute';
				elapsed = Math.floor(elapsed / 60);
				if (elapsed >= 60) {
					units = ' hour';
					elapsed = Math.floor(elapsed / 60);
				}
			}
			if (elapsed > 1) {
				units += 's';
			}
			innerHTML += ' ' + elapsed + units + ' ago';
		}
	}
	innerHTML += '. Trying to reconnect.';
	let elem = document.getElementById('jaws-lost');
	if (elem === null) {
		elem = jawsElement('<div id="jaws-lost" class="jaws-lost">' + innerHTML + '</div>');
		document.body.prepend(elem);
		document.body.scrollTop = document.documentElement.scrollTop = 0;
	} else {
		elem.innerHTML = innerHTML;
	}
	setTimeout(jawsReconnect, delay * 1000);
}

function jawsHandleReconnect(e) {
	if (e.currentTarget.readyState === 4) {
		if (e.currentTarget.status === 204) {
			window.location.reload();
		} else {
			jawsLost();
		}
	}
}

function jawsReconnect() {
	const req = new XMLHttpRequest();
	req.open("GET", window.location.protocol + "//" + window.location.host + "/jaws/.ping", true);
	req.addEventListener('readystatechange', jawsHandleReconnect);
	req.send(null);
}

function jawsFailed() {
	if (jaws instanceof WebSocket) {
		jaws = new Date();
		jawsReconnect();
	}
}

function jawsUnloading() {
	if (jaws instanceof WebSocket) {
		jaws.removeEventListener('close', jawsFailed);
		jaws.removeEventListener('error', jawsFailed);
		jaws.close();
		jaws = null;
	}
}

function jawsElement(html) {
	const template = document.createElement('template');
	template.innerHTML = html;
	return template.content;
}

function jawsWhere(elem, pos) {
	let where = null;
	if (pos === 'null') {
		return null;
	}
	if (pos) {
		where = document.getElementById(pos);
		if (!(where instanceof Node) || where.parentElement !== elem) {
			where = elem.children[parseInt(pos, 10)];
		}
	}
	if (!(where instanceof Node)) {
		console.log("jaws: id " + elem.id + " has no position " + pos);
	}
	return where;
}

function jawsInsert(elem, data) {
	const idx = data.indexOf('\n');
	const pos = data.substring(0, idx);
	const where = jawsWhere(elem, pos);
	if (pos === 'null' || where instanceof Node) {
		elem.insertBefore(jawsAttachChildren(jawsElement(data.substring(idx + 1))), where);
	}
}

function jawsSetAttr(elem, data) {
	const idx = data.indexOf('\n');
	const attr = data.substring(0, idx);
	const val = data.substring(idx + 1);
	if (elem.getAttribute(attr) !== val) {
		elem.setAttribute(attr, val);
	}
}

function jawsMessage(e) {
	const orders = e.data.split('\n');
	for (let i = 0; i < orders.length; i++) {
		if (orders[i]) {
			const parts = orders[i].split('\t');
			// Isolate each order: the server batches independent element updates
			// into one frame, so a single failing order (e.g. targeting an element
			// a prior order removed) must not abandon the rest of the frame.
			try {
				jawsPerform(parts.shift(), parts.shift(), parts.shift());
			} catch (err) {
				console.error("jaws: " + orders[i] + ": " + err);
			}
		}
	}
}

function jawsWarnDirtyNoChange(id, operation) {
	if (jawsDebug) {
		console.warn("jaws: " + operation + " " + id + ": a jaws.Element was marked dirty but it generated the same HTML");
	}
}

function jawsVar(name, data, operation) {
	const keys = name.split('.').filter(key => key !== "");
	if (keys.length > 0) {
		let obj = window;
		const lastkey = keys[keys.length - 1];
		const path = keys.slice(1).join(".");
		name = keys[0];
		for (let i = 0; i < keys.length - 1; i++) {
			if (!Object.hasOwn(obj, keys[i])) {
				throw "jaws: object undefined: " + name;
			}
			obj = obj[keys[i]];
		}
		switch (operation) {
			case undefined:
				if (data === undefined) {
					data = obj[lastkey];
				} else {
					obj[lastkey] = data;
				}
				if (jaws instanceof WebSocket && jaws.readyState === 1) {
					const id = window.jawsNames[name];
					if (typeof id === 'string' && id.startsWith(jawsIdPrefix)) {
						jaws.send("Set\t" + id + "\t" + path + "=" + JSON.stringify(data) + "\n");
					}
				}
				return data;
			case 'Call':
				if (typeof obj[lastkey] === 'function') {
					obj[lastkey](data);
					return;
				}
				throw "jaws: not a function: " + name + path;
			case 'Set':
				if (jawsShouldSet(obj[lastkey], data)) {
					obj[lastkey] = data;
				}
				return data;
			default:
				throw "jaws: unknown operation: " + operation;
		}
	}
}

function jawsPerform(what, id, data) {
	let path = "";
	if (what === 'Set' || what === 'Call') {
		const equalPos = data.indexOf("=");
		if (equalPos > 0) {
			path = data.slice(0, equalPos);
		}
		data = data.slice(equalPos + 1);
	}
	data = JSON.parse(data);
	switch (what) {
		case 'Reload':
			window.location.reload();
			return;
		case 'Redirect':
			window.location.assign(data);
			return;
		case 'Alert':
			jawsAlert(data);
			return;
		case 'Order':
			jawsOrder(data);
			return;
	}
	const elem = document.getElementById(id);
	if (elem === null) {
		throw "jaws: element not found: " + id;
	}
	let where = null;
	switch (what) {
		case 'Inner':
			if (elem.innerHTML !== data) {
				jawsRemoving(elem);
				elem.innerHTML = data;
				jawsAttachChildren(elem);
			} else {
				jawsWarnDirtyNoChange(id, what);
			}
			return;
		case 'Value':
			jawsSetValue(elem, data);
			return;
		case 'Append':
			elem.appendChild(jawsAttachChildren(jawsElement(data)));
			return;
		case 'Replace':
			if (elem.outerHTML !== data) {
				jawsRemoving(elem);
				elem.replaceWith(jawsAttachChildren(jawsElement(data)));
			} else {
				jawsWarnDirtyNoChange(id, what);
			}
			return;
		case 'Delete':
			jawsRemoving(elem);
			elem.remove();
			return;
		case 'Remove':
			where = jawsWhere(elem, data);
			if (where instanceof Node) {
				jawsRemoving(where);
				elem.removeChild(where);
			}
			return;
		case 'Insert':
			jawsInsert(elem, data);
			return;
		case 'SAttr':
			jawsSetAttr(elem, data);
			return;
		case 'RAttr':
			elem.removeAttribute(data);
			return;
		case 'SClass':
			elem.classList.add(data);
			return;
		case 'RClass':
			elem.classList.remove(data);
			return;
		case 'Call':
			jawsVar(path, data, what);
			return;
		case 'Set':
			if (elem.dataset.jawsname) {
				jawsVar(elem.dataset.jawsname + "." + path, data, what);
			} else {
				console.log("jaws: id " + id + " is not a JsVar");
			}
			return;
	}
	throw "jaws: unknown operation: " + what;
}

function jawsPageshow(e) {
	if (e.persisted) {
		window.location.reload();
	}
}

function jawsConnect() {
	if (document.querySelector('meta[name="jawsDebug"]') !== null) {
		jawsDebug = true;
	}
	let wsScheme = 'ws://';
	if (window.location.protocol === 'https:') {
		wsScheme = 'wss://';
	}
	window.addEventListener('beforeunload', jawsUnloading);
	window.addEventListener('pageshow', jawsPageshow);
	jaws = new WebSocket(wsScheme + window.location.host + '/jaws/' + encodeURIComponent(document.querySelector('meta[name="jawsKey"]').content));
	jaws.addEventListener('message', jawsMessage);
	jaws.addEventListener('close', jawsFailed);
	jaws.addEventListener('error', jawsFailed);
}

window.jawsNames = {};
jawsAttachChildren(document);
if (document.readyState === 'complete' || document.readyState === 'interactive') {
	jawsConnect();
} else {
	window.addEventListener('DOMContentLoaded', jawsConnect);
}
