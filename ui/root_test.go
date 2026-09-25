package pitchforkui_test

import (
	"net/http"
	"testing"
	pu "trident.li/pitchfork/ui"
	urltest "trident.li/pitchfork/ui/urltest"
)

func TestUI_Main_Misc(t *testing.T) {
	tests := []urltest.URLTest{
		{
			Desc:     "RootTest",
			Method:   "GET",
			Path:     "/",
			Username: "",
			Header:   nil,
			BodyVals: nil,
			RC:       http.StatusOK,
			Positive: []string{},
			Negative: []string{},
		},

		/* Missing pages check */
		urltest.URLTest_404("/404"),
	}

	/* Our Root */
	root := pu.NewPfRootUI(pu.TestingUI)

	for _, u := range tests {
		urltest.Test_URL(t, root.H_root, u)
	}
}
