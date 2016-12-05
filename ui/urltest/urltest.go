package urltest

/*
 * $ go test trident.li/pitchfork/ui -run UI_Main -v
 * ok   command-line-arguments  0.007s
 */

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"testing"
	pf "trident.li/pitchfork/lib"
	pu "trident.li/pitchfork/ui"
)

type URLTest struct {
	Desc     string      /* Description of the test (primarily for grepping to be able to find it) */
	Method   string      /* Request Method: GET, POST, HEAD, PUT, etc */
	Path     string      /* Request Path: The path of the request */
	Username string      /* Request Username: as who should the request run? */
	Header   http.Header /* Request Headers */
	BodyVals url.Values  /* Request Body Values Body of the request (or nil) */
	RC       int         /* Response: Expected HTTP return code (or 0 to ignore this check) */
	Positive []string    /* Response: Expected Regexps that must     be present in the body */
	Negative []string    /* Response: Expected Regexps that must NOT be present in the body */
}

func URLTest_404(path string) URLTest {
	return URLTest{"404Test" + path,
		"GET", path,
		"",
		nil,
		nil,
		http.StatusNotFound, []string{"Not Found"}, []string{}}

}

/* Fakes a HTTP request with all headers etc properly set for easy one-shot testing */
func Test_URL(t *testing.T, handler http.HandlerFunc, u URLTest) {
	fail := false

	pf.Logf("Test_URL(%s) %s %q", u.Desc, u.Method, u.Path)

	/* Make sure there is space for adding headers */
	if u.Header == nil {
		u.Header = make(http.Header)
	}

	/* The body, if any */
	var bd io.Reader = nil

	if len(u.BodyVals) > 0 {
		/* Need to add a CSRF token? */
		if u.Method == "POST" {
			tok, _ := pu.Csrf_token(u.Method, pf.Config.Http_host, u.Path, "", u.Username)

			u.BodyVals.Add(pu.CSRF_TOKENNAME, tok)
		}

		bd = bytes.NewBufferString(u.BodyVals.Encode())

		/* We are posting a form (otherwise Go's http ParseForm does not process it) */
		u.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	/* Fake a request */
	req, err := http.NewRequest(u.Method, u.Path, bd)
	if err != nil {
		t.Fatalf("[%s] %s %q: %s", u.Desc, u.Method, u.Path, err.Error())
		return
	}

	/* Fake remote IP + port */
	req.RemoteAddr = "192.0.2.34:56789"
	req.Host = pf.Config.Http_host

	/* Fake a few extra headers */
	u.Header.Set("User-Agent", pf.Config.UserAgent)
	u.Header.Set("Origin", "https://"+pf.Config.Http_host)
	u.Header.Set("Host", pf.Config.Http_host)

	/* Any user? */
	if u.Username != "" {
		/*
		 * Create a valid token for this user
		 * Note: we cheat heavily, we do not even check if
		 *       the user is in the database
		 */
		var token_claims pf.SessionClaims
		token_claims.UserDesc = "Fake Test User"
		token_claims.IsSysAdmin = false

		token, err := pf.Token_New("websession", u.Username, pf.TOKEN_EXPIRATIONMINUTES, &token_claims).Sign()
		if err != nil {
			t.Fatalf("[%s] %s %q: token creation failed %s", u.Desc, u.Method, u.Path, err.Error())
			return
		}

		/* Set the Cookie header */
		u.Header.Set("Cookie", pu.G_cookie_name+"="+token)
	}

	/* Set the headers */
	req.Header = u.Header

	/* a Closeable + ResponseWriter capable Recorder */
	rr := NewClosableRecorder()

	/* The handler for requests */
	h := http.HandlerFunc(handler)

	/* Call the http.Handler with our fake request + recorder */
	h.ServeHTTP(rr, req)

	t.Logf("[%s] %s %q [%d]", u.Desc, u.Method, u.Path, rr.Code)

	/* Status makes sense? (rc of 0 ignores this test) */
	if u.RC != 0 && rr.Code != u.RC {
		t.Errorf("[%s] %s %q: got %v want %v", u.Desc, u.Method, u.Path, rr.Code, u.RC)
		fail = true
	}

	/* Get the complete body */
	bbody, err := ioutil.ReadAll(rr.Body)
	if err != nil {
		t.Errorf("[%s] Failed to retrieve response body: %s [fatal]", u.Desc, err.Error())
		return
	}

	body := string(bbody)

	var ok bool

	/* Check for positive matches */
	for _, m := range u.Positive {
		ok, err = regexp.MatchString(m, body)
		if err != nil {
			t.Fatalf("[%s] %s %q [%d]: regexp %q is invalid", u.Desc, u.Method, u.Path, rr.Code, m)
		} else if !ok {
			t.Errorf("[%s] %s %q [%d]: POS[%q] but it was not in body", u.Desc, u.Method, u.Path, rr.Code, m)
			fail = true
		}
	}

	/* Check for negative matches */
	for _, m := range u.Negative {
		ok, err = regexp.MatchString(m, body)
		if err != nil {
			t.Fatalf("[%s] %s %q [%d]: regexp %q is invalid", u.Desc, u.Method, u.Path, rr.Code, m)
		} else if ok {
			t.Errorf("[%s] %s %q [%d]: NEG[%q] but it was in body", u.Desc, u.Method, u.Path, rr.Code, m)
			fail = true
		}
	}

	if fail {
		t.Errorf("[%s] Failed: Body: %v", u.Desc, body)
	}
}
