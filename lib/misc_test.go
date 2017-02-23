// Pitchfork Misc Testing.
package pitchfork

/*
 * $ go test trident.li/pitchfork/lib -run Misc -v
 * ok  	command-line-arguments	0.007s
 */

import (
	"testing"
)

// test_where tests the where_strippath() function
func test_where(t *testing.T, path string, workdir string, gopath string, expected string) {
	result := where_strippath(path, workdir, gopath)

	if result != expected {
		t.Errorf("Expected %q to encode as %q but got %q", path, expected, result)
	} else {
		t.Logf("Expected %q to encode as %q and got %q", path, expected, result)
	}
}

// TestMisc_Where tests the where_strippath with multiple positive and negative results.
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

// test_outesc tests the OutEsc() function.
func test_outesc(t *testing.T, str string, exp string) {
	enc := OutEsc(str)

	if enc != exp {
		t.Errorf("Expected %q to encode as %q but got %q", str, exp, enc)
	}

	return
}

// TestMisc_OutEsc() tests the OutEsc function.
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

// test_url_ensureslash tests the URL_EnsureSlash() function.
func test_url_ensureslash(t *testing.T, url string, expected string) {
	nurl := URL_EnsureSlash(url)

	if nurl != expected {
		t.Errorf("Expected %q to encode as %q but got %q", url, expected, nurl)
	} else {
		t.Logf("Expected %q to encode as %q and got %q", url, expected, nurl)
	}
}

// TestMisc_URL_EnsureSlash tests the URL_EnsureSlash with multiple inputs/outputs.
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

// test_url_append tests the URL_Append() function.
func test_url_append(t *testing.T, url1 string, url2 string, expected string) {
	url := URL_Append(url1, url2)

	if url != expected {
		t.Errorf("Expected %q + %q to encode as %q but got %q", url1, url2, expected, url)
	} else {
		t.Logf("Expected %q + %q to encode as %q and got %q", url1, url2, expected, url)
	}
}

// TestMisc_URL_Append tests the URL_Append() function against multiple inputs/outputs.
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

func ExampleSplitArgs() {
	// Produces "FirstArgument" "Second Argument" "Third Argument" "Fourth Argument"
	SplitArgs("FirstArgument \"Second Argument\" \"Third Argument\" \"Fourth Argument\"")

	// Produces "FirstArgument" "Second Argument" "FourthArgument"
	SplitArgs("FirstArgument 'Second Argument' FourthArgument")

	// Produces "First" "There is a ' in the middle"
	SplitArgs("First \"There is a ' in the middle\"")
}

/* ExampleTrackTime provides an example of TrackTime and TrackStart usage */
func ExampleTrackTime() {
	fmt.Printf("Example - start\n")

	t1 := TrackStart()
	for i := 0; i < 10; i++ {
		fmt.Printf("Example - loop %d", i)
	}

	te := TrackTime(t1, "Example")
	fmt.Printf("Example - took: %s", te)
}

func ExampleTrackTime_deferred() {
	fmt.Printf("Example - start\n")

	defer TrackTime(TrackStart(), ThisFunc()+":Time Check")

	for i := 0; i < 10; i++ {
		fmt.Printf("Example - loop %d", i)
	}

	/*
		         * Now the function ends, the defered functions are called
			          * which causes the TrackTime function to report the result.
	*/
}
func ExampleSortKeys() {
	tbl := map[string]string{
		"two":   "Two",
		"three": "Three",
		"one":   "One",
		"four":  "Four",
	}

	keys := SortKeys(tbl)

	for _, key := range keys {
		fmt.Printf("%q = %q", key, tbl[key])
	}
}
