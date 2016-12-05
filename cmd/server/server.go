/* Trident Pitchfork Server */

package pf_cmd_server

import (
	"flag"
	"fmt"
	"log"
	"log/syslog"
	"net/http"
	"strings"

	/* Pitchfork Libraries */
	pf "trident.li/pitchfork/lib"
	pu "trident.li/pitchfork/ui"
)

func Serve(dname string, appname string, version string, copyright string, website string, app_schema_version int, newui pu.PfNewUI, starthook func()) {
	var err error
	var use_syslog bool
	var disablestamps bool
	var insecurecookies bool
	var disabletwofactor bool
	var loglocation bool
	var verbosedb bool
	var confroot string
	var daemonize bool
	var pidfile string
	var username string
	var debug bool
	var showversion bool

	ldname := strings.ToLower(dname)

	pf.SetAppDetails(appname, version, copyright, website)

	flag.StringVar(&confroot, "config", "", "Configuration File Directory")
	flag.BoolVar(&use_syslog, "syslog", false, "Log to syslog")
	flag.BoolVar(&disablestamps, "disablestamps", false, "Disable timestamps in logs")
	flag.BoolVar(&insecurecookies, "insecurecookies", false, "Insecure Cookies (for testing directly against the daemon instead of going through nginx/apache")
	flag.BoolVar(&disabletwofactor, "disabletwofactor", false, "Disable Two Factor Authentication Check (development only)")
	flag.BoolVar(&loglocation, "loglocation", false, "Log Code location in log messages")
	flag.BoolVar(&verbosedb, "verbosedb", false, "Verbose DB output (Query Logging)")
	flag.BoolVar(&daemonize, "daemonize", false, "Daemonize")
	flag.StringVar(&pidfile, "pidfile", "", "PID File (useful in combo with daemonize)")
	flag.StringVar(&username, "username", "", "Change to user")
	flag.BoolVar(&debug, "debug", false, "Enable Debug output")
	flag.BoolVar(&showversion, "version", false, "Show version")

	flag.Parse()

	if showversion {
		fmt.Print(pf.VersionText())
		return
	}

	if daemonize {
		/* Part of this won't return */
		pf.Daemon(0, 0)

		/* Mandatory */
		use_syslog = true
	}

	if use_syslog {
		logwriter, e := syslog.New(syslog.LOG_NOTICE, ldname)
		if e != nil {
			fmt.Printf("Could not open syslog: %s", err.Error())
			return
		}

		/* Output to syslog */
		log.SetOutput(logwriter)

		/* Disable the timestamp, syslog takes care of that */
		disablestamps = true
	}

	/* Disable timestamps in the log? */
	if disablestamps {
		flags := log.Flags()
		flags &^= log.Ldate
		flags &^= log.Ltime
		log.SetFlags(flags)
	}

	/* Store the PID */
	pid := pf.GetPID()
	if pidfile != "" {
		pf.StorePID(pidfile, pid)
	}

	/* Drop privileges */
	if username != "" {
		err = pf.SetUID(username)
	}

	pf.CheckTwoFactor = !disabletwofactor
	pf.LogLocation = loglocation
	pf.Debug = debug

	pf.Logf("%s Daemon %s (%s) starting up", appname, dname, pf.AppVersionStr())

	/* Setup lib */
	err = pf.Setup(ldname, confroot, verbosedb, app_schema_version)
	if err != nil {
		return
	}

	/* Setup UI */
	err = pu.Setup(ldname, !insecurecookies)
	if err != nil {
		return
	}

	/* Everything goes through the UI root */
	r := pu.NewPfRootUI(newui)
	http.HandleFunc("/", r.H_root)

	/* Notify that we are ready */
	pf.Logf("%s is running on node %s", ldname, pf.Config.Nodename)

	pf.Starts()
	defer pf.Stops()

	/* Call Starthook in background goroutine when provided */
	if starthook != nil {
		go starthook()
	}

	/* Tell what HTTP port we are serving on */
	pf.Logf("%s serving on %s port %s", ldname, pf.Config.Http_host, pf.Config.Http_port)

	/* Listen and Serve the HTTP interface */
	err = http.ListenAndServe(pf.Config.Http_host+":"+pf.Config.Http_port, nil)
	if err != nil {
		log.Fatal(err)
		return
	}

	pf.Log("done")
	return
}
