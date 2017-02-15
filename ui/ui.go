package pitchforkui

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	pf "trident.li/pitchfork/lib"
)

/* Constants */
var ErrNotPOST = errors.New("Not a POST request")
var ErrInvalidCSRF = errors.New("Invalid CSRF Token")
var ErrMissingValue = errors.New("Missing value")

// PfNewUI is an overridable NewUI constructor rototype
type PfNewUI func() (cui PfUI)

// PfUIMenuI is an overridable UIMenu prototype
type PfUIMenuI func(cui PfUI, menu *PfUIMenu)

// PfUILoginI is an overridable UILogin prototype
type PfUILoginI func(cui PfUI, lp *PfLoginPage) (p interface{}, err error)

// PfUI interface, so that it can be extended
type PfUI interface {
	pf.PfCtx
	PfUIi
}

// PfUIi interface containing most of the internals
type PfUIi interface {
	UIInit(w http.ResponseWriter, r *http.Request) (err error)
	FormValue(key string) (val string, err error)
	FormValueM(key string) (vals []string, err error)
	FormValueNoCSRF(key string) (val string, err error)
	GetMethod() (method string)
	IsGET() (isget bool)
	IsPOST() (ispost bool)
	GetHTTPHost() string
	GetHTTPHeader(name string) (val string)
	GetPath() (path []string)
	GetPathString() (path string)
	SetPath(path []string)
	GetSubPath() (path string)
	SetSubPath(path string)
	GetFullPath() (path string)
	GetFullURL() (path string)
	GetArg(key string) (val string)
	GetArgCSRF(key string) (val string)
	ParseClientIP(remaddr string, xff string, xffc []*net.IPNet) (ip net.IP, addr string, err error)
	NoSubs() bool
	SetClientIP() (err error)
	SetBearerAuth(t bool)
	AddHeader(h string, v string)
	SetHeader(h string, v string)
	Flush()
	SetExpires(minutes int)
	SetContentType(ctype string)
	SetFileName(fname string)
	SetStaticFile(file string)
	SetRaw(raw []byte)
	SetJSON(json []byte)
	JSONAnswer(status string, message string)
	AddCrumb(link string, desc string, long string)
	DelCrumb() (link string, desc string, long string)
	GetCrumbPath() (path string)
	GetCrumbParts() (path []string)
	Page_def() (p *PfPage)
	Page_show(name string, data interface{})
	page_render(w http.ResponseWriter)
	SetRedirect(path string, status int)
	GetBody() (body []byte)
	GetFormFileReader(key string) (file io.ReadCloser, filename string, err error)
	GetFormFile(key string, maxsize string, b64 bool) (val string, err error)
	QueryArgSet(q string) (ok bool)
	HandleForm(cmd string, args []string, obj interface{}) (msg string, err error)
	HandleFormS(cmd string, autoop bool, args []string, obj interface{}) (msg string, err error)
	HandleCmd(cmd string, args []string) (msg string, err error)
	SetPageMenu(menu *PfUIMenu)
	MenuPath(menu PfUIMenu, path *[]string)
	UIMenu(menu PfUIMenu)
	InitToken()
}

// PfUIS is the standard implementation of PfUI
type PfUIS struct {
	pf.PfCtx                                 /* Pitchfork Context (not really embedded, a pointer) */
	http_host            string              /* HTTP Host */
	path                 []string            /* URL Path */
	subpath              string              /* Sub path */
	fullpath             string              /* Full path */
	args                 url.Values          /* Arguments */
	r                    *http.Request       /* The HTTP request as */
	w                    http.ResponseWriter /* Our output channel */
	hasflushed           bool                /* Did we Flush() out our headers? */
	token_exp            bool                /* Is the Token going to expire soon? */
	token_recv           string              /* The token received by the UI */
	bearer_auth          bool                /* Cookie or Bearer auth? */
	redirect             string              /* Redirection URL */
	show_name            string              /* The name of the template we are going to show */
	show_data            interface{}         /* Custom template data (PfPage based) */
	staticfile           string              /* Static file to return */
	contenttype          string              /* Content Type of data to be output */
	raw                  []byte              /* Raw output */
	expires              string              /* Custom expiration */
	headers              http.Header         /* Outgoing headers */
	crumbs               PfLinkCol           /* Crumbbar items */
	crumbpath            string              /* Path for the base of crumbs */
	pagemenu             *PfUIMenu           /* Menu for the page */
	pagemenudepth        int                 /* How deep inside the page we are */
	f_uimainmenuoverride PfUIMenuI           /* Override for MainMenu */
	f_uisubmenuoverride  PfUIMenuI           /* Override for SubMenu */
	f_uiloginoverride    PfUILoginI          /* Override for Login */
	csrf_checked         bool                /* If CSRF Token has been checked already */
	csrf_valid           bool                /* If the CSRF Token was valid */
	language             string              /* Accept-Language: header */
}

// PfPage contains the information passed to a to-be-rendered template
type PfPage struct {
	URL              string     // URL of the current page.
	Title            string     // Title for the page; HTML <title> attribute.
	PageTitle        string     // PageTitle for the page, shown at the top of a page in a <h1>.
	Menu             PfLinkCol  // Menu (top menu bar) with global menu entries.
	SubMenu          PfLinkCol  // SubMenu (second menu bar) with more specific/context-local options.
	Crumbs           PfLinkCol  // Crumbs of where we are located in the hierarchy of menus.
	LogoImg          string     // The logo of the instance.
	HeaderImg        string     // A nice image shown as a header for index pages etc.
	AdminName        string     // The name of the administrator(s).
	AdminEmail       string     // The email address of the administrator(s).
	AdminEmailPublic bool       // Whether to show the email address to public (not loggedin) pages.
	CopyYears        string     // String indicating the years of copyright for a site.
	TheUser          *pf.PfUser // The active user, if any
	NoIndex          bool       // Whether the NoIndex option is active for this page
	CSS              []string   // List of CSS files to reference in the HTML head
	Javascript       []string   // List of Javascript files to reference in the HTML head
	SysName          string     // Name of the system
	Version          string     // Version of the instance
	PublicURL        string     // Public URL of the instance
	PeopleDomain     string     // Domain where people have their resources
	RenderStamp      string     // When the page was rendered
	UI               PfUI       // UI details
}

// Permissions aliased from Pitchfork lib for convienience.
//
// Note: Keep in sync with lib/ctx.
const (
	PERM_NOTHING        = pf.PERM_NOTHING
	PERM_NONE           = pf.PERM_NONE
	PERM_GUEST          = pf.PERM_GUEST
	PERM_USER           = pf.PERM_USER
	PERM_USER_SELF      = pf.PERM_USER_SELF
	PERM_USER_NOMINATE  = pf.PERM_USER_NOMINATE
	PERM_USER_VIEW      = pf.PERM_USER_VIEW
	PERM_GROUP_MEMBER   = pf.PERM_GROUP_MEMBER
	PERM_GROUP_ADMIN    = pf.PERM_GROUP_ADMIN
	PERM_GROUP_WIKI     = pf.PERM_GROUP_WIKI
	PERM_GROUP_FILE     = pf.PERM_GROUP_FILE
	PERM_GROUP_CALENDAR = pf.PERM_GROUP_CALENDAR
	PERM_SYS_ADMIN      = pf.PERM_SYS_ADMIN
	PERM_SYS_ADMIN_CAN  = pf.PERM_SYS_ADMIN_CAN
	PERM_CLI            = pf.PERM_CLI
	PERM_API            = pf.PERM_API
	PERM_OAUTH          = pf.PERM_OAUTH
	PERM_LOOPBACK       = pf.PERM_LOOPBACK
	PERM_HIDDEN         = pf.PERM_HIDDEN
	PERM_NOCRUMB        = pf.PERM_NOCRUMB
	PERM_NOSUBS         = pf.PERM_NOSUBS
	PERM_NOBODY         = pf.PERM_NOBODY
)

// ErrNoRows - aliased from pitchfork lib
var ErrNoRows = pf.ErrNoRows

// g_securecookies determines if cookies should be marked insecure (--insecurecookies
var g_securecookies bool

// G_cookie_name configures the name of the cookies we set
var G_cookie_name = "_pitchfork"

// NewPfUI creates a new PfUI.
//
// Typically called only by the H_root through an overridden function
// executed from the top of H_root.
func NewPfUI(ctx pf.PfCtx, mainmenuoverride PfUIMenuI, submenuoverride PfUIMenuI, uiloginoverride PfUILoginI) (cui PfUI) {
	cui = &PfUIS{PfCtx: ctx, f_uimainmenuoverride: mainmenuoverride, f_uisubmenuoverride: submenuoverride, f_uiloginoverride: uiloginoverride, hasflushed: false, csrf_checked: false}
	cui.SetOutUnbuffered(cui, "OutUnbuffered")
	return
}

// UIInit initializes a UI from the request details provided.
//
// This is called from H_root only.
//
// It basically stores the Golang HTTP writer and reader
// and figures out a couple of standard and often used variables
// so that these are ready for consumption.
//
// It also parses the Query URL and prepares the crumbbar and related items.
func (cui *PfUIS) UIInit(w http.ResponseWriter, r *http.Request) (err error) {
	/* The response and request, needed for forms etc */
	cui.w = w

	/* The request, needed for forms etc */
	cui.r = r

	/* Init PfCtx */
	err = cui.Init()
	if err != nil {
		return
	}

	/* The host they used */
	cui.http_host = cui.r.Host

	/* Full Path */
	cui.fullpath = cui.r.URL.Path

	/* Change back chars encoded by the CLI util:
	 * - %2F -> /
	 * - %B6 -> '
	 */
	path := strings.Split(cui.fullpath, "/")
	for i := 0; i < len(path); i++ {
		path[i] = strings.Replace(path[i], "%2F", "/", -1)
		path[i] = strings.Replace(path[i], "%2f", "/", -1)
		path[i] = strings.Replace(path[i], "%B6", "", -1)
		path[i] = strings.Replace(path[i], "%b6", "", -1)
	}

	/* First part is always empty, skip it */
	cui.path = path[1:]

	/* Initial Crumb Path */
	cui.crumbpath = "/"

	/* Parse Query */
	cui.args, err = url.ParseQuery(cui.r.URL.RawQuery)
	if err != nil {
		return
	}

	return
}

// UIMainMenuOverride calls the menu override function when configured.
//
// This targets the Main menu.
//
// Before each menu is executed an override gives the possibility to
// change the menu from the application. This call makes that happen.
func (cui *PfUIS) UIMainMenuOverride(menu *PfUIMenu) {
	if cui.f_uimainmenuoverride != nil {
		cui.f_uimainmenuoverride(cui, menu)
	}
}

// UISubMenuOverride calls the sub menu override function when configured.
//
// This targets the Sub menu.
//
// Before each menu is executed an override gives the possibility to
// change the menu from the application. This call makes that happen.
func (cui *PfUIS) UISubMenuOverride(menu *PfUIMenu) {
	if cui.f_uisubmenuoverride != nil {
		cui.f_uisubmenuoverride(cui, menu)
	}
}

// checkCSRF checks the CSRF tokens.
//
// Called from GetArg, FormValue and friends to verify the CSRF token.
//
// It caches the CSRF token verification in the cui
// thus avoiding the need to repetively verify it.
//
// The token comes from the X-XSRF-TOKEN HTTP header
// or from the HTTP POST field indicated with CSRF_TOKENNAME.
//
// Normally, for Pitchfork, the HTTP POST field is used.
// The X-XSRF-TOKEN is primarily for CSRF checks for API usage.
//
// That token is passed to csrf_Check to actually verify it.
//
// When a CSRF check fails, that failure is tracked in IPTrk.
// This indeed means that just based on failing CSRF verification
// an IP could get locked out from logging in.
func (cui *PfUIS) checkCSRF() (valid bool) {
	/* Already checked and thus Cached? */
	if cui.csrf_checked {
		return cui.csrf_valid
	}

	valid = false

	cui.parseform()

	/* Did the token arrive in a HTTP header? */
	tok := cui.GetHTTPHeader("X-XSRF-TOKEN")
	if tok != "" {
		valid = csrf_Check(cui, tok)
	} else {
		/* Get token from the Form (direct access to avoid CSRF check) */
		vs, ok := cui.r.Form[CSRF_TOKENNAME]
		if !ok || len(vs) < 1 || vs[0] == "" {
			cui.Errf("Missing expected CSRF token for URL %q", cui.GetFullPath())
		} else {
			tok = vs[0]
			/* Check CSRF token validity */
			valid = csrf_Check(cui, tok)
		}
	}

	// Use the Accept-Language header to determine what language to use
	language := cui.GetHTTPHeader("Accept-Language")
	if language != "" {
		cui.language = language
		cui.SetLanguage(language)
	}

	// We are checking CSRF, if invalid, track it
	if !valid {
		ip_ := cui.GetClientIP()
		ip := ip_.String()
		pf.Iptrk_count(ip)

		/*
		 * Indeed, too many CSRF issues gets one
		 * locked out by IP address, thus the actual
		 * login after that will fail
		 */
	}

	/* Now cached */
	cui.csrf_valid = valid
	cui.csrf_checked = true
	return
}

// defaultMaxMemory sets the default maximum memory for form parsing
// https://golang.org/src/net/http/request.go?s=28722:28757#L924
const defaultMaxMemory = 32 << 20 // 32 MB

// Gets argument from POST values only - mandatory CSRF check
// The function to use for multiple returns (eg a map).
//
// See FormValue for more details.
func (cui *PfUIS) FormValueM(key string) (vals []string, err error) {
	return cui.formvalueA(key, true)
}

// FormValue returns the value posted by a POST form, after checking CSRF.
//
// Gets argument from POST values only - mandatory CSRF check.
// This is the right function to use.
func (cui *PfUIS) FormValue(key string) (val string, err error) {
	return cui.formvalueS(key, true)
}

// FormValueNoCSRF gets a value from a POST form, but skips CSRF check.
//
// *AVOID* using this as much as possible: only when known that an
// outside thing might do a direct POST.
//
// Currently this is only used by the the Oauth2 code.
// The key refers to the fieldname of the form item to retrieve.
// It returns a value and an error.
func (cui *PfUIS) FormValueNoCSRF(key string) (val string, err error) {
	return cui.formvalueS(key, false)
}

// formvalueS gets a single string value from a form, optionally performing CSRF verification.
//
// It returns a single string or ErrMissingValue.
//
// See formvalueA for more details.
func (cui *PfUIS) formvalueS(key string, docsrf bool) (val string, err error) {
	vals, err := cui.formvalueA(key, false)
	if len(vals) > 0 {
		val = vals[0]
		return
	}

	val = ""
	err = ErrMissingValue
	return
}

// formvalueA gets one or more values from a form, optionally performing CSRF verification.
//
// The key refers to the fieldname of the form item to retrieve.
// It returns a value and an error.
//
// Only POST requests are handled by this function.
// First this parses the form, it then, optionally, verifies the CSRF token.
// If the CSRF checking is enabled and the token is valid, or the CSRF checking
// is disabled, the value related to the form is returned.
//
// In case an expected key is not found this is reported as an error
// and a debug message is logged when debugging is enabled.
func (cui *PfUIS) formvalueA(key string, docsrf bool) (vals []string, err error) {
	/* Only works when there actually was a POST request */
	if !cui.IsPOST() {
		err = ErrNotPOST
		return
	}

	cui.parseform()
	if docsrf && !cui.checkCSRF() {
		err = ErrInvalidCSRF
		return
	}

	vals, ok := cui.r.PostForm[key]
	if ok && len(vals) > 0 {
		cui.Dbgf("Fetched key %q = %+v", key, vals)
		return
	}

	cui.Dbgf("Missing value for key %q", key)
	cui.Dbgf("OTHER values: %+v", cui.r.PostForm)
	err = ErrMissingValue
	return
}

// SetBearerAuth is used to indicate that BearerAuth is used
// instead of a HTTP cookie.
func (cui *PfUIS) SetBearerAuth(t bool) {
	cui.bearer_auth = true
}

// GetMethod is used for retrieving the HTTP method used for the request.
//
// Typically this is POST or GET, but other HTTP methods would be possible.
//
// One should use IsGET and IsPOST where possible.
func (cui *PfUIS) GetMethod() (method string) {
	return cui.r.Method
}

// IsGET returns if the request is a GET request
func (cui *PfUIS) IsGET() (isget bool) {
	return cui.GetMethod() == "GET"
}

// IsPOST returns if the request is a POST request
func (cui *PfUIS) IsPOST() (ispost bool) {
	return cui.GetMethod() == "POST"
}

// GetHTTPHost is used for retrieving the HTTP Host used for the request
func (cui *PfUIS) GetHTTPHost() (host string) {
	return cui.http_host
}

// GetHTTPHeader retrieves a custom HTTP header from the request
func (cui *PfUIS) GetHTTPHeader(name string) (val string) {
	val = cui.r.Header.Get(name)
	return
}

// GetPath returns the path used for the request as a slice of strings
//
// Typically used to check the last part of the URL which will contain
// the identify of the object being looked at.
func (cui *PfUIS) GetPath() (path []string) {
	return cui.path
}

// GetPathString returns the Path from the request as a single string
func (cui *PfUIS) GetPathString() (path string) {
	return strings.Join(cui.path, "/")
}

// SetPath is used to configure the path.
//
// The path is used for tracking where we are in the URL.
func (cui *PfUIS) SetPath(path []string) {
	cui.path = path
}

// GetSubPath is used to retrieve the sub-path.
//
// The sub-path is relative to a module's entry point.
func (cui *PfUIS) GetSubPath() (path string) {
	return cui.subpath
}

// SetSubPath is used to set the sub-path.
//
// The sub-path is relative to a module's entry point.
func (cui *PfUIS) SetSubPath(path string) {
	cui.subpath = path
}

// GetFullPath is used to retrieve the full path of the current URL.
//
// This is only the Path portion of the URL.
func (cui *PfUIS) GetFullPath() (path string) {
	return cui.fullpath
}

// GetFullURL is used to retrieve the full URL of the request.
//
// This includes the HTTP host portion and the HTTP protocol.
func (cui *PfUIS) GetFullURL() (path string) {
	return cui.r.URL.String()
}

// GetArg is used to retrieve an argument from a Request URL
func (cui *PfUIS) GetArg(key string) (val string) {
	return cui.args.Get(key)
}

// GetArgCSRF is used to get an argument, but with a forced CSRF check
func (cui *PfUIS) GetArgCSRF(key string) (val string) {
	ok := cui.checkCSRF()
	if !ok {
		return
	}

	return cui.args.Get(key)
}

// NoSubs can be used to cause a StatusNotFound when
// there are sub-URLs (more specific paths) under the given path.
func (cui *PfUIS) NoSubs() bool {
	if len(cui.path) > 0 && cui.path[0] != "" {
		H_error(cui, StatusNotFound)
		return true
	}

	return false
}

//
// ParseClientIP parses a client IP, including XFF.
//
// Note: addr will always contain the full XFF
// as received, but only the right hand side will be valid
// we do not discard the components that are untrusted
// as they might have forensic value.
//
// The 'ip' returned is the trusted value though.
//
// The input remaddr is the remote address including the port.
// The xff is the header passed in as X-Forwarded-For.
// The xffc comes from the XFF configuration (pf.Config.XFFc).
//
// This is typically called from SetClientIP but exposed
// so that it can be called by test functions.
//
// It returns the ip address as a net.IP and as a string
// along with an error if any occured, which should be rare.
//
// The incoming xff is sanitized to make it more standardized.
// This as some separate by space, others by comma while the
// semi-official standard is to use comma and space.
// We parse each IP address, thus ensuring they are valid and
// not complete non-sense till we have found the first IP
// that is not in our proxy list (the xffc).
//
// The end result is thus either the remote address when
// none of the XFFs are valid, or the first not-trusted
// proxy IP from the XFF.
func (cui *PfUIS) ParseClientIP(remaddr string, xff string, xffc []*net.IPNet) (ip net.IP, addr string, err error) {
	gotip := false

	remaddr, _, err = net.SplitHostPort(remaddr)
	if err != nil {
		cui.Errf("RemoteAddr is invalid: %s", err.Error())
		return
	}

	//
	// Trust X-Forwarded-For when it is set
	// this as we always sit behind a proxy
	// and then RemoteAddr == 127.0.0.1
	//
	addr = xff
	if addr != "" {
		addr += ","
	}

	// The remote connection was last
	addr += remaddr

	// Standardize to ", " as a separator
	addr = strings.Replace(addr, " ", ",", -1)
	addr = strings.Replace(addr, " ", "", -1)
	addr = strings.Replace(addr, ",,", ",", -1)
	addr = strings.Replace(addr, ",", ", ", -1)

	if addr != "" {
		/* Split it up */
		addrs := strings.Split(addr, ",")

		/* Parse through them till we find a non-trusted XFF IP */
		for i := len(addrs) - 1; i >= 0; i-- {
			addrp := strings.TrimSpace(addrs[i])
			if addrp == "" {
				continue
			}

			trusted := false

			/* Parse the IP */
			ip = net.ParseIP(addrp)

			if ip == nil {
				/* Invalid IP, thus can't trust the XFF at all */
				cui.Errf("XFF: Unparseable IP >>>%s<<< encountered at index %d in %#v", addrp, i, addrs)
				break
			}

			for _, xc := range xffc {
				/* Trusted IP? */
				if xc.Contains(ip) {
					/* Found it */
					trusted = true

					/* Don't look any further... */
					break
				}
			}

			/* Found a non-trusted IP */
			if !trusted {
				gotip = true
				break
			}
		}
	}

	/* If no valid IP yet, then it is a direct request, at least log that */
	if !gotip {
		ip = net.ParseIP(remaddr)
		if ip == nil {
			err = errors.New("Not a valid IP address: " + remaddr)
			return
		}
	}

	return
}

// SetClientIP calls ParseClientIP to parse the XFF header and retrieve the
// first not-trusted proxy IP from it and then calls SetClient to set the
// parameters in UI.
//
// This is typically only called from H_root.
func (cui *PfUIS) SetClientIP() (err error) {
	ua := cui.r.Header.Get("User-Agent")
	remaddr := cui.r.RemoteAddr
	xff := cui.r.Header.Get("X-Forwarded-For")
	xffc := pf.Config.XFFc

	ip, addr, err := cui.ParseClientIP(remaddr, xff, xffc)
	if err != nil {
		return
	}

	/* The remote address and other details, used for audit logs */
	cui.SetClient(ip, addr, ua)

	return
}

// AddHeader add a HTTP header to the stack of headers.
func (cui *PfUIS) AddHeader(h string, v string) {
	if cui.headers == nil {
		cui.headers = make(http.Header)
	}

	cui.headers.Add(h, v)
}

// SetHeader sets a HTTP header to a specific value.
func (cui *PfUIS) SetHeader(h string, v string) {
	if cui.headers == nil {
		cui.headers = make(http.Header)
	}

	cui.headers.Set(h, v)
}

// OutUnbuffered outputs an unbuffered string directly to the Response
//
// This is primarily used for AJAX-style replies, like the search engine
// that flushes answers in a stream to the HTTP connection.
//
// Otherwise, the standard method of Flush() should be used.
func (cui *PfUIS) OutUnbuffered(txt string) {
	fmt.Fprint(cui.w, txt)
}

// Flush flushes all the queued output/headers etc to the client.
//
// It avoids multiple flushing, in case of coder error.
//
// It logs the access, along with the result codes in the access log.
//
// It adds all the required/wanted headers to the output.
//
// And then either sends out a static file, the raw buffer hat is awaiting
// or renders a template and flushes that out to disk.
func (cui *PfUIS) Flush() {
	if cui.hasflushed {
		if cui.IsBuffered() {
			cui.Errf("Flushed again, programmer mistake!")
		}
		return
	}

	cui.hasflushed = true

	/* Log access */
	cui.logaccess()

	/* Extra headers */
	for i, k := range cui.headers {
		for _, v := range k {
			cui.w.Header().Add(i, v)
		}
	}

	/* Set a variety of security headers */
	hdr := make(map[string]string)
	hdr["X-Content-Type-Options"] = "nosniff"
	hdr["X-Frame-Options"] = "SAMEORIGIN"
	hdr["X-XSS-Protection"] = "1; mode=block"
	hdr["Content-Security-Policy"] = pf.Config.CSP

	// Everything, except static files, are
	// dynamically generated, thus make sure
	// that browsers know we do not want these files cached.
	if cui.staticfile == "" && cui.expires == "" {
		// Configure the expires header to a time long long ago
		cui.expires = time.Date(2015, 01, 01, 1, 5, 3, 0, time.UTC).Format(http.TimeFormat)

		// Output a Cache-Control header effectively forbidding caching
		hdr["Cache-Control"] = "no-cache, no-store, max-age=0, must-revalidate"
	}

	// The "shell" return code
	rc := cui.GetReturnCode()
	if rc != 0 {
		hdr["X-ReturnCode"] = strconv.Itoa(rc)
	}

	// Output the headers
	for h, v := range hdr {
		cui.w.Header().Add(h, v)
	}

	/* Expiration */
	if cui.expires != "" {
		cui.w.Header().Set("Expires", cui.expires)
	}

	// The Content Type ;uses Set to force it, there can only be one
	if cui.contenttype != "" {
		cui.w.Header().Set("Content-Type", cui.contenttype)
	}

	/* Serve a static file? */
	if cui.staticfile != "" {
		http.ServeFile(cui.w, cui.r, cui.staticfile)
		return
	}

	/* Set a Token when needed */
	cui.setToken(cui.w)

	/* Needs redirection? */
	if cui.redirect != "" {
		http.Redirect(cui.w, cui.r, cui.redirect, StatusSeeOther)
		return
	}

	/* The HTTP Status */
	status := cui.GetStatus()

	/* Set WWW-Authenticate when we send out a 401 */
	if status == StatusUnauthorized {
		cui.w.Header().Set("WWW-Authenticate", "Bearer realm=\""+pf.System_Get().Name+"\"")
	}

	/* Output a status code - also starts flushing things out */
	cui.w.WriteHeader(status)

	/* Output what was buffered, if any */
	o := cui.Buffered()
	if o != "" {
		fmt.Fprint(cui.w, o)
	}

	if len(cui.raw) > 0 {
		cui.w.Write(cui.raw)
	}

	/* Render a page if needed */
	cui.page_render(cui.w)
}

// SetExpires sets the HTTP Response to expire in the given amount of minutes.
//
// The actual HTTP header (which can be ignored by clients) is output at Flush time.
func (cui *PfUIS) SetExpires(minutes int) {
	cui.expires = time.Now().UTC().Add(time.Duration(minutes) * time.Minute).Format(http.TimeFormat)
}

// SetContentType configures the MIME content type of the response.
//
// The actual header is sent out at flush time.
func (cui *PfUIS) SetContentType(ctype string) {
	/*
	 * Always serve HTML and markdown in UTF-8
	 * This as our Markdown Renderer
	 * produces output with UTF-8
	 */
	if ctype == "text/html" || ctype == "text/markdown" {
		ctype += "; charset=utf-8"
	}

	/* The Content-Type */
	cui.contenttype = ctype
}

// SetFileName adds a header which configures a 'download filename'.
//
// This can be used to indicate the filename as which the to be downloaded
// file will be stored on the client.
//
// Typically called together with SetStaticFile.
func (cui *PfUIS) SetFileName(fname string) {
	cui.AddHeader("Content-Disposition", "inline; filename=\""+fname+"\"")
}

// SetStaticFile sets the static file that should be output at Flush time
//
// The full path of the filename should be used.
//
// The static code output will set the proper Content-Type and other
// such headers.
//
// Typically called together with SetFileName.
func (cui *PfUIS) SetStaticFile(file string) {
	cui.staticfile = file
}

// SetRaw sets the raw output to be output at Flush time.
func (cui *PfUIS) SetRaw(raw []byte) {
	cui.raw = raw
}

// SetJSON sets the content type to the JSON MIME type
// and configures the output bytes for Flush time output.
//
// This allows an already marshalled JSON message to be easily
// output to the client.
func (cui *PfUIS) SetJSON(json []byte) {
	cui.SetContentType("application/json")
	cui.SetRaw(json)
}

// JSONAnswer sets the output so that it returns a JSON Answer.
//
// The status is either 'ok' or 'error', the message contains
// a message string.
//
// The generated JSON blob looks like:
// ```
// { "status": "ok", "message": "Example Message" }
// ```
//
// This result is intended as a generic way of returning
// "ok" and "error" messages to AJAX callers.
//
// The actual result is send to the client at Flush() time.
func (cui *PfUIS) JSONAnswer(status string, message string) {
	type Status struct {
		Status  string
		Message string
	}

	jsn, err := json.Marshal(Status{status, message})
	if err != nil {
		cui.Errf("JSON encoding failed: %s", err.Error())
		jsn = []byte("JSON ENCODING FAILED")
	}

	cui.SetJSON(jsn)
}

// AddCrumb adds a crumb to the crumbbar.
//
// The link describes, part of, the URL of the crumb.
// The desc is the human readable short description of the crumb.
// the long variant is the longer, hover-over, description of the crumb.
func (cui *PfUIS) AddCrumb(link string, desc string, long string) {
	var c PfLink

	if link == "" && desc != "" {
		/* Remove previous component */
		cui.crumbs.Pop()
	} else {
		if len(link) > 0 && link[0] != '?' {
			link += "/"
		}

		/* We do update the path */
		cui.crumbpath += link
	}

	/* But don't actually add a crumb when description is empty */
	if desc != "" {
		c.Link = cui.crumbpath
		c.Desc = desc

		if long == "" {
			long = desc
		}

		c.Long = long

		cui.crumbs.Add(c)
	}
}

// DelCrumb removes the last crumb from the crumbbar.
//
// Used to replace a Crumb item with a more descriptive item
// than defaultly generated from the menu system.
//
// The link, dscription and long version of the crumb are returned
// which could be used to modify these details and add them back again.
func (cui *PfUIS) DelCrumb() (link string, desc string, long string) {
	i := cui.crumbs.Len()

	/* Should not happen, but better than a panic() */
	if i == 0 {
		cui.Err("DelCrumb() on empty Crumb Path")
		return "NOCRUMB", "NOCRUMB", "NOCRUMB"
	}

	c := cui.crumbs.Pop()

	if cui.crumbs.Len() == 0 {
		cui.crumbpath = "/"
	} else {
		cl := cui.crumbs.Last()
		cui.crumbpath = cl.Link
	}

	return c.Link, c.Desc, c.Long
}

// GetCrumbPath returns the current crumb path.
//
// This can be used to determine where in the menu system
// the code is currently located, eg used in MenuOverride.
func (cui *PfUIS) GetCrumbPath() (path string) {
	return cui.crumbpath
}

// GetCrumbParts returns the parts in a crumbbar's path.
//
// This can be used to determine where in the menu system
// the code is currently located, eg used in MenuOverride.
func (cui *PfUIS) GetCrumbParts() (path []string) {
	/* Split it up */
	path = strings.Split(cui.crumbpath, "/")

	/* First part is always empty (/) */
	path = path[1:]

	/* Empty last part if empty */
	if len(path) > 0 && path[len(path)-1] == "" {
		path = path[:len(path)-1]
	}

	/* Return the rest */
	return path
}

// Page_def returns a PfPage with default page properties set.
//
// The Misc and Search Javascript code is included per default.
// Noting that both are optional, and the page functions without.
//
// This is typically called in combo with Page_show() to set the
// returned, but likely expanded page, to be rendered at Flush time.
func (cui *PfUIS) Page_def() (p *PfPage) {
	mainmenu := NewPfUIMenu([]PfUIMentry{
		{"/group", "Group", PERM_USER, h_group, nil},
		{"/user", "User", PERM_USER, h_user, nil},
		{"/system", "System", PERM_SYS_ADMIN, h_system, nil},
		{"/cli", "CLI", PERM_CLI, h_cli, nil},
	})

	/* Give the chance to override the main menu */
	cui.UIMainMenuOverride(&mainmenu)

	sys := pf.System_Get()

	g_title := sys.Name
	g_adminname := sys.AdminName
	g_adminemail := sys.AdminEmail
	g_adminemailpublic := sys.AdminEmailPublic
	g_copyyears := sys.CopyYears

	g_version := ""
	if sys.ShowVersion && cui.IsLoggedIn() {
		g_version = pf.AppVersionStr()
	}

	/* Generate title from crumbs */
	title := ""
	ptitle := ""
	for _, c := range cui.crumbs.M {
		if title != "" {
			title += " > "
		}
		title += c.Desc

		/* Last crumb is the long page title */
		ptitle = c.Long
	}

	var theuser *pf.PfUser = nil

	if cui.IsLoggedIn() {
		_theuser := cui.TheUser()
		theuser = &_theuser
	}

	/* Generate Sub Menu from pagemenu */
	menu := mainmenu.ToLinkCol(cui, 0)
	submenu := cui.pagemenu.ToLinkCol(cui, cui.pagemenudepth)

	p = &PfPage{
		URL:              cui.GetFullPath(),
		Title:            title + " - " + g_title,
		PageTitle:        ptitle,
		Menu:             menu,
		SubMenu:          submenu,
		Crumbs:           cui.crumbs,
		LogoImg:          sys.LogoImg,
		HeaderImg:        "",
		AdminName:        g_adminname,
		AdminEmail:       g_adminemail,
		AdminEmailPublic: g_adminemailpublic,
		CopyYears:        g_copyyears,
		TheUser:          theuser,
		NoIndex:          sys.NoIndex,
		CSS:              pf.Config.CSS,
		Javascript:       pf.Config.Javascript,
		SysName:          sys.Name,
		Version:          g_version,
		PublicURL:        sys.PublicURL,
		PeopleDomain:     sys.PeopleDomain,
		RenderStamp:      time.Now().UTC().Format(pf.Config.TimeFormat),
		UI:               cui,
	}

	/*
	 * Enable Misc + Search Javascript
	 * Search also works fine when javascript is disabled
	 */
	p.AddJS("misc")
	p.AddJS("search")

	return
}

// AddCSS adds an extra file to the CSS pile.
//
// All CSS files are expected to live in or below the /css/ webroot.
//
// The css_filename is only for the portion after /css/ and without file extension.
// and typically is thus without a directory indicator.
//
// Just "pitchfork" is thus correct and will create a /css/pitchfork.css.
//
// This CSS pile is output in the <head> section of the rendered page.
// Output is in first-added, first-outputed order.
func (p *PfPage) AddCSS(css string) {
	p.CSS = append(p.CSS, css)
}

// AddJS adds an extra file to the JS pile.
//
// The js_filename is only for the portion after /js/ and without file extension.
// and typically is thus without a directory indicator.
//
// Just "pitchfork" is thus correct and will create a /js/pitchfork.js.
//
// This JS pile is output in the <head> section of the rendered page.
// Output is in first-added, first-outputed order.
func (p *PfPage) AddJS(js string) {
	p.Javascript = append(p.Javascript, js)
}

// Page_show configures the name and data to show at Flush time.
//
// It takes two arguments, name which indicates the name of the template to render
// and data which is typically a Page structure (see Page_def) or an extended
//  version of that structure.
func (cui *PfUIS) Page_show(name string, data interface{}) {
	/* Need to delay so that we can set cookies/headers etc */
	cui.show_name = name
	cui.show_data = data
}

// page_render renders a templated page as previously set with Page_show.
//
// It retrieves the template cache, and executes the named template
// from there, passing the data to be shown on the page along.
//
// This part of the code checks also if the client has already disconnected
// which might happen during the start of the request being passed to H_root,
// the processing of the request and our rendering of the template.
// The HTTP server only notices this while trying to flush the data though,
// hence why only then we can check for the error that the client disconnected.
//
// We render an error template, which might be appended to already outputted bytes,
// when the rendering of the template fails some way.
func (cui *PfUIS) page_render(w http.ResponseWriter) {
	if cui.show_name == "" {
		return
	}

	tmp := pf.Template_Get()
	err := tmp.ExecuteTemplate(w, cui.show_name, cui.show_data)

	/* All okay and done */
	if err == nil {
		return
	}

	/*
	 * Ignore Broken Pipe (client disconnected)
	 * These happen and are thus noise in the logs
	 * We do log them under debug level so that we can see them
	 * Can't render anything further either, thus abort here
	 */
	if pf.ErrIsDisconnect(err) {
		cui.Dbgf("Client disconnected during render of %s: %s", cui.show_name, err.Error())
		return
	}

	/* Log error */
	cui.Errf("Rendering template %s failed: %s", cui.show_name, err.Error())

	/* Render an error page instead */
	type Page struct {
		*PfPage
		Messages []string
	}

	cui.SetPageMenu(nil)
	p := Page{cui.Page_def(), []string{"(internal error: Template rendering failed)"}}

	tmp.ExecuteTemplate(w, "misc/error.tmpl", p)
}

// SetRedirect configures the response to be a HTTP level redirect.
//
// The status is typically:
// - StatusMovedPermanently	- HTTP 301 for permanent redirects
// - StatusFound		- HTTP 302 for temporary redirects, non-method preserving
// - StatusSeeOther		- HTTP 303 for temporary redirects, method preserving
//
// StatusTemporaryRedirect (303) and StatusPermanentRedirect (307) are currently
// not commonly implemented enough to be able to be useable.
//
// See also RFC7231 section 6.4.
//
// The actual HTTP header is emitted back to the client at Flush time.
func (cui *PfUIS) SetRedirect(path string, status int) {
	cui.SetStatus(status)

	if path == "" {
		panic("SetRedirect() with empty path!?")
	}

	/* Handle relative paths */
	if path[0] == '?' || path[0] == '#' {
		path = cui.GetFullPath() + path
	}

	/* Lets go on a trip */
	cui.redirect = path

	// cui.Dbg("SetRedirect(%d) %s", status, path)
}

// GetBody returns the full body that was POSTed.
//
// This can be useful for AJAX-style requests where the body
// is not a HTTP form (multipart/form-data or application/x-www-form-urlencoded)
// but contains actual data, thus eg application/json.
//
// Anything submitted using forms like should be using the Form related
// functions like GetFormFile, HandleForm etc.
func (cui *PfUIS) GetBody() (body []byte) {
	body, _ = ioutil.ReadAll(cui.r.Body)
	return body
}

// parseform parses a HTTP form to prepare it for actual fetching of the values.
//
// This is called from PostFormValue and friends before they get values out of the form.
func (cui *PfUIS) parseform() {
	err := cui.r.ParseMultipartForm(defaultMaxMemory)
	if err == http.ErrNotMultipart {
		/*
		 * Ignore, it parsed the form anyway
		 * the form just is not multipart
		 */
	} else if err != nil {
		cui.Errf("parseform() - err: %s", err.Error())
	} else {
		/*
		 * XXX WORKAROUND https://github.com/golang/go/issues/9305
		 * Not needed for golang 1.7+
		 */
		if len(cui.r.PostForm) == 0 {
			if cui.r.PostForm == nil {
				cui.r.PostForm = make(url.Values)
			}

			for k, v := range cui.r.Form {
				cui.r.PostForm[k] = append(cui.r.PostForm[k], v...)
			}
		}
	}
}

// GetFormFileReader returns a handle to a file that was POSTed.
//
// This can be used to stream the POSTed file instead of having to load
// the complete body into memory and then finally storing it to disk.
//
// The key indicates the POST form field name.
//
// Returned are a file handle, from which can be read, a filename, as provided
// by the user (and thus not to be trusted), and an error, if any.
//
// XXX: Normalize the filename that comes in.
func (cui *PfUIS) GetFormFileReader(key string) (file io.ReadCloser, filename string, err error) {
	var fh *multipart.FileHeader
	file, fh, err = cui.r.FormFile(key)

	if err == nil && fh != nil {
		filename = fh.Filename
	}
	return
}

// GetFormFile returns a opionally base64 encoded representation of a file
// as received from a HTTP form.
//
// key indicates the HTTP POST variable name to retrieve the file from.
// maxsize can be either empty, causing the whole file to be read or
// an image size in the format of width-x-height (eg '250x250') or
// a simple number, as a string, to indicate the maximum file size
// to read (in case the file is longer it is truncated).
//
// XXX: report error when file is truncated.
func (cui *PfUIS) GetFormFile(key string, maxsize string, b64 bool) (val string, err error) {
	var file io.ReadCloser
	var bytes []byte

	val = ""

	file, _, err = cui.GetFormFileReader(key)
	if err != nil {
		return
	}

	// When maxsize contains an 'x' it indicates dimensions
	// and thus that it is an image that has to be limited to
	// this amount of pixels (width x height, eg 250x250).
	s := strings.SplitN(maxsize, "x", 2)
	if len(s) == 2 {
		// It is an image specification, read the file and resize it to the given size
		bytes, err = pf.Image_resize(file, maxsize)
	} else if maxsize != "" {
		ms, err2 := strconv.Atoi(maxsize)
		if err2 != nil {
			err = err2
			return
		}

		// Not an image, but we have a maximum size limit, read just that
		bytes = make([]byte, ms)
		_, err = file.Read(bytes)
	} else {
		// Not an image, no size max, just read in the bytes
		bytes, err = ioutil.ReadAll(file)
	}

	if b64 {
		/* base64 encode the string */
		val = base64.StdEncoding.EncodeToString(bytes)
	} else {
		val = string(bytes)
	}
	return
}

// QueryArgSet returns a boolean indicating whether a query argument is set or not
//
// This can be used to test if for instance the query argument 't' is set in:
// https://example.com/test/?t -- true
// https://example.com/test/   -- false
//
// This can be used to test presence of a query argument, with or without it
// having a value.
//
// See also GetArg() and GetArgCSRF() for also being able to get the value.
func (cui *PfUIS) QueryArgSet(q string) (ok bool) {
	cui.parseform()

	q = strings.ToLower(q)

	for k, _ := range cui.r.Form {
		k = strings.ToLower(k)
		if k == q {
			return true
		}
	}

	return false
}

// HandleFormS is the extended version of HandleForm allowing enabling of the autoop option.
//
// The fields of the HTTP POST are matched against the fields of the provided object.
//
// Use this in combination with 'object set|get|add|remove <form-element> <id> <value>' style commands.
// For each matching form-element (name) the command is executed.
//
// When autoop is false, the operand is already in the 'cmd'.
//
// When autoop is true, we determine the op based on the 'submit' button.
//
// When 'submit' is 'Add'/'Remove' the op becomes that and in lowercase
// appended to the cmd.
//
// When autoop is true, we ignore slices unless the op is add or remove.
//
// Only HTTP POST requests are processed with this function and CSRF is enforced.
//
// Fieldtypes of "ignore", "button", "note", "header" are ignored.
//
// Fieldtypes that indicate the field is a booleans are normalized.
//
// Fieldtypes that indicate the field is a file get special treatment
// in that the file is loaded from the HTTP form into the variable.
//
// The amount of total changes are reported in the msg that is returned.
// Errors are reported in the error variable.
func (cui *PfUIS) HandleFormS(cmd string, autoop bool, args []string, obj interface{}) (msg string, err error) {
	updates := 0
	nomods := 0
	op := ""

	/* Nothing to do when it is a GET */
	if cui.IsGET() {
		/* cui.Dbg("HandleForm() - GET is not a POST") */
		return
	}

	cui.Dbgf("HandleForm(%s)", cmd)

	/* Require POST */
	if !cui.IsPOST() {
		cui.Dbg("HandleForm() - Only POST")
		err = errors.New("Only POST supported")
		return
	}

	/*
	 * Parse the form
	 * need to do this in case there are no bools in the struct
	 */
	cui.parseform()

	/* Always check CSRF */
	ok := cui.checkCSRF()
	if !ok {
		err = errors.New("Form expired, please refresh and try again")
		return
	}

	if autoop {
		/* Based on the submit button: Add, Remove or 'set' for anything else */
		val, e := cui.FormValue("submit")
		if e != nil {
			val = ""
		}

		btn := strings.ToLower(val)
		switch btn {
		case "":
			err = errors.New("No submission button pressed")
			return

		case "add":
			op = "add"
			break

		case "remove":
			op = "remove"
			break

		default:
			op = "set"
			break
		}

		/* Extend the command */
		cmd += " " + op
	} else {
		op = "set"
	}

	var vars map[string]string
	vars, err = pf.StructVars(cui, obj, pf.PTYPE_UPDATE, true)

	if err != nil {
		cui.Errf("HandleForm(%s) + : %s", cmd, err.Error())
		return
	}

	for key, ftype := range vars {
		var val string
		var e error

		/* Don't bother fetching a couple of types */
		if ftype == "ignore" || ftype == "button" || ftype == "note" || ftype == "header" {
			continue
		}

		switch ftype {
		case "bool":
			val, e = cui.FormValue(key)
			/* Field not given - then it is off */
			if e == ErrMissingValue || val == "" {
				val = "off"
			} else if e != nil {
				/* Problem in the form */
				continue
			}

			/* Normalize the Boolean */
			val = pf.NormalizeBoolean(val)
			break

		case "file":
			maxsize, _ := pf.StructTag(obj, key, "pfmaximagesize")
			b64_s, _ := pf.StructTag(obj, key, "pfb64")
			b64 := pf.IsTrue(b64_s)
			val, err = cui.GetFormFile(key, maxsize, b64)
			if err != nil {
				/* Ignore, as it just means it was not uploaded/used */
				err = nil
				continue
			}
			break

		default:
			/* Get the value from the form for everything else */
			val, e = cui.FormValue(key)
			if e != nil {
				continue
			}
			break
		}

		if op == "set" && ftype == "slice" {
			/* Ignore, as one can't set slices */
			continue
		}

		if op != "set" {
			if ftype != "slice" {
				/* Ignore, as one can't add/remove from non-slices */
				continue
			}

			if val == "" && op == "set" {
				/* Ignore, as we don't allow adding/removing empty values */
				continue
			}
		}

		/* Run the command */
		cmds := strings.Split(cmd, " ")
		cmds = append(cmds, key)
		for _, a := range args {
			cmds = append(cmds, a)
		}
		cmds = append(cmds, val)

		/* Find the menu entry */
		e = cui.Cmd(cmds)
		if e != nil {
			if pf.ErrIsUnknownCommand(e) {
				continue
			}

			cui.Errf("HandleForm(%v) - Cmd - err: %s", cmds, e.Error())
		}

		if err != nil {
			/* Note: We only retain the last error message */
			err = e
			continue
		}

		msg = cui.Buffered()

		parts := strings.Split(msg, " ")
		if parts[0] == "Updated" {
			/* Updated fine */
			updates++
		} else {
			nomods++
		}
	}

	msg = ""

	if updates > 0 {
		msg = "Updated " + strconv.Itoa(updates) + " fields"
	}

	if nomods > 0 {
		if msg != "" {
			msg += ", "
		}
		msg += strconv.Itoa(nomods) + " fields where not modified"
	}

	if msg == "" {
		msg = "No fields where modified"
	}

	return
}

// HandleForm is the form handler - called for form submissions.
//
// Use this in combination with 'object set <form-element> <id> <value>' style commands.
//
// For each matching form-element (fieldname) the command is executed.
//
// See HandleFormS for more details.
func (cui *PfUIS) HandleForm(cmd string, args []string, obj interface{}) (msg string, err error) {
	return cui.HandleFormS(cmd, false, args, obj)
}

// HandleCmd is the command handler to directly call a command with arguments.
//
// Use this for form POSTs with specific parameters.
// The HTTP POST fields are matched to the CLI menu entry's arguments.
//
// It takes a start of a CLI command in cmd, along with placeholders for
// arguments in args. These args can be pre-filled at which point they
// will be used as static values, not to be replaced with details from the HTTP form.
//
// Each possible argument needs to have space to store that argument, thus
// one needs to provide empty spots in args, if not provided the function
// will return an error.
//
// The arguments that are not specified will be retrieved from the HTTP form.
// This happens by calling WalkMenu, finding the CLI menu entry for the command
// and then completing it based on the fields in the CLI arguments and retrieving
// these from the HTTP form.
//
// CLI argument options are supported.
//
// The file argument option is formatted like:
// ```
// filename#file(#maxsize(#base64]]
// ```
// example:
// ```
// filename#file#10240#yes
// ```
// This indicates that a argument named 'filename' and thus a HTTP form element
// with that name is actually a file. It has an optional maximum size as given
// in the example as 10240 bytes and should be encoded as base64 before passing
// it to the CLI. The function will then load the whole file, encode it as
// base64 and place it in the argument.
//
// Passwords can be indicated with the 'password' and 'twofactor' CLI argument options.
// eg:
// ```
// newpassword#password
// 2facode#twofactor
// ```
// The debug output for these will be masked to protect sensitive information.
//
// A CLI argument option of 'bool' causes the field to be interpreted as a boolean
// and normalized as such.
//
// When the arguments have been loaded from the HTTP form, the command gets executed.
//
// The output of the CLI command is buffered and returned in the msg variable.
// The error variable will contain an error code if any was encountered.
//
// CSRF checking is enforced.
//
// When Debugging is enabled the final command line and the arguments are logged.
// Passwords and twofactor codes, indicated with the relevant CLI argument option,
// are masked in the output to protect these sensitive details.
func (cui *PfUIS) HandleCmd(cmd string, args []string) (msg string, err error) {
	/* No error or message yet */
	err = nil
	msg = ""

	/* Shadow args for debugging but hiding passwords */
	vargs := make([]string, len(args))
	copy(vargs, args)

	/* Nothing to do when it is a GET */
	if cui.IsGET() {
		// cui.Dbg("GET is not a POST")
		return
	}

	/* Require POST */
	if !cui.IsPOST() {
		// cui.Dbg("Only POST")
		err = errors.New("Only POST supported")
		return
	}

	/* Always check CSRF */
	ok := cui.checkCSRF()
	if !ok {
		err = errors.New("Invalid HTML Form submitted")
		return
	}

	cui.Dbgf("HandleCmd(\""+cmd+"\")[%s]", args)

	/* Split up the command */
	cmds := strings.Split(cmd, " ")

	/* Append arguments */
	cmds = append(cmds, args...)

	/* Walk the menu to find the command's menu */
	menu, err := cui.WalkMenu(cmds)

	if err != nil {
		cui.Errf("WalkMenu(%v) failed: %s", cmds, err.Error())
		return
	}

	/* Command was not a menu, but already executed */
	if menu == nil {
	} else {

		/* Some commands don't take arguments */
		if menu.Args != nil {
			for n := range menu.Args {

				/* Is there room for this? */
				if n >= len(args) {
					/* Is it required? */
					if n >= menu.Args_min {
						/* Optional */
						break
					}

					err = errors.New("Invalid argument")
					cui.Err("HandleCmd(" + cmd + ") missing variable room for argument (args:" + strconv.Itoa(len(args)) + ")")
					return
				}

				if args[n] != "" {
					continue
				}

				/* Get the option along with optional arguments */
				opt := strings.Split(menu.Args[n], "#")

				var val string

				/* Any options? */
				normval := true
				if len(opt) > 1 {
					switch opt[1] {
					case "file":
						normval = false
						maxsize := ""
						b64 := false

						if len(opt) > 2 {
							maxsize = opt[2]
						}

						if len(opt) > 3 {
							b64 = pf.IsTrue(opt[3])
						}

						val, err = cui.GetFormFile(opt[0], maxsize, b64)
						if err != nil {
							return
						}

					}
				}

				if normval {
					val, err = cui.FormValue(opt[0])
					if err != nil {
						return
					}
				}

				/* TODO: Verify value type, by filtering them */
				/* menu.args[n][1] */

				/* Mask arguments that should not be logged */
				switch opt[0] {
				case "password":
				case "twofactor":
				case "keyring":
					if val != "" {
						vargs[n] = "*" + opt[0] + "*"
					} else {
						vargs[n] = "(" + opt[0] + " not given)"
					}
					break
				default:
					vargs[n] = val
					break
				}

				cui.Dbg("Arg " + opt[0] + " = " + vargs[n])
				args[n] = val
			}
		}

		cui.Dbg("Filling in menu - done")

		/* Verify that we have the right amount of arguments */
		if len(args) < 0 {
			err = errors.New("Missing arguments")
			return
		}

		/* Execute the command */
		cmds = strings.Split(cmd, " ")
		cmds = append(cmds, args...)
		cui.Dbgf("HandleCmd() - exec: %q", cmds)
		err = cui.Cmd(cmds)
	}

	cui.Dbg("HandleCmd() - done")

	msg = cui.Buffered()
	cui.Dbgf("HandleCmd() - msg: %s", msg)
	if err != nil {
		cui.Errf("HandleCmd() - err: %s", err.Error())
	} else {
		cui.Dbg("HandleCmd() - no error")
	}

	return
}

// InitToken initializes our token from the Request headers provided
//
// We look at both the Authorization and Cookie HTTP headers.
//
// If a Authorization HTTP header is present and it is a BEARER token
// we use that token to check for authentication.
// The cui.bearer_auth flag is set to true when this method is used.
//
// In the case a Cookie header is present that contains our Cookie Name,
// we attempt to use that cookie as our token for authentication.
//
// When the token is present a login by token is attempted.
//
// When the token is valid the login will happen.
//
// When the token is about to expire we set cui.token_exp to indicate this
// which is later used by setToken to refresh the token thus providing
// a new token to the client, avoiding them from timing out.
func (cui *PfUIS) InitToken() {
	var tok = ""
	var err error

	/* Not using bearer auth */
	cui.bearer_auth = false

	/* Look for an Authorization header with Bearer option */
	ah := cui.r.Header.Get("Authorization")
	if ah != "" {
		/* Should be a bearer token */
		if len(ah) > 6 && strings.ToUpper(ah[0:6]) == "BEARER" {
			cui.bearer_auth = true
			tok = ah[7:]
		}
	}

	// When no token found yet, try a Cookie
	if tok == "" {
		/* Look for Cookie */
		var cookie *http.Cookie
		cookie, err = cui.r.Cookie(G_cookie_name)
		if err == nil {
			tok = cookie.Value
		}
	}

	if tok == "" {
		/* Look for "access_token" parameter */
		tok = cui.GetArg("access_token")
	}

	if tok != "" {
		// Received a token, attempt to login with it
		cui.token_recv = tok
		expsoon, err := cui.LoginToken(tok)
		if err != nil {
			cui.Dbgf("LoginToken failed: %s", err.Error())
		} else {
			/* Valid token */
			cui.token_exp = expsoon

			// Check if we need to swap SysAdmin mode
			xtra := cui.r.URL.Query().Get("xtra")
			switch xtra {
			case "swapadmin":
				cui.SwapSysAdmin()
				break

			default:
				// Everything else is silently ignored
				break
			}
		}
	} else {
		// Did not receive a token
		cui.token_recv = ""
	}
}

// setToken outputs a token to the response in case it changed.
//
// The token is output using either the Set-Cookie or the the WWW-Authenticate HTTP headers
// depending on wheher bearer_auth is in use or not.
//
// Users that are logged in get a valid cookie when the token changed.
// Users that are not logged in, while sending a token will receive a invalid cookie
// that replaces the previous cookie.
//
// When the user is logged in and the token is marked to be expiring soon
// a new token is generated to avoid a timeout for the session.
//
// This is an internal function called from Flush().
func (cui *PfUIS) setToken(w http.ResponseWriter) {
	/* Bearer options */
	b := "Bearer realm=\"" + pf.System_Get().Name + "\""

	/* Logged in? */
	if cui.IsLoggedIn() {
		/* No token or token expired? -> Create new one */
		if cui.GetToken() == "" || cui.token_exp {
			cui.Dbg("Generating new token for logged in user")
			err := cui.NewToken()
			if err != nil {
				cui.Err("setToken - No Token: " + err.Error())
				return
			}
		}

		/* New token? */
		if cui.GetToken() != cui.token_recv {
			cui.Dbg("Got a different token than received, sending it to the client")
			/* Authorization or Cookie? */
			if cui.bearer_auth {
				b += " access_token=\"" + cui.GetToken() + "\""
				w.Header().Set("WWW-Authenticate", b)
			} else {
				http.SetCookie(w, &http.Cookie{Name: G_cookie_name, Path: "/", Value: cui.GetToken(), HttpOnly: true, Secure: g_securecookies})
			}
		} else {
			/* Retain old token */
			/* cui.Dbg("Retaining old token") */
		}
	} else if cui.GetToken() != "" || (cui.token_recv != "" && cui.GetToken() == "") {
		cui.Dbg("Not logged in, revoking token")
		/*
		 * Not logged in and had a token? -> Invalidate the cookie
		 */
		if cui.bearer_auth {
			w.Header().Set("WWW-Authenticate", b)
		} else {
			http.SetCookie(w, &http.Cookie{Name: G_cookie_name, Path: "/", Value: "invalid", Expires: time.Date(2015, 01, 01, 1, 5, 3, 0, time.UTC), MaxAge: -1, HttpOnly: true, Secure: g_securecookies})
		}
	} else {
		/* Nothing to do */
	}
}

// NewlineBR replaces \n with <br /> so that the output is properly
// rendered on the user side.
//
// It is primarily used to convert text files which are line delimited
// in a format that will show up in the same formatting in HTML.
func NewlineBR(val string) (safe template.HTML) {
	esc := template.HTMLEscapeString(val)
	return template.HTML(strings.Replace(esc, "\n", "<br />\n", -1))
}

// UIInit initializes a UI's parameters.
//
// It allows enable/disabling securecookies, useful for development
// and specifying the name of the cookies that Pitchfork should emit.
//
// This gets called from Setup.
func UIInit(securecookies bool, cookie_name string) error {
	g_securecookies = securecookies

	/* The cookie name */
	G_cookie_name = strings.ToLower(cookie_name)

	/* No problems to be generated here yet */
	return nil
}
