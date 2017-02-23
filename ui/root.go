package pitchforkui

import (
	"html/template"
	"net/http"
	pf "trident.li/pitchfork/lib"
)

// PfRootUI provides an interface for the root of the UI
type PfRootUI interface {
	H_root(w http.ResponseWriter, r *http.Request)
}

// PfRootUIS is an implementation of PfRootUI
type PfRootUIS struct {
	newui PfNewUI
	PfRootUI
}

// NewPfRootUI creates a new pitchfork standard UI
func NewPfRootUI(newui PfNewUI) (o PfRootUI) {
	return &PfRootUIS{newui: newui}
}

// New creates a new PfUI
func (o *PfRootUIS) New() PfUI {
	return o.newui()
}

// h_index renders the index page ('/')
func h_index(cui PfUI) {
	type Page struct {
		*PfPage
		About template.HTML
	}

	/* TODO: Render Welcome using Markdown renderer */
	val := pf.System_Get().Welcome
	safe := NewlineBR(val)

	p := Page{cui.Page_def(), safe}
	p.HeaderImg = pf.System_Get().HeaderImg
	cui.PageShow("index.tmpl", p)
}

// h_robots handles /robots.txt requests, returning a version that allows or disallows indexing
func h_robots(cui PfUI) {
	if pf.System_Get().NoIndex {
		h_static_file(cui, "robots.txt")
	} else {
		h_static_file(cui, "robots-ok.txt")
	}
}

// H_root is the root page -- where Go's net/http gives the request over to us
func (o *PfRootUIS) H_root(w http.ResponseWriter, r *http.Request) {
	cui := o.New()

	err := cui.UIInit(w, r)
	if err != nil {
		cui.Err(err.Error())
		H_error(cui, StatusBadRequest)
		cui.Flush()
		return
	}

	/* Set cancelation signal */
	abort := w.(http.CloseNotifier).CloseNotify()
	cui.SetAbort(abort)

	path := cui.GetPath()

	/* Get the Client IP & remote address */
	err = cui.SetClientIP()
	if err != nil {
		/* Something wrong with figuring out who they are */
		cui.Errf("SetClientIP: %s", err.Error())
		H_error(cui, StatusServiceUnavailable)
		cui.Flush()
		return
	}

	/* Check for static files/dirs */
	statics := []string{"favicon.ico", "css", "gfx", "js"}
	for _, p := range statics {
		if path[0] == p {
			h_static(cui)
			cui.Flush()
			return
		}
	}

	/*
	 * Homedirectory redirect:
	 * https://example.net/~username/ redirects to /user/username/
	 */
	if len(path[0]) > 0 && path[0][0] == '~' {
		cui.SetRedirect("/user/"+path[0][1:]+"/", StatusFound)
		cui.Flush()
		return
	}

	/* Initialize the token */
	cui.InitToken()

	/* The main menu */
	menu := NewPfUIMenu([]PfUIMentry{
		{"", "Home", PERM_NONE, h_index, nil},

		/* Service Discovery */
		{".well-known", "", PERM_NONE, h_wellknown, nil},

		/* Choice files */
		{"robots.txt", "", PERM_NONE | PERM_NOSUBS, h_robots, nil},

		/* QR Codes */
		{"qr", "", PERM_USER, h_qr, nil},

		/* From mainmenu (hidden as they are shown there) */
		{"user", "User", PERM_USER | PERM_HIDDEN, h_user, nil},
		{"group", "Group", PERM_USER | PERM_HIDDEN, h_group, nil},
		{"system", "System", PERM_SYS_ADMIN | PERM_HIDDEN, h_system, nil},

		/* Extras */
		{"search", "Search", PERM_USER | PERM_HIDDEN, h_search, nil},
		{"cli", "CLI", PERM_CLI, h_cli, nil},
		{"api", "", PERM_LOOPBACK | PERM_API, h_api, nil},
		{"oauth2", "OAuth2", PERM_USER, h_oauth, nil},
		{"login", "Login", PERM_NONE | PERM_USER | PERM_NOSUBS, h_login, nil},
		{"logout", "Logout", PERM_NONE | PERM_USER | PERM_HIDDEN | PERM_NOSUBS, h_logout, nil},
	})

	cui.UIMenu(menu)

	/* Flush it all to the client */
	cui.Flush()
}
