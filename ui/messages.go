package pitchforkui

import (
	pf "trident.li/pitchfork/lib"
)

type newmsg struct {
	Title     string `label:"Subject" pfreq:"yes" hint:"Subject describing the message"`
	Plaintext string `label:"Message" pfreq:"yes" pftype:"text" hint:"The message that you want to post, markdown is supported"`
	Button    string `label:"Post" pftype:"submit"`
}

func h_msg_pathdepth(cui PfUI, path string) int {
	return pf.Msg_PathDepth(cui, path) - pf.Msg_ModPathDepth(cui)
}

func h_msg_fixpaths(cui PfUI, msgs *[]pf.PfMessage, path string) {
	pathdepth := h_msg_pathdepth(cui, path)
	pfx := ""

	for i := 0; i < pathdepth; i++ {
		pfx += "../"
	}

	for m, msg := range *msgs {
		(*msgs)[m].Path = pfx + msg.Path[1:]
	}
}

func h_msg_show(cui PfUI, path string) {
	errmsg := ""

	msgs, err := pf.Msg_GetThread(cui, path, 1, 1, 0, 0)
	if err != nil {
		cui.Errf("Message(%s): %s", path, err.Error())
		H_NoAccess(cui)
		return
	}

	h_msg_fixpaths(cui, &msgs, path)

	/* Output the page */
	type Page struct {
		*PfPage
		Msgs      []pf.PfMessage
		Error     string
		AllowPost bool
		Opt       newmsg
	}

	allowpost := true

	t := pf.Msg_PathType(cui, path)

	tmpl := ""

	switch t {
	case pf.MSGTYPE_SECTION:
		tmpl = "list_section.tmpl"
		if !cui.IsSysAdmin() {
			allowpost = false
		}

		/* Disable HTML body when the same as the Title */
		for m, msg := range msgs {
			if msg.Plaintext == msg.Title {
				msgs[m].HTML = ""
			}
		}
		break

	case pf.MSGTYPE_THREAD:
		tmpl = "list_thread.tmpl"
		break

	case pf.MSGTYPE_MESSAGE:
		tmpl = "show.tmpl"

		/* Mark messages as seen when we show them */
		for _, msg := range msgs {
			if !msg.Seen.Valid {
				pf.Msg_MarkSeen(cui, msg)
			}
		}
		break

	default:
		panic("Unknown Message Type")
	}

	p := Page{cui.Page_def(), msgs, errmsg, allowpost, newmsg{"", "", ""}}
	cui.Page_show("messages/"+tmpl, p)
}

func msg_post_form(cui PfUI, path string) (err error) {
	mopts := pf.Msg_GetModOpts(cui)
	cmd := mopts.Cmdpfx + " post"
	arg := []string{path, "", ""}

	_, err = cui.HandleCmd(cmd, arg)
	return
}

func H_msg(cui PfUI) {
	path := pf.Msg_sep + cui.GetPathString()

	pp := "/"

	for _, p := range cui.GetPath() {
		if p == "" || p[0] == '?' {
			break
		}

		pp += p + "/"

		msg, err := pf.Msg_Get(cui, pp)
		if err != nil {
			cui.Errf("Message(%s): %s", path, err.Error())
			H_NoAccess(cui)
			return
		}

		cui.AddCrumb(p, msg.Title, msg.Title)
	}

	if cui.IsPOST() {
		err := msg_post_form(cui, path)
		if err == nil {
			cui.SetRedirect("#bottom", StatusSeeOther)
			return
		}
	}

	h_msg_show(cui, path)

}
