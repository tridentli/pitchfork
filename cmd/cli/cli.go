/*
 * Trident Pitchfork CLI - Tickly (tcli)
 *
 * This is effectively a HTTP client for Pitchfork's daemon.
 * All requests are sent over HTTP, there is no access directly to anything.
 *
 * This client also serves as an example on how to talk to the Trident API.
 *
 * tcli stores a token in ~/.<xxx_token> for retaining the logged-in state.
 *
 * Custom environment variables:
 * - Select a custom token file with:
 *     ${env_token}=/other/path/to/tokenfile
 *   This is useful if you want to have multiple identities
 *   or want to keep a token around that has the sysadmin bit set
 *
 * - Enable verbosity with:
 *     ${env_verbose}=<anything>
 *
 * - Disable verbosity with
 *     ${env_verbose}=off
 *   or unset the environment variable
 */

package pf_cmd_cli

import (
	"errors"
	"flag"
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	cc "trident.li/pitchfork/cmd/cli/cmd"
)

var g_isverbose = false

func terr(str ...interface{}) {
	fmt.Print("--> ")
	fmt.Println(str...)
}

func verb(str ...interface{}) {
	if g_isverbose {
		fmt.Print("~~~ ")
		fmt.Println(str...)
	}
}

func output(str ...interface{}) {
	fmt.Print(str...)
}

func token_read(filename string) (token string) {
	tokenb, err := ioutil.ReadFile(filename)
	if err != nil {
		return ""
	}

	return string(tokenb)
}

func token_store(filename string, token string) {
	err := ioutil.WriteFile(filename, []byte(token), 0600)
	if err != nil {
		terr("Error while storing token in " + filename + ": " + err.Error())
	}
}

func http_redir(req *http.Request, via []*http.Request) error {
	terr("Redirected connection, this should never happen!")
	return errors.New("I don't want to be redirected!")
}

func CLI(token_name string, env_token string, env_verbose string, env_server string, default_server string) {
	var tokenfile string
	var server string
	var verbose bool
	var readarg bool

	flag.StringVar(&server, "server", "http://localhost:8334", "Server to talk to [env "+env_server+"]")
	flag.StringVar(&tokenfile, "tokenfile", "", "Token to use [env "+env_token+"] (default \"~/"+token_name+"\")")
	flag.BoolVar(&verbose, "v", false, "Enable verbosity [env "+env_verbose+"]")
	flag.BoolVar(&readarg, "r", false, "Read an argument from the CLI, useful for passwords")
	flag.Parse()

	/* Determine verbosity -- based on environment or flag */
	verb_env := os.Getenv(env_verbose)
	if verb_env == "on" || verbose {
		g_isverbose = true
	}

	/*
	 * Figure out where the token is
	 *
	 * This allows multiple user logins/sessions
	 * at the same time, or for instance
	 * dropping the sysadmin bit in one token
	 */
	if tokenfile == "" {
		/* Tokenfile specified in environment? */
		tokenfile = os.Getenv(env_token)
		if tokenfile == "" {
			/* Default to the user home */
			usr, _ := user.Current()
			tokenfile = usr.HomeDir + "/" + token_name
		}
	}

	/* Try to get the token */
	token := token_read(tokenfile)

	if token != "" {
		verb("Read existing token")
	}

	/* Use the server from the flag? */
	if server == "" {
		/* Try the environment variable */
		server = os.Getenv(env_server)

		/* Got a server in the environment? No -> fallback to default */
		if server == "" {
			server = default_server
		}
	}

	args := flag.Args()

	/*
	 * Read an argument from the CLI,
	 * useful for passwords that should
	 * not show up in your shell's history
	 * or in the process list arguments
	 */
	if readarg {
		fd := int(os.Stdin.Fd())
		if !terminal.IsTerminal(fd) {
			terr("Terminal is not a TTY")
			os.Exit(1)
		} else {
			fmt.Print("Hidden argument: ")
			txt, err := terminal.ReadPassword(fd)
			if err != nil {
				terr("Could not read argument: " + err.Error())
				os.Exit(1)
			}

			args = append(args, string(txt))
			fmt.Println("")
		}
	}

	newtoken, rc, err := cc.CLICmd(args, token, server, verb, output)

	if err != nil {
		terr("Error: " + err.Error())

		/* Set a non-0 exit code when something failed */
		if rc == 0 {
			rc = 1
		}
	} else {
		/* Unauthorized? Then kill the token */
		if newtoken == "" && token != "" {
			verb("Unauthorized, destroy old token")
			os.Remove(tokenfile)
		} else if newtoken != "" && newtoken != token {
			verb("Storing new token")
			token_store(tokenfile, newtoken)
		}

		verb("Done")
	}

	os.Exit(rc)
}
