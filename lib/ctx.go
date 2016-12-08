package pitchfork

import (
	"errors"
	"fmt"
	useragent "github.com/mssola/user_agent"
	"math"
	"net"
	"strconv"
	"strings"
)

var ErrLoginIncorrect = errors.New("Login incorrect")

type PfNewUserI func() (user PfUser)
type PfNewGroupI func() (user PfGroup)
type PfMenuI func(ctx PfCtx, menu *PfMenu)
type PfAppPermsI func(ctx PfCtx, what string, perms Perm) (final bool, ok bool, err error)
type PfPostBecomeI func(ctx PfCtx)

type PfCreds struct {
	sel_user  PfUser  /* Selected User */
	sel_group PfGroup /* Selected Group */
}

type PfModOptsI interface {
	IsModOpts() bool
}

type PfModOptsS struct {
	/* CLI command prefix, eg 'group wiki' */
	Cmdpfx string

	/* URL prefix, typically System_Get().PublicURL() */
	URLpfx string

	/* Path Root */
	Pathroot string

	/* URL root, inside the hostname, eg '/group/name/wiki/' */
	URLroot string
}

func (m PfModOptsS) IsModOpts() bool {
	return true
}

func PfModOpts(ctx PfCtx, cmdpfx string, path_root string, web_root string) PfModOptsS {
	urlpfx := System_Get().PublicURL

	web_root = URL_EnsureSlash(web_root)

	return PfModOptsS{cmdpfx, urlpfx, path_root, web_root}
}

/* Context Interface, allowing it to be extended */
type PfCtx interface {
	GetAbort() <-chan bool
	SetAbort(abort <-chan bool)
	StoreCreds() (creds PfCreds)
	RestoreCreds(creds PfCreds)
	SetTx(tx *Tx)
	GetTx() (tx *Tx)
	Err(message string)
	Errf(format string, a ...interface{})
	Log(message string)
	Logf(format string, a ...interface{})
	Dbg(message string)
	Dbgf(format string, a ...interface{})
	Init() (err error)
	SetStatus(code int)
	GetStatus() (code int)
	SetReturnCode(rc int)
	GetReturnCode() (rc int)
	GetLoc() string
	GetLastPart() string
	Become(user PfUser)
	GetToken() (tok string)
	NewToken() (err error)
	LoginToken(tok string) (expsoon bool, err error)
	Login(username string, password string, twofactor string) (err error)
	Logout()
	IsLoggedIn() bool
	IsGroupMember() bool
	IAmGroupAdmin() bool
	IAmGroupMember() bool
	GroupHasWiki() bool
	GroupHasFile() bool
	GroupHasCalendar() bool
	CanBeSysAdmin() bool
	SwapSysAdmin() bool
	IsSysAdmin() bool
	ConvertPerms(str string) (perm Perm, err error)
	IsPerm(perms Perm, perm Perm) bool
	IsPermSet(perms Perm, perm Perm) bool
	CheckPerms(what string, perms Perm) (ok bool, err error)
	CheckPermsT(what string, permstr string) (ok bool, err error)
	TheUser() (user PfUser)
	SelectedSelf() bool
	SelectedUser() (user PfUser)
	SelectedGroup() (grp PfGroup)
	SelectedML() (ml PfML)
	SelectedEmail() (email PfUserEmail)
	SelectedUser2FA() (tfa PfUser2FA)
	HasSelectedUser() bool
	HasSelectedGroup() bool
	HasSelectedML() bool
	SelectMe()
	SelectUser(username string, perms Perm) (err error)
	SelectUser2FA(id int, perms Perm) (err error)
	SelectGroupA(grp PfGroup, gr_name string, perms Perm) (err error)
	SelectGroup(gr_name string, perms Perm) (err error)
	SelectML(ml_name string, perms Perm) (err error)
	SelectEmail(email string) (err error)
	SetModOpts(opts PfModOptsI)
	GetModOpts() (opts interface{})
	PDbgf(what string, perm Perm, format string, a ...interface{})
	Out(txt string)
	Outf(format string, a ...interface{})
	OutLn(format string, a ...interface{})
	SetOutUnbuffered(obj interface{}, fun string)
	OutBuffered(on bool)
	IsBuffered() bool
	Buffered() (o string)
	GetRemote() (remote string)
	SetClient(clientip net.IP, remote string, ua string)
	GetClientIP() net.IP
	GetUserAgent() (string, string, string)
	SelectObject(obj *interface{})
	SelectedObject() (obj *interface{})
	GetLanguage() string

	NewUser() (user PfUser)
	NewUserI() (i interface{})
	NewGroup() (user PfGroup)
	NewGroupI() (i interface{})

	/* Menu Overrides */
	MenuOverride(menu *PfMenu)

	/* menu.go */
	Menu(args []string, menu PfMenu) (err error)
	WalkMenu(args []string) (menu *PfMEntry, err error)
	Cmd(args []string) (err error)
	CmdOut(cmd string, args []string) (msg string, err error)
	Batch(filename string) (err error)

	/* appdata */
	SetAppData(data interface{})
	GetAppData() interface{}
}

type SessionClaims struct {
	JWTClaims
	UserDesc   string `json:"userdesc"`
	IsSysAdmin bool   `json:"issysadmin"`
}

type PfCtxS struct {
	abort          <-chan bool   /* Abort the request */
	status         int           /* HTTP Status code */
	returncode     int           /* Command Line return code */
	loc            string        /* Command tree location */
	output         string        /* Output buffer */
	mode_buffered  bool          /* Buffering of output in effect */
	user           PfUser        /* Authenticated User */
	token          string        /* The authentication token */
	token_claims   SessionClaims /* Parsed Token Claims */
	remote         string        /* The address of the client, including X-Forwarded-For */
	client_ip      net.IP        /* Client's IP addresses */
	ua_full        string        /* The HTTP User Agent */
	ua_browser     string        /* HTTP User Agent: Browser */
	ua_os          string        /* HTTP User Agent: Operating System */
	language       string        /* User's chosen language (TODO: Allow user to select it) */
	sel_user       PfUser        /* Selected User */
	sel_user_2fa   *PfUser2FA    /* Selected User 2FA */
	sel_group      PfGroup       /* Selected Group */
	sel_ml         *PfML         /* Selected Mailing List */
	sel_email      *PfUserEmail  /* Selected User email address */
	sel_obj        *interface{}  /* Selected Object (ctx + struct only) */
	mod_opts       interface{}   /* Module Options for Messages/Wiki/Files etc */
	f_newuser      PfNewUserI    /* Create a new User */
	f_newgroup     PfNewGroupI   /* Create a new Group */
	f_menuoverride PfMenuI       /* Override a menu */
	f_appperms     PfAppPermsI   /* Application Permission Check */
	f_postbecome   PfPostBecomeI /* Post Become() */

	/* Unbuffered Output */
	outunbuf_fun string   /* Function name that handles unbuffered output */
	outunbuf_obj ObjFuncI /* Object where the function lives */

	/* Database internal */
	db_Tx *Tx

	/* Menu internal values */
	menu_walkonly bool
	menu_args     []string
	menu_menu     *PfMEntry

	/* Application Data */
	appdata interface{}
}

type PfNewCtx func() PfCtx

func NewPfCtx(newuser PfNewUserI, newgroup PfNewGroupI, menuoverride PfMenuI, appperms PfAppPermsI, postbecome PfPostBecomeI) PfCtx {
	if newuser == nil {
		newuser = NewPfUserA
	}

	if newgroup == nil {
		newgroup = NewPfGroup
	}

	return &PfCtxS{f_newuser: newuser, f_newgroup: newgroup, f_menuoverride: menuoverride, f_appperms: appperms, f_postbecome: postbecome, language: "en-US", mode_buffered: true}
}

func (ctx *PfCtxS) GetAbort() <-chan bool {
	return ctx.abort
}

func (ctx *PfCtxS) SetAbort(abort <-chan bool) {
	ctx.abort = abort
}

func (ctx *PfCtxS) GetLanguage() string {
	return ctx.language
}

func (ctx *PfCtxS) SetAppData(appdata interface{}) {
	ctx.appdata = appdata
}

func (ctx *PfCtxS) GetAppData() interface{} {
	return ctx.appdata
}

func (ctx *PfCtxS) StoreCreds() (creds PfCreds) {
	creds.sel_user = ctx.sel_user
	creds.sel_group = ctx.sel_group
	return
}

func (ctx *PfCtxS) RestoreCreds(creds PfCreds) {
	ctx.sel_user = creds.sel_user
	ctx.sel_group = creds.sel_group
}

func (ctx *PfCtxS) NewUser() PfUser {
	return ctx.f_newuser()
}

func (ctx *PfCtxS) NewUserI() interface{} {
	return ctx.f_newuser()
}

func (ctx *PfCtxS) NewGroup() PfGroup {
	return ctx.f_newgroup()
}

func (ctx *PfCtxS) NewGroupI() interface{} {
	return ctx.f_newgroup()
}

func (ctx *PfCtxS) MenuOverride(menu *PfMenu) {
	if ctx.f_menuoverride != nil {
		ctx.f_menuoverride(ctx, menu)
	}
}

func (ctx *PfCtxS) SetTx(tx *Tx) {
	ctx.db_Tx = tx
}

func (ctx *PfCtxS) GetTx() (tx *Tx) {
	return ctx.db_Tx
}

func (ctx *PfCtxS) GetRemote() (remote string) {
	return ctx.remote
}

func (ctx *PfCtxS) SetClient(clientip net.IP, remote string, fullua string) {
	ctx.client_ip = clientip
	ctx.remote = remote

	/* Split the UA in several parts */
	ua := useragent.New(fullua)
	ctx.ua_full = fullua
	if ua != nil {
		ctx.ua_browser, _ = ua.Browser()
		ctx.ua_os = ua.OS()
	} else {
		/* Did not parse as it is the CLI */
		if clientip.IsLoopback() {
			ctx.ua_browser = "Tickly"
			ctx.ua_os = "Trident"
		} else {
			ctx.ua_browser = "unknown"
			ctx.ua_os = "unknown"
		}
	}
}

func (ctx *PfCtxS) GetClientIP() net.IP {
	return ctx.client_ip
}

func (ctx *PfCtxS) GetUserAgent() (string, string, string) {
	return ctx.ua_full, ctx.ua_browser, ctx.ua_os
}

func (ctx *PfCtxS) SelectObject(obj *interface{}) {
	ctx.sel_obj = obj
}

func (ctx *PfCtxS) SelectedObject() (obj *interface{}) {
	return ctx.sel_obj
}

func (ctx *PfCtxS) SetModOpts(opts PfModOptsI) {
	ctx.mod_opts = opts
}

func (ctx *PfCtxS) GetModOpts() (opts interface{}) {
	return ctx.mod_opts
}

type Perm uint64

/*
 * Note: Keep in sync with permnames && ui/ui (convienence for all the menus there)
 *
 * It is used as a bitfield, hence multiple perms are possible
 * Check access using the CheckPerms() function
 *
 * The perms use the sel_{user|group|ml} vars to compare against
 *
 * Note: Being sysadmin overrides almost all permissions!
 *
 *       Change the 'false' in PDbgf to 'true' to see what permission
 *       decisions are being made.
 */

const (
	PERM_NOTHING        Perm = 0         /* Nothing / empty permissions */
	PERM_NONE           Perm = 1 << iota /* No access bits needed (unauthenticated) */
	PERM_GUEST                           /* Not authenticated */
	PERM_USER                            /* User (authenticated) */
	PERM_USER_SELF                       /* User when they selected themselves */
	PERM_USER_NOMINATE                   /* User when doing nomination */
	PERM_USER_VIEW                       /* User when just trying to view */
	PERM_GROUP_MEMBER                    /* Member of the group */
	PERM_GROUP_ADMIN                     /* Admin of the group */
	PERM_GROUP_WIKI                      /* Group has Wiki section enabled */
	PERM_GROUP_FILE                      /* Group has File section enabled */
	PERM_GROUP_CALENDAR                  /* Group has Calendar section enabled */
	PERM_SYS_ADMIN                       /* System Administrator */
	PERM_SYS_ADMIN_CAN                   /* Can be a System Administrator */
	PERM_CLI                             /* When CLI is enabled */
	PERM_API                             /* When API is enabled */
	PERM_OAUTH                           /* When OAUTH is enabled */
	PERM_LOOPBACK                        /* Connected from Loopback */
	PERM_HIDDEN                          /* Option is hidden */
	PERM_NOCRUMB                         /* Don't add a crumb for this menu */
	PERM_NOSUBS                          /* No sub menus for this menu entry */
	PERM_NOBODY                          /* Nobody has access */

	/* Application permissions */
	PERM_APP_0
	PERM_APP_1
	PERM_APP_2
	PERM_APP_3
	PERM_APP_4
	PERM_APP_5
	PERM_APP_6
	PERM_APP_7
	PERM_APP_8
	PERM_APP_9
)

var permnames []string

/* String init */
func init() {
	permnames = []string{
		"nothing",
		"none",
		"guest",
		"user",
		"self",
		"user_nominate",
		"user_view",
		"group_member",
		"group_admin",
		"group_wiki",
		"group_file",
		"group_calendar",
		"sysadmin",
		"sysadmin_can",
		"cli",
		"api",
		"oauth",
		"loopback",
		"hidden",
		"nocrumb",
		"nosubs",
		"nobody",
		"app_0",
		"app_1",
		"app_2",
		"app_3",
		"app_4",
		"app_5",
		"app_6",
		"app_7",
		"app_9",
	}

	max := uint64(1 << uint64(len(permnames)))
	if max != uint64(PERM_APP_9) {
		fmt.Printf("Expected %d got %d\n", max, PERM_APP_9)
		panic("Invalid permnames")
	}
}

const (
	StatusOK           = 200
	StatusUnauthorized = 401
)

var Debug = false

/* Constructor */
func (ctx *PfCtxS) Init() (err error) {
	/* Default HTTP status */
	ctx.status = StatusOK

	/* Default Shell Return Code to 0 */
	ctx.returncode = 0

	return err
}

func (ctx *PfCtxS) SetStatus(code int) {
	ctx.status = code
}

func (ctx *PfCtxS) GetStatus() (code int) {
	return ctx.status
}

func (ctx *PfCtxS) SetReturnCode(rc int) {
	ctx.returncode = rc
}

func (ctx *PfCtxS) GetReturnCode() (rc int) {
	return ctx.returncode
}

func (ctx *PfCtxS) GetLoc() string {
	return ctx.loc
}

func (ctx *PfCtxS) GetLastPart() string {
	fa := strings.Split(ctx.loc, " ")
	return fa[len(fa)-1]
}

func (ctx *PfCtxS) Become(user PfUser) {
	/* Use the details from the user */
	ctx.user = user

	/* Select one-self */
	ctx.sel_user = user

	/* Post Become() hook? */
	if ctx.f_postbecome != nil {
		ctx.f_postbecome(ctx)
	}
}

func (ctx *PfCtxS) GetToken() (tok string) {
	return ctx.token
}

func (ctx *PfCtxS) NewToken() (err error) {
	if !ctx.IsLoggedIn() {
		return errors.New("Not authenticated")
	}

	theuser := ctx.TheUser()

	/* Set some claims */
	ctx.token_claims.UserDesc = theuser.GetFullName()
	ctx.token_claims.IsSysAdmin = theuser.IsSysAdmin()

	username := theuser.GetUserName()

	/* Create the token */
	token := Token_New("websession", username, TOKEN_EXPIRATIONMINUTES, &ctx.token_claims)

	/* Sign and get the complete encoded token as a string */
	ctx.token, err = token.Sign()
	if err != nil {
		/* Invalid token when something went wrong */
		ctx.token = ""
	}

	return
}

func (ctx *PfCtxS) LoginToken(tok string) (expsoon bool, err error) {
	/* No valid token */
	ctx.token = ""

	/* Parse the provided token */
	expsoon, err = Token_Parse(tok, "websession", &ctx.token_claims)
	if err != nil {
		return expsoon, err
	}

	/* Who they claim they are */
	user := ctx.NewUser()
	user.SetUserName(ctx.token_claims.Subject)
	user.SetFullName(ctx.token_claims.UserDesc)
	user.SetSysAdmin(ctx.token_claims.IsSysAdmin)

	/* Fetch the details */
	err = user.Refresh(ctx)
	if err == ErrNoRows {
		ctx.Dbgf("No such user %q", ctx.token_claims.Subject)
		return false, errors.New("No such user")
	} else if err != nil {
		ctx.Dbgf("Fetch of user %q failed: %s", ctx.token_claims.Subject, err.Error())
		return false, err
	}

	/* Looking good, become the user */
	ctx.Become(user)

	/* Valid Token */
	ctx.token = tok

	return expsoon, nil
}

func (ctx *PfCtxS) Login(username string, password string, twofactor string) (err error) {
	user := ctx.NewUser()

	err = user.CheckAuth(ctx, username, password, twofactor)
	if err != nil {
		/* Log the error, so that it can be looked up in the log */
		ctx.Errf("CheckAuth(%s): %s", username, err)

		/* Overwrite the error so that we do not leak too much detail */
		err = ErrLoginIncorrect
		return
	}

	/* Force generation of a new token */
	ctx.token = ""

	ctx.Become(user)

	userevent(ctx, "login")
	return nil
}

func (ctx *PfCtxS) Logout() {
	if ctx.token != "" {
		Jwt_invalidate(ctx.token, &ctx.token_claims)
	}

	/* Invalidate user + token */
	ctx.user = nil
	ctx.token = ""
	ctx.token_claims = SessionClaims{}
}

func (ctx *PfCtxS) IsLoggedIn() bool {
	if ctx.user == nil {
		return false
	}

	return true
}

func (ctx *PfCtxS) IsGroupMember() bool {
	if !ctx.HasSelectedUser() {
		return false
	}

	if !ctx.HasSelectedGroup() {
		return false
	}

	ismember, _, state, err := ctx.sel_group.IsMember(ctx.user.GetUserName())
	if err != nil {
		ctx.Log("IsGroupMember: " + err.Error())
		return false
	}

	if !ismember {
		return false
	}

	/* Group Admins can always select users, even when blocked */
	if ctx.IAmGroupAdmin() {
		return true
	}

	/* Normal group users, it depends on wether they can see them */
	return state.can_see
}

func (ctx *PfCtxS) IAmGroupAdmin() bool {
	if !ctx.IsLoggedIn() {
		return false
	}

	if !ctx.HasSelectedGroup() {
		return false
	}

	if ctx.IsSysAdmin() {
		return true
	}

	_, isadmin, _, err := ctx.sel_group.IsMember(ctx.user.GetUserName())
	if err != nil {
		return false
	}
	return isadmin
}

func (ctx *PfCtxS) IAmGroupMember() bool {
	if !ctx.IsLoggedIn() {
		return false
	}

	if !ctx.HasSelectedGroup() {
		return false
	}

	ismember, _, _, err := ctx.sel_group.IsMember(ctx.user.GetUserName())
	if err != nil {
		return false
	}
	return ismember
}

func (ctx *PfCtxS) GroupHasWiki() bool {
	if !ctx.HasSelectedGroup() {
		return false
	}

	return ctx.sel_group.HasWiki()
}

func (ctx *PfCtxS) GroupHasFile() bool {
	if !ctx.HasSelectedGroup() {
		return false
	}

	return ctx.sel_group.HasFile()
}

func (ctx *PfCtxS) GroupHasCalendar() bool {
	if !ctx.HasSelectedGroup() {
		return false
	}

	return ctx.sel_group.HasCalendar()
}

func (ctx *PfCtxS) CanBeSysAdmin() bool {
	if !ctx.IsLoggedIn() {
		return false
	}

	/* Can we be or not? */
	if !ctx.user.CanBeSysAdmin() {
		return false
	}

	/* Could be, if the user wanted */
	return true
}

func (ctx *PfCtxS) SwapSysAdmin() bool {
	/* Not logged, can't be SysAdmin */
	if !ctx.IsLoggedIn() {
		return false
	}

	/* If they cannot be one, then do not toggle either */
	if !ctx.user.CanBeSysAdmin() {
		return false
	}

	/* Toggle state: SysAdmin <> Regular */
	ctx.user.SetSysAdmin(!ctx.user.IsSysAdmin())

	/* Force generation of a new token */
	ctx.token = ""

	return true
}

func (ctx *PfCtxS) IsSysAdmin() bool {
	if !ctx.IsLoggedIn() {
		return false
	}

	/* Not a SysAdmin, easy */
	if !ctx.user.IsSysAdmin() {
		return false
	}

	sys := System_Get()

	/*
	 * SysAdmin IP Restriction in effect?
	 *
	 * Loopback (127.0.0.1 / ::1) are excluded from this restriction
	 */
	if sys.sar_cache == nil || ctx.client_ip.IsLoopback() {
		return true
	}

	/* Check all the prefixes */
	for _, n := range sys.sar_cache {
		if n.Contains(ctx.client_ip) {
			/* It is valid */
			return true
		}
	}

	/* Not in the SARestrict list */
	return false
}

func (ctx *PfCtxS) ConvertPerms(str string) (perm Perm, err error) {
	str = strings.ToLower(str)

	perm = PERM_NOTHING

	p := strings.Split(str, ",")
	for _, pm := range p {
		if pm == "" {
			continue
		}

		found := false
		var i uint
		i = 0
		for _, n := range permnames {
			if pm == n {
				perm += 1 << i
				found = true
				break
			}
			i++
		}

		if !found {
			return PERM_NOTHING, errors.New("Unknown permission: '" + pm + "'")
		}
		break
	}

	return perm, nil
}

func (perm Perm) String() (str string) {

	for i := 0; i < len(permnames); i++ {
		p := uint64(math.Pow(float64(2), float64(i)))

		if uint64(perm)&p == 0 {
			continue
		}

		if str != "" {
			str += ","
		}

		str += permnames[i]
	}

	return str
}

func (ctx *PfCtxS) IsPerm(perms Perm, perm Perm) bool {
	return perms == perm
}

func (ctx *PfCtxS) IsPermSet(perms Perm, perm Perm) bool {
	return perms&perm > 0
}

/*
 * Multiple permissions can be specified
 * thus test from least to most to see
 * if any of them allows access
 */
func (ctx *PfCtxS) CheckPerms(what string, perms Perm) (ok bool, err error) {
	/* No error yet */
	sys := System_Get()

	ctx.PDbgf(what, perms, "Text: %s", perms.String())

	if ctx.IsLoggedIn() {
		ctx.PDbgf(what, perms, "user = %s", ctx.user.GetUserName())
	} else {
		ctx.PDbgf(what, perms, "user = ::NONE::")
	}

	if ctx.HasSelectedUser() {
		ctx.PDbgf(what, perms, "sel_user = %s", ctx.sel_user.GetUserName())
	} else {
		ctx.PDbgf(what, perms, "sel_user = ::NONE::")
	}

	if ctx.HasSelectedGroup() {
		ctx.PDbgf(what, perms, "sel_group = %s", ctx.sel_group.GetGroupName())
	} else {
		ctx.PDbgf(what, perms, "sel_group = ::NONE::")
	}

	/* Nobody? */
	if ctx.IsPermSet(perms, PERM_NOBODY) {
		ctx.PDbgf(what, perms, "Nobody")
		return false, errors.New("Nobody is allowed")
	}

	if ctx.IsPerm(perms, PERM_NOBODY) {
		panic("EHMM")
	}

	/* No permissions? */
	if ctx.IsPerm(perms, PERM_NOTHING) {
		ctx.PDbgf(what, perms, "Nothing")
		return true, nil
	}

	/* CLI when enabled and user is authenticated */
	if ctx.IsPermSet(perms, PERM_CLI) {
		ctx.PDbgf(what, perms, "CLI")
		if ctx.IsLoggedIn() && sys.CLIEnabled {
			ctx.PDbgf(what, perms, "CLI - Enabled")
			return true, nil
		} else {
			err = errors.New("CLI is not enabled")
		}
	}

	/* Loopback calls can always access the API (for tcli) */
	if ctx.IsPermSet(perms, PERM_API) {
		ctx.PDbgf(what, perms, "API")
		if sys.APIEnabled {
			ctx.PDbgf(what, perms, "API - Enabled")
			return true, nil
		} else {
			err = errors.New("API is not enabled")
		}
	}

	/* Is OAuth enabled? */
	if ctx.IsPermSet(perms, PERM_OAUTH) {
		ctx.PDbgf(what, perms, "OAuth")
		if sys.OAuthEnabled {
			ctx.PDbgf(what, perms, "OAuth - Enabled")
			return true, nil
		} else {
			err = errors.New("OAuth is not enabled")
		}
	}

	/* Loopback? */
	if ctx.IsPermSet(perms, PERM_LOOPBACK) {
		ctx.PDbgf(what, perms, "Loopback")
		if ctx.client_ip.IsLoopback() {
			ctx.PDbgf(what, perms, "Is Loopback")
			return true, nil
		} else {
			err = errors.New("Not a Loopback")
		}
	}

	/* User must not be authenticated */
	if ctx.IsPermSet(perms, PERM_GUEST) {
		ctx.PDbgf(what, perms, "Guest")
		if !ctx.IsLoggedIn() {
			ctx.PDbgf(what, perms, "Guest - Not Logged In")
			return true, nil
		}

		ctx.PDbgf(what, perms, "Guest - Logged In")
		return false, errors.New("Must not be authenticated")
	}

	/* User has to have selected themselves */
	if ctx.IsPermSet(perms, PERM_USER_SELF) {
		ctx.PDbgf(what, perms, "User Self")
		if ctx.IsLoggedIn() {
			ctx.PDbgf(what, perms, "User Self - Logged In")
			if ctx.HasSelectedUser() {
				ctx.PDbgf(what, perms, "User Self - Has selected user")
				if ctx.sel_user.GetUserName() == ctx.user.GetUserName() {
					/* Passed the test */
					ctx.PDbgf(what, perms, "User Self - It is me")
					return true, nil
				} else {
					ctx.PDbgf(what, perms, "User Self - Other user")
					err = errors.New("Different user selected")
				}
			} else {
				err = errors.New("No user selected")
			}
		} else {
			err = errors.New("Not Authenticated")
		}
	}

	/* User has to have selected themselves */
	if ctx.IsPermSet(perms, PERM_USER_VIEW) {
		ctx.PDbgf(what, perms, "User View")
		if ctx.IsLoggedIn() {
			ctx.PDbgf(what, perms, "User View - Logged In")
			if ctx.HasSelectedUser() {
				ctx.PDbgf(what, perms, "User View - Has selected user")
				if ctx.sel_user.GetUserName() == ctx.user.GetUserName() {
					/* Passed the test */
					ctx.PDbgf(what, perms, "User View - It is me")
					return true, nil
				} else {
					ok, err = ctx.sel_user.SharedGroups(ctx, ctx.user)
					if ok {
						/* Passed the test */
						ctx.PDbgf(what, perms, "User View - It is in my group")
						return true, nil
					} else {
						ctx.PDbgf(what, perms, "User View - Other user")
						err = errors.New("Different user selected")
					}
				}
			} else {
				err = errors.New("No user selected")
			}
		} else {
			err = errors.New("Not Authenticated")
		}
	}

	/* User has to be a group member + Wiki enabled */
	if ctx.IsPermSet(perms, PERM_GROUP_WIKI) {
		ctx.PDbgf(what, perms, "Group Wiki?")
		if ctx.GroupHasWiki() {
			ctx.PDbgf(what, perms, "HasWiki - ok")
			if ctx.IsGroupMember() {
				ctx.PDbgf(what, perms, "Group member - ok")
				return true, nil
			}
			err = errors.New("Not a group member")
		} else {
			err = errors.New("Group does not have a Wiki")
			return false, err
		}
	}

	/* User has to be a group member + File enabled */
	if ctx.IsPermSet(perms, PERM_GROUP_FILE) {
		ctx.PDbgf(what, perms, "Group File?")
		if ctx.GroupHasFile() {
			ctx.PDbgf(what, perms, "HasFile - ok")
			if ctx.IsGroupMember() {
				ctx.PDbgf(what, perms, "Group member - ok")
				return true, nil
			}
			err = errors.New("Not a group member")
		} else {
			err = errors.New("Group does not have a File")
			return false, err
		}
	}

	/* User has to be a group member + Calendar enabled */
	if ctx.IsPermSet(perms, PERM_GROUP_CALENDAR) {
		ctx.PDbgf(what, perms, "Group Calendar?")
		if ctx.GroupHasCalendar() {
			ctx.PDbgf(what, perms, "HasCalendar - ok")
			if ctx.IsGroupMember() {
				ctx.PDbgf(what, perms, "Group member - ok")
				return true, nil
			}
			err = errors.New("Not a group member")
		} else {
			err = errors.New("Group does not have a Calendar")
			return false, err
		}
	}

	/* No permissions needed */
	if ctx.IsPermSet(perms, PERM_NONE) {
		ctx.PDbgf(what, perms, "None")
		/* Always succeeds */
		return true, nil
	}

	/* Everything else requires a login */
	if !ctx.IsLoggedIn() {
		ctx.PDbgf(what, perms, "Not Authenticated")
		err = errors.New("Not authenticated")
		return false, err
	}

	/*
	 * SysAdmin can get away with almost anything
	 *
	 * The perms only has the PERM_SYS_ADMIN bit set for clarity
	 * that that one only has access for sysadmins
	 */
	if ctx.IsSysAdmin() {
		ctx.PDbgf(what, perms, "SysAdmin?")
		return true, nil
	}
	err = errors.New("Not a SysAdmin")

	/* User has to be authenticated */
	if ctx.IsPermSet(perms, PERM_USER) {
		ctx.PDbgf(what, perms, "User?")
		if ctx.IsLoggedIn() {
			ctx.PDbgf(what, perms, "User - Logged In")
			return true, nil
		}

		err = errors.New("Not Authenticated")
	}

	/* User has to be a group admin */
	if ctx.IsPermSet(perms, PERM_GROUP_ADMIN) {
		ctx.PDbgf(what, perms, "Group admin?")
		if ctx.IAmGroupAdmin() {
			ctx.PDbgf(what, perms, "Group admin - ok")
			return true, nil
		}

		err = errors.New("Not a group admin")
	}

	/* User has to be a group member */
	if ctx.IsPermSet(perms, PERM_GROUP_MEMBER) {
		ctx.PDbgf(what, perms, "Group member?")
		if ctx.IsGroupMember() {
			ctx.PDbgf(what, perms, "Group member - ok")
			return true, nil
		}

		err = errors.New("Not a group member")
	}

	/* User wants to nominate somebody (even themselves) */
	if ctx.IsPermSet(perms, PERM_USER_NOMINATE) {
		ctx.PDbgf(what, perms, "User Nominate")
		if ctx.IsLoggedIn() {
			ctx.PDbgf(what, perms, "User Nominate - Logged In")
			if ctx.HasSelectedUser() {
				ctx.PDbgf(what, perms, "User Nominate - User Selected")
				/* Passed the test */
				return true, nil
			} else {
				err = errors.New("No user selected")
			}
		} else {
			err = errors.New("Not Authenticated")
		}
	}

	/* Can the user become a SysAdmin? */
	if ctx.IsPermSet(perms, PERM_SYS_ADMIN_CAN) {
		if ctx.IsLoggedIn() {
			ctx.PDbgf(what, perms, "Sys Admin Can - Logged In")
			if ctx.CanBeSysAdmin() {
				ctx.PDbgf(what, perms, "Sys Admin Can")
				/* Passed the test */
				return true, nil
			} else {
				err = errors.New("Can't become SysAdmin")
			}
		} else {
			err = errors.New("Not Authenticated")
		}
	}

	/* Let the App Check permissions */
	if ctx.f_appperms != nil {
		final, _ok, _err := ctx.f_appperms(ctx, what, perms)
		if final {
			return _ok, _err
		}

		/* Otherwise we ignore the result as it is not a final decision */
	}

	if err == nil {
		/* Should not happen */
		panic("Invalid permission bits")
	}

	/* Default Deny + report error */
	return false, err
}

func (ctx *PfCtxS) CheckPermsT(what string, permstr string) (ok bool, err error) {
	perms, err := ctx.ConvertPerms(permstr)
	if err != nil {
		return
	}

	return ctx.CheckPerms(what, perms)
}

func (ctx *PfCtxS) TheUser() (user PfUser) {
	/* Return a copy, not a reference */
	return ctx.user
}

func (ctx *PfCtxS) SelectedSelf() bool {
	return ctx.IsLoggedIn() &&
		ctx.HasSelectedUser() &&
		ctx.user.GetUserName() == ctx.sel_user.GetUserName()
}

func (ctx *PfCtxS) SelectedUser() (user PfUser) {
	/* Return a copy, not a reference */
	return ctx.sel_user
}

func (ctx *PfCtxS) SelectedGroup() (grp PfGroup) {
	/* Return a copy, not a reference */
	return ctx.sel_group
}

func (ctx *PfCtxS) SelectedML() (ml PfML) {
	/* Return a copy, not a reference */
	return *ctx.sel_ml
}

func (ctx *PfCtxS) SelectedEmail() (email PfUserEmail) {
	/* Return a copy, not a reference */
	return *ctx.sel_email
}

func (ctx *PfCtxS) SelectedUser2FA() (tfa PfUser2FA) {
	/* Return a copy, not a reference */
	return *ctx.sel_user_2fa
}

func (ctx *PfCtxS) HasSelectedUser() bool {
	return ctx.sel_user != nil
}

func (ctx *PfCtxS) HasSelectedGroup() bool {
	return ctx.sel_group != nil
}

func (ctx *PfCtxS) HasSelectedML() bool {
	return ctx.sel_ml != nil
}

func (ctx *PfCtxS) SelectMe() {
	ctx.sel_user = ctx.user
}

/* This creates a PfUser */
func (ctx *PfCtxS) SelectUser(username string, perms Perm) (err error) {
	ctx.PDbgf("PfCtxS::SelectUser", perms, "%q", username)

	/* Nothing to select, always works */
	if username == "" {
		ctx.sel_user = nil
		return nil
	}

	/* Selecting own user? */
	theuser := ctx.TheUser()
	if theuser != nil && theuser.GetUserName() == username {
		/* Re-use and pass no username to indicate no refresh */
		ctx.sel_user = theuser
		username = ""
	} else {
		ctx.sel_user = ctx.NewUser()
	}

	err = ctx.sel_user.Select(ctx, username, perms)
	if err != nil {
		ctx.sel_user = nil
	}

	return
}

func (ctx *PfCtxS) SelectUser2FA(id int, perms Perm) (err error) {
	ctx.PDbgf("SelectUser2FA", perms, "%d", id)

	/* Nothing to select, always works */
	if id == 0 {
		ctx.sel_user_2fa = nil
		return nil
	}

	/* No user selected, no 2FA selected */
	if !ctx.HasSelectedUser() {
		ctx.sel_user_2fa = nil
		return nil
	}

	ctx.sel_user_2fa = NewPfUser2FA()
	err = ctx.sel_user_2fa.Select(ctx, id, perms)
	if err != nil {
		ctx.sel_user_2fa = nil
	}

	return
}

/* Unless SysAdmin one cannot select a group one is not a member of */
func (ctx *PfCtxS) SelectGroupA(grp PfGroup, gr_name string, perms Perm) (err error) {
	ctx.PDbgf("SelectGroupA", perms, "%q", gr_name)

	/* Nothing to select */
	if gr_name == "" {
		ctx.sel_group = nil
		return nil
	}

	ctx.sel_group = grp
	err = ctx.sel_group.Select(ctx, gr_name, perms)
	if err != nil {
		ctx.sel_group = nil
	}

	return
}

func (ctx *PfCtxS) SelectGroup(gr_name string, perms Perm) (err error) {
	return ctx.SelectGroupA(ctx.NewGroup(), gr_name, perms)
}

func (ctx *PfCtxS) SelectML(ml_name string, perms Perm) (err error) {
	ctx.PDbgf("SelectUserML", perms, "%q", ml_name)

	if !ctx.HasSelectedGroup() {
		return errors.New("No group selected")
	}

	/* Nothing to select */
	if ml_name == "" {
		ctx.sel_ml = nil
		return nil
	}

	ctx.sel_ml = NewPfML()
	err = ctx.sel_ml.Select(ctx, ctx.sel_group, ml_name, perms)

	if err != nil {
		ctx.sel_ml = nil
	}

	return
}

func (ctx *PfCtxS) SelectEmail(email string) (err error) {
	perms := PERM_USER_SELF

	ctx.PDbgf("SelectEmail", perms, "%q", email)

	/* Nothing to select */
	if email == "" {
		ctx.sel_email = nil
		return nil
	}

	/* Fetch email details */
	ctx.sel_email = NewPfUserEmail()
	err = ctx.sel_email.Fetch(email)
	if err != nil {
		/* Did not work */
		ctx.sel_email = nil
		return
	}

	/* Check Permissions */
	var ok bool
	ok, _ = ctx.CheckPerms("SelectEmail", perms)
	if !ok {
		/* Nope, no access */
		ctx.sel_email = nil
	}

	return
}

func (ctx *PfCtxS) Err(message string) {
	ErrA(1, message)
}

func (ctx *PfCtxS) Errf(format string, a ...interface{}) {
	ErrA(1, format, a...)
}

func (ctx *PfCtxS) Log(message string) {
	LogA(1, message)
}

func (ctx *PfCtxS) Logf(format string, a ...interface{}) {
	LogA(1, format, a...)
}

func (ctx *PfCtxS) Dbg(message string) {
	DbgA(1, message)
}

func (ctx *PfCtxS) Dbgf(format string, a ...interface{}) {
	DbgA(1, format, a...)
}

func (ctx *PfCtxS) PDbgf(what string, perm Perm, format string, a ...interface{}) {
	/*
	 * Code level Debug option
	 * Change the 'false' to 'true' and every permission decision will be listed
	 * Remember: sysadmin overrules most permissions, thus test with normal user
	 */
	if false {
		ctx.Dbgf("Perms(\""+what+"\"/"+strconv.Itoa(int(perm))+"): "+format, a...)
	}
}

func (ctx *PfCtxS) Out(txt string) {
	if !ctx.mode_buffered {
		/* Call the function that takes care of Direct output */
		_, err := ObjFunc(ctx.outunbuf_obj, ctx.outunbuf_fun, txt)
		if err != nil {
			ctx.Errf("Unbuffered output failed: %s", err.Error())
		}
	} else {
		/* Buffered output */
		ctx.output += txt
	}
}

func (ctx *PfCtxS) Outf(format string, a ...interface{}) {
	ctx.Out(fmt.Sprintf(format, a...))
}

func (ctx *PfCtxS) OutLn(format string, a ...interface{}) {
	ctx.Outf(format+"\n", a...)
}

func (ctx *PfCtxS) SetOutUnbuffered(obj interface{}, fun string) {
	objtrail := []interface{}{obj}
	ok, obji := ObjHasFunc(objtrail, fun)
	if !ok {
		panic("Unbuffered function " + fun + " is missing")
	}

	ctx.outunbuf_obj = obji
	ctx.outunbuf_fun = fun
}

func (ctx *PfCtxS) OutBuffered(on bool) {
	if !on && ctx.outunbuf_fun == "" {
		panic("Can't enable buffered mode without unbuffered function")
	}

	ctx.mode_buffered = on
}

func (ctx *PfCtxS) IsBuffered() bool {
	return ctx.mode_buffered
}

func (ctx *PfCtxS) Buffered() (o string) {
	o = ctx.output
	ctx.output = ""
	return
}
