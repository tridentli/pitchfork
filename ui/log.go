package pitchforkui

/* Access Logging
 *
 * This logs, in JSON format to the selected access.log
 *
 * When a SIGUSR1 is received the log file is closed and re-opened
 * to support logrotate which moves the file out the way.
 *
 * Note that we have a mutex protecting la_running, but the actual
 * logging happens in a separate go thread so that there is no delay
 * while writing entries to the log (disks are slow).
 */

import (
	"encoding/json"
	"os"
	"os/signal"
	"sync"
	"syscall"

	pf "trident.li/pitchfork/lib"
)

var la_chan chan string
var la_running bool
var la_done chan bool
var la_file *os.File = nil
var la_mutex sync.Mutex

func laOpen() (err error) {
	/* Close any old open ones */
	laClose()

	if pf.Config.LogFile == "" {
		pf.Logf("No log file configured, skipping access logging")
		return
	}

	pf.Dbgf("Opening log file %q", pf.Config.LogFile)
	la_file, err = os.OpenFile(pf.Config.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	return
}

func laClose() {
	if la_file != nil {
		pf.Dbgf("Closing log file")
		la_file.Close()
		la_file = nil
	}
}

func laWrite(txt string) {
	/* Logging disabled */
	if la_file == nil {
		return
	}

	_, err := la_file.WriteString(txt + "\n")
	if err != nil {
		pf.Errf("LogAccess() writing to %s failed: %s", pf.Config.LogFile, err.Error())

		/* Try to re-open access log file */
		laOpen()
	}
}

func laReturn() {
	la_mutex.Lock()
	la_running = true
	la_mutex.Unlock()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGUSR1)

	running := true

	for running {
		select {
		case txt, ok := <-la_chan:
			if !ok {
				running = false
				break
			}

			/* Write the log entry */
			laWrite(txt)
			break

		case s := <-sigChan:
			if s == syscall.SIGUSR1 {
				pf.Dbgf("Received SIGUSR1, acting upon: rotating log file")
				laClose()
				laOpen()
			}
			break
		}
	}

	/* Close the log file */
	laClose()

	/* Tell them we are done */
	la_done <- true

	la_mutex.Lock()
	la_running = false
	la_mutex.Unlock()
}

/* Extends PfUIS */
func (cui *PfUIS) logaccess() {
	/* No LogFile -> Nothing to do */
	if pf.Config.LogFile == "" {
		return
	}

	/* Log the access */
	username := ""
	theuser := cui.TheUser()
	if theuser != nil {
		username = theuser.GetUserName()
	}

	type la_item struct {
		Username    string `json:"username"`
		Nodename    string `json:"nodename"`
		IP          string `json:"ip"`
		XFF         string `json:"xff"`
		HTTP_Method string `json:"method"`
		HTTP_Host   string `json:"host"`
		HTTP_Path   string `json:"path"`
		HTTP_Args   string `json:"args"`
		Template    string `json:"template"`
		StaticFile  string `json:"staticfile"`
	}

	la := la_item{
		Username:    username,
		Nodename:    pf.Config.Nodename,
		IP:          cui.GetClientIP().String(),
		XFF:         cui.GetRemote(),
		HTTP_Method: cui.GetMethod(),
		HTTP_Host:   cui.GetHTTPHost(),
		HTTP_Path:   cui.GetFullPath(),
		HTTP_Args:   cui.r.URL.RawQuery,
		Template:    cui.show_name,
		StaticFile:  cui.staticfile,
	}

	txt, err := json.Marshal(la)
	if err != nil {
		cui.Errf("Could not format access log message: %s", err.Error())
		return
	}

	direct := false

	la_mutex.Lock()
	if la_running {
		direct = true
	}
	la_mutex.Unlock()

	if !direct {
		la_chan <- string(txt)
	} else {
		laWrite(string(txt))
	}
}

func LogAccess_start() (err error) {
	la_chan = make(chan string, 1000)
	la_done = make(chan bool)

	/* Open the file at start, so we can detect initial errors */
	err = laOpen()
	if err != nil {
		return
	}

	/* Start background logging process */
	go laReturn()

	/* All dandy */
	return
}

func LogAccess_stop() {
	la_mutex.Lock()
	defer la_mutex.Unlock()

	if !la_running {
		return
	}

	/* Close the channel */
	close(la_chan)

	/* Wait for it to finish */
	<-la_done
}
