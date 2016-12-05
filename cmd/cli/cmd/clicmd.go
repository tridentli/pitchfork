package pfclicmd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var g_verbosity = ""

func isverbose() bool {
	return g_verbosity != "off"
}

func terr(str ...interface{}) {
	fmt.Print("--> ")
	fmt.Println(str...)
}

func http_redir(req *http.Request, via []*http.Request) error {
	terr("Redirected connection, this should never happen!")
	return errors.New("I don't want to be redirected!")
}

type CLIOutputI func(str ...interface{})

func CLICmd(args []string, token string, server string, verb CLIOutputI, output CLIOutputI) (newtoken string, rc int, err error) {
	/* Initially the tokens remain the same */
	newtoken = token

	u, err := url.Parse(server)
	if err != nil {
		err = errors.New("URL Parsing failed: " + err.Error())
		return
	}

	for n, a := range args {
		/* Encode the path separator ('/') when it appears in a argument */
		args[n] = strings.Replace(a, "/", "%2F", -1)

		/*
		 * Replace empty parameter with paragraph char to avoid '//' situation
		 * which gets replaced with '/' by most HTTP servers and clients
		 */
		if args[n] == "" {
			args[n] = "%B6"
		}
	}

	u.Path = "/api/" + strings.Join(args, "/")

	if verb != nil {
		verb("Request: " + u.String())
	}

	client := &http.Client{
		CheckRedirect: http_redir,
	}

	req, err := http.NewRequest("GET", u.String(), nil)

	if err != nil {
		err = errors.New("Request creation failed: " + err.Error())
		return
	}

	req.Header.Set("User-Agent", "Trident/Tickly (https://trident.li)")

	/* Provide the token */
	if token != "" {
		if verb != nil {
			verb("Providing Authorization header")
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	res, err := client.Do(req)
	if err != nil {
		err = errors.New("Request Failed: " + err.Error())
		return
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		err = errors.New("Request Status: " + res.Status)
		return
	}

	/* Show response headers */
	if verb != nil {
		verb("--- Response Headers ------------")
		for k, v := range res.Header {
			verb(fmt.Sprintf("%s: %v", k, v))
		}
		verb("---------------------------------")
	}

	/* Unauthorized? Then kill the token */
	if res.StatusCode == http.StatusUnauthorized && token != "" {
		if verb != nil {
			verb("Unauthorized, removing token")
		}
		newtoken = ""
	} else {
		/* Keep the old token */
		newtoken = token
	}

	rc = 0
	rc_ := res.Header.Get("X-ReturnCode")
	if rc_ != "" {
		rc, err = strconv.Atoi(rc_)
		if err != nil {
			rc = 0
		}
	}

	ah := res.Header.Get("Www-Authenticate")
	if ah != "" {
		/* Should be a bearer token */
		if len(ah) > 6 && strings.ToUpper(ah[0:6]) == "BEARER" {
			if verb != nil {
				verb("Received a Www-Authenticate header with BEARER token")
			}

			r := regexp.MustCompile("'.+'|\".+\"|\\S+")
			m := r.FindAllString(ah, -1)
			for _, k := range m {
				if len(k) > 13 && strings.ToUpper(k[0:13]) == "ACCESS_TOKEN=" {
					token := k[14 : len(k)-1]
					if verb != nil {
						verb("Found new BEARER token")
					}

					/* The new token */
					newtoken = token
				}
			}
		} else {
			verb("Www-Authenticate header did not contain a BEARER token")
		}
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		err = errors.New("Body Reading failed: " + err.Error())
		return
	}

	output(string(body))
	return
}
