package pitchfork

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"os"
	"strings"
)

type PfConfig struct {
	Conf_root       string       ``                  /* From command line option or default setting */
	File_roots      []string     `json:"file_roots"` /* Where we look for files */
	Var_root        string       `json:"var_root"`   /* Where variable files are stored */
	Tmp_roots       []string     `json:"tmp_roots"`  /* Templates */
	LogFile         string       `json:"logfile"`    /* Where to write our log file (with logrotate support) */
	Token_prv       interface{}  ``
	Token_pub       interface{}  ``
	UserAgent       string       `json:"useragent"`
	CSS             []string     `json:"css"`
	Javascript      []string     `json:"javascript"`
	CSP             string       `json:"csp"`
	XFF             []string     `json:"xff_trusted_cidr"`
	XFFc            []*net.IPNet ``
	Db_host         string       `json:"db_host"`
	Db_port         string       `json:"db_port"`
	Db_name         string       `json:"db_name"`
	Db_user         string       `json:"db_user"`
	Db_pass         string       `json:"db_pass"`
	Db_ssl_mode     string       `json:"db_ssl_mode"`
	Db_admin_db     string       `json:"db_admin_db"`
	Db_admin_user   string       `json:"db_admin_user"`
	Db_admin_pass   string       `json:"db_admin_pass"`
	Nodename        string       `json:"nodename"`
	Http_host       string       `json:"http_host"`
	Http_port       string       `json:"http_port"`
	JWT_prv         string       `json:"jwt_key_prv"`
	JWT_pub         string       `json:"jwt_key_pub"`
	Application     interface{}  `json:"application"`
	Username_regexp string       `json:"username_regexp"`
	UserHomeLinks   bool         `json:"user_home_links"`
	SMTP_host       string       `json:"smtp_host"`
	SMTP_port       string       `json:"smtp_port"`
	SMTP_SSL        string       `json:"smtp_ssl"`
	Msg_mon_from    string       `json:"msg_monitor_from"`
	Msg_mon_to      string       `json:"msg_monitor_to"`
	TimeFormat      string       `json:"timeformat"`
	DateFormat      string       `json:"dateformat"`
	PW_WeakDicts    []string     `json:"pw_weakdicts"`
}

/* SMTP_SSL = ignore | require */

var Config PfConfig

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
		Config.Http_port = "8334"
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

	if Config.Db_name == "" {
		Config.Db_name = toolname
	}

	if Config.Db_user == "" {
		Config.Db_user = toolname
	}

	if Config.JWT_prv == "" {
		Config.JWT_prv = "jwt.prv"
	}

	if Config.JWT_pub == "" {
		Config.JWT_pub = "jwt.pub"
	}

	if len(Config.CSS) == 0 {
		Config.CSS = []string{"style", "form"}
	}

	if Config.CSP == "" {
		Config.CSP = "default-src 'self'; img-src 'self' data:"
	}

	if Config.Username_regexp == "" {
		Config.Username_regexp = "^[a-z][a-z0-9]*$"
	}

	if Config.SMTP_host == "" || Config.SMTP_port == "" || Config.SMTP_SSL == "" {
		err = errors.New("Please configure the SMTP parameters (smtp_host, smtp_port, smtp_ssl)")
		return
	}

	if Config.SMTP_SSL != "require" && Config.SMTP_SSL != "ignore" {
		err = errors.New("Configuration variable 'smtp_ssl' is not set to 'require' or 'ignore' but '" + Config.SMTP_SSL + "'")
		return
	}

	if Config.TimeFormat == "" {
		Config.TimeFormat = "2006-01-02 15:04"
	}

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

	err = cfg.Token_LoadPrv()
	if err != nil {
		return
	}

	err = cfg.Token_LoadPub()
	if err != nil {
		return
	}

	return
}
