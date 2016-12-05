package pitchforkui

import (
	"strconv"
	"trident.li/pitchfork/lib"
)

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
		Button string `label:"Create" hint:"Creates the Mailing List" pftype:"submit"`
	}

	type Page struct {
		*PfPage
		Opt     popt
		Message string
		Error   string
	}

	opt := popt{grp.GetGroupName(), "", ""}
	p := Page{cui.Page_def(), opt, msg, errmsg}
	cui.Page_show("ml/new.tmpl", p)
}

func h_ml_pgp(cui PfUI) {
	grp := cui.SelectedGroup()
	ml := cui.SelectedML()

	key, err := ml.GetKey(cui)

	if err != nil {
		H_error(cui, StatusNotFound)
		return
	}

	fname := grp.GetGroupName() + "-" + ml.ListName + ".asc"

	cui.SetContentType("application/pgp-keys")
	cui.SetFileName(fname)
	cui.SetExpires(60)
	cui.SetRaw(key)
}

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
		Opt     pitchfork.PfML
		Message string
		Error   string
	}

	p := Page{cui.Page_def(), ml, msg, errmsg}
	cui.Page_show("ml/settings.tmpl", p)
}

func h_ml_list(cui PfUI) {
	var ml pitchfork.PfML
	var mls []pitchfork.PfML
	var err error
	var username string
	var admin bool
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

	if cui.IsSysAdmin() {
		admin = true
	} else {
		if cui.IAmGroupAdmin() {
			admin = true
		} else {
			admin = false
		}
	}

	/* Output the page */
	type Page struct {
		*PfPage
		Username  string
		GroupName string
		MLs       []pitchfork.PfML
		Admin     bool
	}

	menu := NewPfUIMenu([]PfUIMentry{
		{"new/", "New Mailing List", PERM_GROUP_ADMIN, h_ml_new, nil},
	})

	cui.SetPageMenu(&menu)

	p := Page{cui.Page_def(), username, grp.GetGroupName(), mls, admin}
	cui.Page_show(template, p)
}

func h_ml_members(cui PfUI) {
	var err error
	var members []pitchfork.PfMLUser
	var ml pitchfork.PfML

	sel_grp := cui.SelectedGroup()
	sel_ml := cui.SelectedML()

	total := 0
	offset := 0

	offset_v := cui.GetArg("offset")
	if offset_v != "" {
		offset, _ = strconv.Atoi(offset_v)
	}

	search := cui.GetArg("search")

	ml.GroupName = sel_grp.GetGroupName()
	ml.ListName = sel_ml.ListName

	total, err = ml.GetMembersMax(search)
	if err != nil {
		cui.Err(err.Error())
		return
	}

	members, err = ml.GetMembers(search, offset, 10)
	if err != nil {
		cui.Err(err.Error())
		return
	}

	/* Output the page */
	type Page struct {
		*PfPage
		GroupName   string
		MLName      string
		Members     []pitchfork.PfMLUser
		PagerOffset int
		PagerTotal  int
		Search      string
	}

	p := Page{cui.Page_def(), sel_grp.GetGroupName(), sel_ml.ListName, members, offset, total, ""}
	cui.Page_show("ml/members.tmpl", p)
}

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

func h_ml_subscribe(cui PfUI) {
	var username string
	grp := cui.SelectedGroup()
	ml := cui.SelectedML()

	submitted_uid, err := cui.FormValue("username")
	if err != nil {
		return
	}

	if submitted_uid != "" {
		username = submitted_uid
	} else {
		if cui.HasSelectedUser() {
			user := cui.SelectedUser()
			username = user.GetUserName()
		}
	}

	if username == "" {
		return
	}

	if !ml_canadd(cui, username, "subscribe to") {
		return
	}

	cmd := "ml member add"
	arg := []string{grp.GetGroupName(), ml.ListName, username}
	msg, err := cui.HandleCmd(cmd, arg)

	var errmsg = ""

	if err != nil {
		/* Failed */
		errmsg = err.Error()
	} else {
		/* Success */
	}

	/* Output the page */
	type popt struct {
		GroupName string `label:"Group Name" pfset:"nobody" pfget:"user" hint:"The group name"`
		ML        string `label:"List Name" pfset:"nobody" pfget:"user" hint:"The Mailing List Name"`
		Username  string `label:"User Name" hint:"The User Name" pfreq:"yes"`
		Button    string `label:"Subscribe" hint:"Subscribe to the list" pftype:"submit"`
	}

	type Page struct {
		*PfPage
		Opt     popt
		Message string
		Error   string
	}

	opt := popt{grp.GetGroupName(), ml.ListName, "", ""}
	p := Page{cui.Page_def(), opt, msg, errmsg}
	cui.Page_show("ml/subscribe.tmpl", p)
}

func h_ml_unsubscribe(cui PfUI) {
	var username string
	grp := cui.SelectedGroup()
	ml := cui.SelectedML()

	submitted_uid, err := cui.FormValue("username")
	if err == nil {
		return
	}

	if submitted_uid != "" {
		username = submitted_uid
	} else {
		if cui.HasSelectedUser() {
			user := cui.SelectedUser()
			username = user.GetUserName()
		}
	}

	if username == "" {
		return
	}

	if !ml_canadd(cui, username, "unsubscribe from") {
		return
	}

	cmd := "ml member remove"
	arg := []string{grp.GetGroupName(), ml.ListName, username}
	msg, err := cui.HandleCmd(cmd, arg)

	var errmsg = ""

	if err != nil {
		/* Failed */
		errmsg = err.Error()
	} else {
		/* Success */
	}

	/* Output the page */
	type popt struct {
		GroupName string `label:"Group Name" pfset:"nobody" pfget:"user" hint:"The name of the group"`
		ML        string `label:"List Name" pfset:"nobody" pfget:"user" hint:"The Mailing List Name"`
		Username  string `label:"User Name" hint:"The User Name" pfreq:"yes"`
		Button    string `label:"Unsubscribe" hint:"Subscribe to the list" pftype:"submit"`
	}

	type Page struct {
		*PfPage
		Opt     popt
		Message string
		Error   string
	}

	opt := popt{grp.GetGroupName(), ml.ListName, "", ""}
	p := Page{cui.Page_def(), opt, msg, errmsg}
	cui.Page_show("ml/unsubscribe.tmpl", p)
}

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
