package pitchfork

import (
	"bytes"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
	"github.com/shurcooL/highlight_go"
	"github.com/sourcegraph/syntaxhighlight"
	"io"
	"regexp"
	"strings"
)

/* Wrap blackfriday */
type PfRenderer struct {
	*blackfriday.HTMLRenderer
	toconly bool
}

func (rnd *PfRenderer) RenderNode(w io.Writer, node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
	if rnd.toconly {
		return blackfriday.Terminate
	}

	if node.Type == blackfriday.CodeBlock {
		if buf, ok := w.(*bytes.Buffer); ok {
			rnd.BlockCode(buf, node.Literal, string(node.Info))
		} else {
			var out bytes.Buffer
			rnd.BlockCode(&out, node.Literal, string(node.Info))
			w.Write(out.Bytes())
		}
		return blackfriday.GoToNext
	}

	return rnd.HTMLRenderer.RenderNode(w, node, entering)
}

/* Override blockcode */
func (rnd *PfRenderer) BlockCode(out *bytes.Buffer, text []byte, lang string) {
	doubleSpace(out)

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

func doubleSpace(out *bytes.Buffer) {
	if out.Len() > 0 {
		out.WriteByte('\n')
	}
}

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

func PfRender(markdown string, toconly bool) (html string) {
	/* Configure Black Friday */
	extensions := 0 |
		blackfriday.NoIntraEmphasis |
		blackfriday.Tables |
		blackfriday.FencedCode |
		blackfriday.Autolink |
		blackfriday.Strikethrough |
		blackfriday.HeadingIDs |
		blackfriday.BackslashLineBreak |
		blackfriday.HardLineBreak |
		blackfriday.TabSizeEight |
		blackfriday.Footnotes |
		blackfriday.AutoHeadingIDs

	/*
	 * Disabled:
	 * - blackfriday.SpaceHeadings |
	 */

	/* Flags to use */
	htmlFlags := 0 |
		blackfriday.UseXHTML |
		blackfriday.Smartypants |
		blackfriday.SmartypantsFractions |
		blackfriday.SmartypantsLatexDashes |
		blackfriday.NoreferrerLinks |
		blackfriday.NofollowLinks

	if toconly {
		htmlFlags += blackfriday.TOC
	}

	params := blackfriday.HTMLRendererParameters{
		Flags: htmlFlags,
	}
	rnd := &PfRenderer{
		HTMLRenderer: blackfriday.NewHTMLRenderer(params),
		toconly:      toconly,
	}

	/* The policy we use */
	p := bluemonday.UGCPolicy()

	/* We additionally allow code, div, span and a-hrefs blocks to have a CSS class */
	p.AllowAttrs("class").Matching(bluemonday.SpaceSeparatedTokens).OnElements("code", "span", "div", "a")

	/* Allow a target of _blank to be set for links */
	blank := regexp.MustCompile("^(_blank)$")
	p.AllowAttrs("target").Matching(blank).OnElements("a")

	/* Render the markdown to HTML using Black Friday */
	unsafe := blackfriday.Run([]byte(markdown), blackfriday.WithRenderer(rnd), blackfriday.WithExtensions(extensions))

	/* Sanitize the HTML with Blue Monday */
	html = string(p.SanitizeBytes(unsafe))

	/* The markdown is now in New Order HTML */
	return
}
