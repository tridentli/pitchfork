package pitchforkui

import (
	"strconv"
	pf "trident.li/pitchfork/lib"
)

// h_system_settings renders the system settings menu and allows for changes
func h_system_settings(cui PfUI) {
	cmd := "system set"
	arg := []string{}

	sys := pf.System_Get()

	msg, err := cui.HandleForm(cmd, arg, sys)

	if msg != "" {
		/* Refresh */
		sys.Refresh()
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
		Message string
		Error   string
		System  pf.PfSys
	}

	p := Page{cui.Page_def(), msg, errmsg, *sys}
	cui.PageShow("system/settings.tmpl", p)
}

// h_system_logA renders the system log details, optionally filtering on user_name or group_name
func h_system_logA(cui PfUI, user_name string, tg_name string) {
	var err error

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

	var audits []pf.PfAudit
	total, _ = pf.System_AuditMax(search, user_name, tg_name)
	audits, err = pf.System_AuditList(search, user_name, tg_name, offset, 10)
	if err != nil {
		cui.Err(err.Error())
		return
	}

	/* Output the page */
	type Page struct {
		*PfPage
		Audits      []pf.PfAudit
		PagerOffset int
		PagerTotal  int
		Search      string
	}

	p := Page{cui.Page_def(), audits, offset, total, search}
	cui.PageShow("system/log.tmpl", p)
}

// h_system_log renders the log for the full system
func h_system_log(cui PfUI) {
	h_system_logA(cui, "", "")
}

// h_system_report renders a report about the system
func h_system_report(cui PfUI) {
	/* Output the page */
	type Page struct {
		*PfPage
		Message string
	}

	cmd := "system report"
	arg := []string{}
	msg, err := cui.CmdOut(cmd, arg)
	if err != nil {
		msg = err.Error() + "\n" + msg
	}

	p := Page{cui.Page_def(), msg}
	cui.PageShow("system/report.tmpl", p)
}

// h_system_index shows the index page for the system options
func h_system_index(cui PfUI) {
	/* Output the page */
	p := cui.Page_def()
	cui.PageShow("system/index.tmpl", p)
}

// h_system handles system options
func h_system(cui PfUI) {
	menu := NewPfUIMenu([]PfUIMentry{
		{"", "", PERM_USER, h_system_index, nil},
		{"log", "Audit Log", PERM_SYS_ADMIN, h_system_log, nil},
		{"report", "Report", PERM_SYS_ADMIN, h_system_report, nil},
		{"settings", "Settings", PERM_SYS_ADMIN, h_system_settings, nil},
		{"iptrk", "IPtrk", PERM_SYS_ADMIN, h_iptrk, nil},
	})

	cui.UIMenu(menu)
}
