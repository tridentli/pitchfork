package pitchfork

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// "group", "format", "file", "wikipath"
func wiki_import(ctx PfCtx, args []string) (err error) {
	var df *os.File
	var gr *gzip.Reader

	gr_name := ctx.SelectedGroup().GetGroupName()
	format := strings.TrimSpace(args[0])
	fname := strings.TrimSpace(args[1])
	path := strings.TrimSpace(args[2])

	if format != "foswiki" {
		err = errors.New("Unsupported import format '" + format + "'")
		return
	}

	if path[len(path)-1:] != "/" {
		err = errors.New("New Path does not end in slash; it needs to be a directory")
		return
	}

	df, err = os.Open(fname)
	if err != nil {
		err = fmt.Errorf("Could not open file %s: %s", fname, err.Error())
		return
	}

	/* Open the tar archive for reading */
	gr, err = gzip.NewReader(df)
	if err != nil {
		return
	}

	tr := tar.NewReader(gr)

	num_wiki_ok := 0
	num_wiki_err := 0
	num_file_ok := 0
	num_file_err := 0
	num_file_dup := 0
	num_skip := 0

	/* Iterate through the files in the archive. */
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			/* end of archive */
			break
		}

		if err != nil {
			break
		}

		fname := hdr.Name

		/* If fname contains a captured 20, replace it with a literal space before insertion */
		re := regexp.MustCompile("([a-zA-Z])20([a-zA-Z])")
		fname = re.ReplaceAllString(fname, "$1 $2")

		fl := len(fname)
		if fl < 5 {
			ctx.Outf("Broken filename: '%s', unknown", fname)
			num_skip++
			continue
		}

		/* Ignore RCS files for the time being */
		if fname[fl-2:] == ",v" {
			continue
		}

		sect := fname[0:5]

		switch sect {
		case "wiki/":
			buf := bytes.NewBuffer(nil)
			_, err := io.Copy(buf, tr)
			if err == nil {

				/* Make sure it is UTF-8 okay (and latin1 etc) */
				md := ToUTF8(buf.Bytes())

				/* Remove .txt from the end of the wiki file name */
				if fname[fl-4:] == ".txt" {
					fname = fname[:fl-4]
				}
				fl = len(fname)

				/* Foswiki uses "WebHome" as the root, we use "/" */
				if fname[fl-7:] == "WebHome" {
					fname = fname[:fl-7]
				}

				/* Special conversion for FosWiki (TML) based pages */
				if format == "foswiki" {
					md = wiki_TML2Markdown(md, gr_name, fname[5:])
				}

				args := []string{gr_name, path + fname[5:], "Imported", fname[5:], md}
				err = wiki_update(ctx, args)
			}

			if err == nil {
				ctx.OutLn("ok '%s'", fname)
				num_wiki_ok++
			} else {
				ctx.OutLn("ERROR '%s': %s", fname, err.Error())
				num_wiki_err++
			}
			break

		case "file/":
			err = File_add_file(ctx, path+fname[5:], "Imported", tr)
			if err == nil {
				ctx.OutLn("ok '%s'", fname)
				num_file_ok++
			} else if err == ErrFilePathExists {
				ctx.OutLn("dup '%s'", fname)
				num_file_dup++
			} else {
				ctx.OutLn("ERROR '%s': %s", fname, err.Error())
				num_file_err++
			}
			break

		default:
			ctx.OutLn("Unknown import section '%s' for '%s', skipping", sect, fname)
			num_skip++
			break
		}
	}

	gr.Close()
	df.Close()

	ctx.OutLn("Wiki: %d ok, %d error", num_wiki_ok, num_wiki_err)
	ctx.OutLn("File: %d ok, %d error, %d duplicates", num_file_ok, num_file_err, num_file_dup)
	ctx.OutLn("Skipped: %d", num_skip)

	return
}

/* https://foswiki.org/System/TopicMarkupLanguage */
func wiki_TML2Markdown(tml string, gr_name string, fname string) (md string) {
	/* It will be markdown soon */
	md = tml

	/* Try to read all the META headers & mark them as comments */
	rm := regexp.MustCompile(`%META:(.*)\{(.*)}%`)
	metas := rm.FindAllStringSubmatch(md, -1)

	for _, m := range metas {
		full := m[0]
		mt := m[1]
		s := m[2]

		switch mt {
		case "FILEATTACHMENT":
			rx_name := regexp.MustCompile(`name="([^"]*)"`)
			rx_attach := regexp.MustCompile(`attachment="([^"]*)"`)
			name_match := rx_name.FindStringSubmatch(s)
			attach_match := rx_attach.FindStringSubmatch(s)
			name := name_match[1]
			attach := attach_match[1]
			flink := strings.Replace(fname, " ", "%20", -1)

			md = strings.Replace(md, full, "["+name+"](/group/"+gr_name+"/file/"+flink+"/"+attach+")", -1)
			break
		default:
			/* Comment it out, thus it will be visible still in the source */
			md = strings.Replace(md, full, "<!---\n"+full+"\n-->\n", -1)
			break
		}

	}

	/* Simple conversions */
	simple := []string{
		"---+ ", "# ",
		"---++ ", "## ",
		"---+++ ", "### ",
		"---++++ ", "#### ",
		"---+++++ ", "##### ",
		"---+!! ", "# ",
		"---++!! ", "## ",
		"---+++!! ", "### ",
		"---++++!! ", "#### ",
		"---+++++!! ", "##### ",
	}

	r := strings.NewReplacer(simple...)
	md = r.Replace(md)

	/* Convert Links [[link][desc]]  -> [desc](link) */
	rl := regexp.MustCompile(`\[\[(.*?)\]\[(.*?)\]\]?`)
	links := rl.FindAllStringSubmatch(md, -1)
	for _, l := range links {
		ldest := strings.Replace(l[1], " ", "%20", -1)
		lname := l[2]
		/* Dbg("LINKwiki: %s -> %s", ldest, lname) */
		nl := "[" + lname + "](" + ldest + ")"
		md = strings.Replace(md, l[0], nl, -1)
	}

	/* Convert Links <a href="Dest" target="_self" title="Name">Name</a> */
	rl = regexp.MustCompile(`<a(.*)href=\"(.*?)\"(.*?)>(.*)</a>?`)
	links = rl.FindAllStringSubmatch(md, -1)
	for _, l := range links {
		ldest := l[2]
		lname := l[4]

		idx := strings.Index(l[2], "\"")
		if idx != -1 {
			ldest = ldest[:idx]
		}

		ldest = strings.Replace(ldest, " ", "%20", -1)
		/* Dbg("LINKhref: %s -> %s", ldest, lname) */
		nl := "[" + lname + "](" + ldest + ")"
		md = strings.Replace(md, l[0], nl, -1)
	}

	/*
	 * We leave inline style (eg <a href="/" style="....">...</a>
	 * as bluemonday takes care of removing those details
	 * thus it does not hurt, and maybe it is there on purpose
	 */

	return
}
