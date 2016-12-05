package pitchforkui

import (
	"errors"
	"strconv"
	pf "trident.li/pitchfork/lib"
)

type PfUserUI struct {
}

func h_user_username(cui PfUI) {
	var msg string
	var err error

	user := cui.SelectedUser()

	if cui.IsPOST() {
		var confirmed_s string
		confirmed_s, err = cui.FormValue("confirm")
		newname, err2 := cui.FormValue("username")

		confirmed := pf.IsTrue(confirmed_s)

		if err == nil && err2 == nil && confirmed && newname != "" {
			if newname == user.GetUserName() {
				err = errors.New("Name did not change")
			} else {
				cmd := "user set ident"
				arg := []string{user.GetUserName(), newname}
				msg, err = cui.HandleCmd(cmd, arg)

				if err == nil {
					/* Logout as identity changed */
					cui.Logout()

					/* Redirect to Login page */
					cui.SetRedirect("/login/", StatusSeeOther)
					return
				}
			}
		}
	}

	var errmsg = ""

	if err != nil {
		/* Failed */
		errmsg = err.Error()
	} else {
		/* Success */
	}

	type np struct {
		UserName string `label:"New username" pfreq:"yes" hint:"The new username"`
		Confirm  bool   `label:"Confirm username change" pfreq:"yes" hint:"Confirm username change"`
		Button   string `label:"Change username" pftype:"submit"`
	}

	/* Output the page */
	type Page struct {
		*PfPage
		Opt     np
		Message string
		Error   string
	}

	p := Page{cui.Page_def(), np{user.GetUserName(), false, ""}, msg, errmsg}
	cui.Page_show("user/username.tmpl", p)
}

func h_user_password(cui PfUI) {
	var msg string
	var err error

	user := cui.SelectedUser()

	if cui.IsPOST() {
		var passc string
		passc, err = cui.FormValue("passwordC")
		pass1, err2 := cui.FormValue("password1")
		pass2, err3 := cui.FormValue("password2")

		if err == nil && err2 == nil && err3 == nil && passc != "" && pass1 != "" && pass1 == pass2 {
			cmd := "user password set"
			arg := []string{"portal", user.GetUserName(), passc, pass1}
			msg, err = cui.HandleCmd(cmd, arg)
		}
	}

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
		Message  string
		Error    string
		PWRules  string
		PWLenMin string
		PWLenMax string
	}

	sys := pf.System_Get()

	pwmin := "8"
	pwmax := ""

	if sys.PW_Enforce {
		if sys.PW_Length > 8 {
			pwmin = strconv.Itoa(sys.PW_Length)
		}

		if sys.PW_LengthMax > 8 {
			pwmax = strconv.Itoa(sys.PW_LengthMax)
		}
	}

	p := Page{cui.Page_def(), msg, errmsg, "", pwmin, pwmax}
	cui.Page_show("user/password.tmpl", p)
}

func H_user_pwreset(cui PfUI) {
	var err error
	var msg string

	if cui.NoSubs() {
		return
	}

	if cui.IsPOST() {
		var confirmed string
		confirmed, err = cui.FormValue("confirm")
		if confirmed == "on" {
			var username string
			username, err = cui.FormValue("username")
			if err == nil {
				cmd := "user password reset"
				arg := []string{username}
				msg, err = cui.HandleCmd(cmd, arg)
			}
		}
	}

	var errmsg = ""

	if err != nil {
		/* Failed */
		errmsg = err.Error()
	} else {
		/* Success */
	}

	type np struct {
		UserName string `label:"Username to reset" pfset:"none" hint:"The username to ask a password reset for"`
		Confirm  bool   `label:"Confirm reset request" pfreq:"yes" hint:"Confirm reset request"`
		Button   string `label:"Request Password reset" pftype:"submit"`
	}

	/* Output the page */
	type Page struct {
		*PfPage
		Opt     np
		Message string
		Error   string
	}

	username := ""
	user := cui.SelectedUser()
	if user != nil {
		username = user.GetUserName()

	}

	p := Page{cui.Page_def(), np{username, false, ""}, msg, errmsg}
	cui.Page_show("user/pwreset.tmpl", p)
}

func h_user_index(cui PfUI) {
	user := cui.SelectedUser()

	/* Output the page */
	type Page struct {
		*PfPage
		User pf.PfUser
	}

	p := Page{cui.Page_def(), user}

	cui.Page_show("user/index.tmpl", p)
}

func h_user_pgp_keys(cui PfUI) {
	var err error

	user := cui.SelectedUser()
	keys, err := user.GetKeys(cui)
	if err != nil {
		/* Temp redirect to unknown */
		H_NoAccess(cui)
		return
	}

	fname := user.GetUserName() + ".asc"

	cui.SetContentType("application/pgp-keys")
	cui.SetFileName(fname)
	cui.SetExpires(60)
	cui.SetRaw(keys)
	return
}

type PfUserDetailForm struct {
	Type   map[string]string `label:"Detail Type" pfreq:"yes" hint:"Select the detail you would like to set" default:"none"`
	Value  string            `label:"Value" pfreq:"yes" hint:"Value of the detail."`
	Button string            `label:"Add Detail" tritype:"submit"`
}

type PfUserLanguageForm struct {
	Language map[string]string `label:"Language" pfreq:"yes" hint:"Select the language to add" default:"none"`
	Skill    map[string]string `label:"Skill Level" pfreq:"yes" hint:"Select the appropriate skill level" default:"none"`
	Button   string            `label:"Add Language" pftype:"submit"`
}

func H_user_profile_post(cui PfUI) (msg string, err error) {
	user := cui.SelectedUser()
	username := user.GetUserName()

	form_button, err1 := cui.FormValue("button")
	if err1 == nil && form_button == "Add Detail" {
		dtype, err2 := cui.FormValue("type")
		if err2 == nil && dtype != "none" {
			value, err2 := cui.FormValue("value")
			if err2 == nil {
				cmd := "user detail set"
				arg := []string{username, dtype, value}
				msg, err = cui.HandleCmd(cmd, arg)
			}
		}
	} else if err1 == nil && form_button == "Add Language" {
		language, err1 := cui.FormValue("language")
		skill, err2 := cui.FormValue("skill")
		if err1 == nil && language != "none" && err2 == nil && skill != "none" {
			cmd := "user language set"
			arg := []string{username, language, skill}
			msg, err = cui.HandleCmd(cmd, arg)
		}
	} else {
		cmd := "user set"
		arg := []string{user.GetUserName()}

		msg, err = cui.HandleForm(cmd, arg, user)
	}

	return
}

func H_user_detail_form() (detail_form PfUserDetailForm, err error) {
	detailset, err := pf.DetailList()
	if err != nil {
		return
	}

	detail_form.Type = make(map[string]string)
	detail_form.Type["none"] = "-- Select --"

	for _, d := range detailset {
		detail_form.Type[d.Type] = d.ToString()
	}

	return
}

func H_user_language_form() (language_form PfUserLanguageForm, err error) {
	languageset, err := pf.LanguageList()
	if err != nil {
		return
	}

	langskillset := pf.LanguageSkillList()

	language_form.Language = make(map[string]string)
	language_form.Skill = make(map[string]string)
	language_form.Language["none"] = "-- Select --"
	language_form.Skill["none"] = "-- Select --"

	for _, l := range languageset {
		language_form.Language[l.Code] = l.ToString()
	}

	for _, s := range langskillset {
		language_form.Skill[s] = s
	}

	return
}

func h_user_profile(cui PfUI) {
	var isedit bool
	var err error
	var msg string
	var errmsg = ""

	/* SysAdmin and User-Self can edit */
	isedit = cui.IsSysAdmin() || cui.SelectedSelf()

	user := cui.SelectedUser()

	if isedit && cui.IsPOST() {
		msg, err = H_user_profile_post(cui)
	}

	if err != nil {
		/* Failed */
		errmsg = err.Error()
	} else {
		/* Success */
	}

	/* Refresh updated version */
	err = user.Refresh(cui)
	if err != nil {
		errmsg += err.Error()
	}

	/* Output the page */
	type Page struct {
		*PfPage
		Message      string
		Error        string
		User         pf.PfUser
		IsEdit       bool
		Details      []pf.PfUserDetail
		DetailForm   PfUserDetailForm
		Languages    []pf.PfUserLanguage
		LanguageForm PfUserLanguageForm
	}

	/* Set the last link nicer */
	cui.AddCrumb("", "Profile", user.GetFullName()+" ("+user.GetUserName()+")'s Profile")

	details, err := user.GetDetails()
	if err != nil {
		cui.Errf("Failed to GetDetails(): %s", err.Error())
		H_error(cui, StatusBadRequest)
		return
	}

	detail_form, err := H_user_detail_form()
	if err != nil {
		cui.Errf("Failed to GetDetailForm(): %s", err.Error())
		H_error(cui, StatusBadRequest)
		return
	}

	languages, err := user.GetLanguages()
	if err != nil {
		cui.Errf("Failed to GetLanguages(): %s", err.Error())
		H_error(cui, StatusBadRequest)
		return
	}

	language_form, err := H_user_language_form()
	if err != nil {
		cui.Errf("Failed to GetDetailForm(): %s", err.Error())
		H_error(cui, StatusBadRequest)
		return
	}

	p := Page{cui.Page_def(), msg, errmsg, user, isedit, details, detail_form, languages, language_form}
	cui.Page_show("user/profile.tmpl", p)
}

func h_user_log(cui PfUI) {
	user := cui.SelectedUser()
	h_system_logA(cui, user.GetUserName(), "")
}

func h_user_list(cui PfUI) {
	if !cui.IsSysAdmin() {
		/* Non-SysAdmin can only see their own page */
		cui.SetRedirect("/user/"+cui.TheUser().GetUserName()+"/", StatusSeeOther)
		return
	}

	total := 0
	offset := 0

	offset_v := cui.GetArg("offset")
	if offset_v != "" {
		offset, _ = strconv.Atoi(offset_v)
	}

	search := cui.GetArg("search")

	user := cui.NewUser()
	total, _ = user.GetListMax(search)
	users, err := user.GetList(cui, search, offset, 10)

	if err != nil {
		cui.Err(err.Error())
		return
	}

	/* Output the page */
	type Page struct {
		*PfPage
		Users       []pf.PfUser
		PagerOffset int
		PagerTotal  int
		Search      string
	}

	cui.SetPageMenu(nil)
	p := Page{cui.Page_def(), users, offset, total, search}
	cui.Page_show("user/list.tmpl", p)
}

func h_user_image(cui PfUI) {
	user := cui.SelectedUser()
	img, err := user.GetImage(cui)
	if err != nil {
		/* Temp redirect to unknown */
		cui.SetRedirect(pf.System_Get().UnknownImg, StatusFound)
		return
	}

	cui.SetContentType("image/png")
	cui.SetExpires(60)
	cui.SetRaw(img)
}

func h_user(cui PfUI) {
	path := cui.GetPath()

	/* No user selected? */
	if len(path) == 0 || path[0] == "" {
		h_user_list(cui)
		return
	}

	/* Select the user */
	err := cui.SelectUser(path[0], PERM_USER_SELF|PERM_USER_VIEW)
	if err != nil {
		cui.Err("User: " + err.Error())
		H_NoAccess(cui)
		return
	}

	user := cui.SelectedUser()

	cui.AddCrumb(path[0], user.GetUserName(), user.GetFullName()+" ("+user.GetUserName()+")")

	cui.SetPath(path[1:])

	/* /user/<username>/{path} */
	menu := NewPfUIMenu([]PfUIMentry{
		{"", "", PERM_USER | PERM_USER_VIEW, h_user_index, nil},
		{"profile", "Profile", PERM_USER_SELF | PERM_USER_VIEW, h_user_profile, nil},
		{"username", "Username", PERM_USER_SELF, h_user_username, nil},
		{"password", "Password", PERM_USER_SELF, h_user_password, nil},
		{"2fa", "2FA Tokens", PERM_USER_SELF, h_user_2fa, nil},
		{"email", "Email", PERM_USER_SELF, h_user_email, nil},
		{"pgp_keys", "Download PGP Keys", PERM_USER_SELF, h_user_pgp_keys, nil},
		{"image.png", "", PERM_USER_VIEW, h_user_image, nil},
		{"log", "Audit Log", PERM_USER_SELF, h_user_log, nil},
		{"pwreset", "Password Reset", PERM_GROUP_ADMIN, H_user_pwreset, nil},

		/*
		 * TODO: Select the user/, pass to Group
		 * {"group", "", PERM_USER_SELF | PERM_USER_VIEW, h_grp},
		 */
	})

	cui.UIMenu(menu)
}
