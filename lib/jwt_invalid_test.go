// Pitchfork JWTInvalid testing
package pitchfork

/*
 * $ go test trident.li/pitchfork/lib -run JWTInvalidate -v
 * ok  	command-line-arguments	20.053s
 *
 * Takes about 20 seconds due to the sleep for the expiry test
 */

import (
	"fmt"
	"testing"
	"time"
)

// TestClaims are simple test claims.
type TestClaims struct {
	JWTClaims
}

// jwtinv_test tests if a token is invalid.
func jwtinv_test(t *testing.T, n int, mins time.Duration) (tok string, claims *TestClaims) {
	tname := fmt.Sprintf("token%d", n)
	claims = &TestClaims{}

	token := Token_New("test", tname, mins, claims)

	tok, err := token.Sign()
	if err != nil {
		t.Errorf("Could not sign token %s: %s", tname, err.Error())
	}

	return
}

// TestJWTInvalidate tests whether invalidation works.
func TestJWTInvalidate(t *testing.T) {
	off := 100
	tok1, claims1 := jwtinv_test(t, 1, 10)

	/* Should not exist yet */
	if JwtInv_test_iscached(tok1) {
		t.Errorf("tok1 was already cached")
	}

	i := Jwt_isinvalidated(tok1, claims1)
	if i {
		t.Errorf("tok1 should not be invalid")
	}

	if !JwtInv_test_iscached(tok1) {
		t.Errorf("tok1 was not cached")
	}

	/* Invalidate it */
	Jwt_invalidate(tok1, claims1)

	i = Jwt_isinvalidated(tok1, claims1)
	if !i {
		t.Errorf("tok1 should be invalid, but is not")
	} else {
		t.Logf("tok1 is correctly invalidated")
	}

	/* Add another few items */
	for n := 0; n < JWT_INVALID_CACHE_MAX; n++ {
		tokN, claimsN := jwtinv_test(t, off+n, 5)
		Jwt_invalidate(tokN, claimsN)
	}

	/* Are they still the same? */
	lc := JwtInv_test_cache_len()
	li := JwtInv_test_list_len()

	if lc == li {
		t.Logf("Item counts in cache and list still match")
	} else {
		t.Errorf("Item counts do not match (%d vs %d)", lc, li)
	}

	/* tok1 should not be there anymore as we did not ref it */
	if JwtInv_test_iscached(tok1) {
		t.Errorf("tok1 was still cached")
	} else {
		t.Logf("tok1 was correctly not cached")
	}

	i = Jwt_isinvalidated(tok1, claims1)
	if !i {
		t.Errorf("tok1 should be invalid, but is not")
	} else {
		t.Logf("tok1 is correctly invalidated")
	}

	/* And another round */
	for n := 0; n < JWT_INVALID_CACHE_MAX; n++ {
		tokN, claimsN := jwtinv_test(t, off+n, 5)
		Jwt_invalidate(tokN, claimsN)
	}

	/* Force expires */
	before, after, err := JwtInv_test_expire()
	if err != nil {
		t.Errorf("Failed during expiration: %s", err.Error())
	} else if after == before {
		t.Errorf("No items expired: %d vs %d", before, after)
	} else if after > before {
		t.Errorf("More items left than before: %d vs %d", before, after)
	} else {
		t.Logf("Successful expiry: %d to %d items", before, after)
	}

	/* Are they still the same? */
	lc = JwtInv_test_cache_len()
	li = JwtInv_test_list_len()

	if lc == li {
		t.Logf("Item counts in cache and list still match")
	} else {
		t.Errorf("Item counts do not match (%d vs %d)", lc, li)
	}

	t.Logf("Done")
}
