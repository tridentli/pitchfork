package pitchfork

import (
	"bufio"
	"errors"
	"html/template"
	"os"
	"path/filepath"
	"strings"
)

const Wiki_Perms_Export os.FileMode = 0755

// "wikipath", "dir"
func wiki_export(ctx PfCtx, args []string) (err error) {
	path := strings.TrimSpace(args[0])
	dname := strings.TrimSpace(args[1])

	if path[len(path)-1:] != "/" {
		err = errors.New("Path does not end in slash; it needs to indicate the root of the wiki to export from")
		return
	}

	if dname[len(dname)-1:] != "/" {
		err = errors.New("Path does not end in slash; it needs to be a directory that will become the file root of the exported directory")
		return
	}

	dname = dname[:len(dname)-1]

	/* Check destination directory */
	err = os.MkdirAll(dname, Wiki_Perms_Export)
	if err != nil && err != os.ErrExist {
		err = errors.New("Destination path '" + dname + "' creation failed: " + err.Error())
		return
	}

	/* Static files: Copy the webroots into it (in reverse) */
	for i, _ := range Config.File_roots {
		root := Config.File_roots[len(Config.File_roots)-1-i]
		uri := filepath.Join(root, "rendered/", path)

		/* Does it have a webroot directory to copy? */
		if _, err = os.Stat(uri); err != nil {
			continue
		}

		ctx.OutLn("Adding Webroot %s", root)

		/* Recursive copy */
		err = CopyDir(ctx, true, uri, dname)
		if err != nil {
			return
		}
	}

	/* Fetch all menu.inc files */
	/* TODO: Use prefix tree? */
	menus := make(map[string]string)

	q := "SELECT path, html_body " +
		"FROM wiki_namespace " +
		"INNER JOIN wiki_page_rev ON wiki_namespace.page_id = wiki_page_rev.page_id " +
		"WHERE path LIKE $1 " +
		"AND path LIKE '%menu.inc' " +
		"ORDER BY path ASC "

	rows, err := DB.Query(q, path+"%")
	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		file_path := ""
		html_body := ""

		err = rows.Scan(&file_path, &html_body)
		if err != nil {
			return
		}

		/* Only store the path name not the 'menu.inc' portion */
		subpath, _ := filepath.Abs(filepath.Dir(file_path))

		/* Pre-rendered menu */
		menus[subpath] = html_body
	}

	/* HTML render all files except *.inc */
	q = "SELECT path, title, html_toc, html_body " +
		"FROM wiki_namespace " +
		"INNER JOIN wiki_page_rev ON wiki_namespace.page_id = wiki_page_rev.page_id " +
		"WHERE path LIKE $1 " +
		"AND path NOT LIKE '%.inc' " +
		"ORDER BY path ASC "

	rows, err = DB.Query(q, path+"%")
	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		file_path := ""
		title := ""
		html_toc := ""
		html_body := ""
		subpath := ""

		err = rows.Scan(&file_path, &title, &html_toc, &html_body)
		if err != nil {
			return
		}

		/* The directory of this file */
		subpath, err = filepath.Abs(filepath.Dir(file_path))

		/* Append index.html? */
		if file_path[len(file_path)-1:] == "/" {
			file_path += "index.html"
		}

		/* Find the best match menu */
		mlen := 0
		menu := ""
		menun := ""

		for m, mp := range menus {
			ml := len(m)

			if mp == "" || ml < mlen || !strings.HasPrefix(subpath, m) {
				continue
			}

			mlen = ml
			menu = mp
			menun = m

			ctx.OutLn("Found menu %s for %s", m, file_path)
		}

		ctx.OutLn("Using menu %s for %s (%s)", menun, file_path, subpath)

		/* The directory */
		fname := dname + subpath

		/* Make sure it exists */
		err = os.MkdirAll(fname, Wiki_Perms_Export)
		if err != nil && err != os.ErrExist {
			err = errors.New("Destination path '" + dname + "' creation failed: " + err.Error())
			return
		}

		/* The full filename */
		fname = dname + file_path

		fo, err := os.Create(fname)
		if err != nil {
			panic(err)
		}

		w := bufio.NewWriter(fo)

		ctx.OutLn("Rendering " + fname)

		type Page struct {
			Title   string
			Menu    template.HTML
			TOC     template.HTML
			Content template.HTML
		}

		/* Blackfriday generated, blue-monday filtered, thus trusted */
		h_menu := HEB(menu)
		h_toc := HEB(html_toc)
		h_body := HEB(html_body)

		/* The page definition */
		p := Page{title, h_menu, h_toc, h_body}

		/* Render it to the file */
		tmp := Template_Get()
		err = tmp.ExecuteTemplate(w, "inc/render.tmpl", p)
		if err != nil {
			panic(err)
		}

		w.Flush()
		fo.Close()
	}

	return
}
