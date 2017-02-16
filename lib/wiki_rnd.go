// Pitchfork Wiki Renderer
//
// The Pitchfork Renderer provides a standard way for rendering Markdown,
// as used in the wiki, into HTML.
//
// This so that it can be used for a variety of parts of the Wiki code.
//
// The markdown renderer uses:
// - blackfriday to render Markdown into HTML.
// - bluemonday to sanitize the HTML.
// - highlight_go and syntaxhighlight to do syntaxhighlighting of code examples.
package pitchfork

import (
	"bytes"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday"
	"github.com/shurcooL/highlight_go"
	"github.com/sourcegraph/syntaxhighlight"
	"regexp"
	"strings"
)

// PfRenderer wraps blackfriday so that we can extend it with extra functionality.
type PfRenderer struct {
	*blackfriday.Html
}

// BlockCode overrides blockcode rendering allowing specification
// of the programming language and proper hilighting
//
// This is a plugin to BlackFriday, and thus takes an output buffer,
// the text included in the markdown and the language this code is
// written in, thus allowing highlighting in the style of that language.
func (rnd *PfRenderer) BlockCode(out *bytes.Buffer, text []byte, lang string) {
	// Add a newline if we are not at the front.
	if out.Len() > 0 {
		out.WriteByte('\n')
	}

	/* Which language? */
	count := 0

	/* Try to find the first language */
	for _, elt := range strings.Fields(lang) {
		if elt[0] == '.' {
			continue
		}

		if len(elt) == 0 {
			continue
		}

		/* HTML5 language indicator */
		out.WriteString(`<pre><code class="language-`)
		attrEscape(out, []byte(elt))
		lang = elt
		out.WriteString(`">`)
		count++
		break
	}

	if count == 0 {
		out.WriteString("<pre><code>")
	}

	highlightedCode, err := highlightCode(text, lang)
	if err == nil {
		out.Write(highlightedCode)
	} else {
		out.WriteString("ERROR: " + err.Error())
		attrEscape(out, text)
	}

	out.WriteString("</code></pre>\n")
}

// highlightCode highlights code based on the given language.
//
// It takes the source text and the language as an input
// and outputs the highlighted code or an error, in case one occurs.
func highlightCode(src []byte, lang string) (highlightedCode []byte, err error) {
	var buf bytes.Buffer

	pfCSS := syntaxhighlight.DefaultHTMLConfig

	lang = strings.ToLower(lang)
	switch lang {
	case "go":
		/*
		 * highlight_go uses go/scanner to loop through code
		 * it then passes these tokens to syntaxhighlight to print them
		 * with better knowledge comes better output
		 */
		err = highlight_go.Print(src, &buf, syntaxhighlight.HTMLPrinter(pfCSS))
		break

	default:
		/* Anything else, let syntaxhighlight figure it out */
		err = syntaxhighlight.Print(syntaxhighlight.NewScanner(src), &buf, syntaxhighlight.HTMLPrinter(pfCSS))
		break
	}

	if err != nil {
		return nil, err
	}

	return buf.Bytes(), err
}

// escapeSingleChar HTML escapes a single character
func escapeSingleChar(char byte) (string, bool) {
	switch char {
	case '"':
		return "&quot;", true
	case '&':
		return "&amp;", true
	case '<':
		return "&lt;", true
	case '>':
		return "&gt;", true
	}
	return "", false
}

// attrEscape HTML escapes an attribute.
func attrEscape(out *bytes.Buffer, src []byte) {
	org := 0

	for i, ch := range src {
		entity, ok := escapeSingleChar(ch)
		if ok {
			if i > org {
				/* Copy all the normal characters since the last escape */
				out.Write(src[org:i])
			}

			org = i + 1
			out.WriteString(entity)
		}
	}

	if org < len(src) {
		out.Write(src[org:])
	}
}

// PfRender renders a markdown text into HTML in standard Pitchfork way.
// Optionally Table of Contents (TOC) only is rendered.
func PfRender(markdown string, toconly bool) (html string) {
	/* Configure Black Friday */
	extensions := 0 |
		blackfriday.EXTENSION_NO_INTRA_EMPHASIS |
		blackfriday.EXTENSION_TABLES |
		blackfriday.EXTENSION_FENCED_CODE |
		blackfriday.EXTENSION_AUTOLINK |
		blackfriday.EXTENSION_STRIKETHROUGH |
		blackfriday.EXTENSION_HEADER_IDS |
		blackfriday.EXTENSION_BACKSLASH_LINE_BREAK |
		blackfriday.EXTENSION_HARD_LINE_BREAK |
		blackfriday.EXTENSION_TAB_SIZE_EIGHT |
		blackfriday.EXTENSION_FOOTNOTES |
		blackfriday.EXTENSION_AUTO_HEADER_IDS

	/*
	 * Disabled:
	 * - blackfriday.EXTENSION_SPACE_HEADERS |
	 */

	/* Flags to use */
	htmlFlags := 0 |
		blackfriday.HTML_SKIP_STYLE |
		blackfriday.HTML_USE_XHTML |
		blackfriday.HTML_USE_SMARTYPANTS |
		blackfriday.HTML_SMARTYPANTS_FRACTIONS |
		blackfriday.HTML_SMARTYPANTS_LATEX_DASHES |
		blackfriday.HTML_NOREFERRER_LINKS |
		blackfriday.HTML_NOFOLLOW_LINKS

	if toconly {
		htmlFlags += blackfriday.HTML_TOC | blackfriday.HTML_OMIT_CONTENTS
	}

	rnd := &PfRenderer{Html: blackfriday.HtmlRenderer(htmlFlags, "", "").(*blackfriday.Html)}

	/* The policy we use */
	p := bluemonday.UGCPolicy()

	/* We additionally allow code, div, span and a-hrefs blocks to have a CSS class */
	p.AllowAttrs("class").Matching(bluemonday.SpaceSeparatedTokens).OnElements("code", "span", "div", "a")

	/* Allow a target of _blank to be set for links */
	blank := regexp.MustCompile("^(_blank)$")
	p.AllowAttrs("target").Matching(blank).OnElements("a")

	/* Render the markdown to HTML using Black Friday */
	unsafe := blackfriday.Markdown([]byte(markdown), rnd, extensions)

	/* Sanitize the HTML with Blue Monday */
	html = string(p.SanitizeBytes(unsafe))

	/* The markdown is now in New Order HTML */
	return
}
