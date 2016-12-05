package pitchfork

import (
	"testing"
)

func TestFileChkPath(t *testing.T) {
	tsts := []struct {
		path string
		ok   bool
	}{
		/* Positive tests */
		{"/", true},               /* Just the tip of the path */
		{"/one", true},            /* One subdirectory */
		{"/one/two/", true},       /* Two subdirectories */
		{"/one/two/three/", true}, /* Three subdirectories */
		{"/space ", true},         /* Gets trimmed */
		/* Negative tests */
		{"NOTASLASH", false},      /* Should start with a slash */
		{"double//slash", false},  /* Double Slash */
		{"/test/../test/", false}, /* Parent directory */
	}

	for i := 0; i < len(tsts); i++ {
		path := tsts[i].path
		ok := tsts[i].ok

		_, err := file_chk_path(path)
		if err == nil {
			if ok {
				t.Logf("file_chk_path(%s) ok", path)
			} else {
				t.Errorf("file_chk_path(%s) failed", path)
			}
		} else {
			if ok {
				t.Errorf("file_chk_path(%s) failed: %s", path, err.Error())
			} else {
				t.Logf("file_chk_path(%s) rejected correctly: %s", path, err.Error())
			}
		}
	}

	return
}
