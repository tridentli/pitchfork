package pitchforkui

import (
	"net/http"
	"strconv"
	pf "trident.li/pitchfork/lib"
)

// HTTP Status Aliases for easy use
const (
	StatusMovedPermanently    = http.StatusMovedPermanently    /* 301 */
	StatusFound               = http.StatusFound               /* 302 */
	StatusSeeOther            = http.StatusSeeOther            /* 303 */
	StatusBadRequest          = http.StatusBadRequest          /* 400 */
	StatusUnauthorized        = http.StatusUnauthorized        /* 401 */
	StatusForbidden           = http.StatusForbidden           /* 403 */
	StatusNotFound            = http.StatusNotFound            /* 404*/
	StatusInternalServerError = http.StatusInternalServerError /* 500 */
	StatusNotImplemented      = http.StatusNotImplemented      /* 501 */
	StatusServiceUnavailable  = http.StatusServiceUnavailable  /* 503 */
)

// H_errmsgs renders Error messages.
//
// Human readable, not computer thus with 200 OK.
func H_errmsgs(cui PfUI, msg []string) {
	type Page struct {
		*PfPage
		Messages []string
	}

	cui.SetPageMenu(nil)

	p := Page{cui.Page_def(), msg}
	p.HeaderImg = pf.System_Get().HeaderImg
	cui.Page_show("misc/error.tmpl", p)
}

// H_errmsg renders an err as an error message
func H_errmsg(cui PfUI, err error) {
	H_errmsgs(cui, []string{err.Error()})
}

// H_errmsg renders a textual string as an error message
func H_errtxt(cui PfUI, txt string) {
	H_errmsgs(cui, []string{txt})
}

// H_error generates a error page with a non-200 HTTP response.
//
// nginx should catch these and replace them with a hardcoded page.
func H_error(cui PfUI, status int) {
	/* HTTP Error */
	cui.SetStatus(status)

	/* Show login page when the request is not authorized */
	if status == StatusUnauthorized {
		h_login(cui)
		return
	}

	type Page struct {
		*PfPage
		Messages []string
	}

	msg := http.StatusText(status)

	status_str := strconv.Itoa(status)

	/* Reply with "maintenance" */
	if status == StatusServiceUnavailable {
		msg = "System is under maintenance"
		status_str = ""
	} else {
		cui.AddCrumb("", "HTTP Error", "Error - HTTP "+status_str+" "+msg)
	}

	var msgs []string

	msgs = append(msgs, msg)

	cui.SetPageMenu(nil)

	p := Page{cui.Page_def(), msgs}
	cui.Page_show("misc/error.tmpl", p)

	/* Log - hiding errors just makes them invisible */
	cui.Errf("HTTP Error %s: %s for %s", status_str, msg, cui.GetFullPath())
}

// H_NoAccess renders a 403 Access Forbidden or 404 Not Found when the users is logged in
func H_NoAccess(cui PfUI) {
	if cui.IsLoggedIn() {
		cui.Errf("NoAccess: Logged In %#v", cui.TheUser())
		H_error(cui, StatusNotFound)
	} else {
		cui.Errf("NoAccess: Not Logged in")
		H_error(cui, StatusUnauthorized)
	}
}
