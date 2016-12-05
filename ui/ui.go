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

/* Types */
type PfNewUI func() (cui PfUI)
type PfUIMenuI func(cui PfUI, menu *PfUIMenu)
type PfUILoginI func(cui PfUI, lp *PfLoginPage) (p interface{}, err error)

type PfUI interface {
	pf.PfCtx
	PfUIi
}

type PfUIi interface {
	UIInit(w http.ResponseWriter, r *http.Request) (err error)
	FormValue(key string) (val string, err error)
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
	SetExpired()
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
	f_uimainmenuoverride PfUIMenuI
	f_uisubmenuoverride  PfUIMenuI
	f_uiloginoverride    PfUILoginI
	csrf_checked         bool /* If CSRF Token has been checked already */
	csrf_valid           bool /* If the CSRF Token was valid */
}

type PfPage struct {
	URL              string
	Title            string
	PageTitle        string
	Menu             PfLinkCol
	SubMenu          PfLinkCol
	Crumbs           PfLinkCol
	LogoImg          string
	HeaderImg        string
	AdminName        string
	AdminEmail       string
	AdminEmailPublic bool
	CopyYears        string
	TheUser          *pf.PfUser
	NoIndex          bool
	CSS              []string
	Javascript       []string
	SysName          string
	Version          string
	PublicURL        string
	PeopleDomain     string
	RenderStamp      string
	UI               PfUI
}

/* Keep in sync with lib/ctx */
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

/* Import this to avoid having to type too much */
var ErrNoRows = pf.ErrNoRows

/* If the cookies should be marked insecure (--insecurecookies) */
var g_securecookies bool

/* Name of the cookies we set */
var G_cookie_name = "_pitchfork"

/* Functions */
func NewPfUI(ctx pf.PfCtx, mainmenuoverride PfUIMenuI, submenuoverride PfUIMenuI, uiloginoverride PfUILoginI) (cui PfUI) {
	cui = &PfUIS{PfCtx: ctx, f_uimainmenuoverride: mainmenuoverride, f_uisubmenuoverride: submenuoverride, f_uiloginoverride: uiloginoverride, hasflushed: false, csrf_checked: false}
	cui.SetOutUnbuffered(cui, "OutUnbuffered")
	return
}

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

func (cui *PfUIS) UIMainMenuOverride(menu *PfUIMenu) {
	if cui.f_uimainmenuoverride != nil {
		cui.f_uimainmenuoverride(cui, menu)
	}
}

func (cui *PfUIS) UISubMenuOverride(menu *PfUIMenu) {
	if cui.f_uisubmenuoverride != nil {
		cui.f_uisubmenuoverride(cui, menu)
	}
}

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

	/* We are checking CSRF, if invalid, track it */
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

/* https://golang.org/src/net/http/request.go?s=28722:28757#L924 */
const defaultMaxMemory = 32 << 20 // 32 MB

/*
 * Gets argument from POST values only - mandatory CSRF check
 * The function to use
 */
func (cui *PfUIS) FormValue(key string) (val string, err error) {
	return cui.formvalueA(key, true)
}

/*
 * Gets argument from POST values only - skip CSRF check
 *
 * *AVOID* using this as much as possible: only when known that an
 * outside thing might do a direct POST.
 *
 * Currently that is only the Oauth2 code.
 */
func (cui *PfUIS) FormValueNoCSRF(key string) (val string, err error) {
	return cui.formvalueA(key, false)
}

func (cui *PfUIS) formvalueA(key string, docsrf bool) (val string, err error) {
	val = ""

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

	vs, ok := cui.r.PostForm[key]
	if ok && len(vs) > 0 {
		val = vs[0]
		return
	}

	cui.Dbgf("Missing value for %q", key)
	err = ErrMissingValue
	return
}

func (cui *PfUIS) SetBearerAuth(t bool) {
	cui.bearer_auth = true
}

func (cui *PfUIS) GetMethod() (method string) {
	return cui.r.Method
}

func (cui *PfUIS) IsGET() (isget bool) {
	return cui.GetMethod() == "GET"
}

func (cui *PfUIS) IsPOST() (ispost bool) {
	return cui.GetMethod() == "POST"
}

func (cui *PfUIS) GetHTTPHost() (host string) {
	return cui.http_host
}

func (cui *PfUIS) GetHTTPHeader(name string) (val string) {
	val = cui.r.Header.Get(name)
	return
}

func (cui *PfUIS) GetPath() (path []string) {
	return cui.path
}

func (cui *PfUIS) GetPathString() (path string) {
	return strings.Join(cui.path, "/")
}

func (cui *PfUIS) SetPath(path []string) {
	cui.path = path
}

func (cui *PfUIS) GetSubPath() (path string) {
	return cui.subpath
}

func (cui *PfUIS) SetSubPath(path string) {
	cui.subpath = path
}

func (cui *PfUIS) GetFullPath() (path string) {
	return cui.fullpath
}

func (cui *PfUIS) GetFullURL() (path string) {
	return cui.r.URL.String()
}

func (cui *PfUIS) GetArg(key string) (val string) {
	return cui.args.Get(key)
}

/* Force a CSRF check */
func (cui *PfUIS) GetArgCSRF(key string) (val string) {
	ok := cui.checkCSRF()
	if !ok {
		return
	}

	return cui.args.Get(key)
}

/* For when there are no more specific URLs */
func (cui *PfUIS) NoSubs() bool {
	if len(cui.path) > 0 && cui.path[0] != "" {
		H_error(cui, StatusNotFound)
		return true
	}

	return false
}

/*
 * Note: addr will always contain the full XFF
 * as received, but only the right hand side will be valid
 * we do not discard the components that are untrusted
 * as they might have forensic value
 *
 * The 'ip' returned is the trusted value though
 */
func (cui *PfUIS) ParseClientIP(remaddr string, xff string, xffc []*net.IPNet) (ip net.IP, addr string, err error) {
	gotip := false

	remaddr, _, err = net.SplitHostPort(remaddr)
	if err != nil {
		cui.Errf("RemoteAddr is invalid: %s", err.Error())
		return
	}

	/*
	 * Trust X-Forwarded-For when it is set
	 * this as we always sit behind a proxy
	 * and then RemoteAddr == 127.0.0.1
	 */
	addr = xff
	if addr != "" {
		addr += ","
	}
	/* The remote connection was last */
	addr += remaddr

	/* Standardize to ", " as a separator */
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

func (cui *PfUIS) AddHeader(h string, v string) {
	if cui.headers == nil {
		cui.headers = make(http.Header)
	}

	cui.headers.Add(h, v)
}

func (cui *PfUIS) SetHeader(h string, v string) {
	if cui.headers == nil {
		cui.headers = make(http.Header)
	}

	cui.headers.Set(h, v)
}

func (cui *PfUIS) OutUnbuffered(txt string) {
	fmt.Fprint(cui.w, txt)
}

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

	rc := cui.GetReturnCode()
	if rc != 0 {
		hdr["X-ReturnCode"] = strconv.Itoa(rc)
	}

	for h, v := range hdr {
		cui.w.Header().Add(h, v)
	}

	/* The Content Type */
	if cui.contenttype != "" {
		cui.w.Header().Set("Content-Type", cui.contenttype)
	}

	/* Expiration */
	if cui.expires != "" {
		cui.w.Header().Set("Expires", cui.expires)
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

func (cui *PfUIS) SetExpired() {
	cui.expires = time.Date(2015, 01, 01, 1, 5, 3, 0, time.UTC).Format(http.TimeFormat)
	cui.SetHeader("Cache-Control", "no-cache, no-store, max-age=0, must-revalidate")
}

func (cui *PfUIS) SetExpires(minutes int) {
	cui.expires = time.Now().UTC().Add(time.Duration(minutes) * time.Minute).Format(http.TimeFormat)
}

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

func (cui *PfUIS) SetFileName(fname string) {
	cui.AddHeader("Content-Disposition", "inline; filename=\""+fname+"\"")
}

func (cui *PfUIS) SetStaticFile(file string) {
	cui.staticfile = file
}

func (cui *PfUIS) SetRaw(raw []byte) {
	cui.raw = raw
}

func (cui *PfUIS) SetJSON(json []byte) {
	cui.SetContentType("application/json")
	cui.SetRaw(json)
}

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

func (cui *PfUIS) GetCrumbPath() (path string) {
	return cui.crumbpath
}

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

func (p *PfPage) AddCSS(css string) {
	p.CSS = append(p.CSS, css)
}

func (p *PfPage) AddJS(js string) {
	p.Javascript = append(p.Javascript, js)
}

func (cui *PfUIS) Page_show(name string, data interface{}) {
	/* Need to delay so that we can set cookies/headers etc */
	cui.show_name = name
	cui.show_data = data
}

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

func (cui *PfUIS) GetBody() (body []byte) {
	body, _ = ioutil.ReadAll(cui.r.Body)
	return body
}

func (cui *PfUIS) parseform() {
	err := cui.r.ParseMultipartForm(defaultMaxMemory)
	if err == http.ErrNotMultipart {
		/*
		 * Ignore, it parsed the form anyway
		 * the form just is not multipart
		 */
	} else if err != nil {
		cui.Errf("parseform() - err: %s", err.Error())
	}
}

func (cui *PfUIS) GetFormFileReader(key string) (file io.ReadCloser, filename string, err error) {
	var fh *multipart.FileHeader
	file, fh, err = cui.r.FormFile(key)

	if err == nil && fh != nil {
		filename = fh.Filename
	}
	return
}

/* Returns a base64 encoded representation of a file */
func (cui *PfUIS) GetFormFile(key string, maxsize string, b64 bool) (val string, err error) {
	var file io.ReadCloser
	var bytes []byte

	val = ""

	file, _, err = cui.GetFormFileReader(key)
	if err != nil {
		return
	}

	/* An Image is expected */
	if maxsize != "" {
		bytes, err = pf.Image_resize(file, maxsize)
	} else {
		/* Not an image, just read in the bytes */
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

/*
 * Form handler - called for form submissions
 *
 * Use this in combination with 'object set <form-element> <value>' style commands
 * For each matching form-element (name) the command is executed
 */
func (cui *PfUIS) HandleForm(cmd string, args []string, obj interface{}) (msg string, err error) {
	return cui.HandleFormS(cmd, false, args, obj)
}

/*
 * When autoop is false, the operand is already in the 'cmd'.
 *
 * When autoop is true, we determine the op based on the 'submit' button.
 * When 'submit' is 'Add'/'Remove' the op becomes that and in lowercase
 * appended to the cmd.
 *
 * When autoop is true, we ignore slices unless the op is add or remove.
 */
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
		if ftype == "ignore" || ftype == "button" || ftype == "note" {
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

/*
 * Command handler - directly call a command with arguments
 *
 * Use this for form posts with specific parameters
 * The parameters are matched to the command
 */
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
		/* Received a token */
		cui.token_recv = tok
		expsoon, err := cui.LoginToken(tok)
		if err != nil {
			cui.Dbgf("LoginToken failed: %s", err.Error())
		} else {
			/* Valid token */
			cui.token_exp = expsoon

			/* Check if we need to swap SysAdmin mode */
			xtra := cui.r.URL.Query().Get("xtra")
			switch xtra {
			case "swapadmin":
				cui.SwapSysAdmin()
				break

			default:
				/* Everything else is silently ignored */
				break
			}
		}
	} else {
		/* Did not receive a token */
		cui.token_recv = ""
	}
}

func (cui *PfUIS) setToken(w http.ResponseWriter) {
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
				b := "Bearer realm=\"" + pf.System_Get().Name + "\" access_token=\"" + cui.GetToken() + "\""
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
			w.Header().Set("WWW-Authenticate", "Bearer realm=\""+pf.System_Get().Name+"\"")
		} else {
			http.SetCookie(w, &http.Cookie{Name: G_cookie_name, Path: "/", Value: "invalid", Expires: time.Date(2015, 01, 01, 1, 5, 3, 0, time.UTC), MaxAge: -1, HttpOnly: true, Secure: g_securecookies})
		}
	} else {
		/* Nothing to do */
	}
}

func NewlineBR(val string) (safe template.HTML) {
	esc := template.HTMLEscapeString(val)
	return template.HTML(strings.Replace(esc, "\n", "<br />\n", -1))
}

func UIInit(securecookies bool, cookie_name string) error {
	g_securecookies = securecookies

	/* The cookie name */
	G_cookie_name = strings.ToLower(cookie_name)

	/* No problems to be generated here yet */
	return nil
}
