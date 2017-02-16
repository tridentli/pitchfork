// Pitchfork Password Dictionary checking
package pitchfork

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

// pw_dict is the map where we keep our weak passwords
var pw_dict map[string]bool
var pw_dicts int

// Pw_checkweak_load loads the dictionaries from disk into pw_dict
func Pw_checkweak_load() (err error) {
	/* Put something in there */
	pw_dict = make(map[string]bool)

	if len(Config.PW_WeakDicts) == 0 {
		Log("No Weak Password Dictionaries configured - skipping")
		return
	}

	Dbg("Loading Weak Password Dictionaries...")
	pw_dicts = 0

	for _, f := range Config.PW_WeakDicts {
		pw_dicts++
		cntpw := 0

		fn := System_findfile("pwdicts/", f)
		if fn == "" {
			err = errors.New("Could not find password dictionary file :" + f)
			return
		}

		file, e := os.Open(fn)
		if e != nil {
			err = e
			return
		}

		defer file.Close()

		Dbgf("Loading Weak Password Dictionary %q...", f)

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			pw := scanner.Text()

			/* Skip comments */
			if len(pw) > 1 && pw[0] == '#' {
				continue
			}

			pw_dict[pw] = true
			cntpw++
		}

		err = scanner.Err()
		if err != nil {
			return
		}

		Dbgf("Loading Weak Password Dictionary %q... done (%d passwords)", f, cntpw)
	}

	Logf("Loaded %d Weak Password Dictionaries with %d unique passwords", pw_dicts, len(pw_dict))

	return
}

// Pw_checkweak checks if a password is in our weak password list
func Pw_checkweak(password string) (isweak bool) {
	_, isweak = pw_dict[strings.ToLower(password)]
	return
}

// Pw_details provides details about te password dictionary checker (used by system_report)
func Pw_details() (msg string) {
	msg = fmt.Sprintf("Password Dictionary Checker: Loaded %d Weak Password Dictionaries with %d unique passwords", pw_dicts, len(pw_dict))
	return
}
