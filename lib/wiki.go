package pitchfork

import (
	"errors"
	"html/template"
	"io/ioutil"
	fp "path/filepath"
	"strconv"
	"strings"
	"time"
)

type PfWikiOpts struct {
	PfModOptsS
}

func Wiki_GetModOpts(ctx PfCtx) PfWikiOpts {
	mopts := ctx.GetModOpts()
	if mopts == nil {
		panic("No File ModOpts configured")
	}

	return mopts.(PfWikiOpts)
}

func Wiki_ModOpts(ctx PfCtx, cmdpfx string, path_root string, web_root string) {
	ctx.SetModOpts(PfWikiOpts{PfModOpts(ctx, cmdpfx, path_root, web_root)})
}

func wiki_PathFix(ctx PfCtx, path string) string {
	mopts := Wiki_GetModOpts(ctx)
	return URL_Append(mopts.Pathroot, path)
}

type PfWikiHTML struct {
	HTML_TOC  template.HTML `pfcol:"html_toc"`
	HTML_Body template.HTML `pfcol:"html_body"`
	Entered   time.Time     `pftable:"wiki_page_rev"`
	UserName  string        `pfcol:"member"`
	FullName  string        `pfcol:"descr" pftable:"member"`
}

type PfWikiMarkdown struct {
	Markdown string `pfcol:"markdown"`
}

type PfWikiRev struct {
	Revision  int
	RevisionB int
	Entered   time.Time
	UserName  string
	FullName  string
	ChangeMsg string
}

type PfWikiPage struct {
	Path     string
	Entered  time.Time
	Title    string
	FullPath string /* Not in the DB, see Fixup() */
}

func (wiki *PfWikiPage) Fixup(ctx PfCtx) {
	mopts := Wiki_GetModOpts(ctx)
	root := mopts.Pathroot

	/* Strip off the ModRoot */
	wiki.Path = wiki.Path[len(root):]

	/* Full Path */
	wiki.FullPath = URL_Append(root, wiki.Path)
}

type PfWikiResult struct {
	Path    string
	Title   string
	Snippet string
}

func Wiki_TitleComponent(title string) string {
	if title == "" {
		title = "Index"
	} else {
		title = strings.ToUpper(string(title[0])) + title[1:]
	}

	return title
}

func Wiki_Title(path string) (title string) {
	t := strings.Split(path, "/")
	title = Wiki_TitleComponent(t[len(t)-1])
	return
}

func Wiki_RevisionMax(ctx PfCtx, path string) (total int, err error) {
	path = wiki_PathFix(ctx, path)

	q := "SELECT COUNT(*) " +
		"FROM wiki_page_rev r " +
		"INNER JOIN wiki_namespace t ON r.page_id = t.page_id " +
		"WHERE path = $1"

	err = DB.QueryRow(q, path).Scan(&total)

	return total, err
}

func Wiki_RevisionList(ctx PfCtx, path string, offset int, max int) (revs []PfWikiRev, err error) {
	revs = nil
	var rows *Rows

	path = wiki_PathFix(ctx, path)

	q := "SELECT r.revision, r.entered, r.member, member.descr, r.changemsg " +
		"FROM wiki_page_rev r " +
		"INNER JOIN wiki_namespace t ON r.page_id = t.page_id " +
		"INNER JOIN member ON r.member = member.ident " +
		"WHERE path = $1 " +
		"ORDER BY entered DESC "

	if max != 0 {
		q += "LIMIT $3 OFFSET $2"
		rows, err = DB.Query(q, path, offset, max)
	} else {
		rows, err = DB.Query(q, path)
	}

	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		var r PfWikiRev

		err = rows.Scan(&r.Revision, &r.Entered, &r.UserName, &r.FullName, &r.ChangeMsg)
		if err != nil {
			revs = nil
			return
		}

		if r.Revision > 0 {
			r.RevisionB = r.Revision - 1
		} else {
			r.RevisionB = 0
		}

		revs = append(revs, r)
	}
	return
}

func Wiki_SearchMax(ctx PfCtx, search string) (total int, err error) {
	/* Restrict the path */
	path := wiki_PathFix(ctx, "") + "%"

	searchq := "%" + search + "%"

	q := "SELECT COUNT(*) FROM " +
		"(SELECT DISTINCT ON (r.page_id) t.path, r.title, r.markdown " +
		"FROM wiki_page_rev r " +
		"INNER JOIN wiki_namespace t ON r.page_id = t.page_id " +
		"INNER JOIN member ON r.member = member.ident " +
		"WHERE t.path ILIKE $1 " +
		"AND r.markdown ILIKE $2 " +
		"ORDER BY r.page_id, path DESC) t"

	err = DB.QueryRow(q, path, searchq).Scan(&total)

	return total, err
}

func Wiki_SearchList(ctx PfCtx, search string, offset int, max int) (results []PfWikiResult, err error) {
	results = nil
	var rows *Rows

	/* Restrict the path */
	path := wiki_PathFix(ctx, "")
	plen := len(path)
	path += "%"

	/* Search match */
	searchq := "%" + search + "%"

	q := "SELECT DISTINCT ON (r.page_id) t.path, r.title, r.markdown " +
		"FROM wiki_page_rev r " +
		"INNER JOIN wiki_namespace t ON r.page_id = t.page_id " +
		"INNER JOIN member ON r.member = member.ident " +
		"WHERE t.path ILIKE $1 " +
		"AND r.markdown ILIKE $2 " +
		"ORDER BY r.page_id, path DESC "

	if max != 0 {
		q += "LIMIT $4 OFFSET $3"
		rows, err = DB.Query(q, path, searchq, offset, max)
	} else {
		rows, err = DB.Query(q, path, searchq)
	}

	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		var r PfWikiResult

		err = rows.Scan(&r.Path, &r.Title, &r.Snippet)
		if err != nil {
			results = nil
			return
		}

		/* Only show the subpart of the path */
		r.Path = r.Path[plen:]

		/* Cut down the snippet quite a bit */
		l := len(r.Snippet)
		if l > 128 {
			o := strings.Index(r.Snippet, search)
			if o < 64 {
				o = 0
			} else {
				o -= 64
			}

			if l > (o + 128) {
				l = o + 128
			}

			r.Snippet = "..." + r.Snippet[o:l] + "..."
		}

		results = append(results, r)
	}
	return
}

func Wiki_ChildPagesMax(ctx PfCtx, path string) (total int, err error) {
	path = wiki_PathFix(ctx, path)

	var args []interface{}

	q := "SELECT COUNT(*) " +
		"FROM wiki_namespace " +
		"INNER JOIN wiki_page_rev ON wiki_namespace.page_id = wiki_page_rev.page_id"

	/* All children */
	DB.Q_AddWhere(&q, &args, "path", "LIKE", path+"%", true, false, 0)

	/* Not the current path */
	DB.Q_AddWhere(&q, &args, "path", "<>", path, true, false, 0)

	err = DB.QueryRow(q, args...).Scan(&total)

	return total, err
}

func Wiki_ChildPagesList(ctx PfCtx, path string, offset int, max int) (paths []PfWikiPage, err error) {
	paths = nil

	query_path := path
	path = wiki_PathFix(ctx, path)

	var rows *Rows
	var args []interface{}

	/* Force a directory */
	path = URL_EnsureSlash(path)

	q := "SELECT path, title, entered " +
		"FROM wiki_namespace " +
		"INNER JOIN wiki_page_rev ON wiki_namespace.page_id = wiki_page_rev.page_id"

	/* All children */
	DB.Q_AddWhere(&q, &args, "path", "LIKE", path+"%", true, false, 0)

	/* Not the current path */
	DB.Q_AddWhere(&q, &args, "path", "<>", path, true, false, 0)

	q += "ORDER BY path ASC "

	if max != 0 {
		q += " LIMIT "
		DB.Q_AddArg(&q, &args, max)
	}

	if offset != 0 {
		q += " OFFSET "
		DB.Q_AddArg(&q, &args, offset)
	}

	rows, err = DB.Query(q, args...)
	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		var p PfWikiPage

		err = rows.Scan(&p.Path, &p.Title, &p.Entered)
		if err != nil {
			paths = nil
			return
		}

		p.Fixup(ctx)

		if PathOffset(p.Path, query_path) == 0 {
			/* Add it to the list */
			paths = append(paths, p)
		}
	}

	return
}

func (wiki *PfWikiMarkdown) Fetch(ctx PfCtx, path string, rev string) (err error) {
	path = wiki_PathFix(ctx, path)

	p := []string{"path"}
	v := []string{path}

	if rev != "" {
		p = append(p, "revision")
		v = append(v, rev)
	}

	j := "INNER JOIN wiki_namespace t ON wiki_page_rev.page_id = t.page_id"
	o := "ORDER BY revision DESC, entered DESC"
	err = StructFetchA(wiki, "wiki_page_rev", j, p, v, o, true)
	if err == ErrNoRows {
		wiki.Markdown = "(This page is still empty, please [edit me](?s=edit))"
	} else if err != nil {
		Log("Fetch('" + path + "'): " + err.Error())
	}
	return
}

func (wiki *PfWikiHTML) Fetch(ctx PfCtx, path string, rev string) (err error) {
	path = wiki_PathFix(ctx, path)

	p := []string{"path"}
	v := []string{path}

	if rev != "" {
		p = append(p, "revision")
		v = append(v, rev)
	}

	j := "INNER JOIN wiki_namespace t ON wiki_page_rev.page_id = t.page_id " +
		"INNER JOIN member ON wiki_page_rev.member = member.ident "
	o := "ORDER BY revision DESC, wiki_page_rev.entered DESC"
	err = StructFetchA(wiki, "wiki_page_rev", j, p, v, o, true)
	if err == ErrNoRows {
		wiki.HTML_Body = "(This page is still empty, please <a href=\"?s=edit\">edit me</a>)"
	} else if err != nil {
		Log("Fetch('" + path + "'): " + err.Error())
	}

	return
}

func wiki_updateA(ctx PfCtx, path string, message string, title string, markdown string) (err error) {
	user := ctx.SelectedUser().GetUserName()
	mopts := Wiki_GetModOpts(ctx)

	q := ""
	create := false

	var m PfWikiMarkdown
	err = m.Fetch(ctx, path, "")
	if err == ErrNoRows {
		/* Need to create it */
		create = true
		err = nil
	} else if err != nil {
		err = errors.New("Could not retrieve existing page")
		return
	} else {
		/* Did it change? */
		if string(m.Markdown) == markdown || markdown == "autocreated" {
			ctx.OutLn("Markdown did not change")
			return
		}
	}

	/* Fixup the path (Fetch() does that itself) */
	path = wiki_PathFix(ctx, path)

	/* Render & Sanitize, body & TOC */
	html_body := PfRender(markdown, false)
	html_toc := PfRender(markdown, true)

	/* Trim the TOC, so that we do not render empty TOCs */
	html_tocs := strings.TrimSpace(string(html_toc))

	/*
	 * XXX: 'member' name might be exposed to other group
	 * if pages shared between groups that do not share members
	 */

	page_id := 0

	if create {
		q = "INSERT INTO wiki_namespace " +
			"(path) " +
			"VALUES($1) " +
			"RETURNING page_id"
		err = DB.QueryRowA(ctx,
			"Created Wiki Page $1",
			q, path).Scan(&page_id)
		if err != nil {
			Logf("Could not insert new wiki path %s", path)
		}
	} else {
		q = "SELECT page_id " +
			"FROM wiki_namespace " +
			"WHERE path = $1"
		err = DB.QueryRow(q, path).Scan(&page_id)
		if err != nil {
			Logf("Could not find existing page_id for path %s", path)
		}
	}

	if err != nil {
		Logf("Could not find page_id for path %s", path)
		return
	}

	/* New revision for this page */
	q = "INSERT INTO wiki_page_rev " +
		"(page_id, revision, title, markdown, html_body, html_toc, member, changemsg) " +
		"SELECT $1, (COALESCE(MAX(revision), 0) + 1), $2, $3, $4, $5, $6, $7 " +
		"FROM wiki_page_rev " +
		"WHERE page_id = $1"
	err = DB.Exec(ctx,
		"Updated Wiki page "+path,
		1, q,
		page_id, title, markdown, html_body, html_tocs, user, message)
	if err != nil {
		return
	}

	/* Walk the directory back and ensure all stages exist */
	path = strings.Replace(path, mopts.Pathroot, "", 1)
	path_len := len(path)

	if path[path_len-1] == '/' {
		path = path[:path_len-1]
	}

	dir_path := fp.Dir(path) + "/"

	if len(dir_path) > 1 {
		wiki_updateA(ctx, dir_path, "autocreated", "autocreated", "autocreated")
	}

	return
}

func wiki_update(ctx PfCtx, args []string) (err error) {
	path := args[0]
	message := args[1]
	title := args[2]
	markdown := args[3]

	return wiki_updateA(ctx, path, message, title, markdown)
}

func wiki_updatef(ctx PfCtx, args []string) (err error) {
	path := args[0]
	message := args[1]
	title := args[2]
	markdownf := args[3]

	b, err := ioutil.ReadFile(markdownf)
	if err != nil {
		return
	}

	markdown := string(b)

	return wiki_updateA(ctx, path, message, title, markdown)
}

func wiki_get(ctx PfCtx, args []string) (err error) {
	path := args[0]
	fmt := args[1]

	rev := ""
	if len(args) == 3 {
		rev = args[2]
	}

	switch fmt {
	case "markdown":
		var w PfWikiMarkdown
		err = w.Fetch(ctx, path, rev)
		if err != nil {
			if err == ErrNoRows {
				err = errors.New("No such page")
			}
		}
		ctx.Out(w.Markdown)
		break

	case "html":
		var w PfWikiHTML
		err = w.Fetch(ctx, path, rev)
		if err != nil {
			if err == ErrNoRows {
				err = errors.New("No such page")
			}
		}
		ctx.Out(string(w.HTML_Body))
		break

	default:
		err = errors.New("Unknown format " + fmt)
		break
	}

	return
}

func wiki_list(ctx PfCtx, args []string) (err error) {
	path := args[0]

	var wps []PfWikiPage
	wps, err = Wiki_ChildPagesList(ctx, path, 0, 0)

	if err != nil {
		return
	}

	for _, wp := range wps {
		ctx.OutLn(wp.Path)
	}

	return
}

func wiki_getrevs(ctx PfCtx, path string, revA string, revB string) (a string, b string, err error) {
	var mA PfWikiMarkdown
	err = mA.Fetch(ctx, path, revA)
	if err != nil {
		if err == ErrNoRows {
			err = errors.New("No such page ('" + path + "') / revision (a: " + revA + ")")
		}
		return
	}

	var mB PfWikiMarkdown
	err = mB.Fetch(ctx, path, revB)
	if err != nil {
		if err == ErrNoRows {
			err = errors.New("No such page ('" + path + "') / revision (a: " + revB + ")")
		}
		return
	}

	return mA.Markdown, mB.Markdown, err
}

func Wiki_Diff(ctx PfCtx, path string, revA string, revB string) (diff []PfDiff, err error) {
	var a string
	var b string

	a, b, err = wiki_getrevs(ctx, path, revA, revB)
	if err != nil {
		return
	}

	return DoDiff(a, b), nil
}

func wiki_diff(ctx PfCtx, args []string) (err error) {
	path := args[0]
	revA := args[1]
	revB := args[2]

	var a string
	var b string

	a, b, err = wiki_getrevs(ctx, path, revA, revB)
	if err != nil {
		return
	}

	Diff_Out(ctx, a, b)

	return nil
}

func wiki_move(ctx PfCtx, args []string) (err error) {
	path := wiki_PathFix(ctx, args[0])
	newpath := wiki_PathFix(ctx, args[1])
	children := args[2]

	if path == newpath {
		return errors.New("Paths are the same")
	}

	var rows *Rows
	q := "SELECT path " +
		"FROM wiki_namespace " +
		"WHERE path LIKE $1 " +
		"ORDER BY path"

	pathq := path

	if IsTrue(children) {
		pathq = path + "%"
	}

	rows, err = DB.Query(q, pathq)

	if err == ErrNoRows {
		err = errors.New("No such page")
		return
	}

	if err != nil {
		ctx.OutLn("Something went wrong")
		return
	}

	defer rows.Close()

	pl := len(path)

	c := 0
	for rows.Next() {
		var p string
		err = rows.Scan(&p)

		np := URL_Append(newpath, p[pl:])

		q := "UPDATE wiki_namespace " +
			"SET path = $1 " +
			"WHERE path = $2"
		err = DB.Exec(ctx,
			"Moved Wiki page $2 to $1",
			1, q,
			np, p)
		if err != nil {
			return errors.New("Could not move the page")
		}

		c++
	}

	ctx.OutLn("Moved page and " + strconv.Itoa(c) + " children")
	return nil
}

func wiki_delete(ctx PfCtx, args []string) (err error) {
	path := wiki_PathFix(ctx, args[0])
	children := args[1]

	var rows *Rows
	q := "SELECT path " +
		"FROM wiki_namespace " +
		"WHERE path LIKE $1 " +
		"ORDER BY path"

	pathq := path

	if IsTrue(children) {
		pathq = path + "%"
	}

	rows, err = DB.Query(q, pathq)

	if err == ErrNoRows {
		err = errors.New("No such page")
		return
	}

	if err != nil {
		ctx.OutLn("Something went wrong")
		return
	}

	defer rows.Close()

	c := 0
	for rows.Next() {
		var p string
		err = rows.Scan(&p)

		q := "DELETE FROM wiki_namespace " +
			"WHERE path = $1"
		err = DB.Exec(ctx,
			"Delete Wiki page $1",
			1, q, p)
		if err != nil {
			return errors.New("Could not delete the page")
		}

		c++
	}

	ctx.OutLn("Deleted page and " + strconv.Itoa(c) + " children")
	return nil
}

func wiki_copy(ctx PfCtx, args []string) (err error) {
	path := wiki_PathFix(ctx, args[0])
	newpath := wiki_PathFix(ctx, args[1])
	children := args[2]

	if path == newpath {
		return errors.New("Paths are the same, at least one has to differ")
	}

	var rows *Rows
	q := "SELECT path, page_id " +
		"FROM wiki_namespace " +
		"WHERE path LIKE $1 " +
		"ORDER BY path"

	pathq := path

	if IsTrue(children) {
		pathq = path + "%"
	}

	rows, err = DB.Query(q, pathq)

	if err == ErrNoRows {
		err = errors.New("No such page")
		return
	}

	if err != nil {
		ctx.OutLn("Something went wrong")
		return
	}

	defer rows.Close()

	pl := len(path)

	c := 0
	for rows.Next() {
		var p string
		var page_id int

		err = rows.Scan(&p, &page_id)

		np := URL_Append(newpath, p[pl:])

		q := "INSERT INTO wiki_namespace " +
			"(path, page_id) " +
			"VALUES($1, $2)"
		err = DB.Exec(ctx,
			"Copy Wiki page "+path+" to $1",
			1, q,
			np, page_id)
		if err != nil {
			return errors.New("Could not copy the page")
		}

		c++
	}

	ctx.OutLn("Copied page and " + strconv.Itoa(c) + " children")
	return nil
}

func Wiki_menu(ctx PfCtx, args []string) (err error) {
	menu := NewPfMenu([]PfMEntry{
		{"update", wiki_update, 4, 4, []string{"wikipath", "message", "title", "markdown"}, PERM_USER, "Update a Wiki page"},
		{"updatef", wiki_updatef, 4, 4, []string{"wikipath", "message", "title", "markdownfile"}, PERM_SYS_ADMIN | PERM_USER, "Update a Wiki page from a file"},
		{"get", wiki_get, 2, 3, []string{"wikipath", "format", "revision"}, PERM_USER, "Get a Wiki page in either markdown or html format"},
		{"list", wiki_list, 1, 1, []string{"wikipath"}, PERM_USER, "List wikipages below a given path"},
		{"diff", wiki_diff, 3, 3, []string{"wikipath", "revision", "revisionB"}, PERM_USER, "Diff two revisions of a wiki page"},
		{"move", wiki_move, 3, 3, []string{"wikipath", "newpath#wikipath", "movekids#bool"}, PERM_USER, "Move a Wiki page"},
		{"delete", wiki_delete, 2, 2, []string{"wikipath", "deletekids#bool"}, PERM_USER, "Delete a Wiki page"},
		{"copy", wiki_copy, 3, 3, []string{"wikipath", "newwikipath#wikipath", "copykids#bool"}, PERM_USER, "Copy a Wiki page"},
		{"import", wiki_import, 3, 3, []string{"format", "file", "wikipath"}, PERM_SYS_ADMIN, "Import from a .triwiki archive"},
		{"export", wiki_export, 2, 2, []string{"wikipath", "dir"}, PERM_SYS_ADMIN, "Export/render wiki to a static version in given path"},
	})

	err = ctx.Menu(args, menu)
	return
}
