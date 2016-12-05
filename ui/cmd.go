package pitchforkui

import (
	pf "trident.li/pitchfork/lib"
)

/* /api/<cmd>/<arg>/<arg...> */
func h_api(cui PfUI) {
	var err error

	/* Force bearer auth */
	if cui.GetToken() == "" {
		cui.SetBearerAuth(true)
	}

	/* Run the command */
	err = cui.Cmd(cui.GetPath())

	if err != nil {
		cui.OutLn("An error occured: %s", err.Error())
	}
}

/* Simple CLI interface */
func h_cli(cui PfUI) {
	var err error
	out := ""
	cmd := ""

	if cui.IsPOST() {
		cmd, err = cui.FormValue("cmd")
	}

	if cmd == "" {
		cmd = "help"
	}

	if err == nil {
		/* Parse the string (obeying quoting) */
		args := pf.SplitArgs(cmd)

		/* Run the command */
		out, err = cui.CmdOut("", args)

		/* Handle anything that causes a logout */
		if !cui.IsLoggedIn() {
			h_logout(cui)
			return
		}
	}

	if err != nil {
		out += "An error occured: "
		out += err.Error() + "\n"
	}

	/* Output the page */
	type popt struct {
		Cmd    string `label:"Command" hint:"The command to execute"`
		Button string `label:"Execute" pftype:"submit"`
	}

	type Page struct {
		*PfPage
		Output string
		Opt    popt
	}

	cui.SetPageMenu(nil)
	cui.AddCrumb("", "CLI", "Tickly (CLI)")

	opt := popt{cmd, ""}
	p := Page{cui.Page_def(), out, opt}
	cui.Page_show("misc/cli.tmpl", p)
}
