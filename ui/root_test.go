package pitchforkui_test

import (
	"net/http"
	"testing"
	pu "trident.li/pitchfork/ui"
	urltest "trident.li/pitchfork/ui/urltest"
)

// TestUI_Main_Misc performs simple tests against the root page
func TestUI_Main_Misc(t *testing.T) {
	tests := []urltest.URLTest{
		/* Root test */
		{"RootTest",
			"GET", "/",
			"",
			nil,
			nil,
			http.StatusOK, []string{}, []string{}},

		/* Missing pages check */
		urltest.URLTest_404("/404"),
	}

	/* Our Root */
	root := pu.NewPfRootUI(pu.TestingUI)

	for _, u := range tests {
		urltest.Test_URL(t, root.H_root, u)
	}
}
