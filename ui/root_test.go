package pitchforkui

import (
	"net/http"
	"testing"

	urltest "trident.li/pitchfork/ui/urltest"
)

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
	root := NewPfRootUI(TestingUI)

	for _, u := range tests {
		urltest.Test_URL(t, root.H_root, u)
	}
}
