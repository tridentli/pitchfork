package pitchforkui

import (
	"strings"

	"trident.li/keyval"
	pf "trident.li/pitchfork/lib"
)

type VerifiedEM struct {
	cui        PfUI
	Action     string `label:"set" pftype:"hidden"`
	EmailAddr  string `label:"Email Address" pfreq:"yes" pfset:"user" hint:"Select the address to use for this group" options:"GetEmailOpts"`
	GroupName  string `label:"groupname" pftype:"hidden"`
	GroupDesc  string `label:"groupdesc" pftype:"hidden"`
	GroupState string `label:"groupstate" pftype:"hidden"`
	Button     string `label:"Select Address" pftype:"submit"`
}

func (vem VerifiedEM) GetEmailOpts(obj interface{}) (kvs keyval.KeyVals, err error) {
	var email pf.PfUserEmail

	cui := obj.(PfUI)
	user := cui.SelectedUser()

	EmList, _ := email.List(cui, user)
	for _, email = range EmList {
		if email.Verified {
			kvs.Add(email.Email, email.Email)
		}
	}
	return
}

func (vem VerifiedEM) ObjectContext() (obj interface{}) {
	return vem.cui
}

func h_user_email_index(cui PfUI) {
	var email pf.PfUserEmail

	type opt_add struct {
		Action string `label:"add" pftype:"hidden"`
		Email  string `label:"Email address" pfreq:"yes" hint:"Email address to add"`
		Button string `label:"Add Email Address" pftype:"submit"`
		Error  string /* Used by pfform() */
	}

	o_add := opt_add{}

	if cui.IsPOST() {
		action, err1 := cui.FormValue("action")
		if err1 != nil {
			o_add.Error = "Invalid input"
			action = "Invalid"
		}

		switch action {
		case "add":
			err := h_user_email_add(cui)
			if err == nil {
				return
			} else {
				o_add.Error = err.Error()
			}
			break

		case "set":
			h_user_email_grpemail(cui)
			break

		case "Invalid":
			break

		default:
			o_add.Error = "Invalid input"
			break
		}
	}

	grp := cui.NewGroup()

	user := cui.SelectedUser()
	recemail, _ := user.Get("recover_email")

	emlist, _ := email.List(cui, user)
	tglist, _ := grp.GetGroups(cui, user.GetUserName())

	/* Build the set of email addresses that can be selected from */

	/* Output the page */
	type Page struct {
		*PfPage
		Add           opt_add
		EmList        []pf.PfUserEmail
		VEmails       []VerifiedEM
		RecoveryEmail string
		Message       string
		Error         string
		PWRules       string
	}

	ves := []VerifiedEM{}

	for _, t := range tglist {
		if !t.GetGroupCanSee() {
			continue
		}

		ve := VerifiedEM{cui, "set", t.GetEmail(), t.GetGroupName(), t.GetGroupDesc(), t.GetGroupState(), ""}
		ves = append(ves, ve)
	}

	p := Page{cui.Page_def(), o_add, emlist, ves, recemail, "", "", ""}
	cui.Page_show("user/email/list.tmpl", p)
}

func h_user_email_add(cui PfUI) (err error) {
	user := cui.SelectedUser()
	cmd := "user email add"
	arg := []string{user.GetUserName(), ""}

	msg, err := cui.HandleCmd(cmd, arg)
	if err != nil {
		return
	}

	/* Split the message */
	msgs := strings.Split(msg, " ")
	email := strings.TrimSpace(msgs[1])

	/* Redirect */
	cui.SetRedirect("/user/"+user.GetUserName()+"/email/"+email+"/", StatusSeeOther)
	return
}

func h_user_email_remove(cui PfUI) (err error) {
	email := cui.SelectedEmail()

	cmd := "user email remove"
	arg := []string{email.Email}
	_, err = cui.HandleCmd(cmd, arg)
	return
}

func h_user_email_upload_key(cui PfUI) (err error) {
	email := cui.SelectedEmail()

	cmd := "user email pgp_add"
	arg := []string{email.Email, ""}
	_, err = cui.HandleCmd(cmd, arg)
	return
}

func h_user_email_set_recover(cui PfUI) (err error) {
	username := cui.SelectedUser().GetUserName()
	email := cui.SelectedEmail()

	cmd := "user set recover_email"
	arg := []string{username, email.Email}

	_, err = cui.HandleCmd(cmd, arg)
	return
}

func h_user_email_verify(cui PfUI) (err error) {
	email := cui.SelectedEmail()

	cmd := "user email confirm_begin"
	arg := []string{email.Email}

	_, err = cui.HandleCmd(cmd, arg)
	return

	return
}

func h_user_email_confirmform(cui PfUI) (err error) {
	cmd := "user email confirm"
	arg := []string{""}

	_, err = cui.HandleCmd(cmd, arg)
	return
}

func h_user_email_confirm(cui PfUI) {
	user := cui.SelectedUser()
	email := cui.SelectedEmail()
	token := cui.GetArg("verifycode")

	if token == "" {
		H_errtxt(cui, "No Verification Code")
		return
	}

	cmd := "user email confirm"
	arg := []string{token}

	_, err := cui.CmdOut(cmd, arg)
	if err != nil {
		H_errmsg(cui, err)
		return
	}

	cui.SetRedirect("/user/"+user.GetUserName()+"/email/"+email.Email+"/", StatusSeeOther)

	return
}

func h_user_email_edit(cui PfUI) {
	var isedit bool
	var err error
	var msg string
	var errmsg = ""

	user := cui.SelectedUser()
	email := cui.SelectedEmail()
	err = email.Fetch(email.Email)
	if err != nil {
		H_errmsg(cui, err)
		return
	}
	err = email.FetchGroups(cui)
	if err != nil {
		H_errmsg(cui, err)
		return
	}

	type opt_confirm struct {
		VerifyCode string `label:"Verification Code" pfreq:"yes" min:"3"`
		Action     string `label:"confirm" pftype:"hidden"`
		Button     string `label:"Confirm" pftype:"submit"`
		Error      string /* Used by pfform() */
	}

	type opt_uploadkey struct {
		Action  string `label:"uploadkey" pftype:"hidden"`
		Keyring string `label:"PGP Key" pfreq:"yes" pftype:"file" hint:"They keyring from which to extract the PGP Key"`
		Button  string `label:"Upload Key" pftype:"submit"`
		Error   string /* Used by pfform() */
	}

	o_confirm := opt_confirm{}
	o_uploadkey := opt_uploadkey{}

	/* SysAdmin and User-Self can edit */
	isedit = cui.IsSysAdmin() || cui.SelectedSelf()

	if isedit && cui.IsPOST() {
		action, err1 := cui.FormValue("action")
		if err1 != nil {
			errmsg = "Invalid input"
			action = "Invalid"
		}

		switch action {
		case "remove":
			err = h_user_email_remove(cui)

			/* Redirect when this worked out */
			if err == nil {
				cui.SetRedirect("/user/"+user.GetUserName()+"/email/", StatusSeeOther)
				return
			}
			break

		case "setrecover":
			err = h_user_email_set_recover(cui)
			break

		case "resend":
			err = h_user_email_verify(cui)
			break

		case "verify":
			err = h_user_email_verify(cui)
			break

		case "confirm":
			err = h_user_email_confirmform(cui)

			if err != nil {
				o_confirm.Error = err.Error()
			}
			break

		case "uploadkey":
			err = h_user_email_upload_key(cui)

			if err != nil {
				o_uploadkey.Error = err.Error()
			}
			break
		}
	}

	/* Fetch fresh copy of email details */
	err = email.Fetch(email.Email)

	if err != nil {
		/* Failed */
		errmsg = err.Error()
	}

	/* Restrict removal to unused email addresses */
	candelete := true
	if len(email.Groups) > 0 {
		candelete = false
	}

	isrecover := false
	recem, err := user.Get("recover_email")
	if err != nil {
		H_error(cui, StatusInternalServerError)
		return
	}

	if email.Email == recem {
		candelete = false
		isrecover = true
	}

	/* Output the package */
	type Page struct {
		*PfPage
		Message   string
		Error     string
		User      pf.PfUser
		Email     pf.PfUserEmail
		IsEdit    bool
		CanDelete bool
		IsRecover bool
		Confirm   opt_confirm
		UploadKey opt_uploadkey
	}

	p := Page{cui.Page_def(), msg, errmsg, user, email, isedit, candelete, isrecover, o_confirm, o_uploadkey}
	cui.Page_show("user/email/edit.tmpl", p)
}

func h_user_email_download_key(cui PfUI) {
	email := cui.SelectedEmail()

	fname := email.PgpKeyID + ".asc"

	/* Output the key */
	cui.SetContentType("application/pgp-keys")
	cui.SetFileName(fname)
	cui.SetExpires(60)
	cui.SetRaw([]byte(email.Keyring))
}

func h_user_email_grpemail(cui PfUI) {
	user := cui.SelectedUser()
	grp, err := cui.FormValue("groupname")
	email, err2 := cui.FormValue("emailaddr")

	if err == nil && err2 == nil {
		cmd := "user email member set"
		arg := []string{user.GetUserName(), grp, email}

		_, err := cui.HandleCmd(cmd, arg)
		if err != nil {
			H_errmsg(cui, err)
			return
		}
	}

	cui.SetRedirect("/user/"+user.GetUserName()+"/email/", StatusSeeOther)
}

func h_user_email(cui PfUI) {
	path := cui.GetPath()

	if len(path) == 0 || path[0] == "" {
		h_user_email_index(cui)
		return
	}

	address := path[0]
	err := cui.SelectEmail(address)
	if err != nil {
		cui.Dbg("Unconfigured email address")
		H_NoAccess(cui)
		return
	}

	cui.AddCrumb(address, address, address)
	cui.SetPath(path[1:])

	var menu = NewPfUIMenu([]PfUIMentry{
		{"", "Email Details", PERM_USER_SELF | PERM_HIDDEN | PERM_NOCRUMB, h_user_email_edit, nil},
		{"confirm", "Confirm Verification Token", PERM_USER_SELF | PERM_HIDDEN | PERM_NOCRUMB, h_user_email_confirm, nil},
		{"download", "Download PGP Key", PERM_USER_SELF | PERM_HIDDEN | PERM_NOCRUMB, h_user_email_download_key, nil},
	})

	cui.UIMenu(menu)
}
