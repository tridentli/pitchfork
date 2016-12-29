package pitchfork

import (
	"bytes"
	"crypto/sha512"
	"errors"
	"io"
	"math"
	"mime"
	"os"
	fp "path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const File_Perms_Dir os.FileMode = 0700
const File_Perms_File os.FileMode = 0600

var ErrFilePathExists = errors.New("Path already exists")

type PfFileOpts struct {
	PfModOptsS
}

func File_GetModOpts(ctx PfCtx) PfFileOpts {
	mopts := ctx.GetModOpts()
	if mopts == nil {
		panic("No File ModOpts configured")
	}

	output, ok := mopts.(PfFileOpts)
	if !ok {
		was, ok := mopts.(PfModOpts)
		return PfFileOpts{PfModOpts(was.ctx, was.cmdpfx, was.path_root, was.web_root)}
	}

	return output
}

func File_ModOpts(ctx PfCtx, cmdpfx string, path_root string, web_root string) {
	ctx.SetModOpts(PfFileOpts{PfModOpts(ctx, cmdpfx, path_root, web_root)})
}

func file_ApplyModOpts(ctx PfCtx, path string) string {
	mopts := File_GetModOpts(ctx)
	return URL_Append(mopts.Pathroot, path)
}

type PfFile struct {
	File_id      int       `pfcol:"id" pftable:"file"`
	Path         string    `pfcol:"path" pftable:"file_namespace"`
	Filename     string    `pfcol:"filename" pftable:"file"`
	Revision     int       `pfcol:"revision"`
	Entered      time.Time `pfcol:"entered" pftable:"file_rev"`
	Description  string    `pfcol:"description"`
	SHA512       string    `pfcol:"sha512"`
	Size         int64     `pfcol:"size"`
	MimeType     string    `pfcol:"mimetype"`
	UserName     string    `pfcol:"member" pftable:"file_rev"`
	FullName     string    `pfcol:"descr" pftable:"member"`
	ChangeMsg    string    `pfcol:"changemsg"`
	FullPath     string    /* Not in the DB, see ApplyModOpts() */
	FullFileName string    /* Not in the DB, see ApplyModOpts() */
}

type PfFileResult struct {
	Path    string
	Snippet string
}

func File_RevisionMax(ctx PfCtx, path string) (total int, err error) {
	q := "SELECT COUNT(*) " +
		"FROM file_rev r " +
		"INNER JOIN file_namespace t ON r.file_id = t.file_id " +
		"WHERE path = $1"

	mopts := File_GetModOpts(ctx)
	path = URL_Append(mopts.Pathroot, path)

	err = DB.QueryRow(q, path).Scan(&total)

	return total, err
}

func File_RevisionList(ctx PfCtx, path string, offset int, max int) (revs []PfFile, err error) {
	revs = nil
	var rows *Rows
	var args []interface{}

	mopts := File_GetModOpts(ctx)
	path = URL_Append(mopts.Pathroot, path)

	q := "SELECT f.id, path, filename, revision, file_rev.entered, " +
		"description, sha512,  size, mimetype, member, " +
		"descr, changemsg " +
		"FROM file_rev " +
		"INNER JOIN file_namespace t ON file_rev.file_id = t.file_id " +
		"INNER JOIN file f ON file_rev.file_id = f.id " +
		"INNER JOIN member ON file_rev.member = member.ident"

	DB.Q_AddWhereAnd(&q, &args, "path", path)

	q += "ORDER BY entered DESC "

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
		var f PfFile

		err = rows.Scan(&f.File_id, &f.Path, &f.Filename, &f.Revision, &f.Entered, &f.Description, &f.SHA512, &f.Size, &f.MimeType, &f.UserName, &f.FullName, &f.ChangeMsg)
		if err != nil {
			revs = nil
			return
		}

		f.ApplyModOpts(ctx)

		/* Add the revision */
		revs = append(revs, f)
	}

	return
}

func File_ChildPagesMax(ctx PfCtx, path string) (total int, err error) {
	var args []interface{}

	path = file_ApplyModOpts(ctx, path)

	q := "SELECT COUNT(*) " +
		"FROM file_namespace " +
		"INNER JOIN file_rev ON file_namespace.file_id = file_rev.file_id " +
		"INNER JOIN file ON file_namespace.file_id = file.id " +
		"INNER JOIN member ON file_rev.member = member.ident"

	/* All children */
	DB.Q_AddWhere(&q, &args, "path", "LIKE", path+"%", true, false, 0)

	/* Not the current path */
	DB.Q_AddWhere(&q, &args, "path", "<>", path, true, false, 0)

	err = DB.QueryRow(q, args...).Scan(&total)

	return
}

func File_ChildPagesList(ctx PfCtx, path string, offset int, max int) (paths []PfFile, err error) {
	paths = nil

	query_path := path
	path = file_ApplyModOpts(ctx, path)

	var rows *Rows
	var args []interface{}

	/* Force a directory */
	path = URL_EnsureSlash(path)

	q := "SELECT file.id, path, filename, revision, file_rev.entered, " +
		"description, sha512,  size, mimetype, member, " +
		"descr, changemsg " +
		"FROM file_namespace " +
		"INNER JOIN file_rev ON file_namespace.file_id = file_rev.file_id " +
		"INNER JOIN file ON file_namespace.file_id = file.id " +
		"INNER JOIN member ON file_rev.member = member.ident"

	/* All children */
	DB.Q_AddWhere(&q, &args, "path", "LIKE", path+"%", true, false, 0)

	/* Not the current path */
	DB.Q_AddWhere(&q, &args, "path", "<>", path, true, false, 0)

	q += " ORDER BY path ASC"

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
		var f PfFile

		err = rows.Scan(&f.File_id, &f.Path, &f.Filename, &f.Revision, &f.Entered, &f.Description, &f.SHA512, &f.Size, &f.MimeType, &f.UserName, &f.FullName, &f.ChangeMsg)
		if err != nil {
			paths = nil
			return
		}

		f.ApplyModOpts(ctx)

		if PathOffset(f.Path, query_path) == 0 {
			paths = append(paths, f)
		}
	}

	return
}

func PathOffset(file_path string, dir_path string) (count int) {
	delta := strings.Replace(file_path, dir_path, "", 1)
	tpl := len(delta) - 1
	if delta[tpl] == '/' {
		delta = delta[0:tpl]
	}
	return strings.Count(delta, "/")
}

func (file *PfFile) Fetch(ctx PfCtx, path string, rev string) (err error) {
	path, err = file_chk_path(path)
	if err != nil {
		return
	}

	mopts := File_GetModOpts(ctx)
	path = URL_Append(mopts.Pathroot, path)

	p := []string{"path"}
	v := []string{path}

	if rev != "" {
		p = append(p, "revision")
		v = append(v, rev)
	}

	j := "INNER JOIN file_namespace ON file_rev.file_id = file_namespace.file_id " +
		"INNER JOIN file ON file_rev.file_id = file.id " +
		"INNER JOIN member ON file_rev.member = member.ident"
	o := "ORDER BY revision DESC, entered DESC"
	err = StructFetchA(file, "file_rev", j, p, v, o, true)
	if err == ErrNoRows {
		/* No reporting */
	} else if err != nil {
		Log(err.Error() + " >>>" + path + "<<<")
	} else {
		file.ApplyModOpts(ctx)
	}

	return
}

/* Add some stuff we do not store in the DB but are useful */
func (file *PfFile) ApplyModOpts(ctx PfCtx) {
	mopts := File_GetModOpts(ctx)
	root := mopts.Pathroot

	/* Strip off the ModRoot */
	file.Path = file.Path[len(root):]

	/* Full Path */
	file.FullPath = URL_Append(root, file.Path)

	if file.Filename != "" {
		file.FullFileName = file_filename(file.Filename, file.Revision)
	}

	return
}

func file_mimetype(path string) (mt string, err error) {
	/* TODO: We should use libmagic or so here, and then reject incorrect extensions */
	ext := strings.ToLower(fp.Ext(path))

	if len(ext) < 2 || ext[0] != '.' {
		err = errors.New("Filenames require extensions")
		return
	}
	ext = ext[1:]

	/* Quick lookup of our own to guarantee that these types are supported */
	types := map[string]string{
		"doc":  "application/msword",
		"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"html": "text/html",
		"md":   "text/markdown", /* RFC7763 */
		"pdf":  "application/pdf",
		"txt":  "text/plain",
		"zip":  "application/zip",
	}

	mt, ok := types[ext]

	if !ok {
		/*
		 * Ask golang which consults possible
		 * mime.types files on the system
		 * but these files might not exist
		 */
		mt = mime.TypeByExtension("." + ext)
	}

	if mt == "" {
		err = errors.New("Unsupported filetype")
		return
	}

	return
}

func File_path_is_dir(path string) (is_dir bool) {
	pl := len(path)

	if pl == 0 || path[pl-1] == '/' {
		return true
	}

	return false
}

func file_chk_path(in string) (out string, err error) {
	/* Require something at least */
	if in == "" {
		err = errors.New("No path provided (empty)")
		return
	}

	if len(in) == 0 || in[0] != '/' {
		err = errors.New("Path has to start with a slash (/)")
		Errf("file_chk_path(%q): %s", in, err.Error())
		return
	}

	/* Trim outside spaces */
	out = strings.TrimSpace(in)

	/*
	 * Test for a variety of broken inputs
	 * Not a smshdtgthr regex as we like to error separately on each part.
	 */
	res := []struct {
		regexp string
		msg    string
		match  bool
	}{
		{`^[a-zA-Z0-9\./,_\+\-\(\)\ ]*$`, "Invalid characters in path, only a-zA-Z0-9./,_+-() and space are allowed", false},
		{`(\.\.\/)`, "Please remove '../' (parent directory) from the path", true},
		{`(\.\/)`, "Please remove './' (current directory) from the path", true},
		{`(\/\/)`, "Please remove '//' (double path separator) from the path", true},
	}

	for i := 0; i < len(res); i++ {
		var matched bool
		matched, err = regexp.MatchString(res[i].regexp, out)

		/* Report any regexp errors that show up to the log */
		if err != nil {
			Errf("file_chk_path(%q): [regexp.MatchString(%s) error): %s", out, res[i].regexp, err.Error())
			/* Override the error message */
			err = errors.New("Internal error")
			return
		}

		/* Did the regexp fire? */
		if matched == res[i].match {
			err = errors.New(res[i].msg)
			Errf("file_chk_path(%q): %s", out, err.Error())
			return
		}
	}

	/* All okay and filtered */
	return
}

/*
 * Generate random filename, using the real filename at the end.
 *
 * This way we ensure unique names but also allow the file
 * to found again if it has to happen that somebody wants
 * to go through all the raw files on the disk.
 *
 * We need unique names as the path can appear in multiple paths
 * and have different files, while on the flipside the same
 * file might be called differently in different paths.
 */
func file_path_to_local(path string) (local string, err error) {
	var pw PfPass
	var loops int

	fname := fp.Base(path)
	local = ""

	num := 1
	for loops := 0; num > 0 && loops < 100; loops++ {
		local, err = pw.GenPass(32)
		if err != nil {
			/* Try again */
			local = ""
			continue
		}

		q := "SELECT COUNT(*) " +
			"FROM file " +
			"WHERE filename LIKE $1"
		err = DB.QueryRow(q, local+"%").Scan(&num)
		if err != nil {
			errstr := "Could not generate random unique filename (SQL)"
			Err(errstr)
			err = errors.New(errstr)
			return
		}
	}

	if err != nil || local == "" || loops >= 100 {
		errstr := "Could not generate random unique filename after 100 loops"
		Err(errstr)
		err = errors.New(errstr)
		return
	}

	/*
	 * Include the base of the filename so it can be
	 * easily identified when going through the raw files
	 */
	local += "_" + fname
	err = nil

	return
}

func file_dirname(filename string) (dname string) {
	/* The root of our files storage */
	dname = Config.Var_root + "files/"
	dname += filename[0:2] + "/" + filename[2:4] + "/"
	return
}

func file_filename(filename string, rev int) (fname string) {
	fname = file_dirname(filename)
	fname += filename + ".r" + strconv.Itoa(rev)
	return
}

const file_hash_chunk = 8192 // we settle for 8KB
/* SHA512 over a file */
func file_hash(filename string) (hashstr string, err error) {
	file, err := os.Open(filename)
	if err != nil {
		return
	}

	defer file.Close()

	info, _ := file.Stat()
	filesize := info.Size()

	blocks := uint64(math.Ceil(float64(filesize) / float64(file_hash_chunk)))

	hash := sha512.New()

	for i := uint64(0); i < blocks; i++ {
		blocksize := int(math.Min(file_hash_chunk, float64(filesize-int64(i*file_hash_chunk))))
		buf := make([]byte, blocksize)

		file.Read(buf)
		io.WriteString(hash, string(buf))
	}

	hashstr = Hex(hash.Sum(nil))

	return
}

func file_store(ctx PfCtx, filename string, file_id int, rev int, file io.Reader) (err error) {
	var out *os.File
	var size int64
	var sha512 string

	/* Ensure that all needed dirs are there */
	fname := file_dirname(filename)
	os.MkdirAll(fname, File_Perms_Dir)

	/* The filename */
	fname = file_filename(filename, rev)

	Dbgf("Storing file into: %s", fname)

	/* Open it */
	out, err = os.OpenFile(fname, os.O_CREATE|os.O_WRONLY, File_Perms_File)
	if err != nil {
		ctx.Errf("Opening file %s failed: %s", fname, err.Error())
		err = errors.New("Could not open destination file")
		return
	}

	/* Copy it */
	size, err = io.Copy(out, file)
	if err != nil {
		ctx.Errf("Copying file %s failed: %s", fname, err.Error())
		err = errors.New("Storing file failed")
		return
	}

	/* Ensure storage */
	err = out.Sync()
	if err != nil {
		ctx.Errf("Syncing file %s failed: %s", fname, err.Error())
		err = errors.New("Sync failed")
		/* Attempt closing (ignore return, like fails) */
		out.Close()
		return
	}

	err = out.Close()
	if err != nil {
		ctx.Errf("Closing file failed: %s", err.Error())
		err = errors.New("Could not close file")
		return
	}

	/* Make as SHA512 of the stored file */
	sha512, err = file_hash(fname)
	if err != nil {
		ctx.Errf("Hashing failed: %s", err.Error())
		err = errors.New("Hashing file failed")
		return
	}

	Dbgf("Stored file %s, size: %d, hash: %s", fname, size, sha512)

	/* Update file size and SHA512 hash */
	q := "UPDATE file_rev " +
		"SET size = $1, sha512 = $2 " +
		"WHERE file_id = $3 " +
		"AND revision = $4 "
	err = DB.Exec(ctx,
		"Uploaded file size set to $1",
		1, q,
		size, sha512, file_id, rev)
	if err != nil {
		err = errors.New("Could not update filesize")
		return
	}

	return
}

func file_add_entry(ctx PfCtx, ftype string, mimetype string, path string, description string, url string) (filename string, file_id int, rev int, err error) {
	var f PfFile

	user := ctx.TheUser().GetUserName()

	path, err = file_chk_path(path)
	if err != nil {
		return
	}

	err = f.Fetch(ctx, path, "")
	if err == nil {
		err = ErrFilePathExists
		return
	} else if err != ErrNoRows {
		ctx.Errf("Expected ErrNoRows for %q: but %s", path, err.Error())
		err = errors.New("Error while checking for existing entry")
		return
	}

	/* Fetch() above already adds modroot */
	mopts := File_GetModOpts(ctx)
	path = URL_Append(mopts.Pathroot, path)

	if url != "" {
		filename = url
	} else {
		/* Random filename, but still including the filename portion of the path */
		filename, err = file_path_to_local(path)
		if err != nil {
			return
		}
	}

	/* Start a transaction */
	err = DB.TxBegin(ctx)
	if err != nil {
		return
	}

	/* Create a new 'file', retrieving it's id */
	q := "INSERT INTO file " +
		"(filename) " +
		"VALUES($1) " +
		"RETURNING id"

	err = DB.QueryRowA(ctx, "Create File "+path, q, filename).Scan(&file_id)
	if err != nil {
		Log("Could not create new file")
		DB.TxRollback(ctx)
		return
	}

	/* Add this file to the namespace */
	q = "INSERT INTO file_namespace " +
		"(path, file_id) " +
		"VALUES($1, $2)"

	err = DB.Exec(ctx,
		"Created "+ftype+" "+path,
		1, q,
		path, file_id)
	if err != nil {
		Log("Could not insert new file_namespace")
		DB.TxRollback(ctx)
		return
	}

	/* New revision for this path */
	q = "INSERT INTO file_rev " +
		"(revision, file_id, description, sha512, size, mimetype, member, changemsg) " +
		"VALUES(1, $1, $2, $3, $4, $5, $6, $7) " +
		"RETURNING revision"
	err = DB.QueryRowA(ctx,
		"Add "+ftype+" "+path+" revision 1",
		q,
		file_id, description, "", 0, mimetype, user, "Added "+ftype+" "+path).Scan(&rev)
	if err != nil {
		DB.TxRollback(ctx)
		return
	}

	/* All okay, commit the transaction */
	err = DB.TxCommit(ctx)

	/* Walk the directory back and ensure all stages exist */
	path = strings.Replace(path, mopts.Pathroot, "", 1)
	path_len := len(path)

	if path[path_len-1] == '/' {
		path = path[:path_len-1]
	}

	dir_path := fp.Dir(path) + "/"

	if len(dir_path) > 1 {
		file_add_dir(ctx, []string{dir_path, "autocreated"})
	}

	return
}

func file_add_dir(ctx PfCtx, args []string) (err error) {
	var f PfFile
	path := args[0]
	description := args[1]

	path, err = file_chk_path(path)
	if err != nil {
		return
	}

	err = f.Fetch(ctx, path, "")
	if err == nil {
		err = ErrFilePathExists
		return
	} else if err != ErrNoRows {
		ctx.Errf("Expected ErrNoRows for %q: but %s", path, err.Error())
		err = errors.New("Error while checking for existing entry")
		return
	}

	if !File_path_is_dir(path) {
		err = errors.New("Path has to start and end with a slash (/) to be a directory")
		return
	}

	_, _, _, err = file_add_entry(ctx, "dir", "inode/directory", path, description, "")

	if err == nil {
		ctx.OutLn("Directory added successfully")
	}

	return
}

func File_add_url(ctx PfCtx, args []string) (err error) {
	path := args[0]
	description := args[1]
	url := args[2]

	path, err = file_chk_path(path)
	if err != nil {
		return
	}

	_, _, _, err = file_add_entry(ctx, "url", "application/url", path, description, url)

	if err == nil {
		ctx.OutLn("URL added successfully")
	}

	return
}

/*
 * Used by the UI directly and also CLI
 */
func File_add_file(ctx PfCtx, path string, description string, file io.Reader) (err error) {
	var filename string
	var file_id int
	var rev int

	path, err = file_chk_path(path)
	if err != nil {
		return
	}

	if path == "" {
		err = errors.New("Empty path given")
		return
	}

	if File_path_is_dir(path) {
		err = errors.New("Path is a directory as it ends in a slash (/)")
		return
	}

	mimetype, err := file_mimetype(path)
	if err != nil {
		return
	}

	/* Insert the file in the DB */
	filename, file_id, rev, err = file_add_entry(ctx, "file", mimetype, path, description, "")
	if err != nil {
		return
	}

	/* Store the file in the File Storage */
	err = file_store(ctx, filename, file_id, rev, file)
	if err != nil {
		return
	}

	return
}

/*
 * !! CLI only !!
 *
 * Don't call through UI as it takes a local filename, don't want to read /etc/passwd ;)
 *
 * This is also why the function is marked as PERM_SYS_ADMIN
 */
func File_add_localfile(ctx PfCtx, args []string) (err error) {
	var file *os.File

	path := args[0]
	description := args[1]
	thefile := args[2]

	if len(thefile) == 0 {
		err = errors.New("Need an actual filename")
		return
	}

	/* Support files in SHARE: (File_root) */
	thefile, err = System_SharedFile(thefile)
	if err != nil {
		return
	}

	mimetype, err := file_mimetype(thefile)
	if err != nil {
		return
	}

	/* Try opening the file */
	file, err = os.Open(thefile)
	if err != nil {
		return
	}

	/* Close the input file when we are done */
	defer file.Close()

	/* When markdown we also render it in HTML */
	desc := description

	if mimetype == "text/markdown" {
		desc += " (Markdown Source)"
	}

	err = File_add_file(ctx, path, desc, file)
	if err != nil {
		return
	}

	/* Nothing else to do when it is not markdown */
	if mimetype != "text/markdown" {
		return
	}

	/* When it is markdown, also render it as HTML */

	info, err := file.Stat()
	if err != nil {
		err = errors.New("Failure statting: " + err.Error())
		return
	}

	/* Seek to the start */
	_, err = file.Seek(0, os.SEEK_SET)
	if err != nil {
		err = errors.New("Could not return to start of file")
	}

	/* Allocate one big buffer and process it all */
	buf := make([]byte, info.Size())
	_, err = io.ReadFull(file, buf)
	if err != nil {
		err = errors.New("Failure reading: " + err.Error())
		return
	}

	markdown := string(buf)

	html_body := PfRender(markdown, false)
	html_toc := PfRender(markdown, true)

	/* Render a pretty TOC */
	html_toc = "<div class=\"wikitoc\">\n" +
		"<b>Table of Contents</b><br />\n" +
		html_toc +
		"</div>"

	md := bytes.NewBufferString(html_toc + html_body)

	/* New attributes for this file */
	path = path[:len(path)-3] + ".html"
	mimetype = "text/html"
	desc = description + " (HTML)"

	/* Insert the file in the DB */
	filename, file_id, rev, err := file_add_entry(ctx, "file", mimetype, path, desc, "")
	if err != nil {
		return
	}

	/* Store the file in the File Storage */
	err = file_store(ctx, filename, file_id, rev, md)
	if err != nil {
		return
	}

	if err == nil {
		ctx.OutLn("File added successfully")
	}

	return
}

/* Called directly by UI and also CLI (TODO) */
func File_Update(ctx PfCtx, path string, desc string, changemsg string, file *os.File) (err error) {
	var rev int

	user := ctx.SelectedUser().GetUserName()

	path, err = file_chk_path(path)
	if err != nil {
		return
	}

	/* Can't update directories */
	if File_path_is_dir(path) {
		err = errors.New("Directories cannot be updated")
		return
	}

	q := ""

	/* Try to get the File */
	var f PfFile
	err = f.Fetch(ctx, path, "")
	if err != nil {
		err = errors.New("Could not retrieve existing file")
		return
	}

	/* TODO: Check SHA512 for changes, thus avoiding need to reload file */

	mimetype, err := file_mimetype(f.FullPath)
	if err != nil {
		return
	}

	h_sha512 := ""

	/* New revision for this file */
	q = "INSERT INTO file_rev " +
		"(revision, file_id, sha512, mimetype, description, member, changemsg) " +
		"(SELECT (COALESCE(MAX(revision), 0) + 1), $1, $2, $3, $4, $5, $6, $7 " +
		"FROM file_rev " +
		"WHERE file_id = $1) " +
		"RETURNING revision"
	err = DB.QueryRowA(ctx,
		"Updated File "+f.FullPath,
		q,
		f.File_id, h_sha512, mimetype, desc, user, changemsg).Scan(&rev)
	if err != nil {
		return
	}

	/* Store the file in the File Storage */
	err = file_store(ctx, f.Filename, f.File_id, rev, file)
	if err != nil {
		return
	}

	return
}

/*
 * !! CLI only !!
 *
 * Don't call through UI as it takes a local filename, don't want to read /etc/passwd ;)
 *
 * This is also why the function is marked as PERM_SYS_ADMIN
 */
func file_update(ctx PfCtx, args []string) (err error) {
	var file *os.File

	path := args[0]
	desc := args[1]
	changemsg := args[2]
	thefile := args[3]

	/* Try opening the file */
	file, err = os.Open(thefile)
	if err != nil {
		return
	}

	err = File_Update(ctx, path, desc, changemsg, file)

	/* Close the input file */
	file.Close()

	return
}

func file_get(ctx PfCtx, args []string) (err error) {
	path := args[0]

	path, err = file_chk_path(path)
	if err != nil {
		return
	}

	rev := ""
	if len(args) == 4 {
		rev = args[3]
	}

	var w PfFile
	err = w.Fetch(ctx, path, rev)
	if err != nil {
		if err == ErrNoRows {
			err = errors.New("No such file")
		}
		return
	}

	ctx.OutLn("%s", file_filename(w.Filename, w.Revision))

	return
}

func file_list(ctx PfCtx, args []string) (err error) {
	path := args[0]

	path, err = file_chk_path(path)
	if err != nil {
		return
	}

	if !File_path_is_dir(path) {
		err = errors.New("Path given is not a directory (need to end in slash)")
		return
	}

	var wps []PfFile
	wps, err = File_ChildPagesList(ctx, path, 0, 0)
	if err != nil {
		return
	}

	for _, wp := range wps {
		if File_path_is_dir(wp.Path) {
			ctx.OutLn("%s [dir]", wp.Path)
		} else {
			ctx.OutLn("%s %s", wp.Path, strconv.FormatInt(wp.Size, 10))
		}
	}

	return
}

func file_move(ctx PfCtx, args []string) (err error) {
	mopts := File_GetModOpts(ctx)
	root := mopts.Pathroot

	path := root + args[0]
	newpath := root + args[1]
	children := args[2]

	if path == newpath {
		return errors.New("Paths are the same")
	}

	path, err = file_chk_path(path)
	if err != nil {
		return
	}

	newpath, err = file_chk_path(newpath)
	if err != nil {
		return
	}

	var rows *Rows

	q := "SELECT path " +
		"FROM file_namespace " +
		"WHERE path LIKE $1 " +
		"ORDER BY path"

	pathq := path

	if IsTrue(children) {
		pathq = path + "%"
	}

	rows, err = DB.Query(q, pathq)

	if err == ErrNoRows {
		err = errors.New("No such file")
		return
	}

	if err != nil {
		ctx.OutLn("Something went wrong while moving files")
		return
	}

	defer rows.Close()

	pl := len(path)

	c := 0
	for rows.Next() {
		var p string
		err = rows.Scan(&p)

		np := newpath + p[pl:]

		q := "UPDATE file_namespace " +
			"SET path = $1 " +
			"WHERE path = $2"
		err = DB.Exec(ctx,
			"Move File $2 to $1",
			1, q,
			np, p)
		if err != nil {
			return errors.New("Could not move the file")
		}

		c++
	}

	ctx.OutLn("Moved file and " + strconv.Itoa(c) + " children")
	return nil
}

func File_delete(ctx PfCtx, path string, children bool) (cnt int, err error) {
	cnt = 0
	mopts := File_GetModOpts(ctx)
	path = mopts.Pathroot + path

	path, err = file_chk_path(path)
	if err != nil {
		return
	}

	var rows *Rows

	q := "SELECT path " +
		"FROM file_namespace " +
		"WHERE path LIKE $1 " +
		"ORDER BY path"

	pathq := path

	if children {
		pathq = path + "%"
	}

	rows, err = DB.Query(q, pathq)

	if err == ErrNoRows {
		err = errors.New("No such file")
		return
	}

	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		var p string
		err = rows.Scan(&p)

		q := "DELETE FROM file_namespace " +
			"WHERE path = $1"
		err = DB.Exec(ctx,
			"Remove file $1",
			1, q,
			p)
		if err != nil {
			err = errors.New("Could not delete the file")
			return
		}

		cnt++
	}

	return
}

func file_delete(ctx PfCtx, args []string) (err error) {
	path := args[0]
	children := args[1]

	cnt, err := File_delete(ctx, path, IsTrue(children))

	if err == nil {
		ctx.OutLn("Deleted file and " + strconv.Itoa(cnt) + " children")
	} else if err == ErrNoRows {
		ctx.OutLn("No documentation matching that path found")
	} else {
		ctx.OutLn("Something went wrong")
	}

	return
}

func file_copy(ctx PfCtx, args []string) (err error) {
	mopts := File_GetModOpts(ctx)
	root := mopts.Pathroot

	path := URL_Append(root, args[0])
	newpath := URL_Append(root, args[1])
	children := args[2]

	if path == newpath {
		return errors.New("Paths is the same")
	}

	path, err = file_chk_path(path)
	if err != nil {
		return
	}

	newpath, err = file_chk_path(newpath)
	if err != nil {
		return
	}

	var rows *Rows
	q := "SELECT path, file_id " +
		"FROM file_namespace " +
		"WHERE path LIKE $1 " +
		"ORDER BY path"

	pathq := path

	if IsTrue(children) {
		pathq = path + "%"
	}

	rows, err = DB.Query(q, pathq)

	if err == ErrNoRows {
		err = errors.New("No such file")
		return
	}

	if err != nil {
		ctx.OutLn("Something went wrong while copying files")
		return
	}

	defer rows.Close()

	pl := len(path)

	c := 0
	for rows.Next() {
		var p string
		var file_id int

		err = rows.Scan(&p, &file_id)

		np := newpath + p[pl:]

		q := "INSERT INTO file_namespace " +
			"(path, file_id) " +
			"VALUES($1, $2)"
		err = DB.Exec(ctx,
			"Copied path $1 from "+path,
			1, q,
			np, file_id)
		if err != nil {
			return errors.New("Could not copy the file")
		}

		c++
	}

	ctx.OutLn("Copied file and " + strconv.Itoa(c) + " children")
	return nil
}

func File_menu(ctx PfCtx, args []string) (err error) {
	var menu = NewPfMenu([]PfMEntry{
		{"add_dir", file_add_dir, 2, 2, []string{"path", "description"}, PERM_USER, "Add a directory"},
		{"add_file", File_add_localfile, 3, 3, []string{"path", "description", "filename"}, PERM_SYS_ADMIN, "Add a file"},
		{"add_url", File_add_url, 3, 3, []string{"path", "description", "url"}, PERM_USER, "Add a URL"},
		{"update", file_update, 4, 4, []string{"path", "description", "changemsg", "filename"}, PERM_SYS_ADMIN, "Update a file"},
		{"list", file_list, 1, 1, []string{"filepath"}, PERM_USER, "List file below a given path"},
		{"move", file_move, 3, 3, []string{"filepath", "newpath#filepath", "movekids#bool"}, PERM_USER, "Move a file"},
		{"delete", file_delete, 2, 2, []string{"filepath", "deletekids#bool"}, PERM_USER, "Delete a file"},
		{"copy", file_copy, 3, 3, []string{"filepath", "newfilepath#filepath", "sharekids#bool"}, PERM_USER, "Copy a file"},
		{"get", file_get, 1, 1, []string{"filepath"}, PERM_SYS_ADMIN, "Retrieve name of local file"},
	})

	err = ctx.Menu(args, menu)
	return
}
