package pitchfork_test

/*
 * Support for setting up tests
 *
 * TODO: requires active database connection and valid config for now...
 */

import (
	"os"
	"testing"
	pf "trident.li/pitchfork/lib"
	pftst "trident.li/pitchfork/lib/test"
)

func TestMain(m *testing.M) {
	pftst.Test_setup()

	/* Services */
	pf.Starts()
	defer pf.Stops()

	os.Exit(m.Run())
}
