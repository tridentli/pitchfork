package pitchforkui

import (
	"strconv"
	pf "trident.li/pitchfork/lib"
)

func h_group_add(cui PfUI) {
	cmd := "group add"
	arg := []string{""}

	msg, err := cui.HandleCmd(cmd, arg)

	var errmsg = ""

	if err != nil {
		/* Failed */
		errmsg = err.Error()
	} else {
		group_name, _ := cui.FormValue("group")
		if group_name != "" {
			/* Success */
			cui.SetRedirect("/group/"+group_name+"/settings/", StatusSeeOther)
			return
		}
	}

	/* Output the page */
	type grpnew struct {
		Group  string `label:"Group Name" pfreq:"yes" hint:"The name of the group"`
		Button string `label:"Create" pftype:"submit"`
	}

	type Page struct {
		*PfPage
		Group   grpnew
		Message string
		Error   string
	}

	var grp grpnew
	p := Page{cui.Page_def(), grp, msg, errmsg}
	cui.Page_show("group/new.tmpl", p)
}

func h_group_settings(cui PfUI) {
	grp := cui.SelectedGroup()

	cmd := "group set"
	arg := []string{grp.GetGroupName()}

	msg, err := cui.HandleForm(cmd, arg, grp)

	var errmsg = ""

	if err != nil {
		/* Failed */
		errmsg = err.Error()
	} else {
		/* Success */
	}

	err = grp.Refresh()
	if err != nil {
		errmsg += err.Error()
	}

	/* Output the page */
	type Page struct {
		*PfPage
		Tg      pf.PfGroup
		Message string
		Error   string
	}

	p := Page{cui.Page_def(), grp, msg, errmsg}
	cui.Page_show("group/settings.tmpl", p)
}

func h_group_log(cui PfUI) {
	grp := cui.SelectedGroup()
	h_system_logA(cui, "", grp.GetGroupName())
}

func h_group_members(cui PfUI) {
	path := cui.GetPath()

	if len(path) != 0 && path[0] != "" {
		H_group_member_profile(cui)
		return
	}

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

	grp := cui.SelectedGroup()

	total, err = grp.GetMembersTot(search)
	if err != nil {
		cui.Err("error: " + err.Error())
		return
	}

	members, err := grp.GetMembers(search, cui.TheUser().GetUserName(), offset, 10, false, false)
	if err != nil {
		cui.Err(err.Error())
		return
	}

	/* Output the page */
	type Page struct {
		*PfPage
		Group       pf.PfGroup
		Users       []pf.PfGroupMember
		PagerOffset int
		PagerTotal  int
		Search      string
		IsAdmin     bool
	}

	isadmin := cui.IAmGroupAdmin()

	p := Page{cui.Page_def(), grp, members, offset, total, search, isadmin}
	cui.Page_show("group/members.tmpl", p)
}

func h_group_languages(cui PfUI) {
	H_error(cui, StatusNotImplemented)
}

func group_member_cmd(cui PfUI, cmd string) {
	grp := cui.SelectedGroup()
	user := cui.SelectedUser()

	arg := []string{grp.GetGroupName(), user.GetUserName()}

	_, err := cui.HandleCmd(cmd, arg)
	if err != nil {
		cui.Err(err.Error())
		return
	}

	cui.SetRedirect("/group/"+grp.GetGroupName()+"/members/", StatusSeeOther)
	return
}

func h_group_member_approve(cui PfUI) {
	group_member_cmd(cui, "group member approve")
}

func h_group_member_unblock(cui PfUI) {
	group_member_cmd(cui, "group member unblock")
}

func h_group_member_block(cui PfUI) {
	group_member_cmd(cui, "group member block")
}

func h_group_member_promote(cui PfUI) {
	group_member_cmd(cui, "group member promote")
}

func h_group_member_demote(cui PfUI) {
	group_member_cmd(cui, "group member demote")
}

func h_group_index(cui PfUI) {

	/* Output the page */
	type Page struct {
		*PfPage
		GroupName string
		GroupDesc string
	}

	grp := cui.SelectedGroup()

	p := Page{cui.Page_def(), grp.GetGroupName(), grp.GetGroupDesc()}
	cui.Page_show("group/index.tmpl", p)
}

func h_group_list(cui PfUI) {
	grp := cui.NewGroup()
	var groups []pf.PfGroupUser
	var err error

	if !cui.IsSysAdmin() {
		groups, err = grp.GetGroups(cui.TheUser().GetUserName(), true)
	} else {
		groups, err = grp.GetGroupsAll()
	}

	if err != nil {
		return
	}

	grps := make(map[string]string)
	for i := range groups {
		grps[groups[i].GroupName] = groups[i].GroupDesc
	}

	/* Output the page */
	type Page struct {
		*PfPage
		Groups map[string]string
	}

	menu := NewPfUIMenu([]PfUIMentry{
		{"add", "Add Group", PERM_GROUP_ADMIN, h_group_add, nil},
	})

	cui.SetPageMenu(&menu)

	p := Page{cui.Page_def(), grps}
	cui.Page_show("group/list.tmpl", p)
}

func H_group_member_profile(cui PfUI) {
	path := cui.GetPath()

	/* Select the user */
	err := cui.SelectUser(path[0], PERM_USER_VIEW)
	if err != nil {
		cui.Err("User: " + err.Error())
		H_NoAccess(cui)
		return
	}

	h_user(cui)
	return
}

func h_group_pgp_keys(cui PfUI) {
	grp := cui.SelectedGroup()

	keys, err := grp.GetKeys(cui)
	if err != nil {
		/* Temp redirect to unknown */
		H_error(cui, StatusNotFound)
		return
	}

	fname := grp.GetGroupName() + ".asc"

	cui.SetContentType("application/pgp-keys")
	cui.SetFileName(fname)
	cui.SetExpires(60)
	cui.SetRaw(keys)
}

func h_group_contacts_vcard(cui PfUI) {
	grp := cui.SelectedGroup()

	vcard, err := grp.GetVcards()
	if err != nil {
		H_errmsg(cui, err)
		return
	}

	fname := grp.GetGroupName() + ".vcf"

	cui.SetContentType("text/vcard")
	cui.SetFileName(fname)
	cui.SetExpires(60)
	cui.SetRaw([]byte(vcard))
	return
}

func h_group_contacts(cui PfUI) {
	fmt := cui.GetArg("format")

	if fmt == "vcard" {
		h_group_contacts_vcard(cui)
		return
	}

	grp := cui.SelectedGroup()

	members, err := grp.GetMembers("", "", 0, 0, false, false)
	if err != nil {
		H_errmsg(cui, err)
		return
	}

	/* Output the page */
	type Page struct {
		*PfPage
		Members []pf.PfGroupMember
	}

	p := Page{cui.Page_def(), members}
	cui.Page_show("group/contacts.tmpl", p)
}

func h_group_file(cui PfUI) {
	/* Module options */
	pf.Group_FileMod(cui)

	/* Call the module */
	H_file(cui)
}

func h_group_wiki(cui PfUI) {
	/* Module options */
	pf.Group_WikiMod(cui)

	/* Call the module */
	H_wiki(cui)
}

func h_group(cui PfUI) {
	path := cui.GetPath()

	if len(path) == 0 || path[0] == "" {
		cui.SetPageMenu(nil)
		h_group_list(cui)
		return
	}

	/* New group creation */
	if path[0] == "add" && cui.IsSysAdmin() {
		cui.AddCrumb(path[0], "Add Group", "Add Group")
		cui.SetPageMenu(nil)
		h_group_add(cui)
		return
	}

	/* Check member access to group */
	err := cui.SelectGroup(cui.GetPath()[0], PERM_GROUP_MEMBER)
	if err != nil {
		cui.Err("Group: " + err.Error())
		H_NoAccess(cui)
		return
	}

	grp := cui.SelectedGroup()

	cui.AddCrumb(path[0], grp.GetGroupName(), grp.GetGroupDesc())

	cui.SetPath(path[1:])

	/* /group/<grp>/{path} */
	menu := NewPfUIMenu([]PfUIMentry{
		{"", "", PERM_GROUP_MEMBER, h_group_index, nil},
		{"settings", "Settings", PERM_GROUP_ADMIN, h_group_settings, nil},
		{"members", "Members", PERM_GROUP_MEMBER, h_group_members, nil},
		{"pgp_keys", "PGP Keys", PERM_GROUP_MEMBER, h_group_pgp_keys, nil},
		{"ml", "Mailing List", PERM_GROUP_MEMBER, h_ml, nil},
		{"wiki", "Wiki", PERM_GROUP_WIKI, h_group_wiki, nil},
		{"log", "Audit Log", PERM_GROUP_ADMIN, h_group_log, nil},
		{"file", "Files", PERM_GROUP_FILE, h_group_file, nil},
		{"contacts", "Contacts", PERM_GROUP_MEMBER, h_group_contacts, nil},

		/* Hidden options (actions) */
		{"approve", "Approve Member", PERM_GROUP_ADMIN | PERM_HIDDEN | PERM_NOCRUMB, h_group_member_approve, nil},
		{"unblock", "Unblock Member", PERM_GROUP_ADMIN | PERM_HIDDEN | PERM_NOCRUMB, h_group_member_unblock, nil},
		{"block", "Block Member", PERM_GROUP_ADMIN | PERM_HIDDEN | PERM_NOCRUMB, h_group_member_block, nil},
		{"demote", "Demote To Admin", PERM_GROUP_ADMIN | PERM_HIDDEN | PERM_NOCRUMB, h_group_member_demote, nil},
		{"promote", "Promote To Admin", PERM_GROUP_ADMIN | PERM_HIDDEN | PERM_NOCRUMB, h_group_member_promote, nil},
		// TODO: {"calendar", "Calendar", PERM_GROUP_CALENDAR, h_calendar},
	})

	cui.UIMenu(menu)
}
