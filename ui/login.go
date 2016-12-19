package pitchforkui

type login struct {
	Username  string `label:"Username" hint:"Your username" min:"3" pfreq:"yes" placeholder:"user@example.com"`
	Password  string `label:"Password" hint:"Your password" min:"6" pfreq:"yes" pftype:"password" placeholder:"4.very/difficult_p4ssw0rd"`
	TwoFactor string `label:"Two Factor Code" hint:"Two Factor Token (if configured)" placeholder:"314159"`
	Comeback  string `label:"Comeback" pftype:"hidden"`
	Required  string `label:"Required" pftype:"note" pfreq:"yes" htmlclass:"required"`
	Cookies   string `label:"Cookies" pftype:"note"`
	Button    string `label:"Sign In" pftype:"submit"`
	Message   string `label:"Message" pfomitempty:"yes" pftype:"note"`
	Error     string `label:"Error" htmlclass:"error" pfomitempty:"yes" pftype:"note"`
}

type PfLoginPage struct {
	*PfPage
	Login login
}

func h_login(cui PfUI) {
	var has_u2f bool
	var has_duo bool
	var errmsg = ""
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
	} else if cui.IsPreLoggedIn() {
		comeback, _ := cui.FormValue("comeback")

		/* Load 2FA options */
		user := cui.SelectedUser()
		tokens, err := user.Fetch2FA()
		for _, token := range tokens {
			switch token.Type {
			case "U2F":
				has_u2f = true
				break
			case "DUO":
				has_duo = true
				break
			default:
				break
			}
		}

		/* Generate 2FA Page2 */
		/* Output the page */
		type Page struct {
			*PfPage
			U2F      bool
			DUO      bool
			Comeback string
			Message  string
			Error    string
		}

		p := Page{cui.Page_def(), has_u2f, has_duo, comeback, msg, errmsg}
		cui.Page_show("misc/login2.tmpl", p)
		return
	}

	h_loginui(cui, msg, err)
}

func h_login2(cui PfUI) {
	return
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
