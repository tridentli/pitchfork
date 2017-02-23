package pitchforkui

import (
	pf "trident.li/pitchfork/lib"
)

// h_iptrk shows the user interface of the IP Tracking mechanism
func h_iptrk(cui PfUI) {
	var err error
	var msg string

	if cui.GetMethod() == "POST" {
		cmd := "system iptrk remove"
		arg := []string{""}

		_, err = cui.HandleCmd(cmd, arg)
	}

	ts, err2 := pf.IPtrk_List(cui)

	if err2 == pf.ErrNoRows {
		msg = "Currently there are no entries"
		err2 = nil
	}

	if err == nil && err2 != nil {
		err = err2
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
		Entries []pf.IPtrkEntry
		Message string
		Error   string
	}

	p := Page{cui.Page_def(), ts, msg, errmsg}
	cui.PageShow("system/iptrk.tmpl", p)
}
