// Pitchfork CLI Menu handling code
package pitchfork

import (
	"bufio"
	"encoding/csv"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Mutex needed to only allow one batch script at a time.
var batchmutex = &sync.Mutex{}

// Standard error message for commands that are not known.
const ERR_UNKNOWN_CMDPFX = "Unknown command: "

// PfFunc is used as a prototype for all CLI menu functions.
type PfFunc func(ctx PfCtx, args []string) (err error)

// PfMEntry is a CLI menu entry with all the possible parameters.
type PfMEntry struct {
	Cmd      string
	Fun      PfFunc
	Args_min int
	Args_max int
	Args     []string
	Perms    Perm
	Desc     string
}

// PfMenu is a series of PfMEntry and thus make up a menu.
type PfMenu struct {
	M []PfMEntry
}

// NewPfMenu creates a new PfMenu.
func NewPfMenu(m []PfMEntry) PfMenu {
	return PfMenu{M: m}
}

// NewPfMEntry creates a new entry for a menu.
func NewPfMEntry(Cmd string, Fun PfFunc, Args_min int, Args_max int, Args []string, Perms Perm, Desc string) PfMEntry {
	return PfMEntry{Cmd, Fun, Args_min, Args_max, Args, Perms, Desc}
}

// Add allows one to add a entry to a menu, use in combo with NewPfMEntry().
func (menu *PfMenu) Add(m ...PfMEntry) {
	menu.M = append(menu.M, m...)
}

// Replace replaces the function of a menu thus allowing it to be overridden.
func (menu *PfMenu) Replace(cmd string, fun PfFunc) {
	for i, m := range menu.M {
		if m.Cmd == cmd {
			menu.M[i].Fun = fun
			return
		}
	}
}

// Remove removes an entry from a menu, thus allowing it to be forcefully disabled.
func (menu *PfMenu) Remove(cmd string) {
	for i, m := range menu.M {
		if cmd == m.Cmd {
			menu.M = append(menu.M[:i], menu.M[i+1:]...)
			return
		}
	}
}

// AddPerms OR's new permissions into the permissions, useful to mark a menu item hidden.
func (menu *PfMenu) AddPerms(cmd string, perms Perm) {
	for i, m := range menu.M {
		if cmd == m.Cmd {
			menu.M[i].Perms |= perms
			return
		}
	}
}

// DelPerms removes (AND-OR) permissions from a permission entry.
func (menu *PfMenu) DelPerms(cmd string, perms Perm) {
	for i, m := range menu.M {
		if cmd == m.Cmd {
			menu.M[i].Perms &^= perms
			return
		}
	}
}

// SetPerms changes the permissions of an entry.
func (menu *PfMenu) SetPerms(cmd string, perms Perm) {
	for i, m := range menu.M {
		if cmd == m.Cmd {
			menu.M[i].Perms = perms
			return
		}
	}
}

// Menu either provides help about a given menu or executes the given command given enough parameters are provided.
func (ctx *PfCtxS) Menu(args []string, menu PfMenu) (err error) {
	err = nil
	ok := false
	arg := "help"

	ctx.MenuOverride(&menu)

	if len(args) > 0 && args[0] != "" {
		arg = strings.ToLower(args[0])
	}

	if arg == "help" {
		// Walk only, thus don't show help
		if ctx.menu_walkonly {
			err = errors.New("help not allowed during menuwalk")
			return
		}

		if ctx.loc == "" {
			ctx.OutLn(AppName + " Help")
		} else {
			ctx.OutLn(AppName + " Help for: \"" + ctx.loc + "\"")
		}

		// Give a bit of detail about the user at the top
		if ctx.IsLoggedIn() {
			ss := ""

			// User is a sysadmin
			if ctx.IsSysAdmin() {
				ss = " [sysadmin]"
			} else if ctx.TheUser().CanBeSysAdmin() {
				// User can be a sysadmin, but is not
				ss = " [NOT sysadmin]"
			}

			ctx.OutLn("User: %s%s", ctx.TheUser().GetUserName(), ss)
		} else {
			// User is not authenticated at all
			ctx.OutLn("User: [Not authenticated]")
		}
		ctx.OutLn("")

		// Special introductuary header at the top menu
		if ctx.loc == "" {
			ctx.Out("" +
				"Welcome to the " + AppName + " menu system which is command line interface (CLI) based.\n" +
				"Note that when a command is not in the help menu the selected user might not have permissions for it.\n" +
				"\n" +
				"Each section, items marked [SUB], has its own 'help' command.\n" +
				"\n" +
				"The following commands are available on the root level:\n")
		}

		for _, m := range menu.M {
			opts := ""

			// Skip menu items that are not allowed
			ok, _ = ctx.CheckPerms("Menu("+m.Cmd+")/help", m.Perms)
			if !ok {
				continue
			}

			if m.Args != nil {
				for o := range m.Args {
					opt := strings.Split(m.Args[o], "#")
					opts += "<" + opt[0] + "> "
				}
				opts = strings.TrimSpace(opts)
			} else if m.Args_max == -1 {
				opts = "[SUB]"
			}

			ctx.OutLn(" %-20s %-20s %-20s", m.Cmd, opts, m.Desc)
		}

		return
	}

	for _, m := range menu.M {
		if m.Cmd != arg {
			continue
		}

		nargs := args[1:]

		if ctx.loc != "" {
			ctx.loc += " "
		}

		ctx.loc += arg

		_, err = ctx.CheckPerms("Menu("+m.Cmd+")", m.Perms)
		if err != nil {
			user := "<<notloggedin>>"
			if ctx.IsLoggedIn() {
				user = ctx.TheUser().GetUserName()
			}

			ctx.Log("User " + user + " tried access to command '" + ctx.loc + "': " + err.Error())
			ctx.SetStatus(StatusUnauthorized)
			return
		}

		// Walk Only & command & return the menu?
		if m.Args != nil && ctx.menu_walkonly {
			ctx.menu_menu = &m
			return
		}

		if m.Args_min > len(nargs) {
			err = errors.New("Not enough arguments for '" + ctx.loc + "' (got " + strconv.Itoa(len(nargs)) + ", need at least " + strconv.Itoa(m.Args_min) + ")")
			return
		}

		if m.Args_max != -1 {
			if len(nargs) > m.Args_max {
				err = errors.New("Too many arguments for '" + ctx.loc + "' (got " + strconv.Itoa(len(nargs)) + ", but want a maximum of " + strconv.Itoa(m.Args_min) + ")")
				return
			}
		}

		// Execute the menu
		err = m.Fun(ctx, nargs)
		return
	}

	msg := ERR_UNKNOWN_CMDPFX
	if ctx.loc != "" {
		msg += ctx.loc + " "
	}
	msg += arg

	err = errors.New(msg)
	return
}

// ErrIsUnknownCommand checks if an error undicates an unknown command
func ErrIsUnknownCommand(err error) bool {
	s := err.Error()
	sl := len(s)
	el := len(ERR_UNKNOWN_CMDPFX)
	return sl > el && s[:el] == ERR_UNKNOWN_CMDPFX
}

// Cmd simply executes a command by calling the correct menu entry.
func (ctx *PfCtxS) Cmd(args []string) (err error) {
	ctx.loc = ""

	return ctx.Menu(args, MainMenu)
}

// CmdOut executes a command, buffering the output of the command and returning it.
func (ctx *PfCtxS) CmdOut(cmd string, args []string) (msg string, err error) {
	cmds := []string{}
	if cmd != "" {
		cmds = strings.Split(cmd, " ")
		cmds = append(cmds, args...)
	} else {
		cmds = args
	}
	err = ctx.Cmd(cmds)
	msg = ctx.Buffered()
	return
}

// Batch executes a batch of commands from the given file.
func (ctx *PfCtxS) Batch(filename string) (err error) {
	/* Only allow one batch at a time */
	batchmutex.Lock()
	defer batchmutex.Unlock()

	/* Restrict to only .cli files */
	lf := len(filename)
	if lf <= 4 || filename[lf-4:] != ".cli" {
		err = errors.New("Not a .cli batch file")
		return
	}

	/* Open the batch file */
	ctx.OutLn("Opening batch file: " + filename)
	f, err := os.Open(filename)
	if err != nil {
		return
	}

	/* Remember the current working directory */
	oldwd, err := os.Getwd()
	if err != nil {
		return
	}

	/* And return to it when we exit this */
	defer os.Chdir(oldwd)
	/* If we didn't do this, things would be true magic */

	/*
	 * Change location to the directory of the file
	 * This helps with relative paths inside the batch files
	 */
	dir := filepath.Dir(filename)
	ctx.OutLn("Changing work-directory to " + dir)
	err = os.Chdir(dir)
	if err != nil {
		return
	}

	r := csv.NewReader(bufio.NewReader(f))

	/* Space is the separator */
	r.Comma = ' '

	/* Ignore Comments */
	r.Comment = '#'

	/* Allow any number of records */
	r.FieldsPerRecord = -1

	var line []string

	for {
		line, err = r.Read()
		if err == io.EOF {
			err = nil
			break
		}

		if err != nil {
			err = errors.New("Problem while reading CSV command file: " + err.Error())
			break
		}

		ctx.OutLn("Command: %#q", line)

		/* Skip empty lines */
		if len(line) == 0 {
			continue
		}

		err = ctx.Cmd(line)
		if err != nil {
			break
		}
	}

	ctx.OutLn("Batch processing done")

	return
}

// WalkMenu searching for the entry related to a given command, returning the entry instead of executing it.
func (ctx *PfCtxS) WalkMenu(args []string) (menu *PfMEntry, err error) {
	ctx.menu_menu = nil
	ctx.menu_walkonly = true

	err = ctx.Cmd(args)

	ctx.menu_walkonly = false

	return ctx.menu_menu, err
}
