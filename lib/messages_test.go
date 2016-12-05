package pitchfork

/*
 * $ go test trident.li/pitchfork/lib -run Msg -v
 * ok  	command-line-arguments	0.007s
 */

import (
	"testing"
)

func test(t *testing.T, path string, succeed bool) {
	/* Fake Ctx */
	ctx := testingctx()

	/* Configure Simple ModOpts */
	Msg_ModOpts(ctx, "test msg", "/", "/", 2, "Test")

	/* Call the function */
	err := Msg_PathValid(ctx, &path)

	ok := true
	if err != nil {
		ok = false
	}

	if (ok && succeed) || (!ok && !succeed) {
		return
	}

	to := "match"
	if !succeed {
		to = "NOT match"
	}

	did := "matched"
	if !ok {
		did = "did NOT match"
	}

	t.Errorf("Expected path '%s' to %s but it %s", path, to, did)

	return
}

func TestMsg_PathValid(t *testing.T) {
	/* Positive tests */
	test(t, "/", true)
	test(t, "/a/", true)
	test(t, "/aa/", true)
	test(t, "/b/", true)
	test(t, "/b/c/", true)
	test(t, "/b/c/d/", true)
	test(t, "/b/c/d-e-f01234567890/", true)

	/*
	 * Positive email addresses tests
	 * (the regex allows a lot more and also invalid email addresses)
	 */
	test(t, "/b/c/you@example.net/", true)
	test(t, "/b/c/you+there@example.net/", true)
	test(t, "/b/c/Your.Name@example.net/", true)
	test(t, "/b/c/Your.Name@example.net/", true)
	test(t, "/b/c/Your_Name@example.net/", true)

	/* Negative tests */
	test(t, "", false)
	test(t, "//", false)
	test(t, "neg", false)
	test(t, "neg/", false)
	test(t, "/neg", false)
	test(t, "/neg//", false)
}
