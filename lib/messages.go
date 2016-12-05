package pitchfork

import (
	"bufio"
	"errors"
	pq "github.com/lib/pq"
	"html/template"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

/* The separator between the message IDs */
const Msg_sep = "/"

type PfMsgOpts struct {
	PfModOptsS

	/*
	 * How deep messages are still considered Subjects
	 * Thread_depth + 1 == messages
	 */
	Thread_depth int

	/* Subforum title */
	Title string
}

type PfMessage struct {
	Id        int           `pfcol:"id"`
	Path      string        `pfcol:"path"`
	Depth     int           `pfcol:"depth"`
	Title     string        `pfcol:"title"`
	Plaintext string        `pfcol:"plaintext"`
	HTML      template.HTML `pfcol:"html"`
	Entered   time.Time     `pfcol:"entered"`
	UserName  string        `pfcol:"member"`
	FullName  string        `pfcol:"descr" pftable:"member"`
	Seen      pq.NullTime   `pfcol:"entered" pftable:"msg_read"`
	/* TODO: extra properties: locked, hidden, etc */
}

var Msg_Props = "" +
	"SELECT msg.id, msg.path, msg.depth, msg.title, " +
	"msg.plaintext, msg.html, msg.entered, m.ident, m.descr, " +
	"msg_read.entered " +
	"FROM msg_messages msg " +
	"INNER JOIN member m ON msg.member = m.ident " +
	"LEFT OUTER JOIN msg_read ON msg.id = msg_read.id AND msg_read.member = $1"

/* We ignore effective root for this as that should always be valid */
func Msg_PathValid(ctx PfCtx, path *string) (err error) {
	/*
	 * Verify that it is a valid path
	 *
	 * Only accept:
	 * - /
	 * - /path/
	 * - /path/component2/
	 * - /path/component2/3rdcomponent/
	 * - etc
	 */
	re := `^([\` + Msg_sep + `])(([[a-zA-Z0-9\-_@+.])+([\` + Msg_sep + `]))*$`
	ok, err := regexp.MatchString(re, *path)
	if err != nil {
		return
	}

	if !ok {
		mopts := Msg_GetModOpts(ctx)
		ctx.Errf("Invalid Message Path: Modroot: >>>%s<<< Path: >>>%s<<<", mopts.Pathroot, *path)
		err = errors.New("Invalid path provided")
	}

	return
}

type MsgType uint

const (
	MSGTYPE_SECTION MsgType = iota
	MSGTYPE_THREAD
	MSGTYPE_MESSAGE
)

func Msg_GetModOpts(ctx PfCtx) PfMsgOpts {
	mopts := ctx.GetModOpts()
	if mopts == nil {
		panic("No Message ModOpts configured")
	}

	return mopts.(PfMsgOpts)
}

func Msg_ModOpts(ctx PfCtx, cmdpfx string, path_root string, web_root string, thread_depth int, title string) {
	ctx.SetModOpts(PfMsgOpts{PfModOpts(ctx, cmdpfx, path_root, web_root), thread_depth, title})
}

func Msg_PathType(ctx PfCtx, path string) MsgType {
	pd := Msg_PathDepth(ctx, path) - Msg_ModPathDepth(ctx)

	mopts := Msg_GetModOpts(ctx)

	if pd < (mopts.Thread_depth - 1) {
		return MSGTYPE_SECTION
	} else if pd < mopts.Thread_depth {
		return MSGTYPE_THREAD
	}

	return MSGTYPE_MESSAGE
}

/*
 * Calculate the depth of a path
 * A depth of 0 is the root (/)
 */
func Msg_PathDepth(ctx PfCtx, path string) (depth int) {
	mopts := Msg_GetModOpts(ctx)
	return strings.Count(mopts.Pathroot+path, Msg_sep) - 1
}

/*
 * The Module Root's should never have a trailing '/'
 * Hence why we do not substract here compared to Msg_PathDepth()
 */
func Msg_ModPathDepth(ctx PfCtx) (depth int) {
	mopts := Msg_GetModOpts(ctx)
	return strings.Count(mopts.Pathroot, Msg_sep)
}

func Msg_MarkSeen(ctx PfCtx, msg PfMessage) (err error) {
	if msg.Seen.Valid {
		err = errors.New("Already marked as seen")
		return
	}

	/* TODO: Pgsql 9.5: UPSERT IGNORE */
	q := "INSERT INTO msg_read (id,member) VALUES($1,$2)"
	err = DB.Exec(ctx, "Marked message $1 as read for $2", 1, q, msg.Id, ctx.TheUser().GetUserName())
	return
}

func Msg_MarkNew(ctx PfCtx, msg PfMessage) (err error) {
	if !msg.Seen.Valid {
		err = errors.New("Already marked as new")
		return
	}

	q := "DELETE FROM msg_read WHERE id = $1 AND member = $2"
	err = DB.Exec(ctx, "mark message $1 as new for $2", 1, q, msg.Id, ctx.TheUser().GetUserName())
	return
}

func Msg_GetThread(ctx PfCtx, path string, mindepth int, maxdepth int, offset int, max int) (msgs []PfMessage, err error) {
	msgs = nil
	var rows *Rows

	err = Msg_PathValid(ctx, &path)
	if err != nil {
		return
	}

	mopts := Msg_GetModOpts(ctx)
	effroot := mopts.Pathroot
	effrootlen := len(effroot)

	var args []interface{}

	q := Msg_Props + " WHERE true"
	args = append(args, ctx.TheUser().GetUserName())

	/* We want everything below it */
	DB.Q_AddWhere(&q, &args, "msg.path", "LIKE", effroot+path+"%", true, false, 0)

	pthdepth := Msg_PathDepth(ctx, path)
	effdepth := Msg_ModPathDepth(ctx)
	mindepth += pthdepth

	t := Msg_PathType(ctx, path)
	if t == MSGTYPE_MESSAGE && mindepth >= 1 {
		mindepth -= 1
	}

	if maxdepth != -1 {
		maxdepth += pthdepth
	}

	DB.Q_AddWhereOpAnd(&q, &args, "depth", ">=", mindepth)

	if maxdepth != -1 {
		DB.Q_AddWhereOpAnd(&q, &args, "depth", "<=", maxdepth)
	}

	/* In order of appearance */
	q += "ORDER BY msg.entered ASC "

	if max != 0 {
		q += " LIMIT "
		DB.Q_AddArg(&q, &args, max)
	}

	if offset != 0 {
		q += " OFFSET "
		DB.Q_AddArg(&q, &args, offset)
	}

	rows, err = DB.Query(q, args...)

	if err == ErrNoRows {
		err = errors.New("No such thread")
	}

	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		var msg PfMessage
		var html string

		err = rows.Scan(&msg.Id, &msg.Path, &msg.Depth, &msg.Title, &msg.Plaintext, &html, &msg.Entered, &msg.UserName, &msg.FullName, &msg.Seen)
		if err != nil {
			return
		}

		/* Substract the effective path + depth */
		msg.Path = msg.Path[effrootlen:]
		msg.Depth -= effdepth

		/* Bless the rendered HTML */
		msg.HTML = HEB(html)

		/* Add it to the list */
		msgs = append(msgs, msg)
	}
	return
}

func Msg_Get(ctx PfCtx, path string) (msg PfMessage, err error) {
	var html string

	err = Msg_PathValid(ctx, &path)
	if err != nil {
		return
	}

	mopts := Msg_GetModOpts(ctx)
	effroot := mopts.Pathroot

	var args []interface{}

	q := Msg_Props + " " +
		"WHERE msg.path = $2"

	args = append(args, ctx.TheUser().GetUserName())
	args = append(args, effroot+path)

	err = DB.QueryRow(q, args...).Scan(&msg.Id, &msg.Path, &msg.Depth, &msg.Title, &msg.Plaintext, &html, &msg.Entered, &msg.UserName, &msg.FullName, &msg.Seen)

	if err == ErrNoRows {
		err = errors.New("No such message")
	} else if err == nil {
		/* Substract the effective path's depth */
		effdepth := Msg_ModPathDepth(ctx)
		msg.Path = msg.Path[len(effroot):]
		msg.Depth -= effdepth

		/* Mark as filtered HTML */
		msg.HTML = HEB(html)
	}

	return
}

func Msg_Create_With_User(ctx PfCtx, user PfUser, path string, title string, plaintext string, notify bool) (newpath string, err error) {
	/* How deep is it? */
	depth := Msg_PathDepth(ctx, path)

	/* Render the plaintext as markdown into HTML */
	html := PfRender(plaintext, false)

	/*
	 * TODO: Use "UPSERT" instead of update/insert
	 * when we require postgresql 9.5 (Debian jessie has 9.4)
	 */

	u := "UPDATE msg_messages " +
		"SET depth = $2, title = $3, plaintext = $4, html = $5, member = $6 " +
		"WHERE path = $1"

	i := "INSERT INTO msg_messages " +
		"(path, depth, title, plaintext, html, member) " +
		"SELECT $1, $2, $3, $4, $5, $6"

	q := "WITH upsert AS (" + u + " RETURNING *) " + i + " " +
		"WHERE NOT EXISTS (SELECT * FROM upsert) " +
		"RETURNING path"

	mopts := Msg_GetModOpts(ctx)
	effroot := mopts.Pathroot
	p := effroot + path

	err = DB.QueryRowA(ctx, "Post message at path: "+path, q, p, depth, title, plaintext, html, user.GetUserName()).Scan(&newpath)

	if err == ErrNoRows {
		err = nil
	} else if err != nil {
		ctx.Logf("Could not post message to path %s: %s", path, err.Error())
		return
	}

	if notify && Config.Msg_mon_from != "" && Config.Msg_mon_to != "" {
		poster := user.GetFullName()
		ue, err := user.GetPriEmail(ctx, false)
		if err == nil {
			poster += " <" + ue.Email + ">"
		}

		src_name := user.GetFullName()
		src_mail := Config.Msg_mon_from

		dst_name := ""
		dst_mail := Config.Msg_mon_to

		prefix := true
		subject := mopts.Title + " :: " + title

		body := plaintext
		regards := false
		footer := "Posted by " + poster + "\n" + "URL: " + mopts.URLpfx + path
		sysfooter := true

		err = Mail(ctx, src_name, src_mail, dst_name, dst_mail, prefix, subject, body, regards, footer, sysfooter)
		if err != nil {
			ctx.Errf("Sending message notification from %s failed: %s", src_mail, err.Error())
			/* errors for this do not feed back to the user */
			err = nil
		}
	}

	return
}

func Msg_Create(ctx PfCtx, path string, title string, plaintext string, notify bool) (newpath string, err error) {
	newpath, err = Msg_Create_With_User(ctx, ctx.TheUser(), path, title, plaintext, notify)
	return
}

func Msg_Post(ctx PfCtx, path string, title string, plaintext string) (newpath string, err error) {
	err = Msg_PathValid(ctx, &path)
	if err != nil {
		return
	}

	t := Msg_PathType(ctx, path)

	if t == MSGTYPE_SECTION && !ctx.IsSysAdmin() {
		err = errors.New("Non-sysadmins cannot post to sections")
		return
	}

	/* We use the total count of messages as an unique component of the path inside the path */
	/* TODO: this does expose the amount of messages in the system, hence we should figure out a better way */
	var newid uint64
	q := "SELECT COUNT(*) FROM msg_messages"
	err = DB.QueryRow(q).Scan(&newid)
	if err != nil {
		return
	}

	/* New component */
	path += strconv.FormatUint(newid, 10) + Msg_sep

	return Msg_Create(ctx, path, title, plaintext, true)
}

func msg_list(ctx PfCtx, args []string) (err error) {
	path := args[0]

	msgs, err := Msg_GetThread(ctx, path, 1, 1, 0, 0)
	if err != nil {
		return
	}

	if len(msgs) == 0 {
		ctx.OutLn("No messages")
		return
	}

	for _, msg := range msgs {
		ctx.OutLn("%s >>%s<< >>%s<<", msg.Path, msg.Title, msg.Plaintext)
	}

	return
}

func msg_get(ctx PfCtx, args []string) (err error) {
	path := args[0]
	prop := args[1]

	msg, err := Msg_Get(ctx, path)

	if err != nil {
		return
	}

	switch prop {
	case "title":
		ctx.OutLn(msg.Title)
		break

	case "plaintext":
		ctx.Out(msg.Plaintext)
		break

	case "html":
		ctx.Out(string(msg.HTML))
		break

	case "username":
		ctx.Out(msg.UserName)
		break

	case "fullname":
		ctx.Out(msg.FullName)
		break

	default:
		err = errors.New("Unknown property " + prop)
		break
	}

	return
}

func msg_show(ctx PfCtx, args []string) (err error) {
	path := args[0]

	msgs, err := Msg_GetThread(ctx, path, 1, 1, 0, 0)
	if err != nil {
		return
	}

	if len(msgs) == 0 {
		ctx.OutLn("No messages")
		return
	}

	depth := Msg_PathDepth(ctx, path)

	for _, msg := range msgs {
		id := ""

		for i := 0; i < (msg.Depth - depth); i++ {
			id += " "
		}

		seen := "New"
		if msg.Seen.Valid {
			seen = ToString(msg.Seen.Time)
		}

		ctx.OutLn("%s----------------------------------------- %d", id, msg.Id)
		ctx.OutLn("%sPath: %s", id, msg.Path)
		ctx.OutLn("%sTitle: %s", id, msg.Title)
		ctx.OutLn("%sUser: %s (%s)", id, msg.FullName, msg.UserName)
		ctx.OutLn("%sSeen: %s", id, seen)
		ctx.OutLn("%s-----------------------------------------", id)
		ctx.OutLn("%s%s", id, msg.Plaintext)
		ctx.OutLn("%s-----------------------------------------", id)
		ctx.OutLn("")
	}

	return
}

func msg_post(ctx PfCtx, args []string) (err error) {
	path := args[0]
	title := args[1]
	plain := args[2]

	_, err = Msg_Post(ctx, path, title, plain)
	return
}

func msg_create(ctx PfCtx, args []string) (err error) {
	path := args[0]
	title := args[1]
	plain := args[2]

	/* Verify that the name is correct */
	err = Msg_PathValid(ctx, &path)
	if err != nil {
		return
	}

	_, err = Msg_Create(ctx, path, title, plain, true)
	return
}

func msg_import_id(txt string) (id string, err error) {
	i := strings.Index(txt, ")")
	if i == -1 {
		err = errors.New("Header in wrong format: >>>" + txt + "<<<")
		return
	}

	id = strings.TrimSpace(txt[0:i])
	return
}

func msg_subject(txt string) (subject string, err error) {
	id, err := msg_import_id(txt)
	if err != nil {
		return
	}

	subject = strings.TrimSpace(txt[len(id)+1:])
	return
}

func msg_import(ctx PfCtx, args []string) (err error) {
	rootpath := args[0]
	fn := args[1]

	/* Support files in SHARE: (File_root) */
	fn, err = System_SharedFile(fn)
	if err != nil {
		return
	}

	file, err := os.Open(fn)
	if err != nil {
		return
	}

	defer file.Close()

	cnt := 0

	section := ""
	spath := ""
	question := ""
	path := ""
	subject := ""
	body := ""
	sid := ""

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		/* Detect sections */
		if len(line) >= 2 {
			/* Section? */
			if line[0:2] == "# " {

				if question != "" {
					/* Create new question from old body */
					sid, err = msg_import_id(question[3:])
					if err != nil {
						return
					}

					path = spath + "Q" + sid + "/"
					subject, err = msg_subject(question[3:])
					if err != nil {
						return
					}

					_, err = Msg_Create(ctx, path, subject, body, false)
					if err != nil {
						return
					}
					cnt++

					question = ""
					body = ""
				}

				if body != "" {
					err = errors.New("Stray message body: " + body)
					return
				}

				/* Create new section */
				section = line[5:]
				sid, err = msg_import_id(section)
				if err != nil {
					return
				}

				spath = rootpath + "S" + sid + "/"
				subject, err = msg_subject(section)
				if err != nil {
					return
				}

				_, err = Msg_Create(ctx, spath, subject, subject, false)
				if err != nil {
					return
				}
				cnt++

				/* Done here */
				continue
			}

			/* Question? */
			if line[0:3] == "## " {

				if question != "" {
					/* Create new question from old body */
					sid, err = msg_import_id(question[3:])
					if err != nil {
						return
					}

					path = spath + "Q" + sid + "/"
					subject, err = msg_subject(question[3:])
					if err != nil {
						return
					}

					_, err = Msg_Create(ctx, path, subject, body, false)
					if err != nil {
						return
					}
					cnt++
				}

				/* The current question */
				question = line[3:]
				body = ""

				/* Done here */
				continue
			}
		}

		/* Add it to the body */
		body += line + "\n"
	}

	if question != "" {
		sid, err = msg_import_id(question[3:])
		if err != nil {
			return
		}

		path = spath + "Q" + sid + "/"
		subject, err = msg_subject(question[3:])
		if err != nil {
			return
		}

		_, err = Msg_Create(ctx, path, subject, body, false)
		if err == nil {
			cnt++
		}
	}

	if err == nil {
		ctx.OutLn("Imported %d messages", cnt)
	}

	return
}

func msg_mark(ctx PfCtx, args []string) (err error) {
	path := args[0]
	mark := args[1]

	msg, err := Msg_Get(ctx, path)

	if err != nil {
		return
	}

	switch mark {
	case "seen", "read":
		if msg.Seen.Valid {
			err = errors.New("Already seen")
			return
		}

		err = Msg_MarkSeen(ctx, msg)
		return

	case "new":
		err = Msg_MarkNew(ctx, msg)
		return

	default:
		err = errors.New("Unknown mark: " + mark)
		break
	}

	return
}

func msg_purge(ctx PfCtx, args []string) (err error) {
	path := args[0]

	err = Msg_PathValid(ctx, &path)
	if err != nil {
		return
	}

	mopts := Msg_GetModOpts(ctx)
	effroot := mopts.Pathroot
	p := effroot + path

	q := "DELETE FROM msg_messages WHERE path LIKE $1"
	err = DB.Exec(ctx, "Purge Message Path $1", -1, q, p+"%")
	if err == nil {
		ctx.OutLn("Purged all messages under %s", path)
	}

	return
}

func Msg_menu(ctx PfCtx, args []string) (err error) {
	menu := NewPfMenu([]PfMEntry{
		{"list", msg_list, 1, 1, []string{"path"}, PERM_USER, "List messages in a given path"},
		{"get", msg_get, 2, 2, []string{"path", "property"}, PERM_USER, "Get a message, property = title,plaintext,html,username,fullname"},
		{"show", msg_show, 1, 1, []string{"path"}, PERM_USER, "Get a full message thread"},
		{"post", msg_post, 3, 3, []string{"path", "title", "plaintext"}, PERM_USER, "Post a message"},
		{"create", msg_create, 3, 3, []string{"path", "title", "plaintext"}, PERM_SYS_ADMIN, "Post a message"},
		{"import", msg_import, 2, 2, []string{"path", "file"}, PERM_SYS_ADMIN, "Import a Markdown file as a source of messages"},
		{"mark", msg_mark, 2, 2, []string{"path", "mark"}, PERM_USER, "Mark a message as read or new"},
		{"purge", msg_purge, 1, 1, []string{"path"}, PERM_SYS_ADMIN, "Purge a subthread"},
	})

	err = ctx.Menu(args, menu)
	return
}
