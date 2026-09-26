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
// Each store name has routes in attachment order; the last route handles jawsVar.
const jawsStores = new Map();
const jawsJidRx = /^[1-9]\d*$/;
const jawsMaxJid = '9223372036854775807';
const jawsArrayIndexRx = /^(0|[1-9]\d*)$/;
// Milliseconds after WebSocket failure before reconnect probing begins.
const jawsFailureGracePeriod = 5 * 1000;
// Milliseconds; bounds reconnect probes that never receive a network result.
const jawsReconnectTimeout = 10 * 1000;
// Minimum navigation age for a reconnect-triggered page reload.
const jawsReconnectReloadMinPageAge = 60 * 1000;

function jawsIsJid(v) {
	if (typeof v === 'string' && v.startsWith(jawsIdPrefix)) {
		const digits = v.slice(jawsIdPrefix.length);
		return jawsJidRx.test(digits) &&
			(digits.length < jawsMaxJid.length ||
				(digits.length === jawsMaxJid.length && digits <= jawsMaxJid));
	}
	return false;
}

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

function jawsCanSend() {
	return jaws instanceof WebSocket && jaws.readyState === 1;
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
		if (jawsIsJid(elemId) && !jawsIsInputTag(elem.tagName)) {
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
	if (jawsCanSend() && e instanceof Event) {
		if (jawsIsInputOrigin(e.target)) {
			return;
		}
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
	if (jawsCanSend() && e instanceof Event) {
		if (jawsIsInputOrigin(e.target)) {
			return;
		}
		e.stopPropagation();
		e.preventDefault();
		jawsSendClickLike("ContextMenu", e);
	}
}

function jawsInputHandler(e) {
	if (jawsCanSend() && e instanceof Event) {
		let val;
		const elem = e.currentTarget;
		if (!jawsIsJid(elem.id)) {
			return;
		}
		e.stopPropagation();
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

function jawsForgetStore(elem) {
	const name = elem.dataset && elem.dataset.jawsstore;
	const routes = jawsStores.get(name);
	if (routes) {
		const index = routes.findIndex(route => route.id === elem.id);
		if (index !== -1) {
			routes.splice(index, 1);
			if (routes.length === 0) {
				jawsStores.delete(name);
			}
		}
	}
}

function jawsManagedElements(topElem) {
	return topElem.querySelectorAll('[id^="' + jawsIdPrefix + '"]');
}

function jawsRemoving(topElem, retainedTopElem) {
	if (!jawsIsJid(topElem.id)) {
		return;
	}
	let retainedIds = null;
	if (retainedTopElem !== undefined) {
		retainedIds = new Set();
		const retainedElements = jawsManagedElements(retainedTopElem);
		for (let i = 0; i < retainedElements.length; i++) {
			if (jawsIsJid(retainedElements[i].id)) {
				retainedIds.add(retainedElements[i].id);
			}
		}
	}
	// Drop store routes from all old managed descendants. Attachment recreates
	// routes for retained Jids; only absent Jids are reported to the server. topElem
	// survives an Inner update, so callers that remove it drop its route themselves.
	const elements = jawsManagedElements(topElem);
	const ids = [];
	for (let i = 0; i < elements.length; i++) {
		if (jawsIsJid(elements[i].id)) {
			jawsForgetStore(elements[i]);
			if (retainedIds === null || !retainedIds.has(elements[i].id)) {
				ids.push(elements[i].id);
			}
		}
	}
	if (ids.length === 0) {
		return;
	}
	jaws.send("Remove\t" + topElem.id + "\t" + JSON.stringify(ids.join('\t')) + "\n");
}

function jawsAttach(elem, retainedStores) {
	if (!jawsIsJid(elem.id)) {
		return;
	}
	if (elem.hasAttribute("data-jawsstore")) {
		const name = elem.dataset.jawsstore;
		if (jawsStoreKeys(name).length !== 1) {
			throw "jaws: invalid JsVar name: " + name;
		}
		const routes = jawsStores.get(name) || [];
		const index = routes.findIndex(route => route.id === elem.id);
		const previous = retainedStores?.get(elem.id);
		const value = previous?.name === name ? previous.value :
			index !== -1 ? routes[index].value : JSON.parse(elem.dataset.jawsdata);
		if (index !== -1) {
			routes.splice(index, 1);
		}
		routes.push({ id: elem.id, value: value });
		jawsStores.set(name, routes);
		return;
	}
	if (jawsIsInputTag(elem.tagName)) {
		let eventName = 'input';
		if (String(elem.type).toLowerCase() === "number" && elem.hasAttribute("data-jawsnumber")) {
			eventName = 'change';
		}
		elem.addEventListener(eventName, jawsInputHandler, false);
		return;
	}
	elem.addEventListener('click', jawsClickHandler, false);
	elem.addEventListener('contextmenu', jawsContextMenuHandler, false);
}

function jawsAttachChildren(topElem, retainedStores) {
	jawsManagedElements(topElem).forEach(elem => jawsAttach(elem, retainedStores));
	topElem.querySelectorAll('[data-jawsonchangesubmit]').forEach(elem => {
		elem.addEventListener('change', function() { this.form.submit(); });
	});
	return topElem;
}

function jawsAlert(data) {
	const lines = data.split('\n');
	const type = lines.shift();
	const message = lines.join('\n');
	if (typeof bootstrap !== 'undefined') {
		const alertsElem = document.querySelector('[data-jaws-alerts]');
		if (alertsElem) {
			const wrapper = document.createElement('div');
			wrapper.innerHTML = '<div class="alert alert-' + type + ' alert-dismissible" role="alert">' + message +
				'<button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button></div>';
			alertsElem.append(wrapper.firstElementChild);
			return;
		}
	}
	console.log("jaws: " + type + ": " + message);
}

function jawsList(idlist) {
	const elements = [];
	const idstrings = idlist.split(' ');
	for (let i = 0; i < idstrings.length; i++) {
		if (!jawsIsJid(idstrings[i])) {
			continue;
		}
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
	// The reflected property normalizes missing and invalid input types to "text".
	const elemtype = elem.type || elem.getAttribute('type');
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
		// Selection preservation is best-effort and only handles changes where one
		// complete value remains a contiguous substring of the other.
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
	if (!(jaws instanceof Date)) {
		return;
	}
	let delay = 1;
	let innerHTML = 'Server connection lost';
	let elapsed = Math.floor((Date.now() - jaws) / 1000);
	if (elapsed > 0) {
		delay = Math.min(60, elapsed);
		let units = ' second';
		if (elapsed >= 60) {
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
	innerHTML += '. Trying to reconnect.';
	let elem = document.querySelector('[data-jaws-lost]');
	if (elem === null) {
		elem = jawsElement('<div data-jaws-lost class="jaws-lost">' + innerHTML + '</div>');
		document.body.prepend(elem);
		document.body.scrollTop = document.documentElement.scrollTop = 0;
	} else {
		elem.innerHTML = innerHTML;
	}
	setTimeout(jawsReconnect, delay * 1000);
}

function jawsHandleReconnect(e) {
	if (!(jaws instanceof Date)) {
		return;
	}
	// Reloading resets the navigation clock, so repeated failures enter the backoff.
	if (e.currentTarget.status === 204 && performance.now() >= jawsReconnectReloadMinPageAge) {
		window.location.reload();
		return;
	}
	jawsLost();
}

function jawsReconnect() {
	if (!(jaws instanceof Date)) {
		return;
	}
	const req = new XMLHttpRequest();
	req.open("GET", window.location.protocol + "//" + window.location.host + "/jaws/.ping", true);
	req.timeout = jawsReconnectTimeout;
	req.addEventListener('loadend', jawsHandleReconnect, { once: true });
	req.send(null);
}

function jawsFailed() {
	if (jaws instanceof WebSocket) {
		jaws = new Date();
		setTimeout(jawsReconnect, jawsFailureGracePeriod);
	}
}

function jawsUnloading() {
	if (jaws instanceof WebSocket) {
		jaws.removeEventListener('close', jawsFailed);
		jaws.removeEventListener('error', jawsFailed);
		jaws.close();
	}
	jaws = null;
}

function jawsElement(html) {
	const template = document.createElement('template');
	template.innerHTML = html;
	return template.content;
}

const jawsChildIndexRx = /^\d+$/;

function jawsChildByJid(elem, id) {
	let where = null;
	if (jawsIsJid(id)) {
		where = document.getElementById(id);
		if (!(where instanceof Node) || where.parentElement !== elem) {
			where = null;
		}
	}
	return where;
}

function jawsRemoveWhere(elem, id) {
	const where = jawsChildByJid(elem, id);
	if (!(where instanceof Node)) {
		console.log("jaws: id " + elem.id + " has no child " + id);
	}
	return where;
}

function jawsInsertWhere(elem, pos) {
	let where = jawsChildByJid(elem, pos);
	if (!(where instanceof Node) && jawsChildIndexRx.test(pos)) {
		where = elem.children[Number(pos)];
	}
	if (!(where instanceof Node)) {
		console.log("jaws: id " + elem.id + " has no position " + pos);
	}
	return where;
}

function jawsInsert(elem, data) {
	const idx = data.indexOf('\n');
	const pos = data.substring(0, idx);
	const where = jawsInsertWhere(elem, pos);
	if (where instanceof Node) {
		elem.insertBefore(jawsAttachChildren(jawsElement(data.substring(idx + 1))), where);
	}
}

function jawsSetAttr(elem, data) {
	const idx = data.indexOf('\n');
	const attr = data.substring(0, idx);
	const val = data.substring(idx + 1);
	if (attr.toLowerCase() === 'id') {
		throw "jaws: refusing to change reserved attribute 'id'";
	}
	if (elem.getAttribute(attr) !== val) {
		elem.setAttribute(attr, val);
	}
}

function jawsMessage(e) {
	const orders = e.data.split('\n');
	for (let i = 0; i < orders.length; i++) {
		if (orders[i]) {
			const parts = orders[i].split('\t');
			const what = parts.shift();
			// Isolate each order: the server batches independent element updates
			// into one frame, so a single failing order (e.g. targeting an element
			// a prior order removed) must not abandon the rest of the frame.
			try {
				jawsPerform(what, parts.shift(), parts.shift());
			} catch (err) {
				if (what === 'Patch') {
					jawsUnloading();
					window.location.reload();
					return;
				}
				console.error("jaws: " + orders[i] + ": " + err);
			}
		}
	}
}

function jawsWarnSameHTML(id, operation, effect) {
	if (jawsDebug) {
		console.warn("jaws: " + operation + " " + id + ": requested HTML matches the current serialized HTML; " + effect);
	}
}

function jawsStoreKeys(name) {
	if (typeof name !== 'string' || name === '' || name.length > 4096) {
		throw "jaws: invalid JsVar path";
	}
	const keys = name.split('.');
	for (const key of keys) {
		if (key === '' || /[=\x00-\x1f\x7f]/.test(key) ||
			key === '__proto__' || key === 'constructor' || key === 'prototype') {
			throw "jaws: reserved JsVar path component: " + key;
		}
	}
	return keys;
}

function jawsVar(name, data) {
	if (arguments.length > 2) {
		throw "jaws: jawsVar accepts at most two arguments";
	}
	const keys = jawsStoreKeys(name);
	const routes = jawsStores.get(keys[0]);
	const binding = routes && routes[routes.length - 1];
	if (binding === undefined) {
		return arguments.length === 1 ? undefined : false;
	}
	let obj = binding;
	let lastkey = 'value';
	for (let i = 1; i < keys.length; i++) {
		if (Array.isArray(obj) && (!jawsArrayIndexRx.test(lastkey) || Number(lastkey) >= obj.length)) {
			throw "jaws: invalid JsVar array index: " + name;
		}
		if (obj === null || !Object.hasOwn(obj, lastkey)) {
			throw "jaws: JsVar path undefined: " + name;
		}
		obj = obj[lastkey];
		lastkey = keys[i];
	}
	if (arguments.length === 1) {
		return obj != null && Object.hasOwn(obj, lastkey) ? obj[lastkey] : undefined;
	}
	if (!jawsCanSend() || obj === null || typeof obj !== 'object') {
		return false;
	}
	if (Array.isArray(obj) && (!jawsArrayIndexRx.test(lastkey) || Number(lastkey) >= obj.length)) {
		return false;
	}
	let encoded;
	try {
		encoded = JSON.stringify(data);
		if (encoded === undefined) {
			return false;
		}
		jaws.send("Proposal\t" + binding.id + "\t" + keys.slice(1).join('.') + "=" + encoded + "\n");
	} catch {
		return false;
	}
	obj[lastkey] = JSON.parse(encoded);
	return true;
}

function jawsPatch(id, path, encoded) {
	if (!jawsIsJid(id)) {
		throw "jaws: invalid Jid: " + id;
	}
	const elem = document.getElementById(id);
	if (elem === null || !elem.hasAttribute('data-jawsstore')) {
		return; // removed binding
	}
	const name = elem.dataset.jawsstore;
	const binding = jawsStores.get(name)?.find(route => route.id === id);
	if (binding === undefined) {
		return; // removed binding
	}
	const keys = path === '' ? [] : jawsStoreKeys(path);
	let obj = binding;
	let lastkey = 'value';
	for (const key of keys) {
		if (Array.isArray(obj) && (!jawsArrayIndexRx.test(lastkey) || Number(lastkey) >= obj.length)) {
			throw "jaws: invalid JsVar array index: " + path;
		}
		if (obj === null || !Object.hasOwn(obj, lastkey)) {
			throw "jaws: JsVar path undefined: " + path;
		}
		obj = obj[lastkey];
		lastkey = key;
	}
	if (obj === null || typeof obj !== 'object') {
		throw "jaws: JsVar path undefined: " + path;
	}
	if (Array.isArray(obj) && (!jawsArrayIndexRx.test(lastkey) || Number(lastkey) >= obj.length)) {
		throw "jaws: invalid JsVar array index: " + path;
	}
	if (encoded === '') {
		if (path === '' || Array.isArray(obj)) {
			throw "jaws: invalid JsVar deletion: " + path;
		}
		delete obj[lastkey];
		return;
	}
	obj[lastkey] = JSON.parse(encoded);
}

function jawsCall(path, data) {
	const keys = jawsStoreKeys(path);
	let obj = window;
	for (let i = 0; i < keys.length - 1; i++) {
		if (!Object.hasOwn(obj, keys[i])) {
			throw "jaws: function path undefined: " + path;
		}
		obj = obj[keys[i]];
	}
	const lastkey = keys[keys.length - 1];
	const fn = Object.hasOwn(obj, lastkey) ? obj[lastkey] : undefined;
	if (typeof fn !== 'function') {
		throw "jaws: not a function: " + path;
	}
	fn.call(obj, data);
}

function jawsPerform(what, id, data) {
	let path = "";
	if (what === 'Patch' || what === 'Call') {
		const equalPos = data.indexOf("=");
		if (equalPos < 0) {
			throw "jaws: missing path separator";
		}
		path = data.slice(0, equalPos);
		data = data.slice(equalPos + 1);
	}
	if (what === 'Patch') {
		jawsPatch(id, path, data);
		return;
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
	if (what === 'Call' && id === '') {
		jawsCall(path, data);
		return;
	}
	if (!jawsIsJid(id)) {
		throw "jaws: invalid Jid: " + id;
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
				jawsWarnSameHTML(id, what, "the DOM update was skipped");
			}
			return;
		case 'Value':
			jawsSetValue(elem, data);
			return;
		case 'Append':
			elem.appendChild(jawsAttachChildren(jawsElement(data)));
			return;
		case 'Replace':
			const replacement = jawsElement(data);
			// Retained routes carry their initial data-jawsdata, which can lag patches.
			const retainedStores = new Map();
			jawsManagedElements(replacement).forEach(node => {
				const name = node.dataset && node.dataset.jawsstore;
				const binding = jawsStores.get(name)?.find(route => route.id === node.id);
				if (binding) {
					retainedStores.set(node.id, { name: name, value: binding.value });
				}
			});
			if (jawsDebug && elem.outerHTML === data) {
				jawsWarnSameHTML(id, what, "the DOM node is still recreated");
			}
			// Drop name routes from all old managed nodes. Retained Jids keep their
			// server Elements, and attaching the fragment recreates their routes.
			jawsRemoving(elem, replacement);
			jawsForgetStore(elem);
			elem.replaceWith(jawsAttachChildren(replacement, retainedStores));
			return;
		case 'Delete':
			jawsRemoving(elem);
			jawsForgetStore(elem);
			elem.remove();
			return;
		case 'Remove':
			where = jawsRemoveWhere(elem, data);
			if (where instanceof Node) {
				jawsRemoving(where);
				jawsForgetStore(where);
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
			if (data.toLowerCase() === 'id') {
				throw "jaws: refusing to remove reserved attribute 'id'";
			}
			elem.removeAttribute(data);
			return;
		case 'SClass':
			elem.classList.add(data);
			return;
		case 'RClass':
			elem.classList.remove(data);
			return;
		case 'Call':
			jawsCall(path, data);
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
	window.addEventListener('pagehide', jawsUnloading);
	window.addEventListener('pageshow', jawsPageshow);
	jaws = new WebSocket(wsScheme + window.location.host + '/jaws/' + encodeURIComponent(document.querySelector('meta[name="jawsKey"]').content));
	jaws.addEventListener('message', jawsMessage);
	jaws.addEventListener('close', jawsFailed);
	jaws.addEventListener('error', jawsFailed);
}

function jawsConnectWhenReady() {
	window.removeEventListener('DOMContentLoaded', jawsConnectWhenReady);
	window.removeEventListener('load', jawsConnectWhenReady);
	jawsConnect();
}

jawsAttachChildren(document);
if (document.readyState === 'complete') {
	jawsConnect();
} else {
	window.addEventListener('DOMContentLoaded', jawsConnectWhenReady);
	window.addEventListener('load', jawsConnectWhenReady);
}
