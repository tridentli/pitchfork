/*
 * Trident Pitchfork Setup
 *
 * Setup is only meant for initial setup tasks.
 * It should be run as the 'postgres' user.
 *
 * For general use, use the CLI or the webinterface and log in.
 */

package pf_cmd_setup

import (
	"errors"
	"flag"
	"fmt"
	"os"
	pf "trident.li/pitchfork/lib"
)

func help(tname string, msg string) {
	fmt.Print("Note: " + msg + "\n" +
		"Usage: " + tname + " [<options>...] <cmd> [<arg>...]\n" +
		"\n" +
		" Options:\n" +
		"       --config <dir>\n" +
		"       --verbosedb\n" +
		"       --force-db-destroy\n" +
		"	--version\n" +
		"	--debug\n" +
		"	--help\n" +
		"\n" +
		" Command:\n" +
		"	help\n" +
		"	setup_db\n" +
		"	setup_test_db\n" +
		"	upgrade_db\n" +
		"	cleanup_db\n" +
		"	adduser <username> <password>\n" +
		"	setpassword <username> <password>\n" +
		"	sudo <username> [<cli commands>]\n" +
		"	version\n" +
		"\n" +
		"Typically to be run from the 'postgres' account\n" +
		"that has access trusted access to PostgreSQL\n" +
		"\n" +
		"The exit code will be zero when no problems are\n" +
		"encountered while non-zero (1 for simple errors,\n" +
		"others depending on the command)\n" +
		"\n")
}

func Setup(tname string, ldname string, appname string, version string, copyright string, website string, app_schema_version int, env_server string, server string, newctx pf.PfNewCtx) {
	var err error = nil
	var confroot string
	var verbosedb bool
	var force bool
	var debug bool
	var dohelp bool

	rc := 0

	pf.SetAppDetails(appname, version, copyright, website)

	flag.StringVar(&confroot, "config", "", "Configuration File Directory")
	flag.BoolVar(&verbosedb, "verbosedb", false, "Verbose DB output (Query Logging)")
	flag.BoolVar(&force, "force-db-destroy", false, "Set for setup_test_db ")
	flag.BoolVar(&debug, "debug", false, "Enable Debug output")
	flag.BoolVar(&dohelp, "help", false, "Show help")
	flag.Parse()

	pf.Debug = debug

	args := flag.Args()

	/* Should have at least one argument left */
	if len(args) < 1 {
		help(tname, "No commands given")
		return
	}

	/* Load configuration */
	err = pf.Config.Load(ldname, confroot)
	if err != nil {
		fmt.Println("Error: " + err.Error())
		return
	}

	/* Init DB */
	pf.DB_Init(verbosedb)
	pf.DB_SetAppVersion(app_schema_version)

	cmd := args[0]

	if dohelp {
		cmd = "help"
	}

	switch cmd {
	case "setup_db":
		err = pf.System_db_setup()
		break

	case "setup_test_db":
		if !force {
			fmt.Println("Error: --force-db-destroy required to force DB cleanup")
		} else {
			err = pf.System_db_cleanup()
			if err != nil {
				fmt.Println("Error: " + err.Error())
				return
			}
			err = pf.System_db_setup()
			if err != nil {
				fmt.Println("Error: " + err.Error())
				return
			}
			err = pf.System_db_test_setup()
			if err != nil {
				fmt.Println("Error: " + err.Error())
				return
			}
		}
		break

	case "upgrade_db":
		err = pf.System_db_upgrade()
		if err != nil {
			fmt.Println("Error: " + err.Error())
			return
		}
		err = pf.App_db_upgrade()
		if err != nil {
			fmt.Println("Error: " + err.Error())
			return
		}
		break

	case "cleanup_db":
		if !force {
			fmt.Println("Error: --force-db-destroy required to force DB cleanup")
		} else {
			err = pf.System_db_cleanup()
		}
		break

	case "adduser":
		if len(args) != 3 {
			err = errors.New("adduser requires 2 arguments: username + password")
		} else {
			err = pf.System_adduser(args[1], args[2])
		}
		break

	case "setpassword":
		if len(args) != 3 {
			err = errors.New("setpassword requires 2 arguments: username + password")
		} else {
			err = pf.System_setpassword(args[1], args[2])
		}
		break

	case "sudo":
		if len(args) <= 2 {
			err = errors.New("Require username + cmd's")
			break
		}

		username := args[1]
		cmd := args[2:]

		ctx := newctx()
		rc, err = sudo(ctx, env_server, server, username, cmd)
		break

	case "version":
		fmt.Print(pf.VersionText())
		break

	default:
		help(tname, "Unknown command: "+args[0])
		break
	}

	if err != nil {
		fmt.Println(tname + " Error: " + err.Error())

		/* Set a non-0 exit code when something failed */
		if rc == 0 {
			rc = 1
		}
	}

	os.Exit(rc)
}
