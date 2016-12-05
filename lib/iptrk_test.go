package pitchfork

/*
 * $ go test trident.li/pitchfork/lib -run IPtrk -v
 * ok  	command-line-arguments	20.053s
 *
 * Takes about 20 seconds due to the sleep for the expiry test
 */

import (
	"testing"
	"time"
)

func addip(t *testing.T, ip string, notlim bool) {
	lim := Iptrk_count(ip)

	to := "allowed"
	if !notlim {
		to = "limited"
	}

	did := "allowed"
	if lim {
		did = "limited"
	}

	if lim != (!notlim) {
		t.Errorf("Expected adding '%s' to be %s but it %s", ip, to, did)
	} else {
		t.Logf("Adding '%s' to be %s and it is %s", ip, to, did)
	}

	return
}

func SixthShouldFail(t *testing.T, ip string) {
	max := 5

	/* Start the IP Tracker */
	Iptrk_reset("")
	Iptrk_start(max, 10*time.Hour, "1 day")
	defer Iptrk_reset("")
	defer Iptrk_stop()

	/* Add the same IP multiple times */
	for i := 0; i < max; i++ {
		addip(t, ip, true)
	}

	/* The next ones should then fail */
	addip(t, ip, false)
	addip(t, ip, false)
}

func TestIPTtrkSixthShouldFail_v4(t *testing.T) {
	SixthShouldFail(t, "192.0.2.4")
}

func TestIPtrkSixthShouldFail_v6(t *testing.T) {
	SixthShouldFail(t, "2001:db8::6")
}

func TestIPtrkMix_v4v6(t *testing.T) {
	max := 5
	ip4 := "192.1.2.4"
	ip6 := "2001:db8::6"

	/* Start the IP Tracker */
	Iptrk_reset("")
	Iptrk_start(max, 10*time.Hour, "1 day")
	defer Iptrk_reset("")
	defer Iptrk_stop()

	/* Add the same IP multiple times */
	for i := 0; i < max; i++ {
		addip(t, ip4, true)
		addip(t, ip6, true)
	}

	/* The next one should then fail */
	addip(t, ip4, false)
	addip(t, ip6, false)
}

func TestIPtrkFlush(t *testing.T) {
	max := 5
	ip6 := "2001:db8::6"

	/* Start the IP Tracker */
	Iptrk_reset("")
	Iptrk_start(max, 10*time.Hour, "1 day")
	defer Iptrk_reset("")
	defer Iptrk_stop()

	/* Add the same IP multiple times */
	Dbgf("Adding IPs")
	for i := 0; i < max; i++ {
		addip(t, ip6, true)
	}

	/* The next one should then fail */
	addip(t, ip6, false)

	/* Reset the count for this IP */
	ok := Iptrk_reset(ip6)
	if !ok {
		t.Errorf("Reset failed")
		return
	}

	/* Should be allowed again */
	addip(t, ip6, true)
}

func TestIPtrkExpire(t *testing.T) {
	max := 5
	ip6 := "2001:db8::6"

	/* Start the IP Tracker */
	Iptrk_reset("")
	Iptrk_start(max, 1*time.Second, "5 seconds")
	defer Iptrk_reset("")
	defer Iptrk_stop()

	/* Add the same IP multiple times */
	Dbgf("Adding IPs")
	for i := 0; i < max; i++ {
		addip(t, ip6, true)
	}

	/* The next one should then fail */
	addip(t, ip6, false)

	/* Sleep a bit */
	Dbgf("Waiting for expiration")
	time.Sleep(20 * time.Second)

	/* Should have expired */
	addip(t, ip6, true)
}
