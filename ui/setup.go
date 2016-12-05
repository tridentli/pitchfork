package pitchforkui

/*
 * Trident Pitchfork UI Setup
  *
   * Split out so that we can call it for Tests cases too next to normal server behaviour
*/

import (
	pf "trident.li/pitchfork/lib"
)

func Setup(toolname string, securecookies bool) (err error) {
	/* Initialize UI Settings */
	err = UIInit(securecookies, "_"+toolname)
	if err != nil {
		pf.Errf("UI Init failed: %s", err.Error())
		return
	}

	/* Load Templates */
	err = pf.Template_Load()
	if err != nil {
		pf.Errf("Template Loading failed: %s", err.Error())
		return
	}

	/* Start Access Logger */
	if pf.Config.LogFile != "" {
		err = LogAccess_start()
		if err != nil {
			pf.Errf("Could not open log file (%s): %s", pf.Config.LogFile, err.Error())
			return
		}
		defer LogAccess_stop()
	} else {
		pf.Logf("Note: Access LogFile disabled")
	}

	return
}
