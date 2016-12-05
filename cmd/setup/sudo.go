/*
 * Trident Pitchfork Setup
 *
 * Setup is only meant for initial setup tasks.
 * It should be run as the 'postgres' user.
 *
 * For general use, use the CLI or the webinterface and log in.
 */

package pf_cmd_setup

import (
	"fmt"
	"os"
	cc "trident.li/pitchfork/cmd/cli/cmd"
	pf "trident.li/pitchfork/lib"
)

func output(str ...interface{}) {
	fmt.Print(str...)
}

func verb(str ...interface{}) {
	fmt.Print("~~~ ")
	fmt.Println(str...)
}

func sudo(ctx pf.PfCtx, env_server string, default_server string, username string, cmd []string) (rc int, err error) {
	/*
	 * Note: We do not directly check OS uid's
	 *
	 * We rely on access to the configuration file instead
	 * If one can access the configuration file, that user
	 * has access to the database credentials and can do
	 * whatever they want anyway.
	 *
	 */

	server := os.Getenv(env_server)
	if server == "" {
		server = default_server
	}

	/* Create a fake user */
	user := ctx.NewUser()

	/* Bypass all checks and simply select this user */
	user.SetUserName(username)

	/* Refresh the whole user from DB, effectively loading it */
	err = user.Refresh(ctx)
	if err != nil {
		return
	}

	/* Become the user */
	ctx.Become(user)

	/* Generate a new Token with the user's credentials */
	err = ctx.NewToken()
	if err != nil {
		return
	}

	/* Get the token that identifies this user */
	token := ctx.GetToken()

	/* Execute a CLI Command as this user (using the token we just made) */
	_, rc, err = cc.CLICmd(cmd, token, server, verb, output)

	/* Note: we throw away the token, not re-usable */

	return
}
