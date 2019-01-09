"use strict";
const search = RegExp("[?&]q=([^&]+)");
function setPath(crumbs, files, q, path, query) {
	if (document.location.pathname != path || document.location.search != query) {
		history.pushState({}, document.title, path + query);
		path = document.location.pathname;
	}

	document.body.classList.add("loading");
	window.scrollTo(0, 0);

	function a(sp, href, text, cls, rel) {
		let r = document.createElement("a");
		r.appendChild(document.createTextNode(text.replace(/_/g, " ")));
		r.setAttribute("href", href);
		if (rel) r.setAttribute("rel", rel);
		if (cls) r.classList.add(cls);
		if (sp)	r.addEventListener("click", function(e){
			e.preventDefault();
			setPath(crumbs, files, q, href, "");
			});
		return r;
	}
	function el(e, c) {
		let r = document.createElement(e);
		r.appendChild(c);
		return r;
	}

	path = path.replace(/\/\/+/g, "/").replace(/(^\/+)|(\/+$)/g, "")
	const p = (path)?path.split("/"):[];
	let s = search.exec(query)
	let f = document.createDocumentFragment();
	f.appendChild(el("li", a(true, "/", document.location.hostname)));
	let h = "/"
	for (let i = 0; i < p.length - (!s); i++) {
		h += p[i];
		f.appendChild(el("li", a(true, h, decodeURIComponent(p[i]))));
		h += "/";
	}

	if (s) {
		s = decodeURIComponent(s[1]);
		q.value = s;
		f.appendChild(el("li", el("span", document.createTextNode(s))));
	} else {
		q.value = "";
		f.appendChild(el("li", document.createTextNode(decodeURIComponent(p[p.length-1]||""))));
	}
	crumbs.innerHTML = "";
	crumbs.appendChild(f);

	const req = new XMLHttpRequest();
	req.onreadystatechange = function() {
		if (this.readyState != 4) return;
		document.body.classList.remove("loading");

		if (this.status != 200) {
			files.innerHTML="<li class=error>"+((this.status == 404)?"Not found":"Load failed")+"</li>";
			return;
		}

		let f = document.createDocumentFragment();
		if (p[0] && !s) f.appendChild(el("li", a(true, "/"+p.slice(0,-1).join("/"), "..", "u")));

		const json = JSON.parse(this.responseText || "[]");
		for (let i = 0; i < json.length; i++) {
			const n = json[i].name
			const p = path+encodeURIComponent(json[i].name);
			if ((json[i].type||"")[0] == "f")
				f.appendChild(el("li", a(false, "/dl/"+p, n, "f", "nofollow")));
			else
				f.appendChild(el("li", a(true, "/"+p, n, "d", "")));
		}

		if (f.childNodes.length) {
			files.innerHTML = "";
			files.appendChild(f);
		} else {
			files.innerHTML="<li class=error>No files found</li>";
		}
	};

	if (path) path += "/"
	req.open("GET", "/idx/" + path + query, true);
	req.send();
}
function onLoad() {
	const path   = document.getElementById("path");
	const files  = document.getElementById("files");
	const search = document.getElementById("search");
	const q      = document.getElementById("q");
	setPath(path, files, q, document.location.pathname, document.location.search);

	window.addEventListener("popstate", function(e) {
		setPath(path, files, q, document.location.pathname, document.location.search);
	});

	search.addEventListener("submit", function(e) {
		e.preventDefault();
		const s = q.value ? "?r=1&q=" + encodeURIComponent(q.value) : "";
		setPath(path, files, q, document.location.pathname, s);
	});
}
if (document.readyState === "loading")
	document.addEventListener("DOMContentLoaded", onLoad);
else
	onLoad();
