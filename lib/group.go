package pitchfork

import (
	"errors"
)

const (
	GROUP_STATE_APPROVED = "approved"
	GROUP_STATE_BLOCKED  = "blocked"
)

type PfGroup interface {
	String() string
	GetGroupName() string
	GetGroupDesc() string
	HasWiki() bool
	HasFile() bool
	HasCalendar() bool
	fetch(group_name string, nook bool) (err error)
	Refresh() (err error)
	Exists(group_name string) (exists bool)
	Select(ctx PfCtx, group_name string, perms Perm) (err error)
	GetGroups(username string, active bool) (groups []PfGroupUser, err error)
	GetGroupsAll() (groups []PfGroupUser, err error)
	GetKeys(ctx PfCtx) (keyfile []byte, err error)
	IsMember(user string) (ismember bool, isadmin bool, out PfMemberState, err error)
	GetMembersTot(search string) (total int, err error)
	GetMembers(search string, username string, offset int, max int, nominated bool, exact bool) (members []PfGroupMember, err error)
	Add_default_mailinglists(ctx PfCtx) (err error)
	Member_add(ctx PfCtx) (err error)
	Member_remove(ctx PfCtx) (err error)
	Member_set_state(ctx PfCtx, state string) (err error)
	Member_set_admin(ctx PfCtx, isadmin bool) (err error)
}

type PfGroupS struct {
	GroupName    string `label:"Group Name" pfset:"nobody" pfget:"group_member" pfcol:"ident"`
	GroupDesc    string `label:"Description" pfcol:"descr" pfset:"group_admin"`
	PGP_Required bool   `label:"PGP Required" pfset:"group_admin"`
	Has_Wiki     bool   `label:"Wiki Module" pfset:"group_admin"`
	Has_File     bool   `label:"Files Module" pfset:"group_admin"`
	Has_Calendar bool   `label:"Calendar Module" pfset:"group_admin"`
	Button       string `label:"Update Group" pftype:"submit"`
}

type PfGroupUser struct {
	GroupName string
	GroupDesc string
	Email     string
	State     string
	Entered   string
	Admin     bool
}

type PfGroupMember interface {
	Set(groupname, username, fullname, affiliation string, groupadmin bool, groupstate, email, pgpkey_id, activity string)
	GetGroupName() string
	GetUserName() string
	GetFullName() string
	GetEmail() string
	GetAffiliation() string
	GetGroupAdmin() bool
	GetGroupState() string
	GetPGPKeyID() string
	HasPGP() bool
	GetActivity() string
}

type PfGroupMemberS struct {
	UserName    string
	FullName    string
	Email       string
	Affiliation string
	GroupAdmin  bool
	GroupState  string
	GroupName   string
	PGPKeyID    string
	Activity    string
}

type PfMemberState struct {
	ident     string
	can_login bool
	can_see   bool
	can_send  bool
	can_recv  bool
	blocked   bool
	hidden    bool
}

/* Should not be directly called, use ctx or cui.NewGroup() instead */
func NewPfGroup() PfGroup {
	return &PfGroupS{}
}

func NewPfGroupMember() PfGroupMember {
	return &PfGroupMemberS{}
}

func (grpm *PfGroupMemberS) Set(groupname, username, fullname, affiliation string, groupadmin bool, groupstate, email, pgpkey_id, activity string) {
	grpm.GroupName = groupname
	grpm.UserName = username
	grpm.FullName = fullname
	grpm.Affiliation = affiliation
	grpm.GroupAdmin = groupadmin
	grpm.GroupState = groupstate
	grpm.Email = email
	grpm.PGPKeyID = pgpkey_id
	grpm.Activity = activity
}

func (grpm *PfGroupMemberS) GetGroupName() string {
	return grpm.GroupName
}

func (grpm *PfGroupMemberS) GetUserName() string {
	return grpm.UserName
}

func (grpm *PfGroupMemberS) GetFullName() string {
	return grpm.FullName
}

func (grpm *PfGroupMemberS) GetEmail() string {
	return grpm.Email
}

func (grpm *PfGroupMemberS) GetAffiliation() string {
	return grpm.Affiliation
}

func (grpm *PfGroupMemberS) GetGroupAdmin() bool {
	return grpm.GroupAdmin
}

func (grpm *PfGroupMemberS) GetGroupState() string {
	return grpm.GroupState
}

func (grpm *PfGroupMemberS) GetPGPKeyID() string {
	return grpm.PGPKeyID
}

func (grpm *PfGroupMemberS) HasPGP() bool {
	return grpm.PGPKeyID != ""
}

func (grpm *PfGroupMemberS) GetActivity() string {
	return grpm.Activity
}

func (grp *PfGroupS) String() string {
	return grp.GroupName
}

func (grp *PfGroupS) GetGroupName() string {
	return grp.GroupName
}

func (grp *PfGroupS) GetGroupDesc() string {
	return grp.GroupDesc
}

func (grp *PfGroupS) HasWiki() bool {
	return grp.Has_Wiki
}

func (grp *PfGroupS) HasFile() bool {
	return grp.Has_File
}

func (grp *PfGroupS) HasCalendar() bool {
	return grp.Has_Calendar
}

func (grp *PfGroupS) fetch(group_name string, nook bool) (err error) {
	/* Make sure the name is mostly sane */
	group_name, err = Chk_ident("Group Name", group_name)
	if err != nil {
		return
	}

	p := []string{"ident"}
	v := []string{group_name}
	err = StructFetchA(grp, "trustgroup", "", p, v, "", true)
	if nook && err == ErrNoRows {
		/* No rows is sometimes okay */
	} else if err != nil {
		grp.GroupName = ""
		grp.GroupDesc = ""
		Log("Group:fetch() " + err.Error() + " '" + group_name + "'")
	}

	return
}

func (grp *PfGroupS) Refresh() (err error) {
	err = grp.fetch(grp.GroupName, false)
	return
}

func (grp *PfGroupS) Exists(group_name string) (exists bool) {
	err := grp.fetch(group_name, true)
	if err == ErrNoRows {
		return false
	}

	return true
}

func (grp *PfGroupS) Select(ctx PfCtx, group_name string, perms Perm) (err error) {
	err = grp.fetch(group_name, false)
	if err != nil {
		/* Failed to fetch */
		return
	}

	/* Check for proper permissions */
	ok, err := ctx.CheckPerms("SelectGroup", perms)
	if err != nil {
		return
	}

	if !ok {
		err = errors.New("Could not select user")
		return
	}

	return
}

/* Return the set of groups that username is connected to*/
/* If active is set nominations will also appear.*/
func (grp *PfGroupS) GetGroups(username string, active bool) (groups []PfGroupUser, err error) {
	var rows *Rows
	groups = nil

	q := "SELECT " +
		"mt.trustgroup, " +
		"grp.descr, " +
		"mt.state, " +
		"mt.email, " +
		"DATE_TRUNC('days', AGE(mt.entered)), " +
		"mt.admin " +
		"FROM member_trustgroup mt " +
		"JOIN trustgroup grp ON mt.trustgroup = grp.ident " +
		"JOIN member_state ms on mt.state = ms.ident " +
		"WHERE mt.member = $1 "
	if active {
		q += "AND ms.can_see = 't' "
	}
	q += "AND mt.state != 'blocked' " +
		"ORDER BY UPPER(grp.descr), mt.entered"
	rows, err = DB.Query(q, username)

	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		var grpuser PfGroupUser

		err = rows.Scan(&grpuser.GroupName, &grpuser.GroupDesc, &grpuser.State, &grpuser.Email, &grpuser.Entered, &grpuser.Admin)
		if err != nil {
			groups = nil
			return
		}

		groups = append(groups, grpuser)
	}

	return
}

func (grp *PfGroupS) GetGroupsAll() (groups []PfGroupUser, err error) {
	var rows *Rows
	groups = nil
	q := "SELECT grp.ident, grp.descr " +
		"FROM trustgroup grp " +
		"ORDER BY UPPER(grp.descr)"
	rows, err = DB.Query(q)

	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		var grp PfGroupUser

		err = rows.Scan(&grp.GroupName, &grp.GroupDesc)
		if err != nil {
			groups = nil
			return
		}

		groups = append(groups, grp)
	}

	return
}

func (grp *PfGroupS) GetKeys(ctx PfCtx) (keyfile []byte, err error) {
	var ml PfML
	mls, err := ml.ListWithUser(ctx, grp, ctx.SelectedUser())
	if err != nil {
		return
	}

	for _, ml := range mls {
		/* Check that I am a member of this group */
		member, _, _, err := grp.IsMember(ctx.TheUser().GetUserName())
		if err != nil {
			return nil, err
		}

		if !member {
			continue
		}

		err = ctx.SelectML(ml.ListName, PERM_GROUP_MEMBER)
		if err != nil {
			return nil, err
		}

		mlist := ctx.SelectedML()

		/* Get the ML List Key */
		keys, err := mlist.GetKey(ctx)
		if err != nil {
			return nil, err
		}

		keyfile = append(keyfile[:], keys[:]...)
	}

	return
}

func (grp *PfGroupS) IsMember(user string) (ismember bool, isadmin bool, out PfMemberState, err error) {
	ismember = false
	isadmin = false

	q := "SELECT " +
		"mt.state, " +
		"mt.admin, " +
		"ms.can_login, " +
		"ms.can_see, " +
		"ms.can_send, " +
		"ms.can_recv, " +
		"ms.blocked, " +
		"ms.hidden " +
		"FROM member_trustgroup mt " +
		"JOIN trustgroup grp ON mt.trustgroup = grp.ident " +
		"JOIN member_state ms ON mt.state = ms.ident " +
		"WHERE mt.member = $1 " +
		"AND mt.trustgroup = $2"
	err = DB.QueryRow(q, user, grp.GroupName).Scan(&out.ident,
		&isadmin, &out.can_login, &out.can_see, &out.can_send,
		&out.can_recv, &out.blocked, &out.hidden)
	if err == ErrNoRows {
		/* Nope */
		err = nil
		return
	} else if err != nil {
		err = errors.New("Could not query member state")
		return
	}

	/* The user is a group member */
	ismember = true
	return
}

func (grp *PfGroupS) GetMembersTot(search string) (total int, err error) {
	q := "SELECT COUNT(*) " +
		"FROM member_trustgroup mt " +
		"INNER JOIN trustgroup grp ON (mt.trustgroup = grp.ident) " +
		"INNER JOIN member m ON (m.ident = mt.member) " +
		"INNER JOIN member_email me ON (me.member = m.ident) " +
		"WHERE grp.ident = $1 " +
		"AND me.email = mt.email"

	if search == "" {
		err = DB.QueryRow(q, grp.GroupName).Scan(&total)
	} else {
		q += "AND (m.ident ~* $2 " +
			"OR m.descr ~* $2 " +
			"OR m.affiliation ~* $2) "

		err = DB.QueryRow(q, grp.GroupName, search).Scan(&total)
	}

	return total, err
}

/* TODO need to allow admins to see hidden users (blocked) */
/* Note: This implementation does not use the 'username' variable, but other implementations might */
func (grp *PfGroupS) GetMembers(search string, username string, offset int, max int, nominated bool, exact bool) (members []PfGroupMember, err error) {
	var rows *Rows

	members = nil

	ord := "ORDER BY m.descr"

	q := "SELECT m.ident, " +
		"m.descr, " +
		"m.affiliation, " +
		"mt.admin, " +
		"mt.state, " +
		"mt.email, " +
		"pgpkey_id " +
		"EXTRACT(day FROM now() - m.activity) as activity, " +
		"FROM member_trustgroup mt " +
		"INNER JOIN trustgroup grp ON (mt.trustgroup = grp.ident) " +
		"INNER JOIN member m ON (mt.member = m.ident) " +
		"INNER JOIN member_state ms ON (ms.ident = mt.state) " +
		"INNER JOIN member_email me ON (me.member = m.ident) " +
		"WHERE grp.ident = $1 " +
		"AND me.email = mt.email"
	if nominated {
		q += "AND (NOT ms.hidden OR ms.ident = 'nominated') "
	} else {
		q += "AND NOT ms.hidden "
	}

	if search == "" {
		if max != 0 {
			q += ord + " LIMIT $3 OFFSET $2"
			rows, err = DB.Query(q, grp.GroupName, offset, max)
		} else {
			q += ord
			rows, err = DB.Query(q, grp.GroupName)
		}
	} else {
		if exact {
			q += "AND (m.ident = $2) " +
				ord
		} else {
			q += "AND (m.ident ~* $2 " +
				"OR m.descr ~* $2 " +
				"OR m.affiliation ~* $2) " +
				ord
		}

		if max != 0 {
			q += " LIMIT $4 OFFSET $3"
			rows, err = DB.Query(q, grp.GroupName, search, offset, max)
		} else {
			rows, err = DB.Query(q, grp.GroupName, search)
		}
	}

	if err != nil {
		Log("Query failed: " + err.Error())
		return
	}

	defer rows.Close()

	for rows.Next() {
		member := NewPfGroupMember().(*PfGroupMemberS)
		member.GroupName = grp.GroupName
		err = rows.Scan(&member.UserName,
			&member.FullName,
			&member.Affiliation,
			&member.GroupAdmin,
			&member.GroupState,
			&member.Email,
			&member.PGPKeyID,
			&member.Activity)
		if err != nil {
			Log("Error listing members: " + err.Error())
			return nil, err
		}

		members = append(members, member)
	}

	return members, nil
}

func (grp *PfGroupS) Add_default_mailinglists(ctx PfCtx) (err error) {
	mls := make(map[string]string)
	mls["admin"] = "Group Administration"
	mls["general"] = "General Discussion"

	for lhs, descr := range mls {
		b := lhs != "admin"

		err = Ml_addv(ctx, grp, lhs, descr, b, b, b)

		if err != nil {
			return
		}
	}

	return
}

func group_add(ctx PfCtx, args []string) (err error) {
	var group_name string

	/* Make sure the name is mostly sane */
	group_name, err = Chk_ident("Group Name", args[0])
	if err != nil {
		return
	}

	grp := ctx.NewGroup()
	err = grp.fetch(group_name, true)
	if err != ErrNoRows {
		err = errors.New("Group already exists")
		return
	}

	q := "INSERT INTO trustgroup " +
		"(ident, descr, shortname, pgp_required, has_wiki) " +
		"VALUES($1, $2, $3, $4, $5)"
	err = DB.Exec(ctx,
		"Created group $1",
		1, q,
		group_name, group_name, group_name, false, false)

	if err != nil {
		err = errors.New("Group creation failed")
		return
	}

	err = ctx.SelectGroup(group_name, PERM_SYS_ADMIN)
	if err != nil {
		return
	}

	/* Fetch our newly created group */
	err = grp.fetch(group_name, true)
	if err != nil {
		err = errors.New("Group creation failed")
		return
	}

	err = grp.Add_default_mailinglists(ctx)
	if err != nil {
		return
	}

	/* Add the user as the initial member */
	ctx.SelectMe()
	grp.Member_add(ctx)
	grp.Member_set_state(ctx, GROUP_STATE_APPROVED)
	grp.Member_set_admin(ctx, true)

	/* All worked */
	ctx.OutLn("Creation of group %s complete", group_name)
	return
}

func group_remove(ctx PfCtx, args []string) (err error) {
	q := "DELETE FROM trustgroup " +
		"WHERE ident = $1"

	err = DB.Exec(ctx,
		"Removed group $1",
		1, q,
		args[0])
	return
}

func group_list(ctx PfCtx, args []string) (err error) {
	grp := ctx.NewGroup()
	user := ctx.TheUser().GetUserName()

	var groups []PfGroupUser
	if ctx.IsSysAdmin() {
		groups, err = grp.GetGroupsAll()
	} else {
		groups, err = grp.GetGroups(user, true)
	}

	if err != nil {
		return
	}

	if len(groups) == 0 {
		ctx.OutLn("No Groups Found")
		return
	}

	for i := range groups {
		ctx.OutLn("%s %s", groups[i].GroupName, groups[i].GroupDesc)
	}

	return
}

func group_member_list(ctx PfCtx, args []string) (err error) {
	grp := ctx.SelectedGroup()
	tmembers, err := grp.GetMembers("", ctx.TheUser().GetUserName(), 0, 0, false, false)

	if err != nil {
		return
	}

	for i := range tmembers {
		ctx.OutLn("%s %s", tmembers[i].GetUserName(), tmembers[i].GetFullName())
	}

	return
}

func group_member_auto_ml(ctx PfCtx, user PfUser) (err error) {
	var rows *Rows

	grp := ctx.SelectedGroup()

	q := "SELECT lhs " +
		"FROM mailinglist " +
		"WHERE automatic " +
		"AND trustgroup = $1"
	rows, err = DB.Query(q, grp.GetGroupName())
	if err != nil {
		return nil
	}

	defer rows.Close()

	for rows.Next() {
		var lhs string

		err = rows.Scan(&lhs)
		if err != nil {
			return
		}

		q = "INSERT INTO member_mailinglist " +
			"(member, lhs, trustgroup) " +
			"VALUES($1, $2, $3)"
		audittxt := "Added user $1 to ML $2"
		/* Doing this here so that we don't have to otherwise forge permissions. */
		err = DB.Exec(ctx,
			audittxt,
			1, q,
			user.GetUserName(),
			lhs,
			grp.GetGroupName())
		if err != nil {
			err = errors.New("Could not modify mailinglist")
		}
	}

	return
}

func (grp *PfGroupS) Member_add(ctx PfCtx) (err error) {
	var email PfUserEmail

	user := ctx.SelectedUser()

	var ismember bool
	ismember, _, _, err = grp.IsMember(user.GetUserName())
	if err != nil {
		return
	}
	if ismember {
		err = errors.New("Already a group member")
		return
	}

	email, err = user.GetPriEmail(ctx, false)
	if err != nil {
		return
	}

	q := "INSERT INTO member_trustgroup " +
		"(member, trustgroup, email, state) " +
		"VALUES($1, $2, $3, $4)"

	err = DB.Exec(ctx,
		"Added member $1 to group $2",
		1, q,
		user.GetUserName(),
		grp.GroupName,
		email.Email,
		"nominated")

	if err != nil {
		return
	}

	err = group_member_auto_ml(ctx, user)
	if err != nil {
		return
	}

	ctx.OutLn("Member added to group")
	return
}

func group_member_add(ctx PfCtx, args []string) (err error) {
	grp := ctx.SelectedGroup()
	return grp.Member_add(ctx)
}

func (grp *PfGroupS) Member_remove(ctx PfCtx) (err error) {
	user := ctx.SelectedUser()

	var ismember bool
	ismember, _, _, err = grp.IsMember(user.GetUserName())
	if err != nil {
		return
	}
	if !ismember {
		err = errors.New("Not a member of this group")
		return
	}

	q := "DELETE FROM member_trustgroup " +
		"WHERE member = $1 " +
		"AND trustgroup = $2"

	err = DB.Exec(ctx,
		"Removed member $1 from group $2",
		1, q,
		user.GetUserName(),
		ctx.SelectedGroup())

	if err == nil {
		ctx.OutLn("Member removed from group")
	}
	return
}

func group_member_remove(ctx PfCtx, args []string) (err error) {
	grp := ctx.SelectedGroup()
	return grp.Member_remove(ctx)
}

func (grp *PfGroupS) Member_set_state(ctx PfCtx, state string) (err error) {
	user := ctx.SelectedUser()

	if !ctx.IAmGroupAdmin() {
		err = errors.New("Not a group admin")
		return
	}

	q := "UPDATE member_trustgroup " +
		"SET state = $1 " +
		"WHERE member = $2 " +
		"AND trustgroup = $3"

	err = DB.Exec(ctx,
		"Set member $2 in group $3 to state $1",
		1, q,
		state,
		user.GetUserName(),
		ctx.SelectedGroup().GetGroupName())

	ctx.OutLn("Member %s in %s marked as %s", user.GetUserName(), ctx.SelectedGroup().GetGroupName, state)
	return
}

func group_member_approve(ctx PfCtx, args []string) (err error) {
	grp := ctx.SelectedGroup()
	return grp.Member_set_state(ctx, GROUP_STATE_APPROVED)
}

func group_member_block(ctx PfCtx, args []string) (err error) {
	grp := ctx.SelectedGroup()
	return grp.Member_set_state(ctx, GROUP_STATE_BLOCKED)
}

func group_member_unblock(ctx PfCtx, args []string) (err error) {
	/* Returns state to 'approved' */
	return group_member_approve(ctx, args)
}

func (grp *PfGroupS) Member_set_admin(ctx PfCtx, isadmin bool) (err error) {
	if !ctx.IAmGroupAdmin() {
		err = errors.New("Not a group admin")
		return
	}

	q := "UPDATE member_trustgroup " +
		"SET admin = $1 " +
		"WHERE member = $2 " +
		"AND trustgroup = $3"

	err = DB.Exec(ctx,
		"Promoted member $2 in group $3",
		1, q,
		isadmin,
		ctx.SelectedUser().GetUserName(),
		ctx.SelectedGroup().GetGroupName())

	ctx.OutLn("Member marked Admin as %s", YesNo(isadmin))
	return
}

func group_member_promote(ctx PfCtx, args []string) (err error) {
	grp := ctx.SelectedGroup()
	return grp.Member_set_admin(ctx, true)
}

func group_member_demote(ctx PfCtx, args []string) (err error) {
	grp := ctx.SelectedGroup()
	return grp.Member_set_admin(ctx, false)
}

func group_member(ctx PfCtx, args []string) (err error) {
	var menu = NewPfMenu([]PfMEntry{
		{"list", group_member_list, 1, 1, []string{"group"}, PERM_GROUP_MEMBER, "List members of this group"},
		{"add", group_member_add, 2, 2, []string{"group", "username"}, PERM_GROUP_ADMIN, "Add a member to the group"},
		{"remove", group_member_remove, 2, 2, []string{"group", "username"}, PERM_GROUP_ADMIN, "Remove a member from the group"},
		{"approve", group_member_approve, 2, 2, []string{"group", "username"}, PERM_GROUP_ADMIN, "Approve a member for a group"},
		{"unblock", group_member_unblock, 2, 2, []string{"group", "username"}, PERM_GROUP_ADMIN, "Unblock the member from this group"},
		{"block", group_member_block, 2, 2, []string{"group", "username"}, PERM_GROUP_ADMIN, "Block the member from this group"},
		{"promote", group_member_promote, 2, 2, []string{"group", "username"}, PERM_GROUP_ADMIN, "Promote user to Admin"},
		{"demote", group_member_demote, 2, 2, []string{"group", "username"}, PERM_GROUP_ADMIN, "Demote user from Admin"},
	})

	if len(args) >= 2 {
		/* Check if we have perms for this group */
		err = ctx.SelectGroup(args[1], PERM_GROUP_MEMBER)
		if err != nil {
			return
		}
	} else {
		/* Nothing selected */
		ctx.SelectGroup("", PERM_NONE)
	}

	if len(args) >= 3 {
		/* Check if we have perms for this group */
		err = ctx.SelectGroup(args[2], PERM_USER_VIEW)
		if err != nil {
			return
		}
	} else {
		/* Nothing selected */
		ctx.SelectUser("", PERM_NONE)
	}

	err = ctx.Menu(args, menu)
	return
}

func group_set_xxx(ctx PfCtx, args []string) (err error) {
	/*
	 * args[.] == what, dropped by ctx.Menu()
	 * args[0] == group
	 * args[1] == val
	 */
	what := ctx.GetLastPart()
	grp := ctx.SelectedGroup()
	val := args[1]

	err = DB.UpdateFieldMsg(ctx, grp, grp.GetGroupName(), "trustgroup", what, val)
	return
}

func group_sget(ctx PfCtx, args []string, fun PfFunc) (err error) {
	grp := ctx.NewGroup()

	perms := PERM_GROUP_MEMBER
	if fun != nil {
		perms = PERM_GROUP_ADMIN
	}

	if len(args) >= 2 {
		/* Check if we have perms for this group */
		err = ctx.SelectGroup(args[1], perms)
		if err != nil {
			return
		}

		grp = ctx.SelectedGroup()
	} else {
		/* No user selected */
		ctx.SelectGroup("", PERM_NONE)
	}

	subjects := []string{"trustgroup"}

	menu, err := StructMenu(ctx, subjects, grp, false, fun)

	if err != nil {
		return
	}

	err = ctx.Menu(args, menu)
	return
}

func group_set(ctx PfCtx, args []string) (err error) {
	return group_sget(ctx, args, group_set_xxx)
}

func group_get(ctx PfCtx, args []string) (err error) {
	return group_sget(ctx, args, nil)
}

func Group_FileMod(ctx PfCtx) {
	grp := ctx.SelectedGroup()
	grpname := grp.GetGroupName()

	/* Set the ModRoot options */
	File_ModOpts(ctx, "group file", "/group/"+grpname, "/group/"+grpname+"/file")
}

func group_file(ctx PfCtx, args []string) (err error) {
	/* Module options */
	Group_FileMod(ctx)

	return File_menu(ctx, args)
}

func Group_WikiMod(ctx PfCtx) {
	grp := ctx.SelectedGroup()
	grpname := grp.GetGroupName()

	/* Set the ModRoot options */
	Wiki_ModOpts(ctx, "group wiki "+grpname, "/group/"+grpname, "/group/"+grpname+"/wiki")
}

func group_wiki(ctx PfCtx, args []string) (err error) {
	grname := args[0]

	err = ctx.SelectGroup(grname, PERM_GROUP_MEMBER)
	if err != nil {
		return
	}

	/* Module options */
	Group_WikiMod(ctx)

	return Wiki_menu(ctx, args[1:])
}

func group_menu(ctx PfCtx, args []string) (err error) {
	menu := NewPfMenu([]PfMEntry{
		{"add", group_add, 1, 1, []string{"group"}, PERM_SYS_ADMIN, "Add a new group"},
		{"remove", group_remove, 1, 1, []string{"group"}, PERM_SYS_ADMIN, "Remove a group"},
		{"list", group_list, 0, 0, nil, PERM_USER, "List all groups"},
		{"set", group_set, 0, -1, nil, PERM_USER, "Set properties of a group"},
		{"get", group_get, 0, -1, nil, PERM_USER, "Get properties of a group"},
		{"member", group_member, 0, -1, nil, PERM_USER, "Member commands"},
		{"file", group_file, 0, -1, nil, PERM_USER, "File"},
		{"wiki", group_wiki, 1, -1, []string{"group"}, PERM_USER, "Wiki"},
	})

	err = ctx.Menu(args, menu)
	return
}
