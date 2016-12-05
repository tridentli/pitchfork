package pitchfork

import (
	"bufio"
	"errors"
	"os"
	"strings"
)

var pw_dict map[string]bool

func Pw_checkweak_load() (err error) {
	/* Put something in there */
	pw_dict = make(map[string]bool)

	if len(Config.PW_WeakDicts) == 0 {
		Log("No Weak Password Dictionaries configured - skipping")
		return
	}

	Dbg("Loading Weak Password Dictionaries...")
	cnt := 0

	for _, f := range Config.PW_WeakDicts {
		cnt++
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

	Logf("Loaded %d Weak Password Dictionaries with %d unique passwords", cnt, len(pw_dict))

	return
}

func Pw_checkweak(password string) (isweak bool) {
	_, isweak = pw_dict[strings.ToLower(password)]
	return
}
