// Pitchfork OAuth 2.0 implementation (UI portion).
//
// templates/oauth2/index.tmpl contains references to the relevant standard documents
package pitchforkui

import (
	"encoding/json"
	"net/url"
	"strings"
	"time"
	pf "trident.li/pitchfork/lib"
)

// oauth2_get gets a POST/GET parameter and set errors where needed
func oauth2_get(cui PfUI, errs *[]string, varname string) (val string) {
	var err error = nil
	val = cui.GetArg(varname)
	if val == "" {
		/* Try to get it from a POST request */
		val, err = cui.FormValueNoCSRF(varname)
	}

	val = strings.TrimSpace(val)
	if err != nil {
		*errs = append(*errs, "Missing "+varname)
	} else if val == "" {
		*errs = append(*errs, "Empty "+varname)
	}

	return val
}

// oauth2_check_redir verifies that the URL is valid
func oauth2_check_redir(redir string, errs *[]string) (u *url.URL, v url.Values) {
	u, err := url.Parse(redir)
	if err == nil {
		v, err = url.ParseQuery(u.RawQuery)
	}

	if err != nil {
		*errs = append(*errs, "Redirect URL could not be properly parsed: "+err.Error())
	}

	return
}

// oauth2_authorize is the OAuth 2.0 Authorization code endpoint
func oauth2_authorize(cui PfUI) {
	var o pf.OAuth_Auth
	var errs []string = nil

	o.RType = oauth2_get(cui, &errs, "response_type")
	o.ClientID = oauth2_get(cui, &errs, "client_id")
	o.Redirect = oauth2_get(cui, &errs, "redirect_uri")
	o.Scope = oauth2_get(cui, &errs, "scope")

	/* Check validity */
	switch o.RType {
	case "code":
		break

	case "token":
		break

	case "":
		/* handled by oauth2_get() */
		break

	default:
		errs = append(errs, "Not supported or unknown response_type "+o.RType)
		break
	}

	u, v := oauth2_check_redir(o.Redirect, &errs)

	if errs != nil {
		H_errmsgs(cui, errs)
		return
	}

	/* Not Logged in? Send to login page so they auth first */
	if !cui.IsLoggedIn() {
		/* h_login sets a 'comeback' url */
		h_login(cui)
		return
	}

	/* Is it a POST? */
	if cui.IsPOST() {
		/* Check if Authorize or Deny */
		but, err := cui.FormValue("button")
		if err != nil {
			errs = append(errs, "No button was pressed")
			H_errmsgs(cui, errs)
			return
		}

		switch but {
		case "Authorize":
			var tok string
			tok, err = pf.OAuth2_AuthToken_New(cui, o)
			if err != nil {
				errs = append(errs, "Could not generate Token")
				H_errmsgs(cui, errs)
				return
			}

			switch o.RType {
			case "code":
				v.Add("code", tok)
				u.RawQuery = v.Encode()
				url := u.String()
				cui.SetRedirect(url, StatusFound)
				return

			case "token":
				v.Add("access_token", tok)
				u.RawQuery = v.Encode()
				url := u.String()
				cui.SetRedirect(url, StatusFound)
				return

			default:
				break
			}

			/* Can't get here due to check above */
			H_error(cui, StatusInternalServerError)
			return

		case "Deny":
			url := o.Redirect
			cui.SetRedirect(url, StatusFound)
			return
		}

		/* Not a valid button, try again */
	}

	/* Show OAuth2 Authorize page */
	type Page struct {
		*PfPage
		Oauth pf.OAuth_Auth
	}

	p := Page{cui.Page_def(), o}
	cui.Page_show("oauth2/authorize.tmpl", p)
}

// oauth2_token is the access token endpoint
func oauth2_token(cui PfUI) {
	var errs []string

	client_id := oauth2_get(cui, &errs, "client_id")
	grant_type := oauth2_get(cui, &errs, "grant_type")
	redirect := oauth2_get(cui, &errs, "redirect_uri")
	code := oauth2_get(cui, &errs, "code")

	/*
	 * We do not check the client_secret as authentication
	 * happens using tokens, the one in 'code'
	 * client_secret := oauth2_get(cui, &errs, "client_secret")
	 */

	/* Check redirect URL */
	oauth2_check_redir(redirect, &errs)

	/* Check the code */
	claims, err := pf.OAuth2_AuthToken_Check(code)

	if err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) != 0 {
		H_errmsgs(cui, errs)
		return
	}

	/* Who they claim they are */
	if claims.ClientID != client_id {
		errs = append(errs, "Mismatching client_id")
		H_errmsgs(cui, errs)
		return
	}

	/* Scope */
	scope := claims.Scope

	/* Grant Type */
	switch grant_type {
	case "authorization_code":
		var at struct {
			Access_token string `json:"access_token"`
			Token_type   string `json:"token_type"`
			Scope        string `json:"scope"`
			Info         struct {
				Name string `json:"name"`
			} `json:"info"`
		}

		var tok string
		tok, err = pf.OAuth2_AccessToken_New(cui, client_id, scope)

		at.Access_token = tok
		at.Token_type = "bearer"
		at.Scope = scope
		at.Info.Name = "Trident/Pitchfork"

		txt, err := json.Marshal(at)
		if err != nil {
			msgs := []string{"JSON encoding failed"}
			H_errmsgs(cui, msgs)
			return
		}

		cui.SetJSON(txt)
		return

	case "token":
		return

	case "password":
		return
	}
}

// oauth2_info is the Information Endpoint
func oauth2_info(cui PfUI) {
	var errs []string

	code := oauth2_get(cui, &errs, "code")
	if code == "" {
		errs = append(errs, "Missing code")
	}

	if len(errs) != 0 {
		H_errmsgs(cui, errs)
		return
	}

	var inf struct {
		ClientID     string `json:"client_id"`
		Access_token string `json:"access_token"`
		Token_type   string `json:"token_type"`
		Scope        string `json:"scope"`
		Expires_in   int64  `json:"expires_in"`
	}

	claims, err := pf.OAuth2_AuthToken_Check(code)
	if err != nil {
		msgs := []string{err.Error()}
		H_errmsgs(cui, msgs)
		return
	}

	inf.ClientID = claims.ClientID
	inf.Access_token = code
	inf.Token_type = "bearer"
	inf.Scope = claims.Scope
	inf.Expires_in = time.Now().Unix() - claims.ExpiresAt

	txt, err := json.Marshal(inf)
	if err != nil {
		msgs := []string{"JSON encoding failed"}
		H_errmsgs(cui, msgs)
		return
	}

	cui.SetJSON(txt)
}

// oauth2_index renders an informational index
func oauth2_index(cui PfUI) {
	p := cui.Page_def()
	cui.Page_show("oauth2/index.tmpl", p)
}

// h_oauth is the entry point for OAuth URLs
func h_oauth(cui PfUI) {
	menu := NewPfUIMenu([]PfUIMentry{
		{"", "OAuth2 / OpenID Connect Information", PERM_NONE, oauth2_index, nil},
		{"authorize", "Authorize", PERM_NONE | PERM_HIDDEN | PERM_NOCRUMB, oauth2_authorize, nil},
		{"token", "Token", PERM_NONE | PERM_HIDDEN | PERM_NOCRUMB, oauth2_token, nil},
		{"info", "Info", PERM_NONE | PERM_HIDDEN | PERM_NOCRUMB, oauth2_info, nil},
	})

	cui.UIMenu(menu)
}
