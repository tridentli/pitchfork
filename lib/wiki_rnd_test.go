package pitchfork

import (
	"strings"
	"testing"
)

func TestPfRenderBody(t *testing.T) {
	md := "# Heading One\n\nSome **bold** text and ~~strikethrough~~.\n\n```go\npackage main\n```\n"
	out := PfRender(md, false)

	if !strings.Contains(out, "<h1 id=\"heading-one\">Heading One</h1>") {
		t.Errorf("Expected heading in body output, got %q", out)
	}
	if !strings.Contains(out, "<strong>bold</strong>") {
		t.Errorf("Expected bold in body output, got %q", out)
	}
	if !strings.Contains(out, "<del>strikethrough</del>") {
		t.Errorf("Expected strikethrough in body output, got %q", out)
	}
	if !strings.Contains(out, `<code class="language-go">`) {
		t.Errorf("Expected highlighted go code block in body output, got %q", out)
	}
}

func TestPfRenderTOCOnly(t *testing.T) {
	md := "# Heading One\n\nBody paragraph that should be omitted in TOC.\n\n## Heading Two\n"
	toc := PfRender(md, true)

	if !strings.Contains(toc, "Heading One") || !strings.Contains(toc, "Heading Two") {
		t.Errorf("Expected headings in TOC output, got %q", toc)
	}
	if strings.Contains(toc, "Body paragraph") {
		t.Errorf("Expected body paragraph to be omitted in TOC-only output, got %q", toc)
	}
}
