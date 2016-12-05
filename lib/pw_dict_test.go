package pitchfork

/*
 * $ go test trident.li/pitchfork/lib -run PW -v
 * ok  	command-line-arguments	0.007s
 */

import (
	"testing"
)

func test_pwweak(t *testing.T, pw string, isok bool) {
	ok := Pw_checkweak(pw)
	if isok != ok {
		msg := "weak"
		if isok {
			msg = "okay"
		}
		t.Errorf("Expected %q to be %s but it was not", pw, msg)
	}

	return
}

func TestPW_Dict(t *testing.T) {
	weaks := []string{
		"password",
		"123456",
		"12345678",
		"1234",
		"qwerty",
		"12345",
		"dragon",
		"pussy",
		"baseball",
		"football",
		"letmein",
		"monkey",
		"696969",
		"abc123",
		"mustang",
	}

	notweaks := []string{
		"nN6aksVA",
		"Xxlql14V",
		"yRg5EaMD",
		"py8O8ZUr",
		"7d8An5Rr",
		"e81tjeQl",
	}

	/* Make sure it is empty (other tests might load them) */
	pw_dict = make(map[string]bool)

	/* Without loading, even simple passwords pass */
	ok := Pw_checkweak("password")
	if ok {
		t.Errorf("Incorrectly expected password 'password' to be weak [dicts not loaded]")
	} else {
		t.Logf("Password 'password' NOT weak [dicts not loaded]")
	}

	Config.File_roots = []string{"../share/"}
	Config.PW_WeakDicts = []string{"10k_most_common.txt"}
	err := Pw_checkweak_load()
	if err != nil {
		t.Errorf("Failed to load password dictionaries: %s", err.Error())
		return
	}
	t.Logf("Correctly loaded password dictionaries")

	/* Weak */
	for _, pw := range weaks {
		test_pwweak(t, pw, true)
	}

	/* Not weak */
	for _, pw := range notweaks {
		/* Not weak */
		test_pwweak(t, pw, false)
	}
}
