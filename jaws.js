// https://github.com/linkdata/jaws

var jaws = null;

function jawsContains(a, v) {
	return a.indexOf(String(v).trim().toLowerCase()) !== -1;
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

function jawsGetName(e) {
	var elem = e;
	while (elem != null) {
		var name = elem.getAttribute('name');
		if (name == null && elem.tagName.toLowerCase() === 'button') {
			name = elem.textContent;
		}
		if (name != null) {
			return name.replaceAll('\t', ' ');
		}
		elem = elem.parentElement;
	}
	return e.id;
}

function jawsClickHandler(e) {
	if (jaws instanceof WebSocket && e instanceof Event) {
		e.stopPropagation();
		var elem = e.target;
		var val = jawsGetName(elem);
		while (elem != null) {
			if (elem.id.startsWith('Jid.') && !jawsIsInputTag(elem.tagName)) {
				val += "\t" + elem.id;
			}
			elem = elem.parentElement;
		}
		jaws.send("Click\t\t" + JSON.stringify(val) + "\n");
	}
}

function jawsInputHandler(e) {
	if (jaws instanceof WebSocket && e instanceof Event) {
		e.stopPropagation();
		var val;
		var elem = e.currentTarget;
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
	var elements = topElem.querySelectorAll('[id^="Jid."]');
	if (elements.length == 0) return;
	var val = '';
	for (var i = 0; i < elements.length; i++) {
		if (i > 0) {
			val += '\t';
		}
		val += elements[i].id;
	}
	jaws.send("Remove\t" + topElem.id + "\t" + JSON.stringify(val) + "\n");
}

function jawsAttach(elem) {
	if (jawsIsInputTag(elem.tagName)) {
		elem.addEventListener('input', jawsInputHandler, false);
	} else {
		elem.addEventListener('click', jawsClickHandler, false);
	}
}

function jawsAttachChildren(topElem) {
	var elements = topElem.querySelectorAll('[id^="Jid."]');
	for (var i = 0; i < elements.length; i++) {
		jawsAttach(elements[i]);
	}
	return topElem;
}

function jawsAlert(data) {
	var lines = data.split('\n');
	var type = lines.shift();
	var message = lines.join('\n');
	if (typeof bootstrap !== 'undefined') {
		var alertsElem = document.getElementById('jaws-alerts');
		if (alertsElem) {
			var wrapper = document.createElement('div');
			wrapper.innerHTML = '<div class="alert alert-' + type + ' alert-dismissible" role="alert">' + message +
				'<button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button></div>';
			alertsElem.append(wrapper);
			return;
		}
	}
	console.log("jaws: " + type + ": " + message);
}

function jawsList(idlist) {
	var i;
	var elements = [];
	var idstrings = idlist.split(' ');
	for (i = 0; i < idstrings.length; i++) {
		var elem = document.getElementById(idstrings[i]);
		if (elem) {
			elem.dataset.jidsort = i;
			elements.push(elem);
		}
	}
	elements.sort(function (a, b) {
		return +a.dataset.jidsort - +b.dataset.jidsort;
	});
	for (i = 0; i < elements.length; i++) {
		delete elements[i].dataset.jidsort;
	}
	return elements;
}

function jawsOrder(idlist) {
	var i;
	var elements = jawsList(idlist);
	for (i = 0; i < elements.length; i++) {
		elements[i].parentElement.appendChild(elements[i]);
	}
}

function jawsSetValue(elem, str) {
	var elemtype = elem.getAttribute('type');
	if (jawsIsCheckable(elemtype)) {
		elem.checked = jawsIsTrue(str);
		return;
	}
	if (elem.tagName.toLowerCase() === 'option') {
		elem.selected = jawsIsTrue(str);
		return;
	}
	if (elem.tagName.toLowerCase() === 'textarea') {
		if (elem.textContent != str) {
			elem.textContent = str;
			// work around browser bug where textContent
			// and textLength go out of sync sometimes
			if (elem.textLength != elem.textContent.length) {
				var parent = elem.parentNode;
				var clone = document.createElement(elem.tagName);
				for (var i = 0; i < elem.attributes.length; i++) {
					clone.setAttribute(elem.attributes[i].nodeName, elem.attributes[i].nodeValue);
				}
				clone.textContent = str;
				parent.insertBefore(clone, elem);
				elem.remove();
				jawsAttach(clone);
			}
		}
		return;
	}
	if (elem.value == str) {
		return;
	}
	if (jawsHasSelection(elemtype)) {
		var ss = elem.selectionStart;
		var se = elem.selectionEnd;
		var oldVal = elem.value;
		var delta = str.indexOf(oldVal);
		elem.value = str;
		if (delta == -1) {
			delta = oldVal.indexOf(str);
			if (delta == -1) return;
			delta = -delta;
		}
		elem.selectionStart = ss + delta;
		elem.selectionEnd = se + delta;
		return;
	}
	elem.value = str;
}

function jawsLost() {
	var delay = 1;
	var innerHTML = 'Server connection lost';
	if (jaws instanceof Date) {
		var elapsed = Math.floor((new Date() - jaws) / 1000);
		if (elapsed > 0) {
			var units = ' second';
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
			if (elapsed > 1) units += 's';
			innerHTML += ' ' + elapsed + units + ' ago';
		}
	}
	innerHTML += '. Trying to reconnect.';
	var elem = document.getElementById('jaws-lost');
	if (elem == null) {
		elem = jawsElement('<div id="jaws-lost" class="jaws-alert">' + innerHTML + '</div>');
		document.body.prepend(elem);
		document.body.scrollTop = document.documentElement.scrollTop = 0;
	} else {
		elem.innerHTML = innerHTML;
	}
	setTimeout(jawsReconnect, delay * 1000);
}

function jawsHandleReconnect(e) {
	if (e.currentTarget.readyState == 4) {
		if (e.currentTarget.status == 204) {
			window.location.reload();
		} else {
			jawsLost();
		}
	}
}

function jawsReconnect() {
	var req = new XMLHttpRequest();
	req.open("GET", window.location.protocol + "//" + window.location.host + "/jaws/.ping", true);
	req.addEventListener('readystatechange', jawsHandleReconnect);
	req.send(null);
}

function jawsFailed(e) {
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
	var template = document.createElement('template');
	template.innerHTML = html;
	return template.content;
}

function jawsWhere(elem, pos) {
	var where = null;
	if (pos && pos !== 'null') {
		where = document.getElementById(pos);
		if (where == null) {
			where = elem.children[parseInt(pos)];
		}
	}
	if (!(where instanceof Node)) {
		console.log("jaws: id " + elem.id + " has no position " + pos);
	}
	return where;
}

function jawsInsert(elem, data) {
	var idx = data.indexOf('\n');
	var where = jawsWhere(elem, data.substring(0, idx));
	if (where instanceof Node) {
		elem.insertBefore(jawsAttachChildren(jawsElement(data.substring(idx + 1))), where);
	}
}

function jawsSetAttr(elem, data) {
	var idx = data.indexOf('\n');
	elem.setAttribute(data.substring(0, idx), data.substring(idx + 1));
}

function jawsMessage(e) {
	var orders = e.data.split('\n');
	var i;
	for (i = 0; i < orders.length; i++) {
		if (orders[i]) {
			var parts = orders[i].split('\t');
			jawsPerform(parts.shift(), parts.shift(), parts.shift());
		}
	}
}

function jawsGlobal(id, data, callit) {
	var keys = id.split('.');
	if (keys.length > 0) {
		var obj = window;
		var i;
		for (i = 0; i < keys.length - 1; i++) {
			obj = obj[keys[i]];
		}
		if (obj !== null) {
			if (callit) {
				return obj[keys[keys.length - 1]](data);
			} else {
				return (obj[keys[keys.length - 1]] = data);
			}
		}
	}
}

function jawsPerform(what, id, data) {
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
		case 'Call':
			jawsGlobal(id, data, true);
			return;
		case 'Set':
			jawsGlobal(id, data, false);
			return;
	}
	var elem = document.getElementById(id);
	if (elem === null) {
		console.log("jaws: id not found: " + id);
		return;
	}
	var where = null;
	switch (what) {
		case 'Inner':
			jawsRemoving(elem);
			elem.innerHTML = data;
			jawsAttachChildren(elem);
			break;
		case 'Value':
			jawsSetValue(elem, data);
			break;
		case 'Append':
			elem.appendChild(jawsAttachChildren(jawsElement(data)));
			break;
		case 'Replace':
			jawsRemoving(elem);
			elem.replaceWith(jawsAttachChildren(jawsElement(data)));
			break;
		case 'Delete':
			jawsRemoving(elem);
			elem.remove();
			break;
		case 'Remove':
			where = jawsWhere(elem, data);
			if (where instanceof Node) {
				jawsRemoving(where);
				elem.removeChild(where);
			}
			break;
		case 'Insert':
			jawsInsert(elem, data);
			break;
		case 'SAttr':
			jawsSetAttr(elem, data);
			break;
		case 'RAttr':
			elem.removeAttribute(data);
			break;
		case 'SClass':
			elem.classList.add(data);
			break;
		case 'RClass':
			elem.classList.remove(data);
			break;
		default:
			console.log("jaws: unknown operation: " + what);
			return;
	}
}

function jawsPageshow(e) {
	if (e.persisted) {
		window.location.reload();
	}
}

function jawsConnect() {
	var wsScheme = 'ws://';
	if (window.location.protocol === 'https:') {
		wsScheme = 'wss://';
	}
	window.addEventListener('beforeunload', jawsUnloading);
	window.addEventListener('pageshow', jawsPageshow);
	jaws = new WebSocket(wsScheme + window.location.host + '/jaws/' + encodeURIComponent(jawsKey));
	jaws.addEventListener('open', function () { jawsAttachChildren(document); });
	jaws.addEventListener('message', jawsMessage);
	jaws.addEventListener('close', jawsFailed);
	jaws.addEventListener('error', jawsFailed);
}

if (document.readyState === 'complete' || document.readyState === 'interactive') {
	jawsConnect();
} else {
	window.addEventListener('DOMContentLoaded', jawsConnect);
}
