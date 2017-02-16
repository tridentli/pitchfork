package pitchforkui

import (
	"os"
	"testing"
	pf "trident.li/pitchfork/lib"
	pftst "trident.li/pitchfork/lib/test"
)

// testingctx creates a testing context
func testingctx() pf.PfCtx {
	return pf.NewPfCtx(nil, nil, nil, nil, nil)
}

// TestingUI creates a testing UI context
func TestingUI() PfUI {
	ctx := testingctx()
	return NewPfUI(ctx, nil, nil, nil)
}

// TestMain sets up a testing environment
func TestMain(m *testing.M) {
	toolname := pftst.Test_setup()

	/* UI Setup */
	err := Setup(toolname, false)
	if err != nil {
		pf.Errf("Failed to setup server PU: %s", err.Error())
		os.Exit(1)
	}

	/* Services */
	pf.Starts()
	defer pf.Stops()

	os.Exit(m.Run())
}
