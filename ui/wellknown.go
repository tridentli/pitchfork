package pitchforkui

import (
	"encoding/json"
	"net/url"
	"strings"
	pf "trident.li/pitchfork/lib"
)

// h_wellknown handles /.wellknown/ URLs
func h_wellknown(cui PfUI) {
	p := ""
	path := cui.GetPath()

	if len(path) != 0 {
		p = path[0]
	}

	switch p {
	case "":
		p := cui.Page_def()
		cui.PageShow("misc/wellknown.tmpl", p)
		return

	case "webfinger":
		if pf.System_Get().OAuthEnabled {
			h_webfinger(cui)
			return
		}
		break

	default:
		break
	}

	H_error(cui, StatusNotFound)
}

// h_webfinger handles webfinger (OpenID 1.0) support
func h_webfinger(cui PfUI) {
	var err error
	var username string

	/* The Spec we honor */
	spec := "http://openid.net/specs/connect/1.0/issuer"

	/* Arguments */
	res := cui.GetArg("resource")
	rel := cui.GetArg("rel")

	/* We need to ignore the rel if we do not know it */

	/* Check the resource */

	if res == "" {
		cui.Err("No resource specified")
		H_error(cui, StatusNotFound)
		return
	}

	if strings.Contains(res, "@") {
		/* Assume email address */
		c := strings.Split(res, "@")
		if c[1] != pf.System_Get().PeopleDomain {
			cui.Err("Unknown domain: " + c[1])
			H_error(cui, StatusNotFound)
			return
		}

		username = c[0]
	} else {
		/* Website link */
		var u *url.URL
		u, err = url.Parse(res)
		if err != nil {
			/* Not a valid URL, thus not found */
			cui.Err("Resource is not a valid URL: " + res)
			H_error(cui, StatusNotFound)
			return
		}

		loc := u.Scheme + "://" + u.Host
		if loc != pf.System_Get().PublicURL {
			/* Not our public URL */
			cui.Err("Resource is not our public URL: " + res)
			H_error(cui, StatusNotFound)
			return
		}

		/* The username */
		username = strings.TrimSuffix(u.Path, "/")

		if len(username) == 0 || username[0] != '~' {
			/* Not a valid user URL */
			cui.Err("Unknown user '" + username + "' for " + res)
			H_error(cui, StatusNotFound)
			return
		}

		/* Strip the '~' */
		username = username[1:]
	}

	/* We should verify that the username exists, but we won't reveal this */

	type jrdlnk struct {
		Rel  string `json:"rel"`
		Href string `json:"href"`
	}

	type jrd struct {
		Subject string   `json:"subject"`
		Links   []jrdlnk `json:"links"`
	}

	var j jrd
	var jl jrdlnk

	/* Always just use the email address */
	j.Subject = "acct:" + username + "@" + pf.System_Get().PeopleDomain
	j.Links = make([]jrdlnk, 0)

	if rel == spec {
		jl.Rel = spec
		jl.Href = pf.System_Get().PublicURL + "/oauth2/"
		j.Links = append(j.Links, jl)
	}

	txt, err := json.Marshal(j)
	if err != nil {
		msgs := []string{"JSON encoding failed"}
		H_errmsgs(cui, msgs)
		return
	}

	cui.SetHeader("Access-Control-Allow-Origin", "*")
	cui.SetContentType("application/jrd+json")
	cui.SetRaw(txt)
}
