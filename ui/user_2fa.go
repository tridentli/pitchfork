package pitchforkui

import (
	"encoding/base32"
	"strconv"
	"strings"
	"trident.li/keyval"
	pf "trident.li/pitchfork/lib"
)

type TFATok struct {
	cui         PfUI
	CurPassword string `label:"Current Password" pfreq:"yes" hint:"Your current password" pftype:"password"`
	Descr       string `label:"Description" pfreq:"yes" hint:"iPhone, Android etc."`
	Type        string `label:"OTP Type" pfreq:"yes" hint:"The Two Factor Token Type" options:"GetTypeOpts"`
	Button      string `label:"Create" pftype:"submit"`
}

func NewTFATok(cui PfUI) (tok *TFATok) {
	return &TFATok{cui, "", "", "TOTP", ""}
}

func (tok *TFATok) GetTypeOpts(obj interface{}) (kvs keyval.KeyVals, err error) {
	return pf.TwoFactorTypes(), nil
}

func (tok *TFATok) ObjectContext() (obj interface{}) {
	return tok.cui
}

func h_user_2fa_list(cui PfUI) {
	user := cui.SelectedUser()
	tokens, err := user.Fetch2FA()

	var errmsg = ""

	if err != nil {
		/* Failed */
		errmsg = err.Error()
	} else {
		/* Success */
	}

	/* Output the page */
	type Page struct {
		*PfPage
		Tok     *TFATok
		Tokens  []pf.PfUser2FA
		Message string
		Error   string
		PWRules string
	}

	tok := NewTFATok(cui)
	p := Page{cui.Page_def(), tok, tokens, "", errmsg, ""}
	cui.Page_show("user/2fa/list.tmpl", p)
}

func h_user_2fa_add(cui PfUI) {
	var err error
	errmsg := ""
	msg := ""
	qr := ""

	user := cui.SelectedUser()

	if cui.IsPOST() {
		cmd := "user 2fa add"
		arg := []string{user.GetUserName(), "", "", ""}

		msg, err = cui.HandleCmd(cmd, arg)

		if err != nil {
			errmsg = err.Error()
		} else {
			/* Parse the message */
			lines := strings.Split(msg, "\n")
			for _, l := range lines {
				s := strings.SplitN(l, ":", 2)
				switch s[0] {
				case "URL":
					u := strings.TrimSpace(s[1])
					qr = base32.StdEncoding.EncodeToString([]byte(u))

					/*
					 * We encode the URL in base32, as base64 includes
					 * slashes indeed, we transfer this in a URL, but
					 * that goes in HTTPS
					 *
					 * Doing base32 ensures we do not get clashes
					 * with URL encoding
					 */
					break
				}
			}
		}
	}

	/* Output the page */
	type Page struct {
		*PfPage
		User    pf.PfUser
		Message string
		Error   string
		QR      string
	}

	p := Page{cui.Page_def(), user, msg, errmsg, qr}
	cui.Page_show("user/2fa/create.tmpl", p)
}

func user_2fa_mod(cui PfUI, how string) (err error) {
	user := cui.SelectedUser()
	token := cui.SelectedUser2FA()
	token_id := strconv.Itoa(token.Id)

	cmd := "user 2fa " + how
	arg := []string{user.GetUserName(), token_id, "" /* Current password */}

	/* Enable needs a 'twofactorcode' argument, thus add a argument slot */
	if how == "enable" {
		arg = append(arg, "")
	}

	_, err = cui.HandleCmd(cmd, arg)
	return err
}

func h_user_2fa(cui PfUI) {
	button, err := cui.FormValue("button")
	if err == nil && button == "Create" {
		h_user_2fa_add(cui)
		return
	}

	path := cui.GetPath()

	/* No token selected? */
	if len(path) == 0 || path[0] == "" {
		h_user_2fa_list(cui)
		return
	}

	id, err := strconv.Atoi(path[0])
	if err != nil {
		cui.Err("User2FA: " + err.Error())
		H_error(cui, StatusNotFound)
		return
	}

	/* Select the token */
	err = cui.SelectUser2FA(id, PERM_USER_SELF|PERM_USER_VIEW)
	if err != nil {
		cui.Err("User2FA: " + err.Error())
		H_NoAccess(cui)
		return
	}

	user := cui.SelectedUser()
	token := cui.SelectedUser2FA()

	cui.AddCrumb(path[0], strconv.Itoa(token.Id),
		token.Name+" ("+token.Type+" "+strconv.Itoa(token.Id)+")")

	cui.SetPath(path[1:])

	var pe_err string
	var pd_err string
	var pr_err string

	/* Some action we need to apply? */
	if cui.IsPOST() {
		var button string
		button, err = cui.FormValue("button")
		if err != nil {
			H_errmsg(cui, err)
			return
		}

		switch button {
		case "Enable":
			err = user_2fa_mod(cui, "enable")
			if err != nil {
				pe_err = err.Error()
			}
			break

		case "Disable":
			err = user_2fa_mod(cui, "disable")
			if err != nil {
				pd_err = err.Error()
			}
			break

		case "Remove":
			err = user_2fa_mod(cui, "remove")
			if err == nil {
				url := "/user/" + user.GetUserName() + "/2fa/"
				cui.SetRedirect(url, StatusSeeOther)
				return
			} else {
				pr_err = err.Error()
			}
			break

		default:
			H_errtxt(cui, "Unknown action")
			break
		}
	}

	var isedit bool
	var errmsg = ""

	/* SysAdmin and User-Self can edit */
	isedit = cui.IsSysAdmin() || cui.SelectedSelf()

	/* Fetch updated version */
	err = token.Refresh()

	if err != nil {
		/* Failed */
		errmsg = err.Error()
	}

	type popt_enable struct {
		CurPassword   string `label:"Current Password" pfreq:"yes" hint:"Your current password" pftype:"password"`
		TwoFactorCode string `label:"Two Factor Code" pfreq:"yes"`
		Button        string `label:"Enable" pftype:"submit" htmlclass:"allow"`
		Error         string /* Used by pfform() */
	}

	type popt_disable struct {
		CurPassword string `label:"Current Password" pfreq:"yes" hint:"Your current password" pftype:"password"`
		Button      string `label:"Disable" pftype:"submit" htmlclass:"deny"`
		Error       string /* Used by pfform() */
	}

	type popt_remove struct {
		CurPassword string `label:"Current Password" pfreq:"yes" hint:"Your current password" pftype:"password"`
		Button      string `label:"Remove" pftype:"submit" htmlclass:"deny"`
		Error       string /* Used by pfform() */
	}

	/* Output the package */
	type Page struct {
		*PfPage
		User    pf.PfUser
		Error   string
		Token   pf.PfUser2FA
		IsEdit  bool
		Enable  popt_enable
		Disable popt_disable
		Remove  popt_remove
	}

	pe := popt_enable{Error: pe_err}
	pd := popt_disable{Error: pd_err}
	pr := popt_remove{Error: pr_err}

	p := Page{cui.Page_def(), user, errmsg, token, isedit, pe, pd, pr}
	cui.Page_show("user/2fa/edit.tmpl", p)
}
