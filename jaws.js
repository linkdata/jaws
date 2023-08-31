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

function jawsHandler(e) {
	if (jaws instanceof WebSocket && e instanceof Event) {
		var elem = e.currentTarget;
		var jid = elem.id.substring(4)
		if (jid) {
			var val = elem.value;
			if (jawsIsCheckable(elem.getAttribute('type'))) {
				val = elem.checked;
			} else if (elem.tagName.toLowerCase() === 'option') {
				val = elem.selected;
			}
			jaws.send(jid + "\n" + e.type + "\n" + val);
		}
	}
}

function jawsAttach(topElem) {
	var elements = topElem.querySelectorAll('[id^="Jid."]');
	for (var i = 0; i < elements.length; i++) {
		var elem = elements[i];
		if (jawsIsInputTag(elem.tagName)) {
			elem.addEventListener('input', jawsHandler, false);
		} else {
			elem.addEventListener('click', jawsHandler, false);
		}
	}
	return topElem;
}

function jawsAlert(type, message) {
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
	if (elem.value == str) {
		return
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
	var text = '<h2>Connection Lost</h2>';
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
			text += '<p>Contact lost ' + elapsed + units + ' ago.</p>';
		}
	}
	document.documentElement.innerHTML = text + '<p>Trying to reconnect.</p>';
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
	return template.content.firstChild;
}

function jawsWhere(elem, pos) {
	var where = null;
	if (pos && pos !== 'null') {
		where = document.getElementById('Jid.' + pos)
		if (where == null) {
			where = elem.children[parseInt(pos)];
		}
	}
	return where;
}

function jawsMessage(e) {
	var lines = e.data.split('\n');
	var jid = lines.shift();
	var what = lines.shift();
	var where = null;
	var data = null;
	switch (what) {
		case 'Reload':
			window.location.reload();
			return;
		case 'Redirect':
			window.location.assign(lines.shift());
			return;
		case 'Alert':
			jawsAlert(lines.shift(), lines.join('\n'));
			return;
		case 'Inner':
		case 'Value':
		case 'Append':
			data = lines.join('\n');
			break;
		case 'Remove':
			break;
		case 'Insert':
		case 'Replace':
		case 'SAttr':
			where = lines.shift();
			data = lines.join('\n');
			break;
		case 'RAttr':
			where = lines.shift();
			break;
		default:
			console.log("jaws: unknown operation: " + what);
			return;
	}
	jid = 'Jid.' + jid
	var elem = document.getElementById(jid);
	if (elem === null) {
		console.log("jaws: id not found: " + jid);
		return;
	}
	switch (what) {
		case 'Inner':
			elem.innerHTML = data;
			jawsAttach(elem);
			break;
		case 'Value':
			jawsSetValue(elem, data);
			break;
		case 'Remove':
			elem.remove();
			break;
		case 'Append':
			elem.appendChild(jawsAttach(jawsElement(data)));
			break;
		case 'Insert':
		case 'Replace':
			var target = jawsWhere(elem, where);
			if (target instanceof Node) {
				if (what === 'Replace') {
					elem.replaceChild(jawsAttach(jawsElement(data)), target);
				} else {
					elem.insertBefore(jawsAttach(jawsElement(data)), target);
				}
			} else {
				console.log("jaws: id " + jid + " has no position " + where);
			}
			break;
		case 'SAttr':
			elem.setAttribute(where, data);
			break;
		case 'RAttr':
			elem.removeAttribute(where);
			break;
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
	jaws.addEventListener('open', function () { jawsAttach(document); });
	jaws.addEventListener('message', jawsMessage);
	jaws.addEventListener('close', jawsFailed);
	jaws.addEventListener('error', jawsFailed);
}

if (document.readyState === 'complete' || document.readyState === 'interactive') {
	jawsConnect();
} else {
	window.addEventListener('DOMContentLoaded', jawsConnect);
}
