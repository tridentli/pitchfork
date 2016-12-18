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

/* Whether we verify 2FA tokens or not (--disabletwofactor) */
var CheckTwoFactor = true

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

func NewPfUser2FA() *PfUser2FA {
	return &PfUser2FA{}
}

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

func (tfa *PfUser2FA) Refresh() (err error) {
	return tfa.fetch(tfa.Id)
}

/* Extends PfUserS object */
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
	if ctx.IsPermSet(perms, PERM_USER_SELF) &&
		ctx.IsLoggedIn() &&
		tfa.UserName == ctx.TheUser().GetUserName() {
		return nil
	}

	return errors.New("Could not select 2FA Token")
}

/*
 * If id is set to a value other than zero this function will
 * compare with that one and only one token, even if it disabled.
 */
func (user *PfUserS) Verify_TwoFactor(ctx PfCtx, twofactor string, id int) (err error) {
	var pw PfPass
	var rows *Rows
	var args []interface{}
	var second_stage bool
	second_stage = false
	err = nil

	/* Don't check TwoFactor Authentication */
	if CheckTwoFactor == false {
		ctx.SetLoginComplete()
		return
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
				ctx.SetLoginComplete()
				q := "UPDATE second_factors " +
					"SET counter = counter + 1 " +
					"WHERE id = $1"
				DB.Exec(ctx,
					"Increased HOTP counter for 2FA $1",
					1, q,
					t_id)
				return
			}
			break

		case "TOTP":
			if pw.VerifyTOTP(t_key, twofactor) {
				/* Correct */
				ctx.SetLoginComplete()
				return
			}
			break

		case "SOTP":
			if pw.VerifySOTP(t_key, twofactor) {
				/* Correct, remove Single-use OTP code */
				ctx.SetLoginComplete()
				q := "DELETE FROM second_factors " +
					"WHERE id = $1"
				DB.Exec(ctx,
					"Used SOTP code $1 (SOTP code removed)",
					1, q, t_id)
				return
			}
			break

		case "U2F":
			/* If we have a U2F 2FA configured, set second stage to true
			however we continue to process in case we get a first stage token */
			second_stage = true
			break

		case "DUO":
			second_stage = true
			break

		default:
			err = errors.New("Unknown Hash Type")
			return
		}
	}
	/* Successulf stage 1 2FA matches will have returned by now */

	if has2fa {
		if !second_stage {
			if twofactor == "" {
				/* When 2FA is configured, we require it */
				err = errors.New("2FA required, not provided")
			} else {
				err = errors.New("Invalid 2FA")
			}
		} else {
			/* Trigger a second stage */
			return
		}
	} else {
		if twofactor != "" {
			/* If 2FA set but not required, fail */
			err = errors.New("Invalid 2FA")
		}
		if System_Get().Require2FA {
			/* If we require 2FA */
			err = errors.New("2FA required but not configured for this user")
		}
	}
	return
}

func (tfa *PfUser2FA) String() (out string) {
	out = tfa.UserName + " " + strconv.Itoa(tfa.Id)
	out += " " + tfa.Name + " " + tfa.Type + "\n"
	out += "   " + tfa.Entered.String() + " " + strconv.FormatBool(tfa.Active) + "\n"
	return
}

func CreateKey(length int) (out string, err error) {
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

func EncodeKey(secret string) (out string) {
	out = base32.StdEncoding.EncodeToString([]byte(secret))
	return
}

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

func user_2fa_add(ctx PfCtx, args []string) (err error) {
	var id int
	var secret string

	/* username := args[0] */
	pw := args[1]
	token_type := strings.ToUpper(args[2])
	descr := args[3]

	user := ctx.SelectedUser()

	/* SysAdmins can bypass the password check */
	if !ctx.TheUser().IsSysAdmin() {
		err = user.Verify_Password(ctx, pw)
		if err != nil {
			return
		}
	}

	ctx.Dbg("Adding 2FA for " + user.GetUserName() + ", Type: " + token_type)

	switch token_type {
	case "TOTP", "HOTP":
		counter := 0

		secret, err = CreateKey(10)
		if err != nil {
			return
		}

		key := EncodeKey(secret)

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
			secret, err = CreateKey(6)
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

	case "DUO":

		break
	case "U2F":

		break
	default:
		err = errors.New("Unknown 2FA Token Type: " + token_type)
		break
	}

	return
}

func user_2fa_active_mod(ctx PfCtx, id string, curpassword string, active bool, code string) (err error) {
	var member string
	var curact bool

	user := ctx.SelectedUser()

	/* SysAdmins can bypass the password check */
	if !ctx.TheUser().IsSysAdmin() {
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

	if !ctx.TheUser().IsSysAdmin() && user.GetUserName() != member {
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

func user_2fa_enable(ctx PfCtx, args []string) (err error) {
	/* username := args[0] */
	id := args[1]
	pw := args[2]
	tf := args[3]

	err = user_2fa_active_mod(ctx, id, pw, true, tf)
	return
}

func user_2fa_disable(ctx PfCtx, args []string) (err error) {
	/* username := args[0] */
	id := args[1]
	pw := args[2]

	err = user_2fa_active_mod(ctx, id, pw, false, "")
	return
}

func user_2fa_remove(ctx PfCtx, args []string) (err error) {
	user := ctx.SelectedUser()
	username := user.GetUserName()
	id := args[1]
	pw := args[2]

	/* SysAdmins can bypass the password check */
	if !ctx.TheUser().IsSysAdmin() {
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

func user_2fa_types(ctx PfCtx, args []string) (err error) {
	types := TwoFactorTypes()
	for t_type, t_descr := range types {
		ctx.OutLn("%s %s\n", ToString(t_type), ToString(t_descr))
	}
	return
}

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
