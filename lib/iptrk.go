package pitchfork

/*
 * Note: iptrk uses non-audit versions of DB queries, otherwise we would generate double traffic
 *
 * IP tracking is done in a DB so that it is distributed between nodes
 */

import (
	"errors"
	"time"
)

type IPtrkEntry struct {
	Blocked bool
	IP      string
	Count   int
	Entered time.Time
	Last    time.Time
}

type IPtrkS struct {
	cmd string
	ip  string
	chn chan bool
}

var IPtrk_Max int
var IPtrk chan IPtrkS
var IPtrk_done chan bool
var IPtrk_running bool

func iptrk_add(ip string) (ret bool) {
	ret = true

	cnt := 0

	/* Add a new hit */

	/*
	 * TODO: Postgres 9.5+
	 *
	 * q := "INSERT INTO iptrk (ip) VALUES($1) " +
	 *	"ON CONFLICT (ip) " +
	 *	"DO UPDATE SET count = iptrk.count + EXCLUDED.count, last = NOW() " +
	 *	"RETURNING count"
	 * err := DB.QueryRowNA(q, ip).Scan(&cnt)
	 */

	q := "INSERT INTO iptrk " +
		"(ip) " +
		"VALUES($1) " +
		"RETURNING count"
	err := DB.QueryRowNA(q, ip).Scan(&cnt)
	if err != nil && DB_IsPQErrorConstraint(err) {
		q = "UPDATE iptrk " +
			"SET count = count + 1 " +
			"WHERE ip = $1 " +
			"RETURNING count"
		err = DB.QueryRowNA(q, ip).Scan(&cnt)
	}

	if err != nil {
		Errf("Chk: %q %v %q", q, ip, err.Error())
		return
	}

	q = "SELECT count " +
		"FROM iptrk " +
		"WHERE ip = $1"
	err = DB.QueryRow(q, ip).Scan(&cnt)

	if err != nil {
		Errf("Chk: %q %v %q", q, ip, err.Error())
		return
	}

	/* Below the limit? */
	if cnt <= IPtrk_Max {
		/* Not blocked */
		ret = false
	}

	return
}

func iptrk_expire(t string) bool {
	Dbgf("Expiring")

	/* Expire tracking */
	q := "DELETE FROM iptrk WHERE last < (NOW() - INTERVAL '" + t + "')"
	err := DB.ExecNA(-1, q)
	if err != nil {
		Errf("ExpireTrk: %s", err.Error())
	}

	return true
}

func iptrk_flush(ip string) bool {
	var err error

	if ip == "" {
		/* Flush the whole IP Tracking table */
		q := "DELETE FROM iptrk"
		err = DB.ExecNA(-1, q)
	} else {
		/* Flush only a single IP */
		q := "DELETE FROM iptrk WHERE ip = $1"
		err = DB.ExecNA(-1, q, ip)
	}

	if err != nil {
		Errf("iptrk_flush() failed: %s", err.Error())
		return false
	}

	return true
}

/* Go routine that manages the ip tracking */
func iptrk_rtn(timeoutchk time.Duration, expire string) {
	IPtrk_running = true

	/* Timer for expiring entries */
	tmr_exp := time.NewTimer(timeoutchk)

	for IPtrk_running {
		select {
		case s, ok := <-IPtrk:
			if !ok {
				IPtrk_running = false
				break
			}

			ret := true

			switch s.cmd {
			case "add":
				ret = iptrk_add(s.ip)
				break

			case "wipe":
				ret = iptrk_expire(expire)
				break

			case "flush":
				ret = iptrk_flush(s.ip)
				break

			default:
				panic("Unhandled cmd: " + s.cmd)
			}

			/* Return */
			s.chn <- ret

			break

		case <-tmr_exp.C:
			Dbgf("Timer: Expire")
			iptrk_expire(expire)

			/* Restart timer */
			tmr_exp = time.NewTimer(timeoutchk)
			break
		}
	}

	IPtrk_done <- true
}

func iptrk_cmd(cmd string, ip string) (ret bool) {
	/* Create result channel */
	chn := make(chan bool)

	IPtrk <- IPtrkS{cmd, ip, chn}

	/* Wait for result */
	ret = <-chn
	return
}

func Iptrk_count(ip string) (limited bool) {
	if IPtrk_running {
		limited = iptrk_cmd("add", ip)
	} else {
		limited = iptrk_add(ip)
	}

	return
}

func Iptrk_start(max int, timeoutchk time.Duration, expire string) {
	IPtrk = make(chan IPtrkS, 1000)
	IPtrk_done = make(chan bool)
	IPtrk_Max = max

	go iptrk_rtn(timeoutchk, expire)
}

func Iptrk_stop() {
	if !IPtrk_running {
		return
	}

	/* Close the channel */
	close(IPtrk)

	/* Wait for it to finish */
	<-IPtrk_done
}

func Iptrk_reset(ip string) (ret bool) {
	if IPtrk_running {
		ret = iptrk_cmd("flush", ip)
	} else {
		ret = iptrk_flush(ip)
	}
	return
}

func IPtrk_List(ctx PfCtx) (ts []IPtrkEntry, err error) {
	q := "SELECT " +
		"ip, count, entered, last " +
		"FROM iptrk " +
		"ORDER BY ip"
	rows, err := DB.Query(q)
	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		var t IPtrkEntry

		err = rows.Scan(&t.IP, &t.Count, &t.Entered, &t.Last)
		if err != nil {
			return
		}

		t.Blocked = t.Count > IPtrk_Max

		ts = append(ts, t)
	}

	if len(ts) == 0 {
		err = ErrNoRows
	}

	return
}

func iptrk_list(ctx PfCtx, args []string) (err error) {
	ts, err := IPtrk_List(ctx)

	if err == ErrNoRows {
		ctx.OutLn("There are currently no entries")
		err = nil
		return
	}

	if err != nil {
		return
	}

	ctx.Outf("%16s %16s %7s %10s %s\n", "Entered", "Last", "Status", "Count", "IP")

	for _, t := range ts {
		s := "okay"
		if t.Blocked {
			s = "blocked"
		}

		ctx.Outf("%16s %16s %7s %10d %s\n", Fmt_Time(t.Entered), Fmt_Time(t.Last), s, t.Count, t.IP)
	}

	return
}

func iptrk_flushcmd(ctx PfCtx, args []string) (err error) {
	Iptrk_reset("")
	ctx.OutLn("IPtrk flushed")
	return
}

func iptrk_remove(ctx PfCtx, args []string) (err error) {
	ip := args[0]

	if ip == "" {
		err = errors.New("Missing argument, IP address required")
		return
	}

	ret := Iptrk_reset(ip)
	if ret {
		ctx.OutLn("IP removed from IPtrk table")
	} else {
		ctx.OutLn("No such IP in IPtrk table")
	}

	return
}

func iptrk_menu(ctx PfCtx, args []string) (err error) {
	menu := NewPfMenu([]PfMEntry{
		{"list", iptrk_list, 0, 0, nil, PERM_SYS_ADMIN, "List the contents of the IPtrk tables"},
		{"flush", iptrk_flushcmd, 0, 0, nil, PERM_SYS_ADMIN, "Flush all entries from the IPtrk table"},
		{"remove", iptrk_remove, 1, 1, []string{"ip"}, PERM_SYS_ADMIN, "Remove an entry from IPtrk"},
	})

	err = ctx.Menu(args, menu)
	return
}
