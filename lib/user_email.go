// Pitchfork User Email Management
package pitchfork

import (
	"errors"
	"strconv"
	"strings"
	"time"

	val "github.com/asaskevich/govalidator"
	pfpgp "trident.li/pitchfork/lib/pgp"
)

// PfUserEmail describes the variables for the settings of a user's email
type PfUserEmail struct {
	Member        string          `label:"Member" pfset:"self" pfcol:"member" pftype:"ident" hint:"Owner of the Verification Code"`
	FullName      string          `label:"Full Name" pfset:"none" pftable:"member" pfcol:"descr"`
	Email         string          `label:"Email" pftype:"email" pfset:"none" pfcol:"email"`
	PgpKeyID      string          `label:"PGP Key ID" pfset:"none" pfcol:"pgpkey_id"`
	PgpKeyExpire  time.Time       `label:"PGP Key Expire" pfset:"nobody" pfget:"user" pfcol:"pgpkey_expire"`
	Keyring       string          `label:"Keyring" pftype:"file" pfset:"self" pfcol:"keyring"`
	KeyringUpdate time.Time       `label:"Keyring Updated At" pfset:"nobody" pfget:"user" pfcol:"keyring_update_at"`
	VerifyCode    string          `label:"Verification Code" pfset:"nobody" pfget:"user" pfcol:"verify_token"`
	Verified      bool            `label:"Verified" pfset:"nobody" pfget:"user" pfcol:"verified"`
	Groups        []PfGroupMember /* Used by List() */
}

// NewPfUserEmail creates a new object.
func NewPfUserEmail() *PfUserEmail {
	return &PfUserEmail{}
}

// Fetch retrieves a user's email details from the database.
func NewPfUserEmailI() interface{} {
	return &PfUserEmail{}
}

// Fetch retrieves a user's email details from the database.
func (uem *PfUserEmail) Fetch(email string) (err error) {
	if email == "" {
		err = errors.New("No email address provided")
		return
	}

	p := []string{"email"}
	v := []string{email}
	j := "INNER JOIN member ON member.ident = member_email.member"
	o := ""
	err = StructFetchA(uem, "member_email", j, p, v, o, true)
	if err != nil {
		/* Sometimes email addresses indeed do not exist */
		if err != ErrNoRows {
			Err("UserEmail::Fetch() " + err.Error() + " '" + email + "'")
		}

		err = errors.New("Email address not found")
	}

	return
}

// FetchGroups populates the Groups attribute of a UserEmail object.
func (uem *PfUserEmail) FetchGroups(ctx PfCtx) (err error) {
	var groups []PfGroupMember

	grp := ctx.NewGroup()

	/* Get the groups this user is a member of */
	groups, err = grp.GetGroups(ctx, uem.Member)
	if err != nil {
		return
	}

	for _, g := range groups {
		if uem.Email == g.GetEmail() && g.GetGroupCanSee() {
			uem.Groups = append(uem.Groups, g)
		}
	}
	return
}

// List lists the user's email details.
//
// Given a user, this returns all the email addresses, along with properties
// in the form of an array of PfUserEmail objects.
func (uem *PfUserEmail) List(ctx PfCtx, user PfUser) (emails []PfUserEmail, err error) {
	q := "SELECT member, email, descr, pgpkey_id, pgpkey_expire, keyring, " +
		"keyring_update_at, verify_token, verified " +
		"FROM member_email " +
		"INNER JOIN member ON member_email.member = member.ident " +
		"WHERE member = $1 "

	rows, err := DB.Query(q, user.GetUserName())
	if err != nil {
		err = errors.New("Could not retrieve emails for user")
		return
	}

	defer rows.Close()

	grp := ctx.NewGroup()

	for rows.Next() {
		var em PfUserEmail

		err = rows.Scan(
			&em.Member,
			&em.Email,
			&em.FullName,
			&em.PgpKeyID, &em.PgpKeyExpire,
			&em.Keyring, &em.KeyringUpdate,
			&em.VerifyCode, &em.Verified)
		if err != nil {
			emails = nil
			return
		}

		/* Get the groups this user is a member of */
		em.Groups, err = grp.GetGroups(ctx, em.Member)
		if err != nil {
			emails = nil
			return
		}

		/*
		 * Skip groups that are not matching the email address
		 * or that are not visible to the user
		 */
		removed := 0
		for g := range em.Groups {
			j := g - removed
			if em.Email != em.Groups[j].GetEmail() ||
				(!ctx.IsSysAdmin() && !em.Groups[j].GetGroupCanSee()) {
				em.Groups = em.Groups[:j+copy(em.Groups[j:], em.Groups[j+1:])]
				removed++
			}
		}

		emails = append(emails, em)
	}

	return
}

// GetPriEmail return's the user's primary email address.
//
// It can be used for sending email to the user.
//
// The first email address that is verified is considered
// the primary email address.
//
// If a recovery address is given and it is verified that
// takes precedence.
func (user *PfUserS) GetPriEmail(ctx PfCtx, recovery bool) (tue PfUserEmail, err error) {
	var emails []PfUserEmail
	var recemail string

	/* Try to select the recovery email? */
	if recovery {
		recemail, err = user.Get("recover_password")
	}

	emails, err = tue.List(ctx, user)

	if err != nil {
		return
	}

	/* Select the recovery email if it is verified */
	if recovery {
		for _, tue = range emails {
			if tue.Email == recemail && tue.Verified {
				return
			}
		}
	}

	/* Use the first verified one */
	for _, tue = range emails {
		if tue.Verified {
			return
		}
	}

	tue = *NewPfUserEmail()
	err = errors.New("No active email addresses")
	return
}

// user_email_add allows adding an email address to a user (CLI).
//
// Given a username and email address, the email address is added
// to the list of email addresses for the given user.
func user_email_add(ctx PfCtx, args []string) (err error) {
	username := args[0]
	address := strings.ToLower(args[1])

	err = ctx.SelectUser(username, PERM_USER_SELF)
	if err != nil {
		return
	}

	user := ctx.SelectedUser()
	ctx.Dbg("Adding user " + user.GetUserName() + ", email address: " + address)

	if !val.IsEmail(address) {
		err = errors.New("")
		return
	}

	var email PfUserEmail
	err = email.Fetch(address)
	if err == nil {
		err = errors.New("Email address already exists")
		return
	}

	q := "INSERT INTO member_email " +
		"(member, email) " +
		"VALUES($1, $2)"

	err = DB.Exec(ctx,
		"Added email address $2 for member $1",
		1, q,
		user.GetUserName(), address)
	if err != nil {
		err = errors.New("Could not add email address")
		return
	}

	/* Output a sentence, gets parsed by h_user_email_add() */
	ctx.OutLn("Created %s", address)
	return
}

// user_email_remove allows removing of an email address for a user (CLI).
//
// As an argument the email address is needed.
func user_email_remove(ctx PfCtx, args []string) (err error) {
	err = ctx.SelectEmail(args[0])
	if err != nil {
		return
	}

	email := ctx.SelectedEmail()

	q := "DELETE FROM member_email " +
		"WHERE member = $1 " +
		"AND email = $2"

	err = DB.Exec(ctx,
		"Removed email $2 from member $1",
		1, q,
		email.Member, email.Email)
	if err != nil {
		err = errors.New("Could not remove email address")
	} else {
		ctx.OutLn("Email successfully removed")
	}

	return
}

// user_email_list lists the email addresses for a given user (CLI).
//
// As an argument only a username is expected.
func user_email_list(ctx PfCtx, args []string) (err error) {
	var tue PfUserEmail
	var emails []PfUserEmail

	err = ctx.SelectUser(args[0], PERM_USER_SELF)
	if err != nil {
		return
	}

	user := ctx.SelectedUser()

	emails, err = tue.List(ctx, user)

	for _, tue = range emails {
		ctx.OutLn("%s %s %s", tue.Member, tue.Email, tue.PgpKeyID)
	}

	return
}

// user_group_list lists the groups a user is in along with the email
// addresses selected for those groups (CLI).
//
// As an argument the username is expected.
func user_group_list(ctx PfCtx, args []string) (err error) {
	grp := ctx.NewGroup()

	err = ctx.SelectUser(args[0], PERM_USER_SELF|PERM_GROUP_ADMIN)
	if err != nil {
		return
	}

	user := ctx.SelectedUser()
	if err != nil {
		return
	}

	grus, err := grp.GetGroups(ctx, user.GetUserName())
	if err != nil {
		return
	}

	for _, gru := range grus {
		if ctx.IsSysAdmin() || gru.GetGroupCanSee() {
			ctx.OutLn("%s %s %s %s %s Admin:%s",
				gru.GetGroupName(),
				gru.GetGroupDesc(),
				gru.GetEmail(),
				gru.GetGroupState(),
				gru.GetEntered(),
				strconv.FormatBool(gru.GetGroupAdmin()))
		}
	}

	return
}

// group_email_set configures the email address for a group (CLI).
//
// Given a username, groupname and email address, this call
// configures the system so that for email for the given
// group, the selected email address is used.
func group_email_set(ctx PfCtx, args []string) (err error) {
	username := args[0]
	gr_name := args[1]
	emailaddr := args[2]

	err = ctx.SelectUser(username, PERM_USER_SELF|PERM_GROUP_ADMIN)
	if err != nil {
		return
	}
	err = ctx.SelectGroup(gr_name, PERM_GROUP_MEMBER)
	if err != nil {
		return
	}
	err = ctx.SelectEmail(emailaddr)
	if err != nil {
		return
	}

	user := ctx.SelectedUser()
	if err != nil {
		return
	}
	grp := ctx.SelectedGroup()
	if err != nil {
		return
	}
	email := ctx.SelectedEmail()
	if err != nil {
		return
	}

	q := "UPDATE member_trustgroup " +
		"SET email = $1 " +
		"WHERE member = $2 " +
		"AND trustgroup = $3"

	err = DB.Exec(ctx,
		"Set email to $1 for identity $2 in group $3",
		1, q,
		email.Email, user.GetUserName(), grp.GetGroupName())
	if err != nil {
		err = errors.New("Could not update group email")
	} else {
		ctx.OutLn("Group email updated")
	}

	return
}

// user_email_pgp_add adds a PGP key to an email address (CLI)
//
// It takes an email address and the keyring.
//
// It extracts the keys from the given keyring.
// And fetches the details about the PGP key.
//
// Given a valid and matching key is provided,
// it replaces the keyring for that user's mail address.
func user_email_pgp_add(ctx PfCtx, args []string) (err error) {
	err = ctx.SelectEmail(args[0])
	if err != nil {
		return
	}

	email := ctx.SelectedEmail()
	keyring := args[1]

	/* Extract PGP Key ID */
	var key_id string
	var key_exp time.Time

	key_id, key_exp, err = pfpgp.GetKeyInfo(keyring, email.Email)

	if key_id == "" {
		return
	}

	ctx.Dbg("Keyring size: " + strconv.Itoa(len(keyring)))

	q := "UPDATE member_email " +
		"SET pgpkey_id = $1, " +
		"pgpkey_expire = $2, " +
		"keyring = $3, " +
		"keyring_update_at = NOW() " +
		"WHERE member = $4 " +
		"AND email = $5"

	err = DB.Exec(ctx,
		"Updated PGP Key $1 for identity $5",
		1, q,
		key_id, key_exp, keyring, email.Member, email.Email)
	if err != nil {
		err = errors.New("Could not update keyring")
	} else {
		ctx.OutLn("Key successfully added")
	}

	return
}

// user_email_pgp_get retrieves the PGP key related to the email address.
func user_email_pgp_get(ctx PfCtx, args []string) (err error) {
	err = ctx.SelectEmail(args[0])
	if err != nil {
		return
	}

	email := ctx.SelectedEmail()

	q := "SELECT keyring " +
		"FROM member_email " +
		"WHERE member = $1 " +
		"AND email = $2"

	var keyring string
	err = DB.QueryRow(q, email.Member, strings.ToLower(email.Email)).Scan(&keyring)
	if err != nil {
		err = errors.New("Could not fetch keyring")
	} else if keyring == "" {
		err = errors.New("No PGP key configured")
	} else {
		ctx.OutLn("%s", keyring)
	}

	return
}

// user_email_pgp_check verifies that the email address matches PGP key (CLI).
func user_email_pgp_check(ctx PfCtx, args []string) (err error) {
	now := time.Now()
	toexp := now.Add(time.Duration(30*24) * time.Hour)

	j := "INNER JOIN member ON member.ident = member_email.member"
	p := []string{"pgpkey_id", "pgpkey_expire", "pgpkey_expire"}
	t := []DB_Op{DB_OP_NE, DB_OP_LE, DB_OP_NE}
	v := []interface{}{"", "NOW()", toexp}
	o := "ORDER BY member.ident"
	objs, err := StructFetchMulti(NewPfUserEmailI, "member_email", j, DB_OP_AND, p, t, v, o, 0, 0)
	if err != nil {
		return
	}

	for _, o := range objs {
		uem := strings.ToLower(o.(*PfUserEmail))

		subj := "PGP Key ID " + uem.PgpKeyID + " "

		body := "Dear " + uem.FullName + "," + CRLF +
			CRLF +
			"Your PGP Key with ID " + uem.PgpKeyID + " for " + uem.Email + " "

		if uem.PgpKeyExpire.After(now) {
			/* Already expired */
			subj += "has expired"
			body += "has expired." + CRLF
		} else {
			/* Going to expire */
			subj += "close to expiration"
			body += "is about to expire." + CRLF
		}

		body += CRLF
		body += "Please update your PGP key to continue receiving encrypted messages." + CRLF

		err = Mail(ctx,
			"", "",
			uem.FullName, uem.Email,
			true,
			subj,
			body,
			true,
			"",
			true)
	}

	return
}

// user_email_confirm starts the email confirmation process by
// sending the user a new verification token.
func user_email_confirm_start(ctx PfCtx, args []string) (err error) {
	err = ctx.SelectEmail(args[0])
	if err != nil {
		return
	}

	uem := ctx.SelectedEmail()

	var pw PfPass
	var verifycode string
	verifycode, err = pw.GenPass(16)
	if err != nil {
		return
	}

	verifycodeH := HashIt(verifycode)

	q := "UPDATE member_email " +
		"SET verify_token = $1 " +
		"WHERE member = $2 " +
		"AND email = $3"

	err = DB.Exec(ctx,
		"Send Verification Code to $3",
		1, q,
		verifycodeH, uem.Member, uem.Email)
	if err != nil {
		err = errors.New("Setting verification code failed")
		return
	}

	err = Mail_VerifyEmail(ctx, uem, verifycode)
	return
}

// user_email_confirm allows a user to confirm an email
// address as theirs by providing the code they where
// sent with the verification token (CLI).
//
// This solely needs a verification code.
// The database is checked if such a verification code
// is currently attached to an email address and if it is
// it confirms that email address as verified.
func user_email_confirm(ctx PfCtx, args []string) (err error) {
	verifycode := args[0]

	ctx.Dbg("Verifycode: '" + verifycode + "'")

	verifycode = HashIt(verifycode)

	/* Invalidate token and set to verified when found */
	q := "UPDATE member_email " +
		"SET verify_token = '', " +
		"verified = 't' " +
		"WHERE verify_token = $1"

	err = DB.Exec(ctx,
		"Confirmed email address $1",
		1, q,
		verifycode)
	if err != nil {
		err = errors.New("Could not update verifycode; Verification Code invalid/already confirmed?")
	}
	return
}

// User_Email_Verify allows marking a email address as verified.
//
// It receives the username and emailaddress and updates the database
// marking the email address as verified.
//
// An error is returned if the process did not succeed.
func User_Email_Verify(ctx PfCtx, username string, emailaddr string) (err error) {
	/* Invalid token and set to verified when found */
	q := "UPDATE member_email " +
		"SET verified = 't' " +
		"WHERE member = $1 AND email = $2"

	err = DB.Exec(ctx,
		"Confirmed email address $2",
		1, q,
		username, emailaddr)
	if err != nil {
		err = errors.New("Could not verify email address")
	}
	return
}

// user_email_confirm_admin marks an email address as verified without
// the verification code (CLI).
//
// It takes the username and email address to confirm as verified
// and verifies it.
func user_email_confirm_admin(ctx PfCtx, args []string) (err error) {
	username := args[0]
	emailaddr := args[1]

	return User_Email_Verify(ctx, username, emailaddr)
}

// user_email_menu is the CLI menu for User Email actions (CLI).
func user_email_menu(ctx PfCtx, args []string) (err error) {
	menu := NewPfMenu([]PfMEntry{
		{"add", user_email_add, 2, 2, []string{"username", "email"}, PERM_USER, "Add email address"},
		{"remove", user_email_remove, 1, 1, []string{"email"}, PERM_USER, "Remove email address"},
		{"confirm_begin", user_email_confirm_start, 1, 1, []string{"email"}, PERM_USER, "Send an e-mail confirmation token."},
		{"confirm", user_email_confirm, 1, 1, []string{"verifycode"}, PERM_USER, "Confirm email address"},
		{"confirm_admin", user_email_confirm_admin, 2, 2, []string{"username", "email"}, PERM_SYS_ADMIN, "force an email verification"},
		{"list", user_email_list, 1, 1, []string{"username"}, PERM_USER, "List email addresses"},
		{"pgp_add", user_email_pgp_add, 2, 2, []string{"email", "keyring#file"}, PERM_USER, "Add PGP Key"},
		{"pgp_get", user_email_pgp_get, 1, 1, []string{"email"}, PERM_USER, "Get PGP Key"},
		{"pgp_check", user_email_pgp_check, 0, 0, nil, PERM_SYS_ADMIN, "Check all PGP Keys"},
		{"member", user_email_group_menu, 0, -1, nil, PERM_USER, "Member commands"},
	})

	err = ctx.Menu(args, menu)
	return
}

// user_email_group_menu is the CLI menu for User Group Email actions (CLI).
func user_email_group_menu(ctx PfCtx, args []string) (err error) {
	menu := NewPfMenu([]PfMEntry{
		{"list", user_group_list, 1, 1, []string{"username"}, PERM_USER, "List trust group associated email addresses"},
		{"set", group_email_set, 3, 3, []string{"username", "group", "email"}, PERM_USER, "Select email address to be associated with a trust group"},
	})

	err = ctx.Menu(args, menu)
	return
}
