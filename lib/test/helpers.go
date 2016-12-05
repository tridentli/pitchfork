package pftest

/*
 * Support for setting up tests
 *
 * TODO: requires active database connection and valid config for now...
 */

import (
	"flag"
	"os"
	pf "trident.li/pitchfork/lib"
)

func Test_setup() (toolname string) {
	var debug bool

	/* Required for testing to be active */
	toolname = os.Getenv("PITCHFORK_TOOLNAME")
	confroot := os.Getenv("PITCHFORK_CONFROOT")

	if toolname == "" || confroot == "" {
		pf.Errf("Refusing to test: PITCHFORK_TOOLNAME and PITCHFORK_CONFROOT are not configured")

		/* Note: we exit(0) here, thus 'go test' won't complain as all looks okay */
		os.Exit(0)
	}

	/* Extra flags */
	flag.BoolVar(&debug, "debug", false, "Enable Debug output")

	/* Parse the flags */
	flag.Parse()

	/* Enable Pitchfork Debugging when Testing is verbose */
	pf.Debug = debug

	err := pf.Setup(toolname, confroot, debug, 1)
	if err != nil {
		pf.Errf("Failed to setup server: %s", err.Error())
		os.Exit(1)
	}

	return
}
