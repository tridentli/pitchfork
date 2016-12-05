package pitchfork

/*
 * $ go test trident.li/pitchfork/lib -run Misc -v
 * ok  	command-line-arguments	0.007s
 */

import (
	"testing"
)

func test_where(t *testing.T, path string, workdir string, gopath string, expected string) {
	result := where_strippath(path, workdir, gopath)

	if result != expected {
		t.Errorf("Expected %q to encode as %q but got %q", path, expected, result)
	} else {
		t.Logf("Expected %q to encode as %q and got %q", path, expected, result)
	}
}

func TestMisc_Where(t *testing.T) {
	type WhereTest struct {
		path     string
		workdir  string
		gopath   string
		expected string
	}

	tests := []WhereTest{
		{
			"/Users/jeroen/gocode/src/trident.li/pitchfork/lib/template.go",
			"/tmp",
			"/Users/jeroen/gocode/",
			"trident.li/pitchfork/lib/template.go",
		},
		{
			"/Users/jeroen/gocode/src/trident.li/pitchfork/lib/template.go",
			"/Users/jeroen/gocode/src/trident.li",
			"/something/else",
			"pitchfork/lib/template.go",
		},
		{
			"/var/lib/jenkins/workspace/gopath/src/trident.li/pitchfork/lib/db.go",
			"/",
			"/Users/jeroen/gocode/",
			"/var/lib/jenkins/workspace/gopath/src/trident.li/pitchfork/lib/db.go",
		},
	}

	for _, tst := range tests {
		test_where(t, tst.path, tst.workdir, tst.gopath, tst.expected)
	}
}

func test_outesc(t *testing.T, str string, exp string) {
	enc := OutEsc(str)

	if enc != exp {
		t.Errorf("Expected %q to encode as %q but got %q", str, exp, enc)
	}

	return
}

func TestMisc_OutEsc(t *testing.T) {
	a := []string{
		"test", "test",
		"Test", "Test",
		"Test Test", "Test Test",
		"Esc\x00ape", "Esc%00ape",
		"Bell\x07ringring", "Bell%07ringring",
		"Tab\x09", "Tab%09",
		"Del\x7fete", "Del%7Fete",
	}

	for i := 0; i < len(a); i += 2 {
		test_outesc(t, a[i], a[i+1])
	}
}

func test_url_ensureslash(t *testing.T, url string, expected string) {
	nurl := URL_EnsureSlash(url)

	if nurl != expected {
		t.Errorf("Expected %q to encode as %q but got %q", url, expected, nurl)
	} else {
		t.Logf("Expected %q to encode as %q and got %q", url, expected, nurl)
	}
}

func TestMisc_URL_EnsureSlash(t *testing.T) {
	type urlensure struct {
		url      string
		expected string
	}

	tests := []urlensure{
		{"", "/"},
		{"/", "/"},
		{"/one", "/one/"},
		{"/one/", "/one/"},
	}

	for _, tst := range tests {
		test_url_ensureslash(t, tst.url, tst.expected)
	}
}

func test_url_append(t *testing.T, url1 string, url2 string, expected string) {
	url := URL_Append(url1, url2)

	if url != expected {
		t.Errorf("Expected %q + %q to encode as %q but got %q", url1, url2, expected, url)
	} else {
		t.Logf("Expected %q + %q to encode as %q and got %q", url1, url2, expected, url)
	}
}

func TestMisc_URL_Append(t *testing.T) {
	type urlappend struct {
		url1     string
		url2     string
		expected string
	}

	tests := []urlappend{
		{"", "", "/"},
		{"/", "/two", "/two"},
		{"/one", "two", "/one/two"},
		{"/one", "/two", "/one/two"},
		{"/one/", "two", "/one/two"},
		{"/one/", "/two", "/one/two"},
	}

	for _, tst := range tests {
		test_url_append(t, tst.url1, tst.url2, tst.expected)
	}
}
