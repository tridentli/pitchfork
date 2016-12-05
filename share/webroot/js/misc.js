"use strict";

function loadScript(url, callback)
{
	var head = document.getElementsByTagName("head")[0];
	var script = document.createElement("script");
	script.type = "text/javascript";
	script.src = url;

	// Then bind the event to the callback function.
	// There are several events for cross browser compatibility.
	script.onreadystatechange = callback;
	script.onload = callback;

	// Fire the loading
	head.appendChild(script);
}

function loadJSON(url, ok, fail)
{
	var x = new XMLHttpRequest();

	x.onreadystatechange = function()
	{
		if (x.readyState == XMLHttpRequest.DONE)
		{
			var hstatus = x.status;
			var txt = x.response;

			if (hstatus == 200)
			{
				ok && ok(txt);
			}
			else
			{
				fail && fail(hstatus, txt);
			}
		}
	}

	x.open("GET", url, true);
	x.responseType = 'json';
	x.send();
}

function ajax(url, data, ok, fail)
{
	var x = new XMLHttpRequest();

	x.onreadystatechange = function()
	{
		if (x.readyState == XMLHttpRequest.DONE)
		{
			var hstatus = x.status;
			var txt = x.responseText;

			if (hstatus == 200)
			{
				var msg = JSON.parse(txt);
				ok && ok(msg);
			}
			else
			{
				fail && fail(hstatus, txt);
			}
		}
	}

	x.open("POST", url, true);
	x.setRequestHeader("Content-Type", "application/json;charset=UTF-8");

	var json = JSON.stringify(data);
	x.send(json);

	return x;
}

function seturlparam(key, value, url) {
	if (!url) url = window.location.href;

	/* Split off '#' hashes */
	var hsh = url.indexOf("#");
	if (hsh !== -1)
	{
		hsh = url.substring(hsh);
		url = url.substring(0, hsh);
	}
	else
	{
		hsh = "";
	}

	var re = new RegExp("([?&])" + key + "=.*?(&|$)", "i");
	var sep = url.indexOf("?") !== -1 ? "&" : "?";

	if (url.match(re))
	{
		url = url.replace(re, "$1" + key + "=" + value + "$2");
	}
	else
	{
		url += sep + key + "=" + value;
	}

	url += hsh;

	return url;
}

function save_file(filename, type, contents)
{
	var blob = new Blob([contents], {type: type});
	var url = window.URL.createObjectURL(blob);

	var a = document.createElement("a");
	a.download = filename;
	a.innerHTML = "Download File";
	a.href = url;
	a.onclick = save_file_end;
	a.style.display = "none";
	document.body.appendChild(a);

	a.click();
}

function save_file_end(evt)
{
	document.body.removeChild(evt.target);
}

function fmt_time(d)
{
	var t = new Date(d).toUTCString();
	t = t.replace("GMT", "");
	t = t.replace("00:00:00", "");
	t = t.trim();
	return t;
}

/* Strips the day off so that only date + time remains */
function fmt_time_csv(d)
{
	var t = fmt_time(d);
	var i = t.indexOf(", ");
	t = t.substring(i+2);
	return t;
}

function fmt_number(x)
{
	x = Math.floor(x);
	return x.toLocaleString();
}
