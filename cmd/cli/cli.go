// Trident Pitchfork CLI - Tickly (tcli)
//
// This is effectively a HTTP client for Pitchfork's daemon.
// All requests are sent over HTTP, there is no access directly to anything.
//
// This client also serves as an example on how to talk to the Trident API.
//
// tcli stores a token in ~/.<xxx_token> for retaining the logged-in state.
//
// Custom environment variables:
// - Select a custom token file with:
//     ${env_token}=/other/path/to/tokenfile
//   This is useful if you want to have multiple identities
//   or want to keep a token around that has the sysadmin bit set
//
// - Enable verbosity with:
//     ${env_verbose}=<anything>
//
// - Disable verbosity with
//     ${env_verbose}=off
//   or unset the environment variable

package pf_cmd_cli

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"

	"golang.org/x/crypto/ssh/terminal"
	cc "trident.li/pitchfork/cmd/cli/cmd"
)

// Whether verbosity is enabled
var verbosity = false

// outputVerbose is used to print out verbose messages
func outputVerbose(str ...interface{}) {
	if verbosity {
		fmt.Print("~~~ ")
		fmt.Println(str...)
	}
}

// output is used to print out actual output.
//
// We wrap the fmt.Print function thus allowing
// us easier to find where output() is happening
// and possibly to later implement redirection of
// the output to other output channels or prefix
// extra details to the output, eg a timestamp.
func output(str ...interface{}) {
	fmt.Print(str...)
}

// output_err is used for printing errors
func output_err(str ...interface{}) {
	fmt.Print("--> ")
	fmt.Println(str...)
}

// token_read is used to read a token from a file into a string
func token_read(filename string) (token string) {
	tokenb, err := ioutil.ReadFile(filename)
	if err != nil {
		return ""
	}

	return string(tokenb)
}

// token_store is used to store a token string into a file
func token_store(filename string, token string) {
	err := ioutil.WriteFile(filename, []byte(token), 0600)
	if err != nil {
		output_err("Error while storing token in " + filename + ": " + err.Error())
	}
}

// CLI is the big call that applications call to implement a CLI towards pitchfork
//
// It configures verbosity of the output functions.
// Tries to locate and load an existing stored cookie.
// Determines the location of the daemon's HTTP interface.
// And finally used CLICmd() to execute the command.
//
// Args are the arguments coming from the shell.
// token_name is the name of the token when send to the HTTP server.
// env_token is the name of the environment variable where a token can be found.
// env_verbose is the name of the environment variable that indicates the verbosity level.
// env_server is the name of the environment variable that indicates the location of our HTTP server.
// default_server is the URL of the default HTTP server.
func CLI(token_name string, env_token string, env_verbose string, env_server string, default_server string) {
	var tokenfile string
	var server string
	var verbose bool
	var readarg int

	flag.StringVar(&server, "server", "http://localhost:8333", "Server to talk to [env "+env_server+"]")
	flag.StringVar(&tokenfile, "tokenfile", "", "Token to use [env "+env_token+"] (default \"~/"+token_name+"\")")
	flag.BoolVar(&verbose, "v", false, "Enable verbosity [env "+env_verbose+"]")
	flag.IntVar(&readarg, "r", 0, "Read n arguments from the TTY, useful for passwords/2FA")
	flag.Parse()

	/* Determine verbosity -- based on environment or flag */
	verb_env := os.Getenv(env_verbose)
	if (verb_env != "" && verb_env != "off") || verbose {
		verbosity = true
	} else {
		verbosity = false
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
		outputVerbose("Read existing token")
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
	for readarg > 0 {
		fd := int(os.Stdin.Fd())
		if !terminal.IsTerminal(fd) {
			output_err("Terminal is not a TTY")
			os.Exit(1)
		} else {
			fmt.Print("Hidden argument: ")
			txt, err := terminal.ReadPassword(fd)
			if err != nil {
				output_err("Could not read argument: " + err.Error())
				os.Exit(1)
			}

			args = append(args, string(txt))
			fmt.Println("")
		}
		readarg = readarg - 1
	}

	newtoken, rc, err := cc.CLICmd(args, token, server, verb, output)

	if err != nil {
		output_err("Error: " + err.Error())

		/* Set a non-0 exit code when something failed */
		if rc == 0 {
			rc = 1
		}
	} else {
		/* Unauthorized? Then kill the token */
		if newtoken == "" && token != "" {
			outputVerbose("Unauthorized, destroy old token")
			os.Remove(tokenfile)
		} else if newtoken != "" && newtoken != token {
			outputVerbose("Storing new token")
			token_store(tokenfile, newtoken)
		}

		outputVerbose("Done")
	}

	os.Exit(rc)
}
