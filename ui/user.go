package pitchforkui

import (
	"errors"
	"strconv"

	"trident.li/keyval"
	pf "trident.li/pitchfork/lib"
	pfpgp "trident.li/pitchfork/lib/pgp"
)

// h_user_username handles the username modification page
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

// h_user_pasword handles the user password modification page
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
			arg := []string{"portal", user.GetUserName(), pass1, passc}
			msg, err = cui.HandleCmd(cmd, arg)

			nuser := cui.SelectedUser()

			if nuser != user {
				/* Logout as password changed */
				cui.Logout()

				msg := "Your password has changed, please login again"
				h_relogin(cui, msg)
				return
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

// H_user_pwreset handles password resets
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

// h_user_index handles the user's index page
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

// h_user_pgp_keys returns the PGP keys of all the groups of a user
func h_user_pgp_keys(cui PfUI) {
	keyset := pfpgp.NewIndexedKeySet()
	user := cui.SelectedUser()
	err := user.GetKeys(cui, keyset)
	if err != nil {
		/* Temp redirect to unknown */
		H_NoAccess(cui)
		return
	}

	output := keyset.ToBytes()
	fname := user.GetUserName() + ".asc"

	cui.SetContentType("application/pgp-keys")
	cui.SetFileName(fname)
	cui.SetExpires(60)
	cui.SetRaw(output)
	return
}

// PfUserDetailForm is the struct used for the UserDetail Form
type PfUserDetailForm struct {
	Type   string `label:"Detail Type" pfreq:"yes" hint:"Select the detail you would like to set" options:"GetTypeOpts"`
	Value  string `label:"Value" pfreq:"yes" hint:"Value of the detail"`
	Button string `label:"Add Detail" pftype:"submit"`
}

// NewPfUserDetailForm is used to create a new, initialized, PfUserDetailForm
func NewPfUserDetailForm() (df *PfUserDetailForm) {
	return &PfUserDetailForm{"callsign", "", ""}
}

// GetTypeOpts is used to retrieve Callsign options
func (df *PfUserDetailForm) GetTypeOpts(obj interface{}) (kvs keyval.KeyVals, err error) {
	detailset, err := pf.DetailList()
	if err != nil {
		pf.Errf("ERROR WHILE GETTING DETAILS: %s", err.Error())
		return
	}

	for _, d := range detailset {
		kvs.Add(d.Type, d.ToString())
	}

	return
}

// h_user_profile_details is used to manage a user's profile details (callsign etc)
func h_user_profile_details(cui PfUI) {
	var err error
	var msg string
	var errmsg = ""

	/* SysAdmin and User-Self can edit */
	isedit := cui.IsSysAdmin() || cui.SelectedSelf()

	user := cui.SelectedUser()

	if cui.IsPOST() {
		dtype, err2 := cui.FormValue("type")
		if err2 == nil && dtype != "none" {
			value, err2 := cui.FormValue("value")
			if err2 == nil {
				cmd := "user detail set"
				arg := []string{user.GetUserName(), dtype, value}
				msg, err = cui.HandleCmd(cmd, arg)
			}
		}
	}

	if err != nil {
		/* Failed */
		errmsg = err.Error()
	} else {
		/* Success */
	}

	details, err := user.GetDetails()
	if err != nil {
		cui.Errf("Failed to GetDetails(): %s", err.Error())
		H_error(cui, StatusBadRequest)
		return
	}

	/* Output the page */
	type Page struct {
		*PfPage
		Message    string
		Error      string
		User       pf.PfUser
		IsEdit     bool
		Details    []pf.PfUserDetail
		DetailForm *PfUserDetailForm
	}

	detail_form := NewPfUserDetailForm()

	p := Page{cui.Page_def(), msg, errmsg, user, isedit, details, detail_form}
	cui.Page_show("user/detail.tmpl", p)
}

// PfUserLanguageForm is used as the form for Language options
type PfUserLanguageForm struct {
	Language string `label:"Language" pfreq:"yes" hint:"Select the language to add" options:"GetLanguageOpts"`
	Skill    string `label:"Skill Level" pfreq:"yes" hint:"Select the appropriate skill level" options:"GetSkillOpts"`
	Button   string `label:"Add Language" pftype:"submit"`
}

// NewPfUserLanguageForm creates a new PfUserLanguageForm
func NewPfUserLanguageForm() (lf *PfUserLanguageForm) {
	return &PfUserLanguageForm{"", "", ""}
}

// GetLanguageOpts returns the possible language options
func (lf *PfUserLanguageForm) GetLanguageOpts(obj interface{}) (kvs keyval.KeyVals, err error) {
	languageset, err := pf.LanguageList()
	if err != nil {
		return
	}

	for _, l := range languageset {
		kvs.Add(l.Code, l.ToString())
	}

	return
}

// GetSkillOpts returns the possible skill options for languages
func (n *PfUserLanguageForm) GetSkillOpts(obj interface{}) (kvs keyval.KeyVals, err error) {
	langskillset := pf.LanguageSkillList()

	for _, s := range langskillset {
		kvs.Add(s, s)
	}

	return
}

// h_user_profile_languages manages languages along with their skill option
func h_user_profile_languages(cui PfUI) {
	var err error
	var msg string
	var errmsg = ""

	/* SysAdmin and User-Self can edit */
	isedit := cui.IsSysAdmin() || cui.SelectedSelf()

	user := cui.SelectedUser()

	if cui.IsPOST() {
		language, err1 := cui.FormValue("language")
		skill, err2 := cui.FormValue("skill")
		if err1 == nil && language != "none" && err2 == nil && skill != "none" {
			cmd := "user language set"
			arg := []string{user.GetUserName(), language, skill}
			msg, err = cui.HandleCmd(cmd, arg)
		}
	}

	if err != nil {
		/* Failed */
		errmsg = err.Error()
	} else {
		/* Success */
	}

	languages, err := user.GetLanguages()
	if err != nil {
		cui.Errf("Failed to GetLanguages(): %s", err.Error())
		H_error(cui, StatusBadRequest)
		return
	}

	/* Output the page */
	type Page struct {
		*PfPage
		Message      string
		Error        string
		User         pf.PfUser
		IsEdit       bool
		Languages    []pf.PfUserLanguage
		LanguageForm *PfUserLanguageForm
	}

	language_form := NewPfUserLanguageForm()

	p := Page{cui.Page_def(), msg, errmsg, user, isedit, languages, language_form}
	cui.Page_show("user/language.tmpl", p)
}

// h_user_profile manages the user's profile
func h_user_profile(cui PfUI) {
	var err error
	var msg string
	var errmsg = ""

	/* SysAdmin and User-Self can edit */
	isedit := cui.IsSysAdmin() || cui.SelectedSelf()

	user := cui.SelectedUser()
	path := cui.GetPath()
	cui.AddCrumb(path[0], "Profile", user.GetFullName()+" ("+user.GetUserName()+")")
	cui.SetPath(path[1:])

	if cui.IsPOST() {
		cmd := "user set"
		arg := []string{user.GetUserName()}

		msg, err = cui.HandleForm(cmd, arg, user)
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
		Message string
		Error   string
		User    pf.PfUser
		IsEdit  bool
	}

	p := Page{cui.Page_def(), msg, errmsg, user, isedit}
	cui.Page_show("user/profile.tmpl", p)
}

// h_user_log shows the user specific part of the system log
func h_user_log(cui PfUI) {
	user := cui.SelectedUser()
	h_system_logA(cui, user.GetUserName(), "")
}

// h_user_list lists all the users in the system
func h_user_list(cui PfUI) {
	if !cui.IsSysAdmin() {
		/* Non-SysAdmin can only see their own page */
		cui.SetRedirect("/user/"+cui.TheUser().GetUserName()+"/", StatusSeeOther)
		return
	}

	total := 0
	offset := 0

	offset_v, err := cui.FormValue("offset")
	if err == nil && offset_v != "" {
		offset, _ = strconv.Atoi(offset_v)
	}

	search, err := cui.FormValue("search")
	if err != nil {
		search = ""
	}

	user := cui.NewUser()
	total, _ = user.GetListMax(search)
	users, err := user.GetList(cui, search, offset, pf.PAGER_PERPAGE, false)

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

// h_user_image returns the user's image if the logged-in user is allowed to see it.
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

// h_user is the main entry point for /user/ sub URLs
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
		{"details", "Details", PERM_USER_SELF | PERM_USER_VIEW, h_user_profile_details, nil},
		{"languages", "Languages", PERM_USER_SELF | PERM_USER_VIEW, h_user_profile_languages, nil},
		{"username", "Username", PERM_USER_SELF, h_user_username, nil},
		{"password", "Password", PERM_USER_SELF, h_user_password, nil},
		{"2fa", "2FA Tokens", PERM_USER_SELF, h_user_2fa, nil},
		{"email", "Email", PERM_USER_SELF, h_user_email, nil},
		{"pgp_keys", "Download All PGP Keys", PERM_USER_SELF, h_user_pgp_keys, nil},
		{"image.png", "", PERM_USER_VIEW, h_user_image, nil},
		{"log", "Audit Log", PERM_USER_SELF, h_user_log, nil},
		{"pwreset", "Password Reset", PERM_GROUP_ADMIN, H_user_pwreset, nil},
	})

	cui.UIMenu(menu)
}
