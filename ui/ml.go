package pitchforkui

import (
	"strconv"

	pf "trident.li/pitchfork/lib"
	pfpgp "trident.li/pitchfork/lib/pgp"
)

// h_ml_new allows creation of a new Mailinglist
func h_ml_new(cui PfUI) {
	grp := cui.SelectedGroup()

	cmd := "ml new"
	arg := []string{grp.GetGroupName(), ""}
	msg, err := cui.HandleCmd(cmd, arg)

	var errmsg = ""

	if err != nil {
		/* Failed */
		errmsg = err.Error()
	} else {
		/* Success */
		if msg != "" {
			if !cui.HasSelectedML() {
				ml_name, e := cui.FormValue("ml")
				if e == nil {
					cui.SelectML(ml_name, PERM_GROUP_ADMIN)
				}
			}

			if cui.HasSelectedML() {
				ml := cui.SelectedML()
				cui.SetRedirect("/group/"+grp.GetGroupName()+"/ml/"+ml.ListName+"/settings/", StatusSeeOther)
			}
			return
		}
	}

	/* Output the page */
	type popt struct {
		Group  string `label:"Group Name" pfset:"nobody" pfget:"user" hint:"The Group Name"`
		ML     string `label:"List Name" hint:"The Mailing List Name" pfreq:"yes"`
		Action string `label:"Action" pftype:"hidden"`
		Button string `label:"Create" hint:"Creates the Mailing List" pftype:"submit"`
	}

	type Page struct {
		*PfPage
		Opt     popt
		Message string
		Error   string
	}

	opt := popt{grp.GetGroupName(), "", "create", ""}
	p := Page{cui.Page_def(), opt, msg, errmsg}
	cui.PageShow("ml/new.tmpl", p)
}

// h_ml_pgp returns the PGP key of a group
func h_ml_pgp(cui PfUI) {
	keyset := pfpgp.NewIndexedKeySet()
	grp := cui.SelectedGroup()
	ml := cui.SelectedML()

	err := ml.GetKeys(cui, keyset)

	if err != nil {
		H_error(cui, StatusNotFound)
		return
	}

	output := keyset.ToBytes()
	fname := grp.GetGroupName() + "-" + ml.ListName + ".asc"

	cui.SetContentType("application/pgp-keys")
	cui.SetFileName(fname)
	cui.SetExpires(60)
	cui.SetRaw(output)
}

// h_ml_settings allows changing mailinglist settings
func h_ml_settings(cui PfUI) {
	grp := cui.SelectedGroup()
	ml := cui.SelectedML()

	cmd := "ml set"
	arg := []string{grp.GetGroupName(), ml.ListName}

	msg, err := cui.HandleForm(cmd, arg, ml)

	var errmsg = ""

	if err != nil {
		/* Failed */
		errmsg = err.Error()
	} else {
		/* Success */
	}

	/* Refresh the elements */
	err = ml.Refresh()
	if err != nil {
		errmsg += err.Error()
	}

	/* Output the page */
	type Page struct {
		*PfPage
		Opt     pf.PfML
		Message string
		Error   string
	}

	p := Page{cui.Page_def(), ml, msg, errmsg}
	cui.PageShow("ml/settings.tmpl", p)
}

// h_ml_list lists the mailinglist for a user
func h_ml_list(cui PfUI) {
	var ml pf.PfML
	var mls []pf.PfML
	var err error
	var username string
	username = ""
	template := "ml/list.tmpl"

	grp := cui.SelectedGroup()

	if cui.HasSelectedUser() {
		user := cui.SelectedUser()
		username = user.GetUserName()
		mls, err = ml.ListWithUser(cui, grp, user)
		if err != nil {
			cui.Err(err.Error())
			H_error(cui, StatusUnauthorized)
			return
		}

		template = "ml/list_with_user.tmpl"
	} else {

		mls, err = ml.List(cui, grp)
		if err != nil {
			cui.Err(err.Error())
			H_error(cui, StatusUnauthorized)
			return
		}
	}

	admin := false
	if cui.IsSysAdmin() || cui.IAmGroupAdmin() {
		admin = true
	}

	/* Output the page */
	type Page struct {
		*PfPage
		Username  string
		GroupName string
		MLs       []pf.PfML
		Admin     bool
	}

	menu := NewPfUIMenu([]PfUIMentry{
		{"new/", "New Mailing List", PERM_GROUP_ADMIN, h_ml_new, nil},
	})

	cui.SetPageMenu(&menu)

	p := Page{cui.Page_def(), username, grp.GetGroupName(), mls, admin}
	cui.PageShow(template, p)
}

// h_ml_members lists the members of a mailinglist
func h_ml_members(cui PfUI) {
	var ml pf.PfML

	sel_grp := cui.SelectedGroup()
	sel_ml := cui.SelectedML()

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

	ml.GroupName = sel_grp.GetGroupName()
	ml.ListName = sel_ml.ListName

	total, err = ml.ListGroupMembersMax(search)
	if err != nil {
		cui.Err(err.Error())
		return
	}

	members, err := ml.ListGroupMembers(search, offset, 10)
	if err != nil {
		cui.Err(err.Error())
		return
	}

	/* Output the page */
	type Page struct {
		*PfPage
		GroupName   string
		GroupAdmin  bool
		ML          pf.PfML
		Members     []pf.PfMLUser
		PagerOffset int
		PagerTotal  int
		Search      string
		Admin       bool
	}

	admin := false
	if cui.IsSysAdmin() || cui.IAmGroupAdmin() {
		admin = true
	}

	p := Page{cui.Page_def(), sel_grp.GetGroupName(), admin, sel_ml, members, offset, total, search, admin}
	cui.PageShow("ml/members.tmpl", p)
}

// ml_canadd determines if a user can add a username to a mailinglist
func ml_canadd(cui PfUI, username string, what string) bool {
	ml := cui.SelectedML()

	if ml.Can_add_self {
		return true
	}

	if cui.IsSysAdmin() {
		return true
	}

	if cui.IAmGroupAdmin() {
		return true
	}

	cui.Err("ML: " + username + " Attempt to " + what +
		"restricted ML " + ml.ListName + "-" + ml.GroupName)
	H_error(cui, StatusUnauthorized)

	return false
}

// h_ml_subscribe handles subscribing to a mailinglist
func h_ml_subscribe(cui PfUI) {
	var username string
	var errmsg string
	var msg string
	var err error

	grp := cui.SelectedGroup()
	ml := cui.SelectedML()

	if cui.IsPOST() {
		username, err = cui.FormValue("username")
		if err != nil {
			username = ""
		}

		if username != "" {
			if !ml_canadd(cui, username, "subscribe to") {
				errmsg += "Cannot add users to mailinglist"
			} else {
				cmd := "ml member add"
				arg := []string{grp.GetGroupName(), ml.ListName, username}
				msg, err = cui.HandleCmd(cmd, arg)
			}
		}
	}

	if err != nil {
		/* Failed */
		errmsg += err.Error()
	} else {
		/* Success */
		cui.SetRedirect("/group/"+grp.GetGroupName()+"/ml/", StatusSeeOther)
		return
	}

	/* Output the page */
	type popt struct {
		GroupName string `label:"Group Name" pfset:"nobody" pfget:"user" hint:"The group name"`
		ML        string `label:"List Name" pfset:"nobody" pfget:"user" hint:"The Mailing List Name"`
		Username  string `label:"User Name" hint:"The User Name" pfreq:"yes"`
		Action    string `label:"Action" pftype:"hidden"`
		Button    string `label:"Subscribe" hint:"Subscribe to the list" pftype:"submit"`
	}

	type Page struct {
		*PfPage
		Opt     popt
		Message string
		Error   string
	}

	opt := popt{grp.GetGroupName(), ml.ListName, "", "subscribe", ""}
	p := Page{cui.Page_def(), opt, msg, errmsg}
	cui.PageShow("ml/subscribe.tmpl", p)
}

// h_ml_unsubscribe handles unsubscribing from a mailinglist
func h_ml_unsubscribe(cui PfUI) {
	var username string
	var errmsg string
	var msg string
	var err error

	grp := cui.SelectedGroup()
	ml := cui.SelectedML()

	if cui.IsPOST() {

		username, err = cui.FormValue("username")
		if err != nil {
			username = ""
		}

		if username != "" {
			if !ml_canadd(cui, username, "unsubscribe from") {
				errmsg += "Cannot add users to mailinglist"
			} else {
				cmd := "ml member remove"
				arg := []string{grp.GetGroupName(), ml.ListName, username}
				msg, err = cui.HandleCmd(cmd, arg)
			}
		}
	}

	if err != nil {
		/* Failed */
		errmsg += err.Error()
	} else {
		/* Success */
		cui.SetRedirect("/group/"+grp.GetGroupName()+"/ml/", StatusSeeOther)
		return
	}

	/* Output the page */
	type popt struct {
		GroupName string `label:"Group Name" pfset:"nobody" pfget:"user" hint:"The name of the group"`
		ML        string `label:"List Name" pfset:"nobody" pfget:"user" hint:"The Mailing List Name"`
		Username  string `label:"User Name" hint:"The User Name" pfreq:"yes"`
		Action    string `label:"Action" pftype:"hidden"`
		Button    string `label:"Unsubscribe" hint:"Subscribe to the list" pftype:"submit"`
	}

	type Page struct {
		*PfPage
		Opt     popt
		Message string
		Error   string
	}

	opt := popt{grp.GetGroupName(), ml.ListName, "", "unsubscribe", ""}
	p := Page{cui.Page_def(), opt, msg, errmsg}
	cui.PageShow("ml/unsubscribe.tmpl", p)
}

// h_ml handles mailinglists
func h_ml(cui PfUI) {
	path := cui.GetPath()
	if len(path) == 0 || path[0] == "" {
		cui.SetPageMenu(nil)
		h_ml_list(cui)
		return
	}

	/* New ML creation */
	if path[0] == "new" {
		if cui.IsSysAdmin() || cui.IAmGroupAdmin() {
			cui.AddCrumb(path[0], "New", "Add Mailing List")
			cui.SetPageMenu(nil)
			h_ml_new(cui)
			return
		} else {
			cui.Err("ML: User not permitted to creat ML")
			H_error(cui, StatusUnauthorized)
			return
		}
	}

	/* Select the ml */
	err := cui.SelectML(path[0], PERM_GROUP_MEMBER)
	if err != nil {
		cui.Err("ML: " + err.Error())
		H_NoAccess(cui)
		return
	}

	ml := cui.SelectedML()

	cui.AddCrumb(path[0], ml.ListName, ml.Descr)

	cui.SetPath(path[1:])

	menu := NewPfUIMenu([]PfUIMentry{
		{"", "", PERM_GROUP_MEMBER, h_ml_members, nil},
		{"settings", "Settings", PERM_GROUP_ADMIN, h_ml_settings, nil},
		{"subscribe", "Subscribe", PERM_GROUP_MEMBER, h_ml_subscribe, nil},
		{"unsubscribe", "Unsubscribe", PERM_GROUP_MEMBER, h_ml_unsubscribe, nil},
		{"pgp", "PGP Key", PERM_GROUP_MEMBER, h_ml_pgp, nil},
	})

	cui.UIMenu(menu)
}
