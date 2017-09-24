// Pitchfork User management
package pitchfork

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/pborman/uuid"
)

// Standardized error messages
var (
	Err_NoPassword = errors.New("Please actually provide a password")
)

// PfPostCreateI is a prototype to allow PostCreate to be overridden
type PfPostCreateI func(ctx PfCtx, user PfUser) (err error)

// PfPostFetchI is a prototype to allow PostFetch to be overridden
//
// PostFetch allows overriding what happens after fetching a user from
// the database (whether that succeeded or not)
//
// This allows extra details to be retrieved/derived.
//
// Or in the case of a failure for the application to autocreate
// an account in the database and allow access nevertheless.
//
// in_err passes in the error from the original User.Fetch() call.
// the returned error can pass this on, or return an alternative
// version when reality has changed.
type PfPostFetchI func(ctx PfCtx, user PfUser, username string, in_err error) (err error)

// PfUser is the interface towards the User object
type PfUser interface {
	SetUserName(name string)
	GetUserName() string
	SetFullName(name string)
	GetFullName() string
	SetFirstName(name string)
	GetFirstName() string
	SetLastName(name string)
	GetLastName() string
	SetSysAdmin(isone bool)
	CanBeSysAdmin() bool
	IsSysAdmin() bool
	GetLoginAttempts() int
	GetUuid() string
	GetAffiliation() string
	GetGroups(ctx PfCtx) (groups []PfGroupMember, err error)
	IsMember(ctx PfCtx, groupname string) (ismember bool)
	GetListMax(search string) (total int, err error)
	GetList(ctx PfCtx, search string, offset int, max int, exact bool) (users []PfUser, err error)
	fetch(ctx PfCtx, username string) (err error)
	Refresh(ctx PfCtx) (err error)
	Select(ctx PfCtx, username string, perms Perm) (err error)
	SetRecoverToken(ctx PfCtx, token string) (err error)
	SharedGroups(ctx PfCtx, otheruser PfUser) (ok bool, err error)
	GetImage(ctx PfCtx) (img []byte, err error)
	GetHideEmail() (hide_email bool)
	GetKeys(ctx PfCtx, keyset map[[16]byte][]byte) (err error)
	GetDetails() (details []PfUserDetail, err error)
	GetLanguages() (languages []PfUserLanguage, err error)
	Get(what string) (val string, err error)
	GetTime(what string) (val time.Time, err error)
	SetPassword(ctx PfCtx, pwtype string, password string) (err error)
	CheckAuth(ctx PfCtx, username string, password string, twofactor string) (err error)
	Verify_Password(ctx PfCtx, password string) (err error)
	GetSF() (sf string, err error)
	GetPriEmail(ctx PfCtx, recovery bool) (tue PfUserEmail, err error)
	Fetch2FA() (tokens []PfUser2FA, err error)
	Verify_TwoFactor(ctx PfCtx, twofactor string, id int) (err error)
	GetLastActivity(ctx PfCtx) (entered time.Time, ip string)
	Create(ctx PfCtx, username string, email string, bio_info string, affiliation string, descr string) (err error)
}

// PfUserS implements a standard Pitchfork PfUser
//
// All values have to be exportable, otherwise our StructFetch() etc tricks do not work
//
// But that is also the extent that these should be used, they should always be accessed
// using the interface, not directly (direct access only internally).
//
// Even templates need to use Get*() variants as they receive interfaces.
type PfUserS struct {
	Uuid          string        `label:"UUID" coalesce:"00000000-0000-0000-0000-000000000000" pfset:"nobody" pfget:"sysadmin" pfskipfailperm:"yes"`
	Image         string        `label:"Image" pfset:"self" pfget:"user_view" pftype:"file" pfb64:"yes" hint:"Upload an image of yourself, the system will scale it" pfmaximagesize:"250x250"`
	UserName      string        `label:"User Name" pfset:"self" pfget:"user_view" pfcol:"ident" min:"3" hint:"The username of this user" pfformedit:"no"`
	FullName      string        `label:"Full Name" pfset:"self" pfget:"user_view" pfcol:"descr" hint:"Full Name of this user"`
	FirstName     string        `label:"First Name" pfset:"self" pfget:"user_view" pfcol:"name_first" hint:"The First Name of the user"`
	LastName      string        `label:"Last Name" pfset:"self" pfget:"user_view" pfcol:"name_last" hint:"The Last name of the user"`
	Affiliation   string        `label:"Affiliation" pfset:"self" pfget:"user_view" hint:"Who the user is affiliated to"`
	Postal        string        `label:"Postal Details" pftype:"text" pfset:"self" pfget:"user_view" pfcol:"post_info" hint:"Postal address or other such details"`
	Sms           string        `label:"SMS" pfset:"self" pfget:"user_view" pfcol:"sms_info" hint:"The phone number where to contact the user using SMS messages"`
	Im            string        `label:"I.M." pfset:"self" pfget:"user_view" pfcol:"im_info" hint:"Instant Messaging details"`
	Timezone      string        `label:"Timezone" pfset:"self" pfget:"user_view" pfcol:"tz_info" hint:"Timezone details"`
	Telephone     string        `label:"Telephone" pftype:"tel" pfset:"self" pfget:"user_view" pfcol:"tel_info" hint:"The phone number where to contact the user using voice messages"`
	Airport       string        `label:"Airport" min:"3" max:"3" pfset:"self" pfget:"user_view" hint:"Closest airport for this user"`
	Biography     string        `label:"Biography" pftype:"text" pfset:"self" pfget:"user_view" pfcol:"bio_info" hint:"Biography for this user"`
	IsSysadmin    bool          `label:"System Administrator" pfset:"sysadmin" pfget:"group_admin" pfskipfailperm:"yes" pfcol:"sysadmin" hint:"Wether the user is a System Administrator"`
	CanBeSysadmin bool          `label:"Can Be System Administrator" pfset:"nobody" pfget:"nobody" pfskipfailperm:"yes" pfcol:"sysadmin" hint:"If the user can toggle between Regular and SysAdmin usermode"`
	LoginAttempts int           `label:"Number of failed Login Attempts" pfset:"self,group_admin" pfget:"group_admin" pfskipfailperm:"yes" pfcol:"login_attempts" hint:"How many failed login attempts have been registered"`
	No_email      bool          `label:"Email Disabled" pfset:"sysadmin" pfget:"self,group_admin" pfskipfailperm:"yes" hint:"Email address is disabled due to SMTP errors"`
	Hide_email    bool          `label:"Hide email address" pfset:"self" pfget:"self" pfskipfailperm:"yes" hint:"Hide my domain name when forwarding group emails, helpful for DMARC and SPF"`
	RecoverEmail  string        `label:"Email Recovery address" pfset:"self" pfget:"self" pfskipfailperm:"yes" hint:"The email address used for recovering passwords" pfcol:"recover_email"`
	Furlough      bool          `label:"Furlough" pfset:"self" pfget:"user,user_view" hint:"Extended holiday or furlough"`
	Entered       time.Time     `label:"Entered" pfset:"nobody" pfget:"user,user_view" hint:"Timestamp in UTC"`
	Activity      time.Time     `label:"Last Activity" pfset:"nobody" pfget:"user,user_view" hint:"Timestamp in UTC"`
	Button        string        `label:"Update Profile" pftype:"submit"`
	Password      string        `pfset:"nobody" pfget:"nobody" pfskipfailperm:"yes"`
	Passwd_chat   string        `pfset:"nobody" pfget:"nobody" pfskipfailperm:"yes"`
	f_postcreate  PfPostCreateI /* Function set at NewPfUser() */
	f_postfetch   PfPostFetchI  /* Function set at NewPfUser() */
}

// NewPfUser can be used to create a new user
// but normally it should not be directly called:
// use ctx/cui.NewUser() instead as that can be overridden.
//
// One typically calls this function from the constructor
// of the application NewUser function.
//
func NewPfUser(postcreate PfPostCreateI, postfetch PfPostFetchI) PfUser {
	return &PfUserS{f_postcreate: postcreate, f_postfetch: postfetch}
}

// NewPfUserA creates a new empty Pfuser object
func NewPfUserA() PfUser {
	return NewPfUser(nil, nil)
}

func (user *PfUserS) SetUserName(name string) {
	user.UserName = strings.ToLower(name)
}

func (user *PfUserS) GetUserName() string {
	return user.UserName
}

func (user *PfUserS) SetFullName(name string) {
	user.FullName = name
}

func (user *PfUserS) GetFullName() string {
	return user.FullName
}

func (user *PfUserS) SetFirstName(name string) {
	user.FirstName = name
}

func (user *PfUserS) GetFirstName() string {
	return user.FirstName
}

func (user *PfUserS) SetLastName(name string) {
	user.LastName = name
}

func (user *PfUserS) GetLastName() string {
	return user.LastName
}

func (user *PfUserS) SetSysAdmin(isone bool) {
	user.IsSysadmin = isone
}

func (user *PfUserS) CanBeSysAdmin() bool {
	return user.CanBeSysadmin
}

func (user *PfUserS) IsSysAdmin() bool {
	return user.IsSysadmin
}

func (user *PfUserS) GetLoginAttempts() int {
	return user.LoginAttempts
}

func (user *PfUserS) GetUuid() string {
	return user.Uuid
}

func (user *PfUserS) GetAffiliation() string {
	return user.Affiliation
}

func (user *PfUserS) GetGroups(ctx PfCtx) (groups []PfGroupMember, err error) {
	grp := ctx.NewGroup()
	return grp.GetGroups(ctx, user.GetUserName())
}

func (user *PfUserS) IsMember(ctx PfCtx, groupname string) (ismember bool) {
	groups, err := user.GetGroups(ctx)
	if err != nil {
		return false
	}

	for _, grp := range groups {
		if grp.GetGroupName() == groupname {
			return true
		}
	}

	return false
}

func (user *PfUserS) GetListMax(search string) (total int, err error) {
	if search == "" {
		q := "SELECT COUNT(*) " +
			"FROM member"

		err = DB.QueryRow(q).Scan(&total)
	} else {
		q := "SELECT COUNT(*) " +
			"FROM member " +
			"WHERE ident ~* $1 " +
			"OR descr ~* $1 " +
			"OR affiliation ~* $1 "

		err = DB.QueryRow(q, search).Scan(&total)
	}

	return total, err
}

/* TODO: Verify: Only show member of groups my user is associated with and are non-anonymous */
func (user *PfUserS) GetList(ctx PfCtx, search string, offset int, max int, exact bool) (users []PfUser, err error) {
	users = nil

	/* The fields we match on */
	matches := []string{"ident", "me.email", "descr", "affiliation", "bio_info", "d.value"}

	var p []string
	var t []DB_Op
	var v []interface{}

	for _, m := range matches {
		p = append(p, m)
		t = append(t, DB_OP_ILIKE)

		if exact {
			v = append(v, search)
		} else {
			v = append(v, "%"+search+"%")
		}
	}

	j := "INNER JOIN member_email me ON member.ident = me.member " +
		"LEFT OUTER JOIN member_details d ON d.member = member.ident"

	o := "GROUP BY member.ident " +
		"ORDER BY member.ident"

	objs, err := StructFetchMulti(ctx.NewUserI, "member", j, DB_OP_OR, p, t, v, o, offset, max)
	if err != nil {
		return
	}

	/* Get the groups these folks are in */
	for _, o := range objs {
		u := o.(PfUser)
		users = append(users, u)
	}

	return users, err
}

func (user *PfUserS) fetch(ctx PfCtx, username string) (err error) {
	/* Retain SysAdmin bit */
	sysadminbit := user.IsSysadmin

	/* Force lower case username */
	username = strings.ToLower(username)

	/* Make sure the name is mostly sane */
	username, err = Chk_ident("UserName", username)
	if err != nil {
		return
	}

	p := []string{"ident"}
	v := []string{username}
	err = StructFetch(user, "member", p, v)
	if err != nil {
		user.UserName = ""
		Log(err.Error() + " while fetching user '" + username + "'")
	}

	/* Call our PostFetch hook? */
	if user.f_postfetch != nil {
		err = user.f_postfetch(ctx, user, username, err)
	}

	/* Do not retain the bit when the fetch failed */
	if err == nil {
		/* Can be a SysAdmin? */
		user.CanBeSysadmin = user.IsSysadmin

		/* Retain SysAdmin bit */
		user.IsSysadmin = sysadminbit
	} else {
		/* No sysadmin for this user */
		user.CanBeSysadmin = false
		user.IsSysadmin = false
	}

	return
}

func (user *PfUserS) Refresh(ctx PfCtx) (err error) {
	/* Fetch it if it exists */
	err = user.fetch(ctx, user.UserName)
	return
}

func (user *PfUserS) Select(ctx PfCtx, username string, perms Perm) (err error) {
	/* Selecting own user? */
	if username == "" {
		/* No need to refresh */
	} else {
		/* Fetch it if it exists */
		err = user.fetch(ctx, username)
		if err != nil {
			/* Failed */
			return err
		}
	}

	/*
	 * No permissions needed?
	 * This is used by password recovery
	 */
	if perms.IsSet(PERM_NONE) {
		return nil
	}

	/* SysAdmins can select anybody */
	if ctx.IsSysAdmin() {
		return nil
	}

	/* Can select self */
	if perms.IsSet(PERM_USER_SELF) &&
		ctx.IsLoggedIn() &&
		user.UserName == ctx.TheUser().GetUserName() {
		return nil
	}

	/* Can always nominate people */
	if perms.IsSet(PERM_USER_NOMINATE) {
		return nil
	}

	/* Can we view people? */
	if perms.IsSet(PERM_USER_VIEW) && ctx.IsLoggedIn() {
		/* Only when they share a group */
		var ok bool
		ok, err = user.SharedGroups(ctx, ctx.TheUser())
		if err != nil {
			return
		}

		if ok {
			/* Allowed */
			return nil
		}
	}

	/* Group admins can select users too */
	/* XXX: Need to restrict this as a group admin is not all powerful or all-seeing */
	if perms.IsSet(PERM_GROUP_ADMIN) && ctx.IAmGroupAdmin() {
		return nil
	}

	return errors.New("Could not select user")
}

func (user *PfUserS) SetRecoverToken(ctx PfCtx, token string) (err error) {
	q := "UPDATE member SET " +
		"recover_password = $1, " +
		"recover_password_set_at = NOW() " +
		"WHERE ident = $2"
	err = DB.Exec(ctx,
		"Set recovery password",
		1, q,
		HashIt(token),
		user.GetUserName())
	if err != nil {
		return
	}

	return
}

func (user *PfUserS) SharedGroups(ctx PfCtx, otheruser PfUser) (ok bool, err error) {
	gru_me, err := user.GetGroups(ctx)
	if err != nil {
		return false, err
	}

	gru_th, err := otheruser.GetGroups(ctx)
	if err != nil {
		return false, err
	}

	for _, m := range gru_me {
		for _, t := range gru_th {
			if m.GetGroupName() == t.GetGroupName() {
				/* Check that one can be seen */
				if !m.GetGroupCanSee() && !t.GetGroupAdmin() &&
					!t.GetGroupCanSee() && !m.GetGroupAdmin() {
					continue
				}

				return true, nil
			}
		}
	}

	return false, errors.New("No shared groups")
}

func (user *PfUserS) GetImage(ctx PfCtx) (img []byte, err error) {
	if user.Image != "" {
		return base64.StdEncoding.DecodeString(user.Image)
	}

	err = errors.New("No image configured")
	return
}

func (user *PfUserS) GetHideEmail() (hide_email bool) {
	return user.Hide_email
}

func (user *PfUserS) GetKeys(ctx PfCtx, keyset map[[16]byte][]byte) (err error) {
	groups, err := user.GetGroups(ctx)
	if err != nil {
		return
	}

	for _, tu := range groups {
		if tu.GetGroupState() == "active" || tu.GetGroupState() == "soonidle" {
			err := ctx.SelectGroup(tu.GetGroupName(), PERM_GROUP_MEMBER)
			if err != nil {
				return err
			}

			grp := ctx.SelectedGroup()
			err = grp.GetKeys(ctx, keyset)
			if err != nil {
				return err
			}

		}
	}

	return
}

/* PfUser Internal */
func (user *PfUserS) Get(what string) (val string, err error) {
	q := "SELECT COALESCE(" + DB.QI(what) + ",'') " +
		"FROM member " +
		"WHERE ident = $1"
	err = DB.QueryRow(q, user.UserName).Scan(&val)

	return
}

func (user *PfUserS) GetTime(what string) (val time.Time, err error) {
	q := "SELECT " + DB.QI(what) + " " +
		"FROM member " +
		"WHERE ident = $1"
	err = DB.QueryRow(q, user.UserName).Scan(&val)

	return
}

func (user *PfUserS) SetPassword(ctx PfCtx, pwtype string, password string) (err error) {
	var pw PfPass
	var recpw string

	if password == "" {
		err = Err_NoPassword
		return
	}

	if len(password) < 8 {
		err = errors.New("Please provide a password longer than 8 characters")
		return
	}

	what := ""

	switch pwtype {
	case "portal":
		what = "password"

	case "chat":
		what = "passwd_chat"

	case "jabber":
		what = "passwd_jabber"

	default:
		err = errors.New("Unknown Password Type " + pwtype)
		return
	}

	/* Enforce Password Rules? */
	err = CheckPWRules(ctx, password)
	if err != nil {
		return
	}

	/* Hash the password */
	val, err := pw.Make(password)
	if err != nil {
		return
	}

	/* Get the email we will send a notification too */
	email, err := user.GetPriEmail(ctx, false)
	if err != nil {
		return
	}

	/* UpdateField() masks the value when logging passwords */
	_, err = DB.UpdateFieldNP(ctx, user, user.UserName, "member", what, val)
	if err != nil {
		return
	}

	/* Reset login attempts */
	_, err = DB.UpdateFieldNP(ctx, user, user.UserName, "member", "login_attempts", "0")
	if err != nil {
		return
	}

	/* Invalidate the recovery password */
	recpw, err = user.Get("recover_password")
	if recpw != "" {
		q := "UPDATE member SET " +
			"recover_password = $1, " +
			"recover_password_set_at = NOW() " +
			"WHERE ident = $2"
		err = DB.Exec(ctx,
			"Invalidated recovery password",
			1, q,
			"",
			user.UserName)
		if err != nil {
			return
		}
	}

	err = Mail_PasswordChanged(ctx, email)

	return
}

func (user *PfUserS) CheckAuth(ctx PfCtx, username string, password string, twofactor string) (err error) {
	ip := ctx.GetClientIP().String()

	/* Count attempts */
	lim := Iptrk_count(ip)
	if lim {
		err = errors.New("Too many login attempts from IP: " + ip)
		return
	}

	if password == "" {
		err = Err_NoPassword
		return
	}

	/* Using fetch() here, as we need things without selecting the user */
	err = user.fetch(ctx, username)
	if err != nil {
		return
	}

	if user.LoginAttempts > Config.LoginAttemptsMax {
		err = errors.New("Too many login attempts for this account")
		return
	}

	/* Check the password */
	err = user.Verify_Password(ctx, password)
	if err == nil {
		/* Check the TwoFactor code */
		err = user.Verify_TwoFactor(ctx, twofactor, 0)
		if err == nil {
			/* All okay -> reset login_attempts + update activity field */
			q := "UPDATE member SET login_attempts = 0, activity = NOW() WHERE ident = $1"
			e := DB.ExecNA(1, q, user.UserName)
			if e != nil {
				/* Log failed updates */
				Errf("Updating user.login_attempts/activity failed: %s", e)
			}

			/* Reset attempts from this IP */
			Iptrk_reset(ip)

			return
		}
	}

	/* Failed login attempt (either password or twofactor code is wrong) */
	e := DB.Increase(ctx,
		"Login attempt failed for user $1",
		"member",
		user.UserName,
		"login_attempts")

	/* Log failed updates */
	if e != nil {
		Errf("Updating login_attempts failed: %s", e)
	}

	return
}

/*
 * This only verifies the "portal" password
 *
 * Chat & jabber passwords are directly checked by ircd & ejabberd
 */
func (user *PfUserS) Verify_Password(ctx PfCtx, password string) (err error) {
	if password == "" {
		err = Err_NoPassword
		return
	}

	/* Get current password from DB - just refresh all data */
	err = user.Refresh(ctx)
	if err != nil {
		return
	}

	var pw PfPass
	return pw.Verify(password, user.Password)
}

func (user *PfUserS) GetSF() (sf string, err error) {
	q := "SELECT sf.type AS sft, COUNT(*) AS cnt " +
		"FROM member m " +
		"JOIN second_factors sf ON (sf.member = m.ident) " +
		"WHERE m.ident = $1 AND sf.active = 't'" +
		"GROUP BY sf.type "

	rows, err := DB.Query(q, user.UserName)
	if err != nil {
		return
	}

	defer rows.Close()

	sf = ""
	for rows.Next() {
		var sft string
		var cnt int
		err = rows.Scan(&sft, &cnt)
		if err != nil {
			return
		}

		sf += strconv.Itoa(cnt) + "x" + sft + " "
	}

	sf = strings.TrimSpace(sf)
	if sf == "" {
		sf = "none"
	}

	return
}

func user_set_xxx(ctx PfCtx, args []string) (err error) {
	/*
	 * args[.] == what, dropped by ctx.Menu()
	 * args[0] == user
	 * args[1] == val
	 */
	what := ctx.GetLastPart()
	user := ctx.SelectedUser()
	val := args[1]

	theuser := ctx.TheUser()

	uname := user.GetUserName()
	tuname := theuser.GetUserName()

	/*
	 * We can't set the ident with normal function as the
	 * audit then fails as the user has been renamed already
	 * and then the reference to the old username fails
	 *
	 * Only do this when actually changing ident
	 */
	if what == "ident" && uname != val {
		if len(val) < 3 {
			err = errors.New("Usernames have to be at least 3 characters")
			return
		}

		ctx.Dbgf("NOTE: Renaming user from %s to %s, logged-in-as: %s", uname, val, tuname)

		/* Renaming self? */
		if uname == tuname {
			/* Log out */
			ctx.Logout()
		}

		/* And deselect the user */
		ctx.SelectUser("", PERM_NONE)

		/* Audit logs won't show who renamed the user though */
	}

	if what == "recovery_email" {
		err = ctx.SelectEmail(val)
		if err != nil {
			return
		}

		email := ctx.SelectedEmail()
		if err != nil {
			return
		}

		/* Confirm Validated: */
		if email.Verified != true {
			err = errors.New("Recovery email addresses must be verified")
			return
		}
		if err != nil {
			return
		}
	}

	err = DB.UpdateFieldMsg(ctx, user, uname, "member", what, val)
	return
}

func user_sget(ctx PfCtx, args []string, fun PfFunc) (err error) {
	user := ctx.NewUser()

	/*
	 * args[0] == what
	 * args[1] == user
	 * args[2] == val
	 */

	if len(args) >= 2 {
		/* Check if we have perms for this user */
		err = ctx.SelectUser(args[1], PERM_USER_SELF)
		if err != nil {
			return
		}
		user = ctx.SelectedUser()
	} else {
		/* No user selected */
		ctx.SelectUser("", PERM_NONE)
	}

	subjects := []string{"username"}

	menu, err := StructMenu(ctx, subjects, user, false, fun)

	if err != nil {
		return
	}

	/* Menu drops args[0] (what), get it with GetLastPart() */
	err = ctx.Menu(args, menu)
	return
}

func user_set(ctx PfCtx, args []string) (err error) {
	if len(args) == 0 || args[0] == "help" {
		ctx.OutLn("Note: use 'user set help <username>' to see properties one can set for that user")
	}

	return user_sget(ctx, args, user_set_xxx)
}

func user_get(ctx PfCtx, args []string) (err error) {
	return user_sget(ctx, args, nil)
}

/* XXX If the user created a wiki page, can't delete */
func user_delete(ctx PfCtx, args []string) (err error) {
	username := args[0]

	/* Select the to be deleted user */
	err = ctx.SelectUser(username, PERM_SYS_ADMIN)
	if err != nil {
		return
	}

	user := ctx.SelectedUser()

	/* Deselect the user, won't be there anymore */
	ctx.SelectUser("", PERM_NONE)

	q := "DELETE FROM member WHERE ident = $1"

	err = DB.Exec(ctx,
		"Deleted user $1",
		1, q,
		user.GetUserName())
	return
}

func user_list(ctx PfCtx, args []string) (err error) {
	user := ctx.NewUser()
	num := 0

	users, err := user.GetList(ctx, args[0], 0, 0, false)
	if err != nil {
		return
	}

	for _, u := range users {
		var la string

		lan := u.GetLoginAttempts()

		if lan == 0 {
			la = "none"
		} else {
			la = strconv.Itoa(lan)
		}

		var sf string
		sf, err = u.GetSF()
		if err != nil {
			return
		}

		num++

		ctx.OutLn("[%s] '%s' %s LA: %s, SF: %s",
			u.GetUserName(),
			u.GetFullName(),
			u.GetUuid(),
			la,
			sf)

		groups, e := u.GetGroups(ctx)
		if err != nil {
			return e
		}

		for _, grp := range groups {
			ctx.OutLn(" [%s] <%s> %s (%s)",
				grp.GetGroupName(),
				grp.GetEmail(),
				grp.GetGroupState(),
				grp.GetEntered())
		}
	}

	if num == 0 {
		ctx.OutLn("No matching users found")
	}

	return
}

/* Called with Tx held */
func User_merge(ctx PfCtx, u_new string, u_old string, err_ error) (err error) {
	/* Error state of caller */
	err = err_

	q := ""

	if err == nil {
		q = "UPDATE member_email " +
			"SET member = $1 " +
			"WHERE member = $2"
		err = DB.Exec(ctx,
			"Update Member email $2 to $1",
			-1, q,
			u_new, u_old)
	}

	if err == nil {
		q = "UPDATE member_trustgroup " +
			"SET member = $1 " +
			"WHERE member = $2"
		err = DB.Exec(ctx,
			"Update group member $2 to $1",
			-1, q,
			u_new, u_old)
	}

	if err == nil {
		q = "UPDATE member_mailinglist " +
			"SET member = $1 " +
			"WHERE member = $2"
		err = DB.Exec(ctx,
			"Update ML for $2 to $1",
			-1, q,
			u_new, u_old)
	}

	if err == nil {
		q = "UPDATE audit_history " +
			"SET member = $1 " +
			"WHERE member = $2"
		err = DB.Exec(ctx,
			"Update audit_history $2 to $1",
			-1, q,
			u_new, u_old)
	}

	if err == nil {
		q = "DELETE FROM member " +
			"WHERE ident = $1"
		err = DB.Exec(ctx,
			"Remove member $1",
			-1, q,
			u_old)
	}

	/* Something went wrong */
	if err != nil {
		DB.TxRollback(ctx)
		return
	}

	/* Commit */
	err = DB.TxCommit(ctx)
	return
}

func user_merge(ctx PfCtx, args []string) (err error) {
	u_new := args[0]
	u_old := args[1]

	err = DB.TxBegin(ctx)
	if err != nil {
		return
	}

	return User_merge(ctx, u_new, u_old, err)
}

func (user *PfUserS) Create(ctx PfCtx, username string, email string, bio_info string, affiliation string, descr string) (err error) {
	uuid := uuid.New()

	q := "INSERT INTO member " +
		"(ident, sysadmin, uuid, bio_info, affiliation, descr) " +
		"VALUES($1, $2, $3, $4, $5, $6)"
	err = DB.Exec(ctx,
		"Added new member $1",
		1, q,
		username, false, uuid, bio_info, affiliation, descr)
	if err != nil {
		return
	}

	q = "INSERT INTO member_email " +
		"(member, email, verified) " +
		"VALUES($1, $2, 't')"

	err = DB.Exec(ctx,
		"Added email address $2 to user $1",
		1, q,
		username, email)
	if err != nil {
		return
	}

	/* Did it work? */
	err = user.fetch(ctx, username)
	if err != nil {
		return
	}

	/* Call our PostCreate hook? */
	if user.f_postcreate != nil {
		err = user.f_postcreate(ctx, user)
	}

	return
}

// User_new creates a new user.
//
// This function can be called to create a new user with the given properties.
// Further properties can be configured using the 'set' commands as exposed
// through both the CLI and UI.
//
// This function should be called only when the logged in user is allowed
// to perform such a function.
func User_new(ctx PfCtx, username string, email string, bio_info string, affiliation string, descr string) (err error) {
	user := ctx.NewUser()

	username = strings.ToLower(username)
	email = strings.ToLower(email)

	/* Existence check */
	err = user.fetch(ctx, username)
	if err == nil {
		err = errors.New("User already exists")
		return
	}

	err = user.Create(ctx, username, email, bio_info, affiliation, descr)
	if err != nil {
		/* XXX: Verify error message as it is user-visible */
		return
	}

	return
}

func user_view(ctx PfCtx, args []string) (err error) {
	username := args[0]
	err = ctx.SelectUser(username, PERM_USER_SELF)
	if err != nil {
		return
	}

	user := ctx.SelectedUser()
	last_time, last_ip := user.GetLastActivity(ctx)

	ctx.OutLn("Member: %s \n"+
		"	Full Name: %s\n"+
		"	Affiliation: %s\n"+
		"	UUID: %s\n"+
		"	Last Activity: ( %s | %s )\n"+
		"	Login Attempts: %d\n"+
		"	Groups: \n",
		user.GetUserName(),
		user.GetFullName(),
		user.GetAffiliation(),
		user.GetUuid(),
		last_time, last_ip,
		user.GetLoginAttempts())

	groups, e := user.GetGroups(ctx)
	if err != nil {
		return e
	}

	for _, grp := range groups {
		ctx.OutLn("		[%s] <%s> %s (%s)",
			grp.GetGroupName(),
			grp.GetEmail(),
			grp.GetGroupState(),
			grp.GetEntered())
	}

	return
}

// user_new creates a new user (CLI)
//
// This CLI command creates a new user in the system with
// mostly blank properties, which can be set with separate 'set'
// commands if wanted.
func user_new(ctx PfCtx, args []string) (err error) {
	username := args[0]
	email := args[1]
	bio_info := ""
	affiliation := ""
	descr := ""

	return User_new(ctx, username, email, bio_info, affiliation, descr)
}

// user_pw_set sets the password of a user (CLI).
//
// curpass is only required for non-admin users.
// curpass is the portal password irrespective of pwtype.
func user_pw_set(ctx PfCtx, args []string) (err error) {
	pwtype := args[0]
	username := args[1]
	newpass := args[2]

	err = ctx.SelectUser(username, PERM_USER_SELF)
	if err != nil {
		return
	}

	user := ctx.SelectedUser()

	/* Sysadmins don't need a password */
	if !ctx.TheUser().IsSysAdmin() {
		curpass := args[3]
		/* Check that the current password is correct */
		err = user.Verify_Password(ctx, curpass)
		if err != nil {
			err = errors.New("Invalid currrent password.")
			return
		}
	}

	/* Actually update the password */
	err = user.SetPassword(ctx, pwtype, newpass)
	if err == nil {
		ctx.OutLn("Password updated")

		/* Deselect the user */
		ctx.SelectUser("", PERM_NONE)

		/* If we're doing this as a sysadmin, don't logout */
		if !ctx.TheUser().IsSysAdmin() {
			/* Require users to re-authenticate after password changes */
			if user == ctx.TheUser() {
				ctx.Logout()
			}
		}
	}

	return
}

// user_pw_recover finished the password recovery process
// if the provided token matches (CLI).
//
// Given the username, token and a new password the user
//
// The time at which the recovery password token was set is
// checked, the token cannot be more than a week old.
//
// If the token is valid and not expired, then the new password
// is made effective.
func user_pw_recover(ctx PfCtx, args []string) (err error) {
	var recpw string
	var rectime time.Time

	username := args[0]
	token := args[1]
	new_password := args[2]

	/*
	 * We return the same error for all failures
	 * That does make determining what is wrong harder
	 * for legit users, but any adversary does not learn
	 * much either
	 *
	 * XXX: ratelimit this API call?
	 */
	failerr := errors.New("Invalid recovery details")

	err = ctx.SelectUser(username, PERM_NONE)
	if err != nil {
		ctx.Errf("Can't select user %s for recovery", username)
		return failerr
	}

	user := ctx.SelectedUser()

	recpw, err = user.Get("recover_password")
	/* Error? -> Fail */
	if err != nil {
		ctx.Errf("Failed to fetch recovery password for %s", username)
		return failerr
	}

	/* Get when the recovery password was set */
	rectime, err = user.GetTime("recover_password_set_at")
	if err != nil {
		ctx.Errf("Could not get recovery set time for user %s", username)
		return failerr
	}

	/* No recovery password? -> Fail */
	if recpw == "" {
		ctx.Errf("User %s has no recovery password", username)
		return failerr
	}

	/* Invalid recovery password? -> Fail */
	if HashIt(token) != recpw {
		ctx.Errf("Invalid recovery token for user %s", username)
		return failerr
	}

	/* Give somebody 7 days to use the recovery feature */
	timeout := time.Now().Add(7 * 24 * time.Hour)

	/* Is it expired? */
	if rectime.After(timeout) {
		/* We do report that the recovery period has timed out */
		err = errors.New("Recovery password has expired")
		return
	}

	/* Set the new password & reset login_attempts */
	err = user.SetPassword(ctx, "portal", new_password)
	if err == nil {
		ctx.OutLn("Password updated")
	}

	return
}

// user_pw_resetcount can be used to reset the login attempt counter for a given user (CLI).
//
// Each failed login attempt for a user causes the login_attempts counter for that user to increase.
// This call causes the counter to be reset.
//
// Note that the user can also be locked out on the IPtrk level.
// The combo of login_attempts and IPtrk though ensure that even
// if an adversary uses a diverse set of IP addresses, they
// only have a few attempts to try a specific account.
func user_pw_resetcount(ctx PfCtx, args []string) (err error) {
	username := args[1]

	err = ctx.SelectUser(username, PERM_USER_SELF)
	if err != nil {
		return
	}

	user := ctx.SelectedUser()
	_, err = DB.UpdateFieldNP(ctx, user, user.GetUserName(), "member", "login_attempts", "0")
	return
}

// user_pw is the CLI menu for User Password actions (CLI).
func user_pw(ctx PfCtx, args []string) (err error) {
	menu := NewPfMenu([]PfMEntry{
		{"set", user_pw_set, 3, 4, []string{"pwtype", "username", "newpassword#password", "curpassword#password"}, PERM_USER_SELF, "Set password of type (portal|chat|jabber), requires providing current portal password"},
		{"recover", user_pw_recover, 3, 3, []string{"username", "token#password", "password"}, PERM_NONE, "Set a password using the the recovery token"},
		{"resetcount", user_pw_resetcount, 1, 1, []string{"username"}, PERM_SYS_ADMIN, "Reset authentication failure count"},
	})

	err = ctx.Menu(args, menu)
	return
}

// user_menu is the CLIE menu for User actions (CLI).
func user_menu(ctx PfCtx, args []string) (err error) {
	menu := NewPfMenu([]PfMEntry{
		{"new", user_new, 2, 2, []string{"username", "email"}, PERM_SYS_ADMIN, "Create a new user"},
		{"view", user_view, 1, 1, []string{"username"}, PERM_USER_SELF, "View User Profile"},
		{"set", user_set, 0, -1, nil, PERM_USER_SELF, "Set properties of a user"},
		{"get", user_get, 0, -1, nil, PERM_USER, "Get properties of a user"},
		{"list", user_list, 1, 1, []string{"match"}, PERM_SYS_ADMIN, "List all users"},
		{"merge", user_merge, 2, 2, []string{"into#username", "from#username"}, PERM_SYS_ADMIN, "Merge a user"},
		{"delete", user_delete, 1, 1, []string{"username"}, PERM_SYS_ADMIN, "Delete a new user"},
		{"2fa", user_2fa_menu, 0, -1, nil, PERM_USER, "2FA Token Management"},
		{"email", user_email_menu, 0, -1, nil, PERM_USER, "Email commands"},
		{"password", user_pw, 0, -1, nil, PERM_NONE, "Password commands"},
		{"events", user_events, 0, -1, nil, PERM_USER, "User Events"},
		{"detail", user_detail, 0, -1, nil, PERM_USER, "Manage Contact Details"},
		{"language", user_language, 0, -1, nil, PERM_USER, "Manage Language Skills"},
	})

	return ctx.Menu(args, menu)
}
