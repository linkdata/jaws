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
		var val = elem.value;
		var elemtype = elem.getAttribute('type');
		if (jawsIsCheckable(elemtype)) {
			val = elem.checked;
		} else if (elem.tagName.toLowerCase() === 'option') {
			val = elem.selected;
		}
		jaws.send(elem.id + "\n" + e.type + "\n" + val);
	}
}

function jawsAttach(topElem) {
	var elements = topElem.querySelectorAll('[id]:not([id=""])');
	for (var i = 0; i < elements.length; i++) {
		var elem = elements[i];
		if (elem.id.indexOf('jaws-') !== 0) {
			var ddattr = elem.getAttribute('jaws');
			if (ddattr) {
				var evtypes = ddattr.split(',');
				for (var j = 0; j < evtypes.length; j++) {
					var evtype = evtypes[j].trim();
					if (evtype) elem.addEventListener(evtype, jawsHandler, false);
				}
			} else if (jawsIsInputTag(elem.tagName)) {
				elem.addEventListener('input', jawsHandler, false);
			} else {
				elem.addEventListener('click', jawsHandler, false);
			}
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
	if (elem.tagName.toLowerCase() === 'option') {
		elem.selected = jawsIsTrue(str);
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
	document.documentElement.innerHTML = text;
	setTimeout(jawsReconnect, delay * 1000);
}

function jawsPingHandler(e) {
	if (e.currentTarget.readyState == 4) {
		if (e.currentTarget.status == 200) {
			window.location.reload();
		} else {
			jawsLost();
		}
	}
}

function jawsReconnect(since) {
	var req = new XMLHttpRequest();
	req.addEventListener('readystatechange', jawsPingHandler);
	req.open("GET", window.location.protocol + "//" + window.location.host + "/jaws/ping", true);
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
	}
}

function jawsElement(html) {
	var template = document.createElement('template');
	html = html.trim();
	template.innerHTML = html;
	return template.content.firstChild;
}

function jawsWhere(elem, pos) {
	var where = null;
	if (pos && pos !== 'null') {
		where = document.getElementById(pos);
		if (where == null) {
			where = elem.children[parseInt(pos)];
		}
	}
	return where;
}

function jawsMessage(e) {
	var lines = e.data.split('\n');
	var cmd_or_element = lines.shift();
	var elem = document.getElementById(cmd_or_element);
	if (elem != null) {
		var what = lines.shift();
		var where = null;
		switch (what) {
			case 'inner':
				elem.innerHTML = lines.join('\n');
				jawsAttach(elem);
				return;
			case 'value':
				jawsSetValue(elem, lines.join('\n'));
				return;
			case 'remove':
				elem.remove();
				return;
			case 'insert':
				where = jawsWhere(elem, lines.shift());
				elem.insertBefore(jawsAttach(jawsElement(lines.join('\n'))), where);
				return;
			case 'append':
				elem.appendChild(jawsAttach(jawsElement(lines.join('\n'))));
				return;
			case 'replace':
				var replacePos = lines.shift();
				where = jawsWhere(elem, replacePos);
				if (where instanceof Node) {
					elem.replaceChild(jawsAttach(jawsElement(lines.join('\n'))), where);
				} else {
					console.log("jaws: element " + elem.id + " has no position " + replacePos);
				}
				return;
			case 'sattr':
				elem.setAttribute(lines.shift(), lines.join('\n'));
				return;
			case 'rattr':
				elem.removeAttribute(lines.shift());
				return;
		}
		console.log("jaws: unknown operation: " + what);
		return;
	}
	switch (cmd_or_element) {
		case ' reload':
			window.location.reload();
			return;
		case ' redirect':
			window.location.assign(lines.shift());
			return;
		case ' alert':
			jawsAlert(lines.shift(), lines.join('\n'));
			return;
	}
	console.log("jaws: unknown command or element: " + cmd_or_element);
}

function jawsConnect() {
	var wsScheme = "ws://";
	if (window.location.protocol === 'https:') {
		wsScheme = "wss://";
	}
	window.addEventListener('beforeunload', jawsUnloading);
	jaws = new WebSocket(wsScheme + window.location.host + "/jaws/" + encodeURIComponent(jawsKey));
	jaws.addEventListener('open', function () { jawsAttach(document); });
	jaws.addEventListener('message', jawsMessage);
	jaws.addEventListener('close', jawsFailed);
	jaws.addEventListener('error', jawsFailed);
}

if (document.readyState === "complete" || document.readyState === "interactive") {
	jawsConnect();
} else {
	window.addEventListener('DOMContentLoaded', jawsConnect);
}
