package pfpgp

import (
	"io/ioutil"
	"testing"
	"time"
)

func TestGetKeyInfo(t *testing.T) {
	timeFormat := "2006-01-02T15:04:05.000Z"
	wt, err := time.Parse(timeFormat, "1970-01-01T00:00:00.000Z")
	if err != nil {
		t.Fatalf("failed parsing wantTime: %v", err)
	}

	tests := []struct {
		desc    string
		keyFile string
		email   string
		wantId  string
		wantExp time.Time
		wantErr bool
	}{{
		desc:    "Success reading exp/id",
		keyFile: "key1.asc",
		email:   "morrowc@ops-netman.net",
		wantId:  "AFAB3052A843B36B",
		wantExp: wt,
	}}

	for _, test := range tests {
		key, err := ioutil.ReadFile("testdata/" + test.keyFile)
		if err != nil {
			t.Fatalf("[%v]: failed to read keyfile (%v): %v", test.desc, test.keyFile, err)
		}

		gotId, gotExp, err := GetKeyInfo(string(key), test.email)
		switch {
		case err != nil && !test.wantErr:
			t.Errorf("[%v]: got error when not expecting one: %v", test.desc, err)
		case err == nil && test.wantErr:
			t.Errorf("[%v]: did not get error when expecting one", test.desc)
		case err == nil:
			if gotId != test.wantId {
				t.Errorf("[%v]: got/want ID differences: %v / %v", test.desc, gotId, test.wantId)
			}
			if !test.wantExp.Equal(gotExp) {
				t.Errorf("[%v]: got/want Exp differences: %v / %v", test.desc, gotExp, test.wantExp)
			}
		}
	}
}
