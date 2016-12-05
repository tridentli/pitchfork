"use strict";

var p;
var ta;
var ts;
var d_s_btn;
var edits;
var editor;

function editor_save(evt)
{
	/* Don't actually submit the form */
	evt.preventDefault();

	/* Reset edits & disable the button */
	edits = 0
	d_s_btn.disabled = true;

	/* Export the Markdown ("text" format) */
	var md = editor.exportFile(null, "text");

	var msg = document.getElementById("message").value;

	var data = {message:msg, markdown: md };

	var url = seturlparam("s", "post");

	ajax(url, data, function(json) {
		console.log(json);
		if (!json.hasOwnProperty("Status") ||
		    !json.hasOwnProperty("Message"))
		{
			alert("Saving Failed!? : Invalid answer");
		}
		else
		{
			var s = json["Status"];
			var m = json["Message"];

			if (s == "ok")
			{
				/* Redirect to 'read' version of the page */
				window.location = seturlparam("s", "read");
			}
			else
			{
				alert("Saving Failed!? : " + m);
			}
		}
	}, function(hstatus, txt) {
		alert("Saving Failed!? : HTTP " + hstatus + ": " + txt)
	});
}

function editor_getlines(selection)
{
	var lines = selection.toString();
	if (!lines) return [];

	return lines.split('\n');
}

function editor_prefixlines(lines, prefix)
{
	return lines.map(function (line)
		{
			if (line == "") return line;
		       	return prefix + line;
       		});
}

function editor_replacelines(doc, sel, lines)
{
	var range = sel.getRangeAt(0);
	range.deleteContents();
	range.collapse(false);

	if (!lines) return;

	var fragment = doc.createDocumentFragment();
	lines.forEach(function (line) {
		fragment.appendChild(doc.createTextNode(line));
		fragment.appendChild(doc.createElement("br"));
	});

	range.insertNode(fragment.cloneNode(true));
}

function editor_surround(doc, selection, prefix, postfix, newlines)
{
	/* Insert the prefix */
	var range = selection.getRangeAt(0);
	if (newlines) range.insertNode(doc.createElement("br"));
	range.insertNode(doc.createTextNode(prefix));

	range.collapse(false);

	/* And the postfix */
	selection.removeAllRanges();
	selection.addRange(range);
	if (newlines) range.insertNode(doc.createElement("br"));
	range.insertNode(doc.createTextNode(postfix));
}

function editor_tool(evt)
{
	evt.preventDefault();

	var doc = editor.editorIframeDocument;
	var sel = doc.getSelection();

	// If no selection is made, nothing to do */
	if (sel.rangeCount === 0)
	{
		return;
	}

	switch (evt.target.id)
       	{
	case "tool-bold":
		editor_surround(doc, sel, "**", "**", false);
		break;

	case "tool-italic":
		editor_surround(doc, sel, "*", "*", false);
		break;

	case "tool-code":
		editor_surround(doc, sel, "```", "```", true);
		break;

	case "tool-h1":
		editor_surround(doc, sel, "# ", "", false);
		break;

	case "tool-h2":
		editor_surround(doc, sel, "## ", "", false);
		break;

	case "tool-h3":
		editor_surround(doc, sel, "### ", "", false);
		break;

	case "tool-h4":
		editor_surround(doc, sel, "#### ", "", false);
		break;

	case "tool-indent":
		var lin = editor_getlines(sel);
		var list = editor_prefixlines(lin, "\t");
		editor_replacelines(doc, sel, list);
		break;

	case "tool-quote":
		var lin = editor_getlines(sel);
		var list = editor_prefixlines(lin, "> ");
		editor_replacelines(doc, sel, list);
		break;

	case "tool-list":
		var lin = editor_getlines(sel);
		var list = editor_prefixlines(lin, "\t * ");
		editor_replacelines(doc, sel, list);
		break;
	}
}

function editor_script_loaded()
{
	/* Create a div where we will put our stuff in */
	var d = document.createElement("div");
	d.id = "wikiedit";

	/* Add the textarea to it */
	d.appendChild(ta);

	/* Create a div for the epiceditor */
	var d_e = document.createElement("div");

	var d_e_title = document.createElement("h2");
	d_e_title.innerHTML = "Markdown Editor"
	d_e.appendChild(d_e_title);

	var d_e_tools = document.createElement("div");
	d_e_tools.id = "tools";
	d_e_tools.addEventListener('click', function(evt) { editor_tool(evt); });

	var but = document.createElement("button");
	but.innerHTML = "B"
	but.title = "Make selection bold";
	but.id="tool-bold";
	d_e_tools.appendChild(but);

	but = document.createElement("button");
	but.innerHTML = "i"
	but.title = "Make selection italic";
	but.id="tool-italic";
	d_e_tools.appendChild(but);

	but = document.createElement("button");
	but.innerHTML = "Indent"
	but.title = "Indent selection";
	but.id="tool-indent";
	d_e_tools.appendChild(but);

	but = document.createElement("button");
	but.innerHTML = "* List"
	but.title = "Make selection a list";
	but.id="tool-list";
	d_e_tools.appendChild(but);

	but = document.createElement("button");
	but.innerHTML = "> Quote"
	but.title = "Quote Selection";
	but.id="tool-quote";
	d_e_tools.appendChild(but);

	but = document.createElement("button");
	but.innerHTML = "Code"
	but.title = "Mark selection as example Code";
	but.id="tool-code";
	d_e_tools.appendChild(but);

	but = document.createElement("button");
	but.innerHTML = "H1"
	but.title = "Header 1";
	but.id="tool-h1";
	d_e_tools.appendChild(but);

	but = document.createElement("button");
	but.innerHTML = "H2"
	but.title = "Header 2";
	but.id="tool-h2";
	d_e_tools.appendChild(but);

	but = document.createElement("button");
	but.innerHTML = "H3"
	but.title = "Header 3";
	but.id="tool-h3";
	d_e_tools.appendChild(but);

	but = document.createElement("button");
	but.innerHTML = "H4"
	but.title = "Header 34";
	but.id="tool-h4";
	d_e_tools.appendChild(but);

	d_e.appendChild(d_e_tools);

	var d_e_epic = document.createElement("div");
	d_e_epic.id = "epiceditor";
	d_e.appendChild(d_e_epic);


	/* Create a div for the epiceditor-preview */
	var d_p = document.createElement("div");

	var d_p_title = document.createElement("h2");
	d_p_title.innerHTML = "HTML Preview"
	d_p.appendChild(d_p_title);

	var d_p_epic = document.createElement("div");
	d_p_epic.id = "epiceditor-preview";

	/* Add it to our edit box */
	d_p.appendChild(d_p_epic);


	/* Create a div for the save button */
	var d_s = document.createElement("form");
	d_s.id = "wikiform";
	d_s.className = "styled_form";
	d_s.addEventListener("submit", function(evt) { editor_save(evt); });

	var d_s_txt = document.createElement("label");
	d_s_txt.htmlFor = "message"
	d_s_txt.innerHTML = "Edit summary:"
	d_s.appendChild(d_s_txt);

	var d_s_msg = document.createElement("input");
	d_s_msg.id = "message"
	d_s_msg.name = "message"
	d_s_msg.type = "text"
	d_s_msg.value = ""
	d_s_msg.required = "required"
	d_s_msg.minLength = "8"
	d_s_msg.size = "100";
	d_s.appendChild(d_s_msg);

	var d_s_hint = document.createElement("span");
	d_s_hint.className = "form_hint";
	d_s_hint.innerHTML = "Briefly describe your changes";
	d_s.appendChild(d_s_hint);

	d_s_btn = document.createElement("input");
	d_s_btn.type = "submit"
	d_s_btn.value = "Save Revision"
	d_s_btn.disabled = true;
	d_s.appendChild(d_s_btn);

	/* Add boxes to our div */
	d.appendChild(d_e);
	d.appendChild(d_p);
	d.appendChild(d_s);
	d.appendChild(ts);

	/* Add it all back */
	p.appendChild(d);

	var opts = {
		container: "epiceditor",
		textarea: "wikitext",
		basePath: "/css/epiceditor/themes/",
		clientSideStorage: true,
		localStorageName: "Trident",
		useNativeFullscreen: true,
		parser: marked,
		file: {
			name: window.location.href,
			defaultContent: "",
			autoSave: 1000
		},
		theme: {
			       base: "base/epiceditor.css",
			       preview: "preview/github.css",
			       editor: "editor/epic-light.css"
		       },
		button: {
				preview: true,
				fullscreen: true,
				bar: "auto",
			},
		focusOnLoad: true,
		shortcut: {
			modifier: 18,
			fullscreen: 70,
			preview: 80
		},
		string: {
				togglePreview: "Toggle Preview Mode",
				toggleEdit: "Toggle Edit Mode",
				toggleFullscreen: "Enter Fullscreen"
			},
		autogrow: true
	}

	editor = new EpicEditor(opts).load();

	edits = 0;

	editor.on("update", function () {
		edits++;
		if (edits > 1)
       		{
			d_s_btn.disabled = false;
		}
		document.querySelector("#epiceditor-preview").innerHTML = this.exportFile(null, "html");
	}).emit("update");

	window.addEventListener("beforeunload", function (e) {
		if (edits > 1)
		{
			var confirmationMessage = 'It looks like you have been editing something. ';
			confirmationMessage += 'If you leave before saving, your changes will be lost.';

			(e || window.event).returnValue = confirmationMessage;
			return confirmationMessage;
		}
		return "";
	});
}

function editor_setup()
{
	/* We can have javascripts, thus replace simple textarea with fancy area */

	var frm = document.getElementById("wikiform");
	p = frm.parentNode

	ta = document.getElementById("wikitext");
	ts = document.getElementById("wikisecurity");

	/* Remove the textarea */
	ta.parentNode.removeChild(ta);

	/* Remove the form */
	p.removeChild(frm);

	/* Hide the textarea */
	ta.style.display = "none"

	/* Start Loading */
	loadScript("/js/epiceditor.min.js", editor_script_loaded);
}
	    
/* Enable the Editor on pages that include this */
document.addEventListener("DOMContentLoaded", function(event) { editor_setup(); });

