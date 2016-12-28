package pitchfork

import (
	"errors"
	"github.com/pborman/uuid"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PfSys struct {
	Name             string      `label:"System Name" pfset:"sysadmin" hint:"Name of the System"`
	Welcome          string      `label:"Welcome Text" pftype:"text" pfset:"sysadmin" pfcol:"welcome_text" hint:"Welcome message shown on login page"`
	AdminName        string      `label:"Name of the Administrator(s)" pfset:"sysadmin" hint:"Name of the Administrator, shown at bottom of the page"`
	AdminEmail       string      `label:"Administrator email address" pfset:"sysadmin" hint:"Email address of the Administrator, linked at the bottom of the page"`
	AdminEmailPublic bool        `label:"Show Sysadmin E-mail to non-members" pfset:"sysadmin" hint:"Show SysAdmin email address in the footer of public/not-logged-in pages"`
	CopyYears        string      `label:"Copyright Years" pfset:"sysadmin" hint:"Years that copyright ownership is claimed"`
	EmailDomain      string      `label:"Email Domain" pfset:"sysadmin" pfcol:"email_domain" hint:"The domain where emails are sourced from"`
	PublicURL        string      `label:"Public URL" pfset:"sysadmin" pfcol:"url_public" hint:"The full URL where the system is exposed to the public, used for redirects and OAuth2 (Example: https://example.net)"`
	PeopleDomain     string      `label:"People Domain" pfset:"sysadmin" pfcol:"people_domain" hint:"Domain used for people's email addresses and identifiers (Example: people.example.net)"`
	CLIEnabled       bool        `label:"CLI Enabled" pfset:"sysadmin" pfcol:"cli_enabled" hint:"Show the Web Command Line Interface to Regular users. Default: Off (Always available for Administrators)."`
	APIEnabled       bool        `label:"API Enabled" pfset:"sysadmin" pfcol:"api_enabled" hint:"Enable the API URL (/api/) thus allowing external tools to access the details provided they have authenticated. Default: On"`
	OAuthEnabled     bool        `label:"OAuth/OpenID Enabled" pfset:"sysadmin" pfcol:"oauth_enabled" hint:"Enable OAuth 2.0 and OpenID Connect support (/oauth2/ + /.wellknown/webfinger). Default: On"`
	NoIndex          bool        `label:"No Web Indexing" pfset:"sysadmin" pfcol:"no_index" hint:"Disallow Web crawlers/robots from indexing and following links. Default: On"`
	EmailSig         string      `label:"Email Signature" pftype:"text" pfset:"sysadmin" pfcol:"email_sig" hint:"Signature appended to mailinglist messages"`
	Require2FA       bool        `label:"Require 2FA" pfset:"sysadmin" hint:"Require Two Factor Authentication (2FA) for every Login, If disabled users may still configure 2FA for their account."`
	PW_comment       string      `pfsection:"Password Rules" label:"Setting password rules is not recommended. Please use XKCD style passwords instead." pftype:"note"`
	PW_Enforce       bool        `pfsection:"Password Rules" label:"Enforce Rules" hint:"When enabled the rules below are enforced on new passwords"`
	PW_Length        int         `pfsection:"Password Rules" label:"Minimal Password Length (suggested: 12, min: 8)" min:"8"`
	PW_LengthMax     int         `pfsection:"Password Rules" label:"Maximal Password Length (suggested: 1024)"`
	PW_Letters       int         `pfsection:"Password Rules" label:"Minimum amount of Letters"`
	PW_Uppers        int         `pfsection:"Password Rules" label:"Minimum amount of Uppercase characters"`
	PW_Lowers        int         `pfsection:"Password Rules" label:"Minimum amount of Lowercase characters"`
	PW_Numbers       int         `pfsection:"Password Rules" label:"Minimum amount of Numbers"`
	PW_Specials      int         `pfsection:"Password Rules" label:"Minimum amount of Special characters"`
	SARestrict       string      `label:"IP Restrict SysAdmin" pfset:"sysadmin" pfcol:"sysadmin_restrict" hint:"When provided the given CIDR prefixes, space separated, are the only ones that allow the SysAdmin bit to be enabled. The SysAdmin bit is dropped for SysAdmins coming from different prefixes. Note that 127.0.0.1 and ::1 are always included in the set, thus CLI access remains working."`
	HeaderImg        string      `label:"Header Image" pfset:"sysadmin" pfcol:"header_image" hint:"Image shown on the Welcome page"`
	LogoImg          string      `label:"Logo Image" pfset:"sysadmin" pfcol:"logo_image" hint:"Logo shown in the menu bar"`
	UnknownImg       string      `label:"Unknown Person Image" pfset:"sysadmin" pfcol:"unknown_image" hint:"Logo shown for users who do not have an image set"`
	ShowVersion      bool        `label:"Show version in UI" pfset:"sysadmin" pfcol:"showversion" hint:"Show the version in the UI, default enabled so that users can report issues with remark to the version"`
	Button           string      `label:"Update Settings" pftype:"submit"`
	sar_cache        []net.IPNet /* Cache for parsed version of SARestrict, populated by fetch() */
}

type PfAudit struct {
	Member    string
	What      string
	UserName  string
	GroupName string
	Remote    string
	Entered   time.Time
}

var Started = time.Now().UTC()

var system_cached PfSys
var system_cachedm sync.Mutex

func System_Get() (system *PfSys) {
	system_cachedm.Lock()
	defer system_cachedm.Unlock()

	/* Refresh if we have no data yet */
	if system_cached.Name == "" {
		system_cached.Refresh()
	}

	return &system_cached
}

func System_AuditMax(search string, user_name string, gr_name string) (total int, err error) {
	var args []interface{}

	q := "SELECT COUNT(*) " +
		"FROM audit_history"

	if gr_name != "" {
		DB.Q_AddWhereAnd(&q, &args, "trustgroup", gr_name)
	}

	if user_name != "" {
		DB.Q_AddWhereAnd(&q, &args, "username", user_name)
	}

	if search != "" {
		DB.Q_AddWhere(&q, &args, "member", "=", search, true, true, 0)
		DB.Q_AddWhereOrN(&q, &args, "what")
		DB.Q_AddWhereOrN(&q, &args, "username")
		DB.Q_AddWhereOrN(&q, &args, "trustgroup")
		DB.Q_AddMultiClose(&q)
	}

	err = DB.QueryRow(q, args...).Scan(&total)

	return total, err
}

func System_AuditList(search string, user_name string, gr_name string, offset int, max int) (audits []PfAudit, err error) {
	var args []interface{}
	var rows *Rows

	audits = nil

	q := "SELECT what, " +
		"COALESCE(username, ''), " +
		"COALESCE(trustgroup, ''), " +
		"COALESCE(member, ''), " +
		"entered, " +
		"remote " +
		"FROM audit_history "

	if gr_name != "" {
		DB.Q_AddWhereAnd(&q, &args, "trustgroup", &gr_name)
	}

	if user_name != "" {
		DB.Q_AddWhereAnd(&q, &args, "username", &user_name)
	}

	if search != "" {
		DB.Q_AddWhere(&q, &args, "member", "=", search, true, true, 0)
		DB.Q_AddWhereOrN(&q, &args, "what")
		DB.Q_AddWhereOrN(&q, &args, "username")
		DB.Q_AddWhereOrN(&q, &args, "trustgroup")
		DB.Q_AddMultiClose(&q)
	}

	q += "ORDER BY entered DESC"

	if max != 0 {
		q += " LIMIT "
		DB.Q_AddArg(&q, &args, max)
	}

	if offset != 0 {
		q += " OFFSET "
		DB.Q_AddArg(&q, &args, offset)
	}

	rows, err = DB.Query(q, args...)

	defer rows.Close()

	for rows.Next() {
		var au PfAudit

		err = rows.Scan(&au.What, &au.UserName, &au.GroupName,
			&au.Member, &au.Entered, &au.Remote)
		if err != nil {
			audits = nil
			return
		}

		audits = append(audits, au)
	}
	return
}

func (system *PfSys) fetch() (err error) {
	q := "SELECT key, value " +
		"FROM config"

	rows, err := DB.Query(q)
	if err != nil {
		err = errors.New("Configuration fetch failed")

		/* Some simple defaults */
		system.CLIEnabled = false
		system.APIEnabled = false
		system.LogoImg = "/gfx/logo.png"
		system.HeaderImg = "/gfx/gm.jpg"
		return
	}

	defer rows.Close()

	for rows.Next() {
		var key string
		var value string

		err = rows.Scan(&key, &value)
		if err != nil {
			return
		}

		err = StructMod(STRUCTOP_SET, system, key, value)
		if err != nil {
			Logf("Unknown system configuration variable %q, ignoring: %s", key, err.Error())
			/* Ignore these errors */
			err = nil
		}
	}

	/*
	 * Semi-Sane Defaults
	 * The sysadmin should properly configure the system
	 */
	if system.EmailDomain == "" {
		system.EmailDomain = Config.Nodename
	}

	if system.PeopleDomain == "" {
		system.PeopleDomain = "people." + Config.Nodename
	}

	if system.PublicURL == "" {
		system.PublicURL = "https://" + Config.Nodename
	}

	/* Generate a SARestrict Cache */
	if system.SARestrict == "" {
		/* No SARestrict setting configured */
		system.sar_cache = nil
	} else {
		pfxs := strings.Split(system.SARestrict, " ")

		for _, pfx := range pfxs {
			var n *net.IPNet
			_, n, err = net.ParseCIDR(pfx)
			if err != nil {
				err = errors.New("Invalid CIDR Prefix for SARestrict: " + pfx)
				return
			}

			/* Add it to the cache */
			system.sar_cache = append(system.sar_cache, *n)
		}
	}

	return nil
}

func (system *PfSys) Refresh() (err error) {
	err = system.fetch()
	return
}

/* Create a PfPWRules object */
func (system *PfSys) PWRules() (rules PfPWRules) {
	rules.Min_length = system.PW_Length
	rules.Max_length = system.PW_LengthMax
	rules.Min_letters = system.PW_Letters
	rules.Min_uppers = system.PW_Uppers
	rules.Min_lowers = system.PW_Lowers
	rules.Min_numbers = system.PW_Numbers
	rules.Min_specials = system.PW_Specials
	return
}

func CheckPWRules(ctx PfCtx, password string) (err error) {
	var pw PfPass

	sys := System_Get()

	/* Do we enforce it? */
	if !sys.PW_Enforce {
		return
	}

	rules := sys.PWRules()
	probs := pw.VerifyPWRules(password, rules)

	if len(probs) == 0 {
		return
	}

	err = errors.New("Password Problems encountered: " + strings.Join(probs, ", "))

	return
}

func system_report(ctx PfCtx, args []string) (err error) {
	var msg string
	maxdb := 10

	ctx.OutLn(VersionText())

	ctx.OutLn("Daemon started at %s", Started.String())
	ctx.OutLn("Daemon running for %s", time.Now().UTC().Sub(Started).String())
	ctx.OutLn("")

	msg, err = DB.Check()
	if err != nil {
		if msg != "" {
			msg += "\n"
		}
		msg += err.Error() + "\n"
	}

	ctx.OutLn(msg)
	ctx.OutLn("")

	ctx.OutLn("Database contents:")

	tables := make(map[string]string)
	tables["trustgroup"] = "Groups"
	tables["mailinglist"] = "Mailing Lists"
	tables["member"] = "Members"
	tables["member_email"] = "Member Emails"
	tables["second_factors"] = "Second Factors"
	tables["wiki_namespace"] = "Wiki Pages"
	tables["wiki_page_rev"] = "Wiki Page Revisions"
	keys := SortKeys(tables)

	for _, table := range keys {
		var total int

		desc := tables[table]

		q := "SELECT COUNT(*) FROM " + DB.QI(table)
		err = DB.QueryRow(q).Scan(&total)
		ctx.OutLn("  %s: %s", desc, strconv.Itoa(total))
	}

	ctx.OutLn("")

	var sizes [][]string
	sizes, err = DB.SizeReport(maxdb)
	if err != nil {
		return
	}

	ctx.OutLn("Top " + strconv.Itoa(maxdb) + " Largest Database Tables:")
	for s := range sizes {
		ctx.OutLn("  %-30s %10s", sizes[s][0], sizes[s][1])
	}
	ctx.OutLn("")
	return
}

func System_db_setup() (err error) {
	err = DB.Setup_psql()
	if err != nil {
		return
	}

	err = DB.Setup_DB()
	if err != nil {
		return
	}

	err = DB.Fix_Perms()
	return
}

func System_findfile(subdir string, name string) (fn string) {
	/* Try all the roots to find the file */
	for _, root := range Config.File_roots {
		fn = filepath.Join(root, subdir, name)
		if _, err := os.Stat(fn); err != nil {
			continue
		}

		/* Found it */
		return
	}

	fn = ""
	return
}

func System_SharedFile(thefile string) (fn string, err error) {
	err = nil

	/* Need to get it out of our SHARE directory? */
	if len(thefile) > 5 && thefile[0:6] == "SHARE:" {
		fn = System_findfile("", thefile[6:])
		if fn == "" {
			err = errors.New("Could not find Shared file: " + thefile[6:])
			return
		}

	} else {
		/* Not shared */
		fn = thefile
	}

	return
}

func System_db_test_setup() (err error) {
	err = DB.executeFile("test_data.psql")

	/* Find the Application specific test data */
	fn := System_findfile("dbschemas/", "APP_test_data.psql")
	if fn == "" {
		/* Found it, execute it */
		err = DB.executeFile(fn)
	}

	return
}

func System_db_upgrade() (err error) {
	err = DB.Upgrade()
	if err != nil {
		return
	}

	err = DB.Fix_Perms()
	return
}

func App_db_upgrade() (err error) {
	err = DB.AppUpgrade()
	if err != nil {
		return
	}

	err = DB.Fix_Perms()
	return
}

func System_db_cleanup() (err error) {
	err = DB.Cleanup_psql()
	return
}

/* setup only */
func System_adduser(username string, password string) (err error) {
	/* Hash the password */
	var pw PfPass
	pass, err := pw.Make(password)
	if err != nil {
		return
	}

	id := uuid.New()

	/* Connect to the *tool* database using the postgres account */
	err = DB.connect_pg(Config.Db_name)
	if err != nil {
		return
	}

	q := "INSERT INTO member " +
		"(ident, password, sysadmin, uuid) " +
		"VALUES($1, $2, TRUE, $3)"
	err = DB.Exec(nil,
		"Creating sysadmin user: "+username,
		1, q,
		username, pass, id)
	if err != nil {
		return
	}

	q = "INSERT INTO member_email " +
		"(member, email) " +
		"VALUES($1, $2)"
	err = DB.Exec(nil,
		"Adding sysadmin email for "+username,
		1, q,
		username, username+"@example.net")
	return
}

/* setup only */
func System_setpassword(username string, password string) (err error) {
	/* Hash the password */
	var pw PfPass
	pass, err := pw.Make(password)
	if err != nil {
		return
	}

	/* Connect to the *tool* database using the postgres account */
	err = DB.connect_pg(Config.Db_name)
	if err != nil {
		return
	}

	q := "UPDATE member " +
		"SET password = $1 " +
		"WHERE ident = $2 "
	err = DB.Exec(nil,
		"Forcing password change for user: "+username,
		1, q,
		pass, username)
	if err != nil {
		return
	}

	return
}

/* args: <username> <password> [twofactor] */
func system_login(ctx PfCtx, args []string) (err error) {
	tf := ""
	if len(args) == 3 {
		tf = args[2]
	}

	err = ctx.Login(args[0], args[1], tf)

	if err == nil {
		ctx.OutLn("Login successful")
	}

	return
}

func system_logout(ctx PfCtx, args []string) (err error) {
	ctx.Logout()
	return nil
}

func system_whoami(ctx PfCtx, args []string) (err error) {
	if ctx.IsLoggedIn() {
		theuser := ctx.TheUser()
		ctx.OutLn("Username: %s", theuser.GetUserName())
		ctx.OutLn("Fullname: %s", theuser.GetFullName())
	} else {
		ctx.OutLn("Not authenticated")
	}
	return nil
}

func system_swapadmin(ctx PfCtx, args []string) (err error) {
	if !ctx.SwapSysAdmin() {
		err = errors.New("Swapping failed")
		return
	}

	mod := "Regular"
	if ctx.IsSysAdmin() {
		mod = "SysAdmin"
	}

	ctx.OutLn("Now a %s user", mod)
	return nil
}

func system_set_xxx(ctx PfCtx, args []string) (err error) {
	var fname string
	var fval string
	var ftype string

	/*
	 * args[.] == what, removed by ctx.Menu()
	 * args[0] == val
	 */
	what := ctx.GetLastPart()
	val := args[0]

	sys := System_Get()

	ftype, fname, fval, err = StructDetails(ctx, sys, what, SD_Perms_Check|SD_Tags_Require)

	if err != nil {
		return
	}

	/* Normalize booleans */
	if ftype == "bool" {
		fval = NormalizeBoolean(fval)
		val = NormalizeBoolean(val)
	}

	if fval == val {
		ctx.OutLn("Value for %s was already set to the requested value", what)
		return
	}

	/* Everything is a string */
	q := "UPDATE config " +
		"SET value = $2 " +
		"WHERE key = $1"

	err = DB.Exec(ctx,
		"System Setting "+fname+" set to "+val,
		1, q,
		fname, val)

	if err == nil {
		ctx.OutLn("Updated %s", what)
	}

	return
}

func system_sget(ctx PfCtx, args []string, fun PfFunc) (err error) {
	subjects := []string{}

	sys := System_Get()

	menu, err := StructMenu(ctx, subjects, sys, false, fun)

	if err != nil {
		return
	}

	err = ctx.Menu(args, menu)
	return
}

func system_set(ctx PfCtx, args []string) (err error) {
	err = system_sget(ctx, args, system_set_xxx)

	/* Refresh just in case things changed */
	system_cached.Refresh()

	return
}

func system_get(ctx PfCtx, args []string) (err error) {
	return system_sget(ctx, args, nil)
}

func system_batch(ctx PfCtx, args []string) (err error) {
	na := len(args)

	if na != 1 && na != 3 && na != 4 {
		err = errors.New("Invalid number of arguments; either provide only a filename, or provide a filename along with a username, password and optional twofactor code")
		return
	}

	if na == 3 || na == 4 {
		tf := ""
		if na == 4 {
			tf = args[3]
		}

		err = ctx.Login(args[1], args[2], tf)
		if err != nil {
			return
		}

		ctx.OutLn("Changed user to %s", ctx.TheUser().GetUserName())
	}

	/* Make sure we are a sysadmin now */
	if !ctx.IsSysAdmin() {
		err = errors.New("Batch commands require sysadmin permissions")
		return
	}

	/* Execute the Batch file */
	return ctx.Batch(args[0])
}

func system_auditlog(ctx PfCtx, args []string) (err error) {
	offset := 0
	max := 0
	search := ""
	user_name := ""
	gr_name := ""

	if len(args) >= 1 {
		search = args[0]
	}

	if len(args) >= 2 {
		user_name = args[1]
	}

	if len(args) >= 3 {
		gr_name = args[2]
	}

	if len(args) >= 4 {
		offset, err = strconv.Atoi(args[3])
		if err != nil {
			return
		}
	}

	if len(args) >= 5 {
		max, err = strconv.Atoi(args[4])
		if err != nil {
			return
		}
	}

	audits, err := System_AuditList(search, user_name, gr_name, offset, max)

	if err != nil {
		return
	}

	if len(audits) == 0 {
		err = errors.New("No audit records matched")
		return
	}

	for _, a := range audits {
		ctx.Outf("Entered   : %s\n", tmp_fmt_time(a.Entered))
		ctx.Outf("  Member  : %s\n", a.Member)
		ctx.Outf("  What    : %s\n", a.What)
		ctx.Outf("  Username: %s\n", a.UserName)
		ctx.Outf("  Group   : %s\n", a.GroupName)
		ctx.Outf("  Remote  : %s\n", a.Remote)
		ctx.OutLn("")
	}

	return
}

func system_menu(ctx PfCtx, args []string) (err error) {
	menu := NewPfMenu([]PfMEntry{
		{"report", system_report, 0, 0, nil, PERM_SYS_ADMIN, "Report system statistics"},
		{"login", system_login, 2, 3, []string{"username", "password", "twofactor"}, PERM_NONE, "Login"},
		{"logout", system_logout, 0, 0, nil, PERM_NONE, "Logout"},
		{"whoami", system_whoami, 0, 0, nil, PERM_NONE, "Who Am I?"},
		{"swapadmin", system_swapadmin, 0, 0, nil, PERM_SYS_ADMIN_CAN, "Swap from regular to sysadmin user"},
		{"set", system_set, 0, -1, nil, PERM_SYS_ADMIN, "Configure the system"},
		{"get", system_get, 0, -1, nil, PERM_NONE, "Get values from the system"},
		{"batch", system_batch, 1, 4, []string{"filename", "username", "password", "twofactor"}, PERM_NONE, "Run a batch script (sysadmin level username/password required for non-sysadmin logged in users)"},
		{"iptrk", iptrk_menu, 0, -1, nil, PERM_SYS_ADMIN, "IPtrk control and information"},
		{"auditlog", system_auditlog, 1, 5, []string{"search", "username", "group", "offset#int", "max#int"}, PERM_SYS_ADMIN, "View the Audit Log"},
	})

	err = ctx.Menu(args, menu)
	return
}
