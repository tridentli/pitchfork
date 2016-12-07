package pitchforkui

type login struct {
	Username  string `label:"username" hint:"your_username" min:"3" pfreq:"yes" placeholder:"example_username"`
	Password  string `label:"password" hint:"your_password" min:"6" pfreq:"yes" pftype:"password" placeholder:"example_password"`
	TwoFactor string `label:"2fa" hint:"hint_2fa" placeholder:"example_2fa"`
	Comeback  string `label:"login_comeback" pftype:"hidden"`
	Required  string `label:"login_required" pftype:"note" pfreq:"yes" htmlclass:"required"`
	Cookies   string `label:"login_cookies" pftype:"note"`
	Button    string `label:"sign_in" pftype:"submit"`
	Message   string `label:"message" pfomitempty:"yes" pftype:"note"`
	Error     string `label:"error" htmlclass:"error" pfomitempty:"yes" pftype:"note"`
}

type PfLoginPage struct {
	*PfPage
	Login login
}

func h_login(cui PfUI) {
	cui.SetStatus(StatusUnauthorized)

	cmd := "system login"
	arg := []string{"", "", ""}

	msg, err := cui.HandleCmd(cmd, arg)

	if cui.IsLoggedIn() {
		comeback, _ := cui.FormValue("comeback")
		if comeback == "" || comeback == "Comeback" {
			comeback = "/user/" + cui.TheUser().GetUserName() + "/"
		}

		cui.SetRedirect(comeback, StatusSeeOther)
		return
	}

	h_loginui(cui, msg, err)
}

func h_relogin(cui PfUI, msg string) {
	h_loginui(cui, msg, nil)
}

func h_loginui(cui PfUI, msg string, err error) {
	var errmsg = ""

	if err != nil {
		/* Failed */
		errmsg = err.Error()
	} else {
		/* Success */
	}

	/* Output the page */
	r := "Denotes a required field"
	c := "Note: Web cookies are required beyond this point"

	/* Check Comeback to make sure there are no loops */
	comeback := cui.GetFullURL()
	lcomeback := len(comeback)

	u_login := "/login/"
	u_logout := "/logout/"
	ul_login := len(u_login)
	ul_logout := len(u_logout)

	if comeback == "/" ||
		(lcomeback >= ul_login && comeback[:ul_login] == u_login) ||
		(lcomeback >= ul_logout && comeback[:ul_logout] == u_logout) {
		comeback = ""
	}

	l := login{Required: r, Comeback: comeback, Cookies: c, Message: msg, Error: errmsg}
	p := PfLoginPage{cui.Page_def(), l}

	var pp interface{}

	if cui.(*PfUIS).f_uiloginoverride != nil {
		pp, err = cui.(*PfUIS).f_uiloginoverride(cui, &p)

		if err != nil {
			cui.Errf("Overriden Login failed: %s", err.Error())
			H_error(cui, StatusInternalServerError)
		}
	} else {
		pp = p
	}

	cui.Page_show("misc/login.tmpl", pp)
}
