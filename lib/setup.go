package pitchfork

import (
	"time"
)

/*
 * Trident Pitchfork Lib Setup
 *
 * Split out so that we can call it for Tests cases too next to normal server behaviour
 */

func Setup(toolname string, confroot string, verbosedb bool, app_schema_version int) (err error) {
	/* Load configuration */
	err = Config.Load(toolname, confroot)
	if err != nil {
		Errf("Failed: %s", err.Error())
		return
	}

	/* Our database details */
	DB_Init(verbosedb)

	/* Tell Pitchfork what the App DB version is */
	DB_SetAppVersion(app_schema_version)

	/* Try the database connection */
	err = DB.Connect_def()
	if err != nil {
		Errf("DB connection failed: %s", err.Error())
		return
	}

	/* Load Weak Password Dictionaries */
	err = Pw_checkweak_load()
	if err != nil {
		Errf("Loading Weak Password Dictionaries failed: %s", err.Error())
		return
	}

	/* Initalize translation matrix */
	err = SetupTranslation()
	if err != nil {
		Errf("Loading translation languages failed: %s", err.Error())
		return
	}

	return
}

/* Start background services */
func Starts() {
	/* Start IP Tracker -- against brute force login attempts */
	Iptrk_start(5, 10*time.Hour, "1 hour")

	/* Start JWT Invalidation caching/clearing */
	JwtInv_start(30 * time.Minute)
}

/* Should be deferred  Starts() call */
func Stops() {
	Iptrk_stop()
	JwtInv_stop()
}
