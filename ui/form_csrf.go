// Pitchfork CSRF Strategy
//
// Every request is origin + referer checked
// when mismatch the user is logged out and a iptrk violation is recorded.
//
// We use a JWT encoding to avoid the need to track these CSRF tokens.
// This also means that we use them a lot. Noting that each JWT has its
// own type and thus one cannot use a CSRF JWT for a Session for instance.
//
// cui.GetArg()
//  - value from URL argument (?arg=value)
//  - no CSRF checks done
//
// ctx/cui.CmdOut() + ctx/cui.Cmd()
//  - variables provided and hopefully checked by caller
//  - no CRSF checks
//
// cui.FormValue()
//  - values from POST request Form (Not URL arguments)
//  - Full CSRF check (username, origin)

package pitchforkui

import (
	"encoding/json"
	"html/template"
	"strings"
	pf "trident.li/pitchfork/lib"
)

// Name of the token used for CSRF
const CSRF_TOKENNAME = "pfCSRF"

// init adds extra template functions for being able to add forms that are not pfform but that are csrf protected
func init() {
	pf.Template_FuncAdd("csrf_form", csrf_form)
	pf.Template_FuncAdd("csrf_form_param", csrf_form_param)
}

// CSRFClaims is the claims we use for CSRF
type CSRFClaims struct {
	pf.JWTClaims        // Standard JWT Claims
	Method       string `json:"method"` // Method used for the form
	Host         string `json:"host"`   // Host used for the form
	Path         string `json:"path"`   // Path used for the form
}

// csrf_form_param renders a CSRF-proofed HTML form including a custom parameter
func csrf_form_param(cui PfUI, url string, params string) template.HTML {
	/* Avoid GET urls */
	method := "post"

	/* The Form */
	o := "<form"

	if params != "" {
		o += " " + params
	}

	o += " method=\"" + method + "\""

	if url != "" {
		o += " action=\"" + pf.HE(url) + "\" "
	}

	o += ">\n"
	o += csrf_input(cui, url, method)
	return pf.HEB(o)
}

// csrf_form renders a HTML form header including the CSRF signature that needs to be included in a submitted form
func csrf_form(cui PfUI, url string) template.HTML {
	return csrf_form_param(cui, url, "class=\"styled_form\"")
}

// Csrf_token generates a CSRF token from the given parameters
func Csrf_token(method string, hostname string, path string, url string, username string) (string, string) {
	if method == "" {
		method = "post"
	}

	claims := &CSRFClaims{}
	claims.Method = method
	claims.Host = hostname
	claims.Path = path

	/* Does it not end in a slash? replace last component */
	cpl := len(claims.Path)
	if cpl > 0 && claims.Path[cpl-1] != '/' {
		i := strings.LastIndex(claims.Path, "/")
		if i != -1 {
			claims.Path = claims.Path[:i+1]
		}
	}

	if url == "" || url[0] == '?' {
		/* Use the FullPath */
	} else if url[0] != '/' {
		/* Append to the FullPath */
		claims.Path += url
	} else {
		/* Replace completely */
		claims.Path = url
	}

	/* CSRF Token is valid for an hour */
	token := pf.Token_New(CSRF_TOKENNAME, username, pf.TOKEN_EXPIRATIONMINUTES, claims)
	tok, err := token.Sign()
	if err != nil {
		pf.Errf("Token Signing failed: %s", err.Error())
		return "", ""
	}

	json, _ := json.Marshal(claims)

	return tok, string(json)
}

// csrf_input generates a HTML input field containing a CSRF token
func csrf_input(cui PfUI, url string, method string) (o string) {
	/* We might not have a user, eg login page */
	username := ""
	theuser := cui.TheUser()
	if theuser != nil {
		username = theuser.GetUserName()
	}

	tok, json := Csrf_token(method, cui.GetHTTPHost(), cui.GetFullPath(), url, username)

	o += "<input type=\"hidden\" name=\"" + CSRF_TOKENNAME + "\" value=\"" + tok + "\" />\n"

	/* Include the not-encoded key too, great for debugging as it avoids manual decodes ;) */
	if pf.Debug {
		o += "<input type=\"hidden\" name=\"" + CSRF_TOKENNAME + "debug\" value=\"" + pf.HE(json) + "\" />\n"
	}
	return
}

// csrf_Check verifies a CSRF token
func csrf_Check(cui PfUI, tok string) (ok bool) {
	/* Parse the provided token */
	claims := &CSRFClaims{}
	_, err := pf.Token_Parse(tok, CSRF_TOKENNAME, claims)
	if err != nil {
		cui.Errf("CSRF check failed: token:%q, error:%s", tok, err.Error())
		return false
	}

	/* Verify that the claims match the goal */

	/* Logged in -> username set, otherwise it is unset and empty */
	username := ""
	theuser := cui.TheUser()
	if theuser != nil {
		username = theuser.GetUserName()
	}

	if claims.Subject != username {
		cui.Errf("CSRF check failed: wrong user, %q (token) vs %q (provided)", claims.Subject, username)
		return false
	}

	httphost := cui.GetHTTPHost()
	if claims.Host != httphost {
		cui.Errf("CSRF check failed: wrong host, %q (token) vs %q (provided)", claims.Host, httphost)
		return false
	}

	/* Prefix match the URL, thus shorten it to the path in the claim */
	url := cui.GetFullPath()
	if len(url) > len(claims.Path) {
		url = url[0:len(claims.Path)]
	}
	if claims.Path != url {
		cui.Errf("CSRF check failed: wrong path, %q (token) vs %q (provided)", claims.Path, url)
		return false
	}

	return true
}
