// Pitchfork cfg is used for all configuration elements loaded from the .conf file
package pitchfork

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"os"
	"strings"
)

// PfConfig contains the configuration details for the system, as loaded from the configuration file
type PfConfig struct {
	Conf_root       string       ``                             /* From command line option or default setting */
	File_roots      []string     `json:"file_roots"`            /* Where we look for files */
	Var_root        string       `json:"var_root"`              /* Where variable files are stored */
	Tmp_roots       []string     `json:"tmp_roots"`             /* Templates */
	LogFile         string       `json:"logfile"`               /* Where to write our log file (with logrotate support) */
	Token_prv       interface{}  ``                             // Private portion of the JWT Token
	Token_pub       interface{}  ``                             // Public portion of the JWT Token
	UserAgent       string       `json:"useragent"`             // The HTTP and SMTP/Email user agent to use when contacting other servers
	CSS             []string     `json:"css"`                   // The CSS files to load (HTML meta header)
	Javascript      []string     `json:"javascript"`            // The javascript libraries to load (HTML meta header)
	CSP             string       `json:"csp"`                   // The Content-Security-Protection HTTP header we include in our output
	XFF             []string     `json:"xff_trusted_cidr"`      // The CIDR prefixes that are trusted X-Forwarded-For networks
	XFFc            []*net.IPNet ``                             // Cached parsed version of X-Forward-For configuration
	Db_host         string       `json:"db_host"`               // The database hostname
	Db_port         string       `json:"db_port"`               // The database port
	Db_name         string       `json:"db_name"`               // The database name
	Db_user         string       `json:"db_user"`               // The database user
	Db_pass         string       `json:"db_pass"`               // The database password
	Db_ssl_mode     string       `json:"db_ssl_mode"`           // The database SSL mode (require|ignore)
	Db_admin_db     string       `json:"db_admin_db"`           // The database name used for administrative actions
	Db_admin_user   string       `json:"db_admin_user"`         // The database user used for administrative actions
	Db_admin_pass   string       `json:"db_admin_pass"`         // The database password used for administrative actions
	Nodename        string       `json:"nodename"`              // Name of this node (typically matches the hostname and automatically set by program)
	Http_host       string       `json:"http_host"`             // The Host on which we serve HTTP
	Http_port       string       `json:"http_port"`             // The port on which we serve HTTP
	JWT_prv         string       `json:"jwt_key_prv"`           // Private portion of the JWT Token
	JWT_pub         string       `json:"jwt_key_pub"`           // Public portion of the JWT Token
	Application     interface{}  `json:"application"`           // Application specific configuration see GetAppConfig() / GetAppConfigBool()
	Username_regexp string       `json:"username_regexp"`       // Regular expression for filtering/rejecting usernames
	UserHomeLinks   bool         `json:"user_home_links"`       // If User Home Links are active
	SMTP_host       string       `json:"smtp_host"`             // SMTP Host to use for outbound emails
	SMTP_port       string       `json:"smtp_port"`             // SMTP Port to use for outbound emails
	SMTP_SSL        string       `json:"smtp_ssl"`              // Whether to require SSL for outbound emails (ignore|require)
	Msg_mon_from    string       `json:"msg_monitor_from"`      // Email address used for From: for monitoring messages (messages module)
	Msg_mon_to      string       `json:"msg_monitor_to"`        // Email address used for To: for monitoring messages (messages module)
	TimeFormat      string       `json:"timeformat"`            // Time Format
	DateFormat      string       `json:"dateformat"`            // Date Format
	PW_WeakDicts    []string     `json:"pw_weakdicts"`          // List of filenames containing password dictionaries
	CFG_UserMinLen  string       `json:"username_min_length"`   // Minimum Username length
	CFG_UserExample string       `json:"username_example"`      // Username Example
	TransDefault    string       `json:"translation_default"`   // Translation - Default Language
	TransLanguages  []string     `json:"translation_languages"` // Translation - Available Languages
}

/* SMTP_SSL = ignore | require */

var Config PfConfig

// GetAppConfig gets an application configuration variable (string).
//
// Application configuration values are stored in the 'application' section
// of the application's configuration.
//
// returns a string.
func (cfg *PfConfig) GetAppConfig(varname string) (out string) {
	out = ""

	a, ok := cfg.Application.(map[string]interface{})
	if !ok {
		Errf("No string application configuration variable '%s' found", varname)
		return
	}

	out, ok = a[varname].(string)
	if !ok {
		Errf("Application configuration variable '%s' is not a string", varname)
		return
	}

	return
}

// GetAppConfig gets an application configuration variable (boolean).
//
// Application configuration values are stored in the 'application' section
// of the application's configuration.
//
// returns a boolean.
func (cfg *PfConfig) GetAppConfigBool(varname string) (out bool) {
	out = false

	a, ok := cfg.Application.(map[string]interface{})
	if !ok {
		Errf("No boolean application configuration variable '%s' found", varname)
		return
	}

	out, ok = a[varname].(bool)
	if !ok {
		Errf("Application configuration variable '%s' is not a bool", varname)
		return
	}

	return
}

// Load loads the application configuration for the given
// toolname and from the optionally provided configroot.
//
// The configuration file is a JSON file, interjected with
// comment lines indicated as such as they start with a has ('#') symbol.
//
// Before running the file through the JSON parser we strip these
// comment lines, thus allowing it to be parsed.
//
// The PfConfig structure contains all possible entries.
//
// See doc/conf/example.conf for an example configuration file.
//
// The variables in the 'application' section do not have direct
// accessors in PfConfig, but can be retrieved using the GetAppConfig()
// and GetAppConfigBool() functions.
func (cfg *PfConfig) Load(toolname string, confroot string) (err error) {
	wd, err := os.Getwd()
	if err != nil {
		Errf("Could not determine working directory: %s", err.Error())
		return
	}
	Dbgf("Running from: %s", wd)

	if confroot == "" {
		confroot = "/etc/" + toolname + "/"
	}

	/* Defaults */
	Config.Conf_root = confroot
	Config.UserHomeLinks = true

	/* Open the configuration file */
	fn := Config.Conf_root + toolname + ".conf"

	file, err := os.Open(fn)
	if err != nil {
		err = errors.New("Failed to open configuration file " + fn + ": " + err.Error())
		return
	}
	defer file.Close()

	txt := ""

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		s := strings.TrimSpace(scanner.Text())
		if len(s) == 0 || s[0] != '#' {
			txt += s
		}
	}

	err = scanner.Err()
	if err != nil {
		err = errors.New("Configuration file " + fn + " read error: " + err.Error())
		return
	}

	err = json.Unmarshal([]byte(txt), &cfg)
	if err != nil {
		err = errors.New("Configuration file " + fn + " JSON error: " + err.Error())
		return
	}

	/* When not specified use the system configured name */
	if Config.Nodename == "" {
		Config.Nodename, err = os.Hostname()
		if err != nil {
			return
		}
	}

	/*
	 * Default to IPv4 loopback only
	 * As we live behind nginx most of the time
	 * This option is thus mostly for testing purposes
	 * or if a front-end loadbalancer is listening on
	 * another host. Adjust XFF settings too then.
	 */
	if Config.Http_host == "" {
		Config.Http_host = "127.0.0.1"
	}

	if Config.Http_port == "" {
		Config.Http_port = "8333"
	}

	/* Default User Agent */
	if Config.UserAgent == "" {
		Config.UserAgent = "Trident/Pitchfork (https://trident.li)"
	}

	if Config.Var_root == "" {
		err = errors.New("Missing var_root, please define in " + fn)
		return
	}

	if len(Config.File_roots) == 0 {
		err = errors.New("Missing file_roots, require at least one file root")
		return
	}

	// Default to the toolname
	if Config.Db_name == "" {
		Config.Db_name = toolname
	}

	// Default to the toolname
	if Config.Db_user == "" {
		Config.Db_user = toolname
	}

	// The default name for the JWT private key file
	if Config.JWT_prv == "" {
		Config.JWT_prv = "jwt.prv"
	}

	// The default name for the JWT public key file
	if Config.JWT_pub == "" {
		Config.JWT_pub = "jwt.pub"
	}

	// Minimal CSS configuration if none configured
	if len(Config.CSS) == 0 {
		Config.CSS = []string{"style", "form"}
	}

	// Minimal CSP configuration if none configured
	if Config.CSP == "" {
		Config.CSP = "default-src 'self'; img-src 'self' data:"
	}

	// Default the regular expression for usernames
	if Config.Username_regexp == "" {
		Config.Username_regexp = "^[a-z][a-z0-9]*$"
	}

	// Ensure that SMTP parameters are configured
	if Config.SMTP_host == "" || Config.SMTP_port == "" || Config.SMTP_SSL == "" {
		err = errors.New("Please configure the SMTP parameters (smtp_host, smtp_port, smtp_ssl)")
		return
	}

	// Make sure the SMTP_SSL option is either require or ignore
	if Config.SMTP_SSL != "require" && Config.SMTP_SSL != "ignore" {
		err = errors.New("Configuration variable 'smtp_ssl' is not set to 'require' or 'ignore' but '" + Config.SMTP_SSL + "'")
		return
	}

	// Default Time format (yyyy-mm-dd HH:MM)
	if Config.TimeFormat == "" {
		Config.TimeFormat = "2006-01-02 15:04"
	}

	// Default Date format (yyyy-mm-dd)
	if Config.DateFormat == "" {
		Config.DateFormat = "2006-01-02"
	}

	/* Check that the configuration is sane */
	for _, x := range Config.XFF {
		var xc *net.IPNet

		_, xc, err = net.ParseCIDR(x)
		if err != nil {
			err = errors.New("Trusted XFF IP " + x + " is invalid: " + err.Error())
			return
		}

		/* Add it to the pre-parsed list */
		Config.XFFc = append(Config.XFFc, xc)
	}

	// Load the private key for JWT Tokens
	err = cfg.Token_LoadPrv()
	if err != nil {
		return
	}

	// Load the public key for JWT Tokens
	err = cfg.Token_LoadPub()
	if err != nil {
		return
	}

	// Ensure that a default language is configured
	if cfg.TransDefault == "" {
		cfg.TransDefault = "en-US"
	}

	// Ensure that the Translanguages setting has content too
	if len(cfg.TransLanguages) == 0 {
		cfg.TransLanguages = []string{"en-US.json"}
	}

	return
}
