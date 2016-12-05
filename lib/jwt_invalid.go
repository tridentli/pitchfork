package pitchfork

/*
 * Note: jwt_invalid uses non-audit versions of DB queries, otherwise we would generate double traffic
 *
 * Invalid, but not expired-yet, tokens are stored in SQL.
 *
 * A in-go cache exists keeping a LRU of valid+invalid tokens
 * to avoid hitting SQL all the time
 */

import (
	"container/list"
	"sync"
	"time"
)

const JWT_INVALID_CACHE_MAX = 512

type jwtinvs struct {
	item       *list.Element
	key        string
	isvalid    bool
	expiration int64
}

var jwtinv_cache map[string]jwtinvs
var jwtinv_list *list.List
var jwtinv_exit chan bool
var jwtinv_done chan bool
var jwtinv_running bool
var jwtinv_mutex = &sync.Mutex{}

func init() {
	jwtinv_cache = make(map[string]jwtinvs)
	jwtinv_list = list.New()
}

/* Removes items that have expired. */
func jwtinv_expire() (err error) {
	jwtinv_mutex.Lock()
	defer jwtinv_mutex.Unlock()

	/* Delete entries from SQL */
	q := "DELETE FROM jwt_invalidated WHERE expires < NOW()"
	err = DB.ExecNA(-1, q)
	if err != nil {
		Errf("jwtinv_expire: %s", err.Error())
	}

	now := time.Now().Unix()

	/* Delete entries from cache that have expired */
	for key, isval := range jwtinv_cache {
		if isval.expiration < now {
			jwtinv_cache_del(key)
		}
	}

	return
}

func jwtInvalid_rtn(timeoutchk time.Duration) {
	jwtinv_running = true

	/* Timer for expiring entries */
	tmr_exp := time.NewTimer(timeoutchk)

	for jwtinv_running {
		select {
		case _, ok := <-jwtinv_exit:
			if !ok {
				jwtinv_running = false
				break
			}
			break

		case <-tmr_exp.C:
			jwtinv_expire()

			/* Restart timer */
			tmr_exp = time.NewTimer(timeoutchk)
			break
		}
	}

	jwtinv_done <- true
}

func JwtInv_start(timeoutchk time.Duration) {
	jwtinv_exit = make(chan bool)
	jwtinv_done = make(chan bool)

	go jwtInvalid_rtn(timeoutchk)
}

func JwtInv_stop() {
	if !jwtinv_running {
		return
	}

	/* Close the channel */
	close(jwtinv_exit)

	/* Wait for it to finish */
	<-jwtinv_done
}

/*
 * Mutex should be held when calling this
 * not for calling directly, used by Jwt_invalidate() + Jwt_isinvalidated()
 */
func jwtinv_cache_add(tok string, isvalid bool, claims JWTClaimI) {
	jwtc := claims.GetJWTClaims()
	isval := jwtinvs{nil, tok, isvalid, jwtc.ExpiresAt}

	/* Was not cached before, cache it */
	isval.item = jwtinv_list.PushFront(isval)
	jwtinv_cache[tok] = isval

	/* Getting too big? */
	if len(jwtinv_cache) > JWT_INVALID_CACHE_MAX {
		/* Pop off the top */
		tail := jwtinv_list.Back().Value.(jwtinvs)
		jwtinv_cache_del(tail.key)
	}
}

/*
 * Mutex should be held when calling this
 * not for calling directly
 */
func jwtinv_cache_del(tok string) {
	isval, ok := jwtinv_cache[tok]
	if !ok {
		/* Not in cache -> done */
		return
	}

	jwtinv_list.Remove(isval.item)
	delete(jwtinv_cache, tok)
}

func Jwt_invalidate(tok string, claims JWTClaimI) {
	jwtc := claims.GetJWTClaims()

	jwtinv_mutex.Lock()
	defer jwtinv_mutex.Unlock()

	/* Remove any old edition from local cache (cached 'valid' version) */
	jwtinv_cache_del(tok)

	q := "INSERT INTO jwt_invalidated (token, expires) VALUES($1, TO_TIMESTAMP($2)) " +
		"ON CONFLICT (token) DO NOTHING"
	err := DB.ExecNA(1, q, tok, jwtc.ExpiresAt)
	if err != nil {
		/* Just log it, not much we can do */
		Errf("Insert token_invalid(%q %s %v): %s", q, tok, jwtc.ExpiresAt, err.Error())
	}

	/* Add it to the cache */
	jwtinv_cache_add(tok, false, claims)
}

func Jwt_isinvalidated(tok string, claims JWTClaimI) (invalid bool) {
	/* Invalid by default */
	invalid = true

	jwtinv_mutex.Lock()
	defer jwtinv_mutex.Unlock()

	isval, ok := jwtinv_cache[tok]
	if ok {
		jwtinv_list.MoveToFront(isval.item)
		invalid = !isval.isvalid
		return
	}

	cnt := 0
	q := "SELECT COUNT(*) FROM jwt_invalidated WHERE token = $1"
	err := DB.QueryRowNA(q, tok).Scan(&cnt)
	if err != nil {
		Errf("JWT invalid check failed: %s", err.Error())
		return
	}

	/* Not invalid */
	if cnt == 0 {
		invalid = false
	}

	/* Add it to the cache */
	jwtinv_cache_add(tok, !invalid, claims)

	return
}

/* Hooks for test code */
func JwtInv_test_cache_len() int {
	return len(jwtinv_cache)
}

func JwtInv_test_list_len() int {
	return jwtinv_list.Len()
}

func JwtInv_test_iscached(tok string) (ok bool) {
	_, ok = jwtinv_cache[tok]
	return
}

func JwtInv_test_expire() (before int, after int, err error) {
	jwtinv_mutex.Lock()

	/* Overwrite the expiration times so that we know that half of them go away */
	before = JwtInv_test_cache_len()

	/* Delete entries from cache that have expired */
	n := 0
	for key, isval := range jwtinv_cache {
		n++
		if n%2 == 0 {
			/* Back to the future^Wpast */
			isval.expiration = 1000
			jwtinv_cache[key] = isval
		}
	}

	jwtinv_mutex.Unlock()

	/* Run the expire */
	err = jwtinv_expire()

	/* After we expired */
	after = JwtInv_test_cache_len()

	return
}
