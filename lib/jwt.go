// Pitchfork JWT wrapper module -- JWT helpers between Pitchfork and dgrijalva's JWT library.
package pitchfork

import (
	"errors"
	"fmt"
	jwt "github.com/dgrijalva/jwt-go"
	"io/ioutil"
	"time"
)

/* Default Token expiration time */
const TOKEN_EXPIRATIONMINUTES = 20

// Token but wrapped for ease of use.
type Token struct {
	*jwt.Token
}

// GetClaims gets the claims from a token.
func (tok *Token) GetClaims() JWTClaimI {
	return tok.Token.Claims.(JWTClaimI)
}

// JWTClaimI wraps the claims for ease of use.
type JWTClaimI interface {
	jwt.Claims
	GetJWTClaims() *JWTClaims
}

// JWTClaims provides a standard set of claims.
type JWTClaims struct {
	jwt.StandardClaims
}

// GetJWTClaims returns the claims.
func (jwtc *JWTClaims) GetJWTClaims() *JWTClaims {
	return jwtc
}

// Token_New creates a new JWT token with provides arguments.
func Token_New(ttype string, username string, expmins time.Duration, claims JWTClaimI) (token *Token) {
	now := time.Now()

	jwtc := claims.GetJWTClaims()
	jwtc.Audience = ttype
	jwtc.Subject = username
	jwtc.IssuedAt = now.Unix()
	jwtc.ExpiresAt = now.Add(time.Minute * expmins).Unix()
	jwtc.Issuer = AppName

	jtoken := jwt.NewWithClaims(jwt.SigningMethodES512, claims)
	token = &Token{jtoken}
	return
}

// Sign signs a token using our private key.
func (token *Token) Sign() (tok string, err error) {
	tok, err = token.SignedString(Config.Token_prv)
	return
}

// Token_Parse parses a token from a string, requiring a type and certain claims.
func Token_Parse(tok string, ttype string, claims JWTClaimI) (expsoon bool, err error) {
	expsoon = false

	jtoken, err := jwt.ParseWithClaims(tok, claims, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return Config.Token_pub, nil
	})

	if err != nil || jtoken == nil || !jtoken.Valid {
		var ve *jwt.ValidationError
		ok := false
		if err != nil {
			ve, ok = err.(*jwt.ValidationError)
		}
		if ok {
			if ve.Errors&jwt.ValidationErrorMalformed != 0 {
				err = errors.New("Token does not even look like a token")
			} else if ve.Errors&jwt.ValidationErrorNotValidYet != 0 {
				err = errors.New("Token not active yet")
			} else if ve.Errors&jwt.ValidationErrorExpired != 0 {
				err = errors.New("Token expired")
			} else {
				err = errors.New("Token unhandled")
			}
		} else {
			err = errors.New("Token is invalid")
		}
		return
	}

	if Jwt_isinvalidated(tok, claims) {
		err = errors.New("Token is invalid")
		return
	}

	jwtc := claims.GetJWTClaims()

	/* Avoid type checking */
	if ttype == "" {
		/* Check that it is the right type of token */
		if jwtc.Audience != ttype {
			err = errors.New("Token is not a ttype token")
			return
		}
	}

	/* Is it going to expire soon? */
	then := time.Now().Add(time.Minute * 10).Unix()

	if then > jwtc.ExpiresAt {
		/* Expiring soon, requires refresh of Token */
		expsoon = true
	}

	return
}

// Token_LoadPrv loads our JWT private key.
func (cfg *PfConfig) Token_LoadPrv() (err error) {
	var pem []byte

	fn := Config.Conf_root + Config.JWT_prv
	pem, err = ioutil.ReadFile(fn)
	if err != nil {
		err = errors.New("Could not load JWT Private Key from " + fn + ": " + err.Error())
		return
	}

	/* Parse it */
	cfg.Token_prv, err = jwt.ParseECPrivateKeyFromPEM(pem)
	if err != nil {
		err = errors.New("Could not Parse JWT Private Key from " + fn + ": " + err.Error())
		return
	}

	return
}

// Token_LoadPub loads our JWT public key.
func (cfg *PfConfig) Token_LoadPub() (err error) {
	var pem []byte

	fn := Config.Conf_root + Config.JWT_pub
	pem, err = ioutil.ReadFile(fn)
	if err != nil {
		err = errors.New("Could not load JWT Public Key from " + fn + ": " + err.Error())
		return
	}

	/* Parse it */
	cfg.Token_pub, err = jwt.ParseECPublicKeyFromPEM(pem)
	if err != nil {
		err = errors.New("Could not Parse JWT Public Key from " + fn + ": " + err.Error())
		return
	}

	return
}
