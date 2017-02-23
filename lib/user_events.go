// Pitchfork User events Module
package pitchfork

import (
	"time"
)

// userevent logs an event for a user.
//
// Any arbitrary string can be used as an event.
//
// In addition to the event string, the username,
// IP address, full remote address (including XFF),
// the browser string, the Operating system and
// the full User-Agent are logged.
func userevent(ctx PfCtx, event string) {
	ident := ctx.TheUser().GetUserName()
	ip := ctx.GetClientIP()
	remote := ctx.GetRemote()
	ua_full, ua_browser, ua_os := ctx.GetUserAgent()

	q := "INSERT INTO userevents " +
		"(ident, event, ip, remote, browser, os, fullua) " +
		"VALUES($1, $2, $3, $4, $5, $6, $7)"
	err := DB.ExecNA(
		1, q,
		ident, event, ip.String(), remote, ua_browser, ua_os, ua_full)
	if err != nil {
		Errf("Could not insert entry into userevents: %s", err.Error())
		return
	}

	return
}

// GetLastActivity retrieves the last activity of a user.
//
// To be able to notify the user of their last activity
// this function allows fetching this detail.
//
// The activity includes a timestamp and the IP address
// that the user came from.
//
// It is based on the events table, as this is where
// at minimum login events are logged and logins are
// noted as activity.
//
// We return only the direct remote address, not the
// complete XFF which we also have recorded in the database.
func (user *PfUserS) GetLastActivity(ctx PfCtx) (entered time.Time, ip string) {
	ident := user.GetUserName()

	q := "SELECT entered, ip " +
		"FROM userevents " +
		"WHERE ident = $1 " +
		"ORDER BY entered DESC " +
		"LIMIT 1 " +
		"OFFSET 1"
	err := DB.QueryRow(q, ident).Scan(&entered, &ip)
	if err == ErrNoRows {
		return
	} else if err != nil {
		return
	}

	return
}

// user_events_list lists the events for a user (CLI).
//
// This allows a user to list the events about the given user.
// Users can list these events of themselves only unless the
// user is a sysadmin.
func user_events_list(ctx PfCtx, args []string) (err error) {
	username := args[0]

	err = ctx.SelectUser(username, PERM_USER_SELF|PERM_SYS_ADMIN)
	if err != nil {
		return
	}

	q := "SELECT entered, event, ip, browser, os, fullua " +
		"FROM userevents " +
		"WHERE ident = $1"
	rows, err := DB.Query(q, username)
	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		var e_entered time.Time
		var e_event, e_ip, e_browser, e_os, e_fullua string
		err = rows.Scan(&e_entered, &e_event, &e_ip, &e_browser, &e_os, &e_fullua)

		ctx.OutLn("%s %s %s %s %s %s", e_entered.Format(Config.TimeFormat), e_event, e_ip, e_browser, e_os, e_fullua)
	}
	return
}

// user_events is the CLI menu for User Events (CLI).
func user_events(ctx PfCtx, args []string) (err error) {
	menu := NewPfMenu([]PfMEntry{
		{"list", user_events_list, 1, 1, []string{"username"}, PERM_USER, "List all events related to a user"},
	})
	return ctx.Menu(args, menu)
}
