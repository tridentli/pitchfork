package pitchfork

/* TODO: verify against https://tools.ietf.org/html/rfc7523 */

import (
	"errors"
)

type OAuth_Auth struct {
	ClientID string `label:"Client ID" pfset:"nobody" pfget:"none"`
	Scope    string `label:"Scope" pfset:"nobody" pfget:"none"`
	RType    string `label:"Request Type" pfset:"nobody" pfget:"none"`
	Redirect string `label:"Redirect URL" pfset:"nobody" pfget:"none"`
	Auth     string `label:"Authorize" pftype:"submit"`
	Deny     string `label:"Deny" pftype:"submit" htmlclass:"deny"`
}

type OAuth2Claims struct {
	JWTClaims
	ClientID string `json:"oa_client_id"`
	Scope    string `json:"oa_scope"`
	RType    string `json:"oa_rtype,omitempty"`
	Redirect string `json:"oa_redirect,omitempty"`
}

func OAuth2_AuthToken_New(ctx PfCtx, o OAuth_Auth) (tok string, err error) {
	if !ctx.IsLoggedIn() {
		tok = ""
		err = errors.New("Not authenticated")
		return
	}

	/* Auth Tokens expire after an hour */
	claims := &OAuth2Claims{}
	claims.ClientID = o.ClientID
	claims.Scope = o.Scope
	claims.RType = o.RType
	claims.Redirect = o.Redirect

	username := ctx.TheUser().GetUserName()

	token := Token_New("oauth_auth", username, 1, claims)
	tok, err = token.Sign()
	return
}

func OAuth2_AuthToken_Check(tok string) (claims *OAuth2Claims, err error) {
	_, err = Token_Parse(tok, "oauth_auth", claims)
	return
}

func OAuth2_AccessToken_New(ctx PfCtx, client_id string, scope string) (tok string, err error) {
	if !ctx.IsLoggedIn() {
		tok = ""
		err = errors.New("Not authenticated")
		return
	}

	claims := &OAuth2Claims{}
	claims.ClientID = client_id
	claims.Scope = scope

	username := ctx.TheUser().GetUserName()

	/* Access Tokens - 24 hour validity */
	token := Token_New("oauth_access", username, TOKEN_EXPIRATIONMINUTES, claims)

	tok, err = token.Sign()
	return
}
