"use strict";

function search_parse(sb)
{
	var x = sb.x;

	/* Nothing to do */
	if (x == null) return;

	/* Check where we are */
	var txt = x.responseText;
	var len = txt.length;

	if (txt == null || len == 0)
	{
		/* Show that there are no results */
		var tr = document.createElement("tr");
		tr.className = "noresults"
		var td = document.createElement("td");
		var te = document.createTextNode("(no matches found)");
		td.appendChild(te);
		tr.appendChild(td);

		var win = sb.childNodes[sb.minnodes];
		var table = win.childNodes[0];
		var tbody = table.childNodes[0];
		tbody.appendChild(tr);
		return;
	}

	var o;
	var itm = document.createDocumentFragment();

	while (true)
	{
		/* Skip any white space when it is at the end */
		while (sb.y < len && (txt[sb.y] == ' ' || txt[sb.y] == '\n')) sb.y++;

		/* Find a new line */
		o = txt.indexOf("\n", sb.y);
		if (o == -1) break;

		/* Have a line */
		var obj = txt.substr(sb.y, (o - sb.y));

		var json = JSON.parse(obj);

		/* Table Row */
		var tr = document.createElement("tr");

		/* Source column */
		var td = document.createElement("td");
		var te = document.createTextNode(json["source"]);
		td.appendChild(te);
		tr.appendChild(td);

		/* Content column */
		var td = document.createElement("td");
		var a = document.createElement("a");
		a.href = json["link"];
		var te = document.createTextNode(json["title"]);
		a.appendChild(te);

		/*
		 * Need to block the mousedown event,
		 * otherwise we lose focus and then
		 * the click never reaches the link
		 */
		a.addEventListener("mousedown", function(evt) { evt.preventDefault(); evt.stopPropagation(); });
		td.appendChild(a);
		tr.appendChild(td);

		/* Add to docfrag */
		itm.appendChild(tr);

		/* Skip over what we parsed */
		sb.y = o;
	}

	/* Add them as a batch to the DOM */
	var win = sb.childNodes[sb.minnodes];
	var table = win.childNodes[0];
	var tbody = table.childNodes[0];
	tbody.appendChild(itm);
	return;
}

function search_fail(sb, hstatus, txt)
{
	/* Done, can't abort anymore */
	sb.x = null;

	/* Not logged in anymore, thus point to login page */
	if (hstatus == 401)
	{
		/*
		 * TODO Pop up javascript login to allow relogin
		 * without losing state
		 */
		location.href = "/login/?comeback=" + location.href;
		return;
	}
	else
	{
		/* Ignore other statuses */
	}
}

function search_do(sb, q)
{
	/* Cancel existing query */
	if (sb.x != null)
	{
		/* Abort any outstanding AJAX request */
		sb.x.abort();
		sb.x = null;
	}

	/* Empty / short search */
	if (q.length <= 2)
	{
		/* Close the window, there is no output yet */
		search_close(sb);
		return
	}

	/* Need to open the results window? */
	if (sb.childNodes.length == sb.minnodes)
	{
		var win = document.createElement("div");
		win.className = "searchboxresults";

		var table = document.createElement("table");
		var tbody = document.createElement("tbody");
		table.appendChild(tbody);
		win.appendChild(table);

		sb.appendChild(win);
	}

	/* Ensure the results list is empty */
	var win = sb.childNodes[sb.minnodes];
	var table = win.childNodes[0];
	var tbody = table.childNodes[0];
	table.removeChild(tbody);
	var tbody = document.createElement("tbody");
	table.appendChild(tbody);

	/* The CSRF token to use */
	var csrftoken = sb.elements[0].value;

	/* The URL we send queries to */
	var url = window.location.protocol + "//" +window.location.host + "/search/"
	url = seturlparam("qa", q, url)

	/* The AJAX request */
	var x = new XMLHttpRequest();

	/* Store the AJAX request so we can cancel it */
	sb.x = x;

	/* Our offset in the result */
	sb.y = 0;

	x.onreadystatechange = function()
	{
		if (x.readyState == XMLHttpRequest.DONE)
		{
			if (x.status == 200)
			{
				/* Parse the 'rest' */
				search_parse(sb);

				/* Done, can't abort anymore */
				sb.x = null;
			}
			else
			{
				/* Failed */
				var txt = x.responseText;
				search_fail(sb, x.status, txt);
			}
		}
	}

	/* Chunked encoding -> get results on screen ASAP */
	x.onprogress = function ()
	{
		/* Try to parse a bit more */
		if (x.status == 200)
		{
			search_parse(sb);
		}
	}

	/* Attempt to handle errors */
	x.onerror = function(e)
	{
		var txt = e.target.status;
		search_fail(sb, x.status, txt);
	}

	try
	{
		x.open("GET", url, true);
	}
	catch(err)
	{
		search_fail(sb, 503, err.message);
	}

	x.setRequestHeader("Accept", "application/json-seq");
	x.setRequestHeader("X-XSRF-TOKEN", csrftoken);
	x.send();
}

function search_close(sb)
{
	if (sb.childNodes.length > sb.minnodes)
	{
		sb.removeChild(sb.childNodes[sb.minnodes]);
	}
}

function search_setup()
{
	/* We can have javascripts, thus exploit it */
	var sb = document.getElementById("searchbox");
	if (!sb) return;

	/* use .lastChild, as when Debug is enabled we might have a Debug CSRF */
	var ip = sb.lastChild;

	/* Remember how many children there per default are */
	sb.minnodes = sb.childNodes.length;

	ip.addEventListener("keyup", function(evt) { search_do(sb, evt.target.value); });
	ip.addEventListener("blur", function(evt) { evt.preventDefault(); evt.stopPropagation(); search_close(sb); });
}

/* Load Search functions */
document.addEventListener("DOMContentLoaded", function(event) { search_setup(); });
