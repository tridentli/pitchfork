// Pitchfork Password functions.
package pitchfork

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"strconv"
	"time"
	"unicode"

	// Externals
	"trident.li/go/osutil-crypt"
	cc "trident.li/go/osutil-crypt/common"
)

// PfPass represents Password functions.
type PfPass struct {
}

// PfPWRules details the password rules.
type PfPWRules struct {
	Min_length   int
	Max_length   int
	Min_letters  int
	Min_uppers   int
	Min_lowers   int
	Min_numbers  int
	Min_specials int
}

// GenRand generates a random set of bytes.
func (pw *PfPass) GenRand(length int) (bytes []byte, err error) {
	bytes = make([]byte, length)
	_, err = rand.Read(bytes)
	if err != nil {
		bytes = []byte("NoRandom")
		return
	}

	return
}

// GenRandHex returns a random hex-encoded string of a certain length.
func (pw *PfPass) GenRandHex(length int) (hex string, err error) {
	bytes, err := pw.GenRand(length)
	if err != nil {
		return
	}

	/* Convert them into a comfortable range */
	const alphanum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	for i, b := range bytes {
		bytes[i] = alphanum[b%byte(len(alphanum))]
	}

	hex = Hex(bytes)
	return
}

// GenPass generates a random password in base64 of given length.
func (pw *PfPass) GenPass(length int) (pass string, err error) {
	bytes, err := pw.GenRand(length)
	if err != nil {
		return
	}

	pass = base64.URLEncoding.EncodeToString(bytes)[0:length]

	return
}

// Make creates a SHA512 encoded password from a given plaintext string.
func (pw *PfPass) Make(password string) (hash string, err error) {
	c, err := crypt.NewFromHash("$6$")
	if err != nil {
		return
	}
	return c.Generate([]byte(password), nil)
}

// Verify verifies if a given plaintext password matches the given hashedPassword.
//
// hashedPassword is in the semi-standardized /etc/shadow passwd format.
//
// The format can be either:
//   $<hashtype>$<salt>$<hash>
// or:
//   $<hashtype>$rounds=<iter>$<salt>$<hash>
//
// An error is returned if the verification failed, nil if all is okay.
func (pw *PfPass) Verify(password string, hashedPassword string) (err error) {
	c, err := crypt.NewFromHash(hashedPassword)
	if err != nil {
		return
	}

	err = c.Verify(hashedPassword, []byte(password))
	if err == cc.ErrKeyMismatch {
		err = errors.New("Provided password does not match stored password")
	}
	return
}

// calc_otp calculates the OTP code given the key and value.
func calc_otp(key string, value int64) int {
	hash := hmac.New(sha1.New, []byte(key))
	err := binary.Write(hash, binary.BigEndian, value)
	if err != nil {
		return -1
	}
	h := hash.Sum(nil)

	offset := h[19] & 0x0f

	truncated := binary.BigEndian.Uint32(h[offset : offset+4])

	truncated &= 0x7fffffff
	code := truncated % 1000000

	return int(code)
}

// VerifyHOTP verifies a HOTP key based on the counter and twofactor string provided.
func (pw *PfPass) VerifyHOTP(key string, counter int64, twofactor string) bool {
	var i int64

	tf, err := strconv.Atoi(twofactor)
	if err != nil {
		return false
	}

	min := counter
	if min > 0 {
		min--
	}

	max := counter + 3

	for i = min; i < max; i++ {
		code := calc_otp(key, i)
		if code == tf {
			return true
		}
	}

	return false
}

// VerifyTOTP verifies a key and twofactor.
func (pw *PfPass) VerifyTOTP(key string, twofactor string) bool {
	tf, err := strconv.Atoi(twofactor)
	if err != nil {
		return false
	}

	t0 := int64(time.Now().Unix() / 30)
	minT := t0 - (5 / 2)
	maxT := t0 + (5 / 2)

	for t := minT; t <= maxT; t++ {
		if calc_otp(key, t) == tf {
			return true
		}
	}

	return false
}

// SOTPHash creates a Single-use OTP hash from a secret.
func (pw *PfPass) SOTPHash(secret string) (out string) {
	h := sha256.New()
	h.Write([]byte(secret))
	return Hex(h.Sum(nil))
}

// VerifySOTP verifies if a SOTP key is valid.
func (pw *PfPass) VerifySOTP(key string, twofactor string) bool {
	enc := pw.SOTPHash(twofactor)

	if key == enc {
		return true
	}

	return false
}

// VerifyPWRules verifies if a password matches the given Password Rules.
func (pw *PfPass) VerifyPWRules(password string, r PfPWRules) (probs []string) {
	var letters = 0
	var uppers = 0
	var lowers = 0
	var numbers = 0
	var specials = 0

	probs = nil

	if password == "" {
		probs = append(probs, "No password was provided")
	} else if r.Min_length != 0 && len(password) < r.Min_length {
		probs = append(probs, "Password is too short")
	} else if r.Max_length != 0 && len(password) > r.Max_length {
		probs = append(probs, "Password is too long: (>"+strconv.Itoa(r.Max_length)+")")
		/* Nothing else matters */
		return
	}

	/* Is the password weak? (only works if dictionaries are configured */
	if Pw_checkweak(password) {
		probs = append(probs, "Password is a weak common password")
	}

	pos := 0
	for _, s := range password {
		pos++
		switch {
		case unicode.IsNumber(s):
			numbers++

		case unicode.IsUpper(s):
			uppers++
			letters++

		case unicode.IsLower(s):
			lowers++
			letters++

		case unicode.IsPunct(s) || unicode.IsSymbol(s):
			specials++

		case unicode.IsLetter(s):
			letters++

		default:
			probs = append(probs, "Invalid character encountered at position "+strconv.Itoa(pos))
		}
	}

	if r.Min_uppers != 0 && uppers < r.Min_uppers {
		probs = append(probs, "Not enough upper case letters ("+strconv.Itoa(r.Min_uppers)+"+)")
	}

	if r.Min_lowers != 0 && lowers < r.Min_lowers {
		probs = append(probs, "Not enough lower case letters ("+strconv.Itoa(r.Min_uppers)+"+)")
	}

	if r.Min_specials != 0 && specials < r.Min_specials {
		probs = append(probs, "Not enough special character ("+strconv.Itoa(r.Min_specials)+"+)")
	}

	if r.Min_numbers != 0 && numbers < r.Min_numbers {
		probs = append(probs, "Not enough numbers ("+strconv.Itoa(r.Min_numbers)+"+)")
	}

	return probs
}
