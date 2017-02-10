package pitchfork

import (
	"crypto/md5"
	"errors"

	pfpgp "trident.li/pitchfork/lib/pgp"
)

type PfML struct {
	ListName     string `label:"List Name" pfset:"nobody" pfget:"user" pfcol:"lhs" hint:"Name of this Mailing List"`
	GroupName    string `label:"Group Name" pfset:"nobody" pfget:"user" pfcol:"trustgroup" hint:"Group this list belongs to"`
	Descr        string `label:"Description" pfset:"group_admin" hint:"Description of this list"`
	Members_only bool   `label:"Members Only" pfset:"group_admin" hint:"Only list members can post to this list"`
	Can_add_self bool   `label:"Can Add Self" pfset:"group_admin" hint:"Can members of the group subscribe themselves to the list?"`
	Automatic    bool   `label:"Automatic" pfset:"group_admin" hint:"If new group members are automatically added to the list"`
	Always_crypt bool   `label:"Always Encrypt" pfset:"group_admin" hint:"Always require messages to be PGP encrypted"`
	Pubkey       string `label:"Public PGP Key" pfset:"nobody" pfget:"user" pfcol:"pubkey"`
	Seckey       string `label:"Secret PGP Key" pfset:"nobody" pfget:"nobody" pfcol:"seckey" pfskipfailperm:"yes"`
	Address      string /* Not retrieved with Fetch() */
	Button       string `label:"Update Configuration" pftype:"submit"`
	Members      int    /* Not retrieved with Fetch() */
	Subscribed   bool   /* Not retrieved with Fetch() */
}

type PfMLUser struct {
	UserName      string
	Uuid          string
	FullName      string
	Affiliation   string
	IsSysAdmin    bool
	LoginAttempts int
}

func (mlu *PfMLUser) GetUserName() string {
	return mlu.UserName
}

func (mlu *PfMLUser) GetFullName() string {
	return mlu.FullName
}

func (mlu *PfMLUser) GetAffiliation() string {
	return mlu.Affiliation
}

func NewPfML() *PfML {
	return &PfML{}
}

func (ml *PfML) fetch(gr_name string, ml_name string) (err error) {
	/* Make sure the name is mostly sane */
	gr_name, err = Chk_ident("Group Name", gr_name)
	if err != nil {
		return
	}

	/* Make sure the name is mostly sane */
	ml_name, err = Chk_ident("Mailinglist Name", ml_name)
	if err != nil {
		return
	}

	p := []string{"trustgroup", "lhs"}
	v := []string{gr_name, ml_name}
	err = StructFetch(ml, "mailinglist", p, v)
	if err != nil {
		ml.ListName = ""
		Log(err.Error() + " '" + ml_name + "'")
	}

	return
}

func (ml *PfML) Select(ctx PfCtx, grp PfGroup, ml_name string, perms Perm) (err error) {
	ctx.Dbg("SelectML(" + ml_name + ")")

	err = ml.fetch(grp.GetGroupName(), ml_name)
	if err != nil {
		ml = nil
	}

	if ctx.IsSysAdmin() {
		/* All okay */
		return
	}

	/* Required to be group admin? */
	if ctx.IsPermSet(perms, PERM_GROUP_ADMIN) {
		if ctx.IAmGroupAdmin() {
			/* Yep */
			return
		}
	} else if ctx.IsPermSet(perms, PERM_GROUP_MEMBER) && ctx.IsGroupMember() {
		/* All okay */
		return
	} else {
		err = errors.New("Not a group member / unknown group")
		/* Nope */
	}

	/* Reset */
	ml = nil
	return
}

func (ml *PfML) Refresh() (err error) {
	err = ml.fetch(ml.GroupName, ml.ListName)
	return
}

func (ml *PfML) ListGroupMembersMax(search string) (total int, err error) {
	q := "SELECT COUNT(*) " +
		"FROM member_mailinglist mlm " +
		"INNER JOIN member m ON (mlm.member = m.ident) " +
		"WHERE mlm.trustgroup = $1 " +
		"AND mlm.lhs = $2 "

	if search == "" {
		err = DB.QueryRow(q, ml.GroupName, ml.ListName).Scan(&total)
	} else {
		q += "AND (m.ident ~* $3 " +
			"OR m.descr ~* $3 " +
			"OR m.affiliation ~* $3) "

		err = DB.QueryRow(q, ml.GroupName, ml.ListName, search).Scan(&total)
	}

	return
}

func (ml *PfML) ListGroupMembers(search string, offset int, max int) (members []PfMLUser, err error) {
	var rows *Rows

	ord := "ORDER BY m.descr"

	q := "SELECT m.ident, m.uuid, m.descr, m.affiliation, m.sysadmin, m.login_attempts " +
		"FROM member_mailinglist mlm " +
		"INNER JOIN member m ON (mlm.member = m.ident) " +
		"WHERE mlm.trustgroup = $1 " +
		"AND mlm.lhs = $2 "

	if search == "" {
		if max != 0 {
			q += ord + " LIMIT $4 OFFSET $3"
			rows, err = DB.Query(q, ml.GroupName, ml.ListName, offset, max)
		} else {
			q += ord
			rows, err = DB.Query(q, ml.GroupName, ml.ListName)
		}
	} else {
		q += "AND (m.ident ~* $3 " +
			"OR m.descr ~* $3 " +
			"OR m.affiliation ~* $3) " +
			ord

		if max != 0 {
			q += " LIMIT $5 OFFSET $4"
			rows, err = DB.Query(q, ml.GroupName, ml.ListName, search, offset, max)
		} else {
			rows, err = DB.Query(q, ml.GroupName, ml.ListName, search)
		}
	}

	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		var mlu PfMLUser

		err = rows.Scan(&mlu.UserName, &mlu.Uuid, &mlu.FullName, &mlu.Affiliation, &mlu.IsSysAdmin, &mlu.LoginAttempts)

		members = append(members, mlu)
	}

	return
}

func (ml *PfML) IsMember(user PfUser) (ok bool, err error) {
	cnt := 0
	ok = false

	q := "SELECT COUNT(*) " +
		"FROM member_mailinglist " +
		"WHERE member = $1 " +
		"AND trustgroup = $2 " +
		"AND lhs = $3 "

	err = DB.QueryRow(q, user.GetUserName(), ml.GroupName, ml.ListName).Scan(&cnt)

	if err != nil {
		return
	}

	if cnt != 0 {
		ok = true
	}

	return
}

func (ml *PfML) List(ctx PfCtx, grp PfGroup) (mls []PfML, err error) {
	mls = nil

	q := "SELECT ml.lhs, ml.descr, ml.members_only, ml.always_crypt," +
		" COALESCE(members.num, 0) AS members" +
		" FROM mailinglist ml" +
		" LEFT OUTER JOIN (" +
		" SELECT lhs, trustgroup, COUNT(*) AS num" +
		" FROM member_mailinglist mm" +
		"  GROUP BY mm.trustgroup,mm.lhs" +
		") as members ON (ROW(ml.lhs,ml.trustgroup) = ROW(members.lhs,members.trustgroup))" +
		"WHERE ml.trustgroup = $1" +
		"ORDER BY ml.lhs"
	rows, err := DB.Query(q, grp.GetGroupName())
	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		var ml PfML

		ml.GroupName = grp.GetGroupName()

		err = rows.Scan(&ml.ListName, &ml.Descr, &ml.Members_only,
			&ml.Always_crypt, &ml.Members)
		if err != nil {
			return
		}

		ml.Address = ml.GroupName + "-" + ml.ListName + "@" + System_Get().EmailDomain

		mls = append(mls, ml)
	}

	return
}

func (ml *PfML) ListWithUser(ctx PfCtx, grp PfGroup, user PfUser) (mls []PfML, err error) {
	mls = nil

	q := "SELECT ml.lhs, ml.descr, ml.members_only, ml.always_crypt," +
		" ml.can_add_self, " +
		" COALESCE(members.num, 0) AS members, " +
		" COALESCE(subs.num, 0) as subs" +
		" FROM mailinglist ml" +
		" LEFT OUTER JOIN (" +
		"  SELECT lhs, trustgroup, COUNT(*) AS num" +
		" FROM member_mailinglist mm" +
		" WHERE mm.member = $2" +
		"  AND mm.trustgroup = $1" +
		"  GROUP BY mm.trustgroup,mm.lhs " +
		" ) as subs on (ROW(ml.lhs,ml.trustgroup) = ROW(subs.lhs, subs.trustgroup)) " +
		" LEFT OUTER JOIN (" +
		" SELECT lhs, trustgroup, COUNT(*) AS num" +
		" FROM member_mailinglist mm" +
		" WHERE mm.trustgroup = $1" +
		" GROUP BY mm.trustgroup,mm.lhs " +
		") as members ON (ROW(ml.lhs,ml.trustgroup) = ROW(members.lhs,members.trustgroup))" +
		" WHERE ml.trustgroup = $1" +
		" ORDER BY ml.lhs"

	rows, err := DB.Query(q, grp.GetGroupName(), user.GetUserName())
	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		var ml PfML

		ml.GroupName = grp.GetGroupName()

		err = rows.Scan(&ml.ListName, &ml.Descr, &ml.Members_only,
			&ml.Always_crypt, &ml.Can_add_self, &ml.Members, &ml.Subscribed)
		if err != nil {
			return
		}

		ml.Address = ml.GroupName + "-" + ml.ListName + "@" + System_Get().EmailDomain

		mls = append(mls, ml)
	}

	return
}

func (ml *PfML) GetKey(ctx PfCtx, keyset map[[16]byte][]byte) (err error) {
	var key string

	q := "SELECT COALESCE(pubkey, '') " +
		"FROM mailinglist " +
		"WHERE trustgroup = $1 " +
		"AND lhs = $2 "

	err = DB.QueryRow(q, ml.GroupName, ml.ListName).Scan(&key)
	if err != nil {
		return
	}

	/* Only append a list key when it exists */
	if key != "" {
		keyset[md5.Sum([]byte(key))] = []byte(key)
	}

	/* List active members/collect keys */
	err = ListKeys(ctx, keyset, ml.GroupName, ml.ListName)
	if err != nil {
		return
	}

	return
}

func ml_list(ctx PfCtx, args []string) (err error) {
	gr_name := args[0]

	var ml PfML

	err = ctx.SelectGroup(gr_name, PERM_GROUP_MEMBER)
	if err != nil {
		return
	}

	grp := ctx.SelectedGroup()

	mls, err := ml.List(ctx, grp)
	if err != nil {
		return
	}

	for _, ml := range mls {
		ctx.OutLn(ml.ListName + "\t" + ml.Descr)
	}

	return
}

func ml_member_list(ctx PfCtx, args []string) (err error) {
	gr_name := args[0]
	ml_name := args[1]

	err = ctx.SelectGroup(gr_name, PERM_GROUP_MEMBER)
	if err != nil {
		return
	}

	err = ctx.SelectML(ml_name, PERM_GROUP_MEMBER)
	if err != nil {
		return
	}

	grp := ctx.SelectedGroup()
	ml := ctx.SelectedML()

	q := "SELECT member " +
		"FROM member_mailinglist " +
		"WHERE trustgroup = $1 AND lhs = $2"

	rows, err := DB.Query(q, grp.GetGroupName(), ml.ListName)
	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		var username string

		err = rows.Scan(&username)
		if err != nil {
			return
		}

		ctx.OutLn(username)
	}
	return
}

func ListKeys(ctx PfCtx, keyset map[[16]byte][]byte, gr_name string, ml_name string) (err error) {
	q := "SELECT me.keyring " +
		"FROM member_email me, " +
		"member_mailinglist ml, " +
		"member_trustgroup mt " +
		"WHERE me.member = ml.member " +
		" AND me.member = mt.member " +
		" AND me.email = mt.email " +
		" AND mt.trustgroup = ml.trustgroup " +
		" AND (mt.state = 'active' or mt.state = 'soonidle') " +
		" AND ml.trustgroup = $1 " +
		" AND ml.lhs = $2"

	rows, err := DB.Query(q, gr_name, ml_name)
	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		var key string

		err = rows.Scan(&key)
		if err != nil {
			return
		}
		keyset[md5.Sum([]byte(key))] = []byte(key)
	}
	return
}

func ml_member_mod(ctx PfCtx, args []string, add bool) (err error) {
	var ok bool

	gr_name := args[0]
	ml_name := args[1]
	us_name := args[2]

	if !ctx.HasSelectedML() {
		err = errors.New("No ML selected")
		return
	}

	ml := ctx.SelectedML()

	/* Does the ML allow self-adding? */
	if !ml.Can_add_self {
		/* Requires GROUP_ADMIN perms */
		err = ctx.SelectGroup(gr_name, PERM_GROUP_ADMIN)
		if err != nil {
			return
		}
	}

	/* Get the user */
	err = ctx.SelectUser(us_name, PERM_USER_SELF|PERM_GROUP_ADMIN)
	if err != nil {
		return
	}

	user := ctx.SelectedUser()
	grp := ctx.SelectedGroup()

	/* Is the user a group member? */
	var ismember bool
	ismember, _, _, err = grp.IsMember(user.GetUserName())
	if err != nil {
		return
	}

	if !ismember {
		err = errors.New("Not a group member")
		return
	}

	/* Already a member? */
	ok, err = ml.IsMember(user)
	if err != nil {
		return
	}

	q := ""
	audittxt := ""
	success := ""

	if add {
		if ok {
			err = errors.New("Already a list member")
			return
		}

		/* Add the user */
		q = "INSERT INTO member_mailinglist " +
			"(member, lhs, trustgroup) " +
			"VALUES($1, $2, $3)"
		audittxt = "Added user $1 to ML $2"
		success = "subscribed user"
	} else {
		if !ok {
			err = errors.New("Not a list member")
			return
		}

		q = "DELETE FROM member_mailinglist " +
			"WHERE member = $1 " +
			"AND lhs = $2 " +
			"AND trustgroup = $3"
		audittxt = "Removed user $1 from ML $2"
		success = "unsubscribed user"
	}

	err = DB.Exec(ctx,
		audittxt,
		1, q,
		us_name, ml_name, gr_name)
	if err != nil {
		err = errors.New("Could not modify mailinglist")
		return
	}

	ctx.OutLn("Successfully " + success)

	return
}

func ml_member_add(ctx PfCtx, args []string) (err error) {
	err = ml_member_mod(ctx, args, true)
	return
}

func ml_member_remove(ctx PfCtx, args []string) (err error) {
	err = ml_member_mod(ctx, args, false)
	return
}

func ml_member(ctx PfCtx, args []string) (err error) {
	var menu = NewPfMenu([]PfMEntry{
		{"list", ml_member_list, 2, 2, []string{"group", "ml"}, PERM_GROUP_MEMBER, "List members of this Mailing List"},
		{"add", ml_member_add, 3, 3, []string{"group", "ml", "username"}, PERM_GROUP_MEMBER, "Add a member to the Mailing List"},
		{"remove", ml_member_remove, 3, 3, []string{"group", "ml", "username"}, PERM_GROUP_MEMBER, "Remove a member from the Mailing List"},
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
		/* Check if we have perms for this ML */
		err = ctx.SelectML(args[2], PERM_GROUP_MEMBER)
		if err != nil {
			return
		}
	} else {
		/* Nothing selected */
		ctx.SelectML("", PERM_NONE)
	}

	err = ctx.Menu(args, menu)
	return
}

func ml_remove(ctx PfCtx, args []string) (err error) {
	gr_name := args[0]
	ml_name := args[1]

	err = ctx.SelectGroup(gr_name, PERM_GROUP_ADMIN)
	if err != nil {
		return
	}

	err = ctx.SelectML(ml_name, PERM_GROUP_ADMIN)
	if err != nil {
		return
	}

	grp := ctx.SelectedGroup()
	ml := ctx.SelectedML()

	q := "DELETE FROM mailinglist " +
		"WHERE lhs = $1 " +
		"AND trustgroup = $2"
	err = DB.Exec(ctx,
		"Removed ML $1",
		1, q,
		ml.ListName, grp.GetGroupName())
	return
}

func ml_pgp_create(ctx PfCtx, ml PfML) (err error) {
	seckey, pubkey, err := pfpgp.CreateKey(ml.Address, ml.GroupName+" "+ml.ListName, ml.Descr)

	q := "UPDATE mailinglist " +
		"SET seckey = $1, " +
		"pubkey = $2, " +
		"key_update_at = NOW() " +
		"WHERE trustgroup = $3 " +
		"AND lhs = $4"
	err = DB.Exec(ctx,
		"Added PGP key for group $3 ML $4",
		1, q,
		seckey, pubkey, ml.GroupName, ml.ListName)
	return
}

func ml_getit(ctx PfCtx, args []string) (ml PfML, err error) {
	gr_name := args[0]
	ml_name := args[1]

	err = ctx.SelectGroup(gr_name, PERM_GROUP_MEMBER)
	if err != nil {
		return
	}

	err = ctx.SelectML(ml_name, PERM_GROUP_MEMBER)
	if err != nil {
		return
	}

	ml = ctx.SelectedML()
	return
}

func ml_newkeys(ctx PfCtx, args []string) (err error) {
	ml, err := ml_getit(ctx, args)
	if err != nil {
		return
	}

	return ml_pgp_create(ctx, ml)
}

func ml_pubkey(ctx PfCtx, args []string) (err error) {
	ml, err := ml_getit(ctx, args)
	if err != nil {
		return
	}

	ctx.OutLn(ml.Pubkey)

	return
}

func ml_seckey(ctx PfCtx, args []string) (err error) {
	ml, err := ml_getit(ctx, args)
	if err != nil {
		return
	}

	ctx.OutLn(ml.Seckey)

	return
}

/* Group must be selected with PERM_GROUP_ADMIN */
func Ml_addv(ctx PfCtx, grp PfGroup, ml_name string, descr string, member_only bool, can_add_self bool, automatic bool) (err error) {
	q := "INSERT INTO mailinglist " +
		"(lhs, descr, members_only, " +
		"can_add_self, automatic, trustgroup) " +
		"VALUES($1, $2, $3, $4, $5, $6)"
	err = DB.Exec(ctx,
		"Added ML $1",
		1, q,
		ml_name, descr, member_only, can_add_self, automatic, grp.GetGroupName())

	if err == nil {
		ctx.OutLn("Created group " + grp.GetGroupName() + " mailinglist " + ml_name)
	}

	return
}

func ml_new(ctx PfCtx, args []string) (err error) {
	gr_name := args[0]
	ml_name := args[1]
	descr := ml_name

	/* Check if we are admin for this group */
	err = ctx.SelectGroup(gr_name, PERM_GROUP_ADMIN)
	if err != nil {
		return
	}

	var ml PfML

	/* Check if the ML already exists */
	err = ml.fetch(gr_name, ml_name)
	if err == nil {
		err = errors.New("Mailinglist already exists")
		return
	}

	grp := ctx.SelectedGroup()

	err = Ml_addv(ctx, grp, ml_name, descr, true, true, false)
	if err != nil {
		return
	}

	/* Try to select it */
	err = ctx.SelectML(ml_name, PERM_GROUP_MEMBER)
	if err != nil {
		return
	}

	ml = ctx.SelectedML()

	/* Create PGP keys for it */
	ml_pgp_create(ctx, ml)

	return
}

func ml_set_xxx(ctx PfCtx, args []string) (err error) {
	/*
	 * args[.] == what, dropped by ctx.Menu()
	 * args[0] == grp
	 * args[1] == ml
	 * args[2] == val
	 */

	what := ctx.GetLastPart()
	ml := ctx.SelectedML()
	val := args[2]

	set := make(map[string]string)
	set["lhs"] = ml.ListName
	set["trustgroup"] = args[0]

	err = DB.UpdateFieldMultiMsg(ctx, ml, set, "mailinglist", what, val)
	return
}

func ml_sget(ctx PfCtx, args []string, fun PfFunc) (err error) {
	/*
	 * args[0] == what
	 * args[1] == grp
	 * args[2] == ml
	 * args[3] == val
	 */

	/* View Only */
	perms := PERM_GROUP_MEMBER

	/* Trying to set something */
	if fun != nil {
		/* Admin mode */
		perms = PERM_GROUP_ADMIN
	}

	if len(args) >= 2 {
		/* Check if we have perms for this group */
		err = ctx.SelectGroup(args[1], perms)
		if err != nil {
			return
		}
	} else {
		/* No group selected */
		ctx.SelectGroup("", PERM_NONE)
	}

	if len(args) >= 3 {
		/* Check if we have perms for this ML */
		err = ctx.SelectML(args[2], PERM_GROUP_MEMBER)
		if err != nil {
			return
		}
	} else {
		/* No user selected */
		ctx.SelectML("", PERM_NONE)
	}

	ml := ctx.SelectedML()

	menu, err := StructMenu(ctx, []string{"group", "ml"}, ml, false, fun)

	if err != nil {
		return
	}

	/* Menu drops args[0] (what), get it with GetLastPart() */
	err = ctx.Menu(args, menu)
	return
}

func ml_set(ctx PfCtx, args []string) (err error) {
	return ml_sget(ctx, args, ml_set_xxx)
}

func ml_get(ctx PfCtx, args []string) (err error) {
	return ml_sget(ctx, args, nil)
}

func ml_menu(ctx PfCtx, args []string) (err error) {
	menu := NewPfMenu([]PfMEntry{
		{"new", ml_new, 2, 2, []string{"group", "ml"}, PERM_GROUP_ADMIN, "Add a new Mailinglist"},
		{"remove", ml_remove, 2, 2, []string{"group", "ml"}, PERM_GROUP_ADMIN, "Remove a Mailinglist"},
		{"list", ml_list, 1, 1, []string{"group"}, PERM_USER, "List all mailinglists for a group"},
		{"member", ml_member, 0, -1, nil, PERM_USER, "Member functions"},
		{"set", ml_set, 0, -1, nil, PERM_GROUP_ADMIN, "Set properties of a list"},
		{"get", ml_get, 0, -1, nil, PERM_USER, "Get properties of a list"},
		{"pubkey", ml_pubkey, 2, 2, []string{"group", "ml"}, PERM_GROUP_MEMBER, "Get the PGP Public Key of the list"},
		{"seckey", ml_seckey, 2, 2, []string{"group", "ml"}, PERM_SYS_ADMIN, "Get the PGP Secret Key of the list"},
		{"newkeys", ml_newkeys, 2, 2, []string{"group", "ml"}, PERM_GROUP_ADMIN, "Force generating a new PGP Key for the list"},
	})

	err = ctx.Menu(args, menu)
	return
}
