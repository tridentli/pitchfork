// Pitchfork User 2FA (Two Factor Authentication)
package pitchfork

import (
	"encoding/base32"
	"encoding/hex"
	"errors"
	purl "net/url"
	"strconv"
	"strings"
	"time"

	"trident.li/keyval"
)

// CheckTwoFactor is a setting that controls whether we verify 2FA tokens or not (--disabletwofactor).
var CheckTwoFactor = true

// PfUser2FA describes the variables for a User's 2FA configuration.
type PfUser2FA struct {
	Id       int       `label:"ID" pfset:"none" pfcol:"id"`
	UserName string    `label:"UserName" pfset:"self" pfcol:"member" pftype:"ident" hint:"Owner of the Token"`
	Name     string    `label:"Description" pfset:"self" pfcol:"descr" hint:"Helpful name of the token"`
	Type     string    `label:"Type" pfset:"self" pfcol:"type"`
	Entered  time.Time `label:"Entered" pfset:"nobody" pfget:"user"`
	Active   bool      `label:"Active" pfset:"self" pfcol:"active" hint:"Is the token active"`
	Key      string    `label:"Key" pfset:"self" pfcol:"key"`
	Counter  int       `trilanel:"Count" pfset:"self" pfcol:"counter" hint:"HOTP counter"`
}

// NewPfUser2FA creates a new PfUser2FA object.
func NewPfUser2FA() *PfUser2FA {
	return &PfUser2FA{}
}

// TwoFactorTypes lists the types of
// Two Factor Authentications that Pitchfork supports.
//
// These types are stored in the database thus allowing
// the descriptions of the various types to be updated.
func TwoFactorTypes() (types keyval.KeyVals) {
	q := "SELECT type, descr " +
		"FROM second_factor_types " +
		"ORDER BY type"
	rows, err := DB.Query(q)

	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		var t_type string
		var t_descr string

		err = rows.Scan(&t_type, &t_descr)
		if err != nil {
			types = nil
			return
		}

		types.Add(t_type, t_descr)
	}

	return
}

// fetch fetches a PfUser2FA by id.
func (tfa *PfUser2FA) fetch(id int) (err error) {
	p := []string{"id"}
	v := []string{strconv.Itoa(id)}
	err = StructFetch(tfa, "second_factors", p, v)
	if err != nil {
		tfa.Id = 0
		Log(err.Error() + " '" + strconv.Itoa(id) + "'")
	}

	return
}

// Refresh refreshes a PfUser2FA from the database.
func (tfa *PfUser2FA) Refresh() (err error) {
	return tfa.fetch(tfa.Id)
}

// Fetch2FA retrieves a 2FA for a user (Extends PfUserS object).
func (user *PfUserS) Fetch2FA() (tokens []PfUser2FA, err error) {
	q := "SELECT id, member, descr, type, entered, active, key, counter " +
		"FROM second_factors " +
		"WHERE member = $1 "
	rows, err := DB.Query(q, user.GetUserName())

	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		var tfa PfUser2FA

		err = rows.Scan(&tfa.Id, &tfa.UserName, &tfa.Name, &tfa.Type, &tfa.Entered,
			&tfa.Active, &tfa.Key, &tfa.Counter)
		if err != nil {
			tokens = nil
			return
		}

		tokens = append(tokens, tfa)
	}
	return
}

// Select selects a PfUser2FA
//
// System administrators can select any kind of 2FA object.
// All other users can only select 2fa tokens that they themself own.
func (tfa *PfUser2FA) Select(ctx PfCtx, id int, perms Perm) (err error) {
	/* Fetch it if it exists */
	err = tfa.fetch(id)
	if err != nil {
		/* Failed */
		return err
	}

	/* SysAdmins can select any */
	if ctx.IsSysAdmin() {
		return nil
	}

	/* Can select self */
	if perms.IsSet(PERM_USER_SELF) &&
		ctx.IsLoggedIn() &&
		tfa.UserName == ctx.TheUser().GetUserName() {
		return nil
	}

	return errors.New("Could not select 2FA Token")
}

// Verify_TwoFactor verifies a given twofactor code
//
// If id is set to a value other than zero this function will
// compare with that one and only one token, even if it disabled.
//
// The function checks the 2fa tokens for the user, optionally
// restricting to checking just the given id.
//
// Based on the type of the token it verifies that type's codes
// and allows further access if the code is correct.
func (user *PfUserS) Verify_TwoFactor(ctx PfCtx, twofactor string, id int) (err error) {
	var pw PfPass
	var rows *Rows
	var args []interface{}

	/* Don't check TwoFactor Authentication */
	if CheckTwoFactor == false {
		return nil
	}

	q := "SELECT id, type, counter, key " +
		"FROM second_factors"

	DB.Q_AddWhereAnd(&q, &args, "member", user.GetUserName())

	if id > 0 {
		DB.Q_AddWhereAnd(&q, &args, "id", id)
	} else {
		DB.Q_AddWhereAnd(&q, &args, "active", true)
	}

	rows, err = DB.Query(q, args...)

	defer rows.Close()

	if err != nil {
		err = errors.New("Verifying Two Factor Authentication failed: " + err.Error())
		return
	}

	has2fa := false

	for rows.Next() {
		has2fa = true

		/* When 2FA is configured, we require it */
		if twofactor == "" {
			return errors.New("2FA required, not provided")
		}

		var t_id int
		var t_type string
		var t_counter int64
		var t_key string

		err = rows.Scan(&t_id, &t_type, &t_counter, &t_key)
		if err != nil {
			return
		}

		switch t_type {
		case "HOTP":
			if pw.VerifyHOTP(t_key, t_counter, twofactor) {
				/* Correct, increase counter */
				q := "UPDATE second_factors " +
					"SET counter = counter + 1 " +
					"WHERE id = $1"
				DB.Exec(ctx,
					"Increased HOTP counter for 2FA $1",
					1, q,
					t_id)
				return nil
			}
			break

		case "TOTP":
			if pw.VerifyTOTP(t_key, twofactor) {
				/* Correct */
				return nil
			}
			break

		case "SOTP":
			if pw.VerifySOTP(t_key, twofactor) {
				/* Correct, remove Single-use OTP code */
				q := "DELETE FROM second_factors " +
					"WHERE id = $1"
				DB.Exec(ctx,
					"Used SOTP code $1 (SOTP code removed)",
					1, q, t_id)
				return nil
			}
			break

		default:
			return errors.New("Unknown Hash Type")

		}
	}

	/* If the user has no 2FA tokens, then allow them in */
	if has2fa {
		return errors.New("Invalid 2FA")
	}

	/* If 2FA set but not required, fail */
	if !has2fa && twofactor != "" {
		return errors.New("Invalid 2FA")
	}

	/* If we require 2FA */
	if System_Get().Require2FA {
		return errors.New("2FA required but not configured for this user")
	}

	return nil
}

// String converts a PfUser2FA into a human readable string.
//
// Useful for displaying an overview of the token.
func (tfa *PfUser2FA) String() (out string) {
	out = tfa.UserName + " " + strconv.Itoa(tfa.Id)
	out += " " + tfa.Name + " " + tfa.Type + "\n"
	out += "   " + tfa.Entered.String() + " " + strconv.FormatBool(tfa.Active) + "\n"
	return
}

// tfa_createKey creates a random key to be used by 2FA.
//
// It requests 10 random hex numbers and decodes that
// into hexadecimal representation.
func tfaCreateKey(length int) (out string, err error) {
	/* Generate Key */
	var pw PfPass
	pwd, err := pw.GenRandHex(10)
	if err != nil {
		return
	}

	hex, err := hex.DecodeString(pwd)
	if err != nil {
		return
	}

	out = string(hex)
	return
}

// tfaEncodeKey base32 encodes a given secret key.
func tfaEncodeKey(secret string) (out string) {
	out = base32.StdEncoding.EncodeToString([]byte(secret))
	return
}

// user_2fa_list lists the 2FA tokens for a given user (CLI).
func user_2fa_list(ctx PfCtx, args []string) (err error) {
	var tfa PfUser2FA
	var tokens []PfUser2FA

	user := ctx.SelectedUser()

	tokens, err = user.Fetch2FA()

	for _, tfa = range tokens {
		ctx.OutLn(tfa.String())
	}

	return
}

// user_2fa_add adds a 2fa token to a user (CLI)
//
// Is intended to be used for adding a new 2fa token.
// The first argument is the username, the second
// the password, which is used to verify that the
// person changing these security properties is
// at minimum controlling the password.
//
// The token_type indicates what type of token
// one wants to add.
//
// The descr is a short description field primarily
// to be able to distinguish the different tokens.
func user_2fa_add(ctx PfCtx, args []string) (err error) {
	var id int
	var secret string

	/* username := args[0] */
	pw := args[1]
	token_type := strings.ToUpper(args[2])
	descr := args[3]

	user := ctx.SelectedUser()

	/* SysAdmins can bypass the password check */
	if !ctx.IsSysAdmin() {
		err = user.Verify_Password(ctx, pw)
		if err != nil {
			return
		}
	}

	ctx.Dbg("Adding 2FA for " + user.GetUserName() + ", Type: " + token_type)

	switch token_type {
	case "TOTP", "HOTP":
		counter := 0

		secret, err = tfaCreateKey(10)
		if err != nil {
			return
		}

		key := tfaEncodeKey(secret)

		/*
		 * Create otpauth:// URL
		 *
		 * Format: https://github.com/google/google-authenticator/wiki/Key-Uri-Format
		 */
		var u purl.URL

		u.Path = "otpauth://" + strings.ToLower(token_type) + "/"
		u.Path += AppName
		u.Path += ":"
		u.Path += user.GetUserName()
		u.Path += " ("
		u.Path += descr
		u.Path += ")"

		p := purl.Values{}
		p.Add("secret", key)
		p.Add("issuer", AppName)

		/* Add counter which is required by Google Authenticator */
		if token_type == "HOTP" {
			p.Add("counter", strconv.Itoa(counter))
		}

		u.RawQuery = p.Encode()
		url := u.String()

		/* Add DB Record */
		q := "INSERT INTO second_factors " +
			"(member, type, descr, entered, active, key, counter)" +
			"VALUES($1, $2, $3, now(), 'f', $4, $5) " +
			"RETURNING id"

		err = DB.QueryRowA(ctx,
			"Add 2FA Token $2: $3",
			q,
			user.GetUserName(), token_type, descr, secret, counter).Scan(&id)
		if err != nil {
			err = errors.New("Could not add 2FA Token")
			return
		}

		ctx.OutLn("Name: %s", descr)
		ctx.OutLn("Token Type: %s", token_type)
		ctx.OutLn("URL: %s", url)

		if token_type == "HOTP" {
			ctx.OutLn("Counter: %s", counter)
		}

		break

	case "SOTP":
		var pw PfPass
		count := 5
		for count > 0 {
			secret, err = tfaCreateKey(6)
			if err != nil {
				return
			}

			count_str := strconv.Itoa(count)

			key := pw.SOTPHash(secret)

			q := "INSERT INTO second_factors " +
				"(member, type, descr, entered, active, key)" +
				"VALUES($1, $2, $3, now(), 't', $4) " +
				"RETURNING id"

			name := descr + "-" + count_str

			err = DB.QueryRowA(ctx,
				"Add 2FA Token $2: $3",
				q,
				user.GetUserName(), token_type, name, key).Scan(&id)
			if err != nil {
				err = errors.New("Could not add email address")
				return
			}

			count--

			ctx.OutLn("Name: %s", name)
			ctx.OutLn("Token Type: %s", token_type)
			ctx.OutLn("Code: %s", secret)
		}
		break

	default:
		err = errors.New("Unknown 2FA Token Type: " + token_type)
		break
	}

	return
}

// user_2fa_active_mod modifies a user's 2fa token.
//
// Given the user's password (to verify they still have
// it when changing this security related property) and
// the ID of the 2FA token.
// A sysadmin does not have to provide a valid password
// and bypasses the password check.
//
// The code is required when activating a token.
// As it proves that one can generate a proper code related
// to this token, and thus enabling it can be performed
// without a direct fear of locking the user out, in case
// they would not be able to generate the code for the token.
//
// Disabling a token can be done with solely the current
// password of the user.
func user_2fa_active_mod(ctx PfCtx, id string, curpassword string, active bool, code string) (err error) {
	var member string
	var curact bool

	user := ctx.SelectedUser()

	/* SysAdmins can bypass the password check */
	if !ctx.IsSysAdmin() {
		err = user.Verify_Password(ctx, curpassword)
		if err != nil {
			return
		}
	}

	q := "SELECT member, active " +
		"FROM second_factors " +
		"WHERE id = $1"

	err = DB.QueryRow(q, id).Scan(&member, &curact)
	if err != nil {
		err = errors.New("No such token")
		return
	}

	if !ctx.IsSysAdmin() && user.GetUserName() != member {
		ctx.Log("User " + user.GetUserName() + " attempted to access token " + id + " of user " + member)
		err = errors.New("No such token")
		return
	}

	newstate := "active"
	if !active {
		newstate = "inactive"
	}

	if curact == active {
		err = errors.New("Token " + id + " already in " + newstate + " state")
		return
	}

	if newstate == "active" {
		id_int, err := strconv.Atoi(id)
		if err != nil {
			return errors.New("ID not numeric")
		}
		if user.Verify_TwoFactor(ctx, code, id_int) != nil {
			return errors.New("Token value not correct")
		}
	}

	q = "UPDATE second_factors " +
		"SET active = $1 " +
		"WHERE member = $2 " +
		"AND id = $3"

	act := "t"
	if !active {
		act = "f"
	}

	err = DB.Exec(ctx,
		"Change 2FA Token $3 to "+newstate,
		1, q,
		act, member, id)
	if err != nil {
		err = errors.New("Could not modify 2FA token state")
		return
	}

	ctx.OutLn("State of 2FA token %s changed to %s", id, newstate)
	return
}

// user_2fa_enable enables a user's 2fa token (CLI).
//
// Given a TokenID, a valid user password and a code from running the token's function
// we can enable that token.
//
// See user_2fa_active_mod for a few more details.
func user_2fa_enable(ctx PfCtx, args []string) (err error) {
	/* username := args[0] */
	id := args[1]
	pw := args[2]
	tf := args[3]

	err = user_2fa_active_mod(ctx, id, pw, true, tf)
	return
}

// user_2fa_disable disables a user's 2fa token (CLI).
//
// Given a TokenID and the user's current password we
// can disable the token with this command.
func user_2fa_disable(ctx PfCtx, args []string) (err error) {
	/* username := args[0] */
	id := args[1]
	pw := args[2]

	err = user_2fa_active_mod(ctx, id, pw, false, "")
	return
}

// user_2fa_remove removes a user's 2fa token (CLI)
//
// A sysadmin can remove a 2fa token without password details
// but for any other user the current password is needed
// to remove the token.
func user_2fa_remove(ctx PfCtx, args []string) (err error) {
	user := ctx.SelectedUser()
	username := user.GetUserName()
	id := args[1]
	pw := args[2]

	/* SysAdmins can bypass the password check */
	if !ctx.IsSysAdmin() {
		err = user.Verify_Password(ctx, pw)
		if err != nil {
			return
		}
	}

	q := "DELETE from second_factors " +
		"WHERE member = $1 " +
		"AND id = $2 AND (active = 'f' OR type = 'SOTP')"

	err = DB.Exec(ctx,
		"Removed 2FA Token $2",
		1, q,
		username, id)
	if err != nil {
		err = errors.New("Could not remove 2FA token")
		return
	}

	ctx.OutLn("2FA Token %s removed", id)

	return
}

// user_2fa_types lists the types of 2fa tokens (CLI).
func user_2fa_types(ctx PfCtx, args []string) (err error) {
	types := TwoFactorTypes()
	for _, kv := range types {
		key := kv.Key.(string)
		val := kv.Value.(string)
		ctx.OutLn("%s %s\n", key, val)
	}
	return
}

// user_2fa_menu is the CLI menu for User 2FA details (CLI).
func user_2fa_menu(ctx PfCtx, args []string) (err error) {
	perms := PERM_USER_SELF

	menu := NewPfMenu([]PfMEntry{
		{"list", user_2fa_list, 1, 1, []string{"username"}, perms, "List tokens"},
		{"add", user_2fa_add, 4, 4, []string{"username", "curpassword#password", "type#2fatoken", "descr"}, perms, "Add tokens"},
		{"enable", user_2fa_enable, 4, 4, []string{"username", "id", "curpassword#password", "twofactorcode#int"}, perms, "Enable token"},
		{"disable", user_2fa_disable, 3, 3, []string{"username", "id", "curpassword#password"}, perms, "Disable token"},
		{"remove", user_2fa_remove, 3, 3, []string{"username", "id", "curpassword#password"}, perms, "Remove token"},
		{"types", user_2fa_types, 0, 0, nil, PERM_NONE, "List available 2FA Types"},
	})

	if len(args) >= 2 {
		/* Check if we have perms for this user */
		err = ctx.SelectUser(args[1], perms)
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
