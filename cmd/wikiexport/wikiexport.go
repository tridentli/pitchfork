/*
 * Wiki Export
 *
 * This gathers all the relevant files of a FosWiki installation
 * and stores them in a .wiki file (a .tar.gz)
 */

package pf_cmd_wikiexport

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
)

var g_verbosity = 0

func terr(format string, arg ...interface{}) {
	fmt.Print("--> ")
	fmt.Printf(format+"\n", arg...)
}

func verb(level int, format string, arg ...interface{}) {
	if g_verbosity > level {
		fmt.Print("~~~ ")
		fmt.Printf(format+"\n", arg...)
	}
}

func vrb2(format string, arg ...interface{}) {
	verb(2, format, arg...)
}

func vrb1(format string, arg ...interface{}) {
	verb(1, format, arg...)
}

func info(format string, arg ...interface{}) {
	fmt.Printf(format+"\n", arg...)
}

func addfile(tw *tar.Writer, fname string, dname string) (ok bool) {
	var numb int64
	var file *os.File
	var stat os.FileInfo
	var err error

	ok = false

	file, err = os.Open(fname)
	if err != nil {
		terr("Could not open %s: %s", fname, err.Error())
		return
	}

	defer file.Close()

	stat, err = file.Stat()
	if err != nil {
		terr("Could not stat %s: %s", fname, err.Error())
		return
	}

	/* Header for this file */
	header := new(tar.Header)
	header.Name = dname
	header.Size = stat.Size()
	header.Mode = int64(stat.Mode())
	header.ModTime = stat.ModTime()

	/* add the tarball header */
	err = tw.WriteHeader(header)
	if err != nil {
		terr("Could not append tarheader for %s: %s", fname, err.Error())
		return
	}

	/*  Add the file data to the tarball */
	numb, err = io.Copy(tw, file)
	if err != nil {
		terr("Could not copy %s: %s", fname, err.Error())
		return
	}

	vrb1("Added %s with %d bytes", fname, numb)

	ok = true
	return
}

func WikiExport(tname string) {
	var err error = nil

	/* The arguments */
	aoff := 1
	for ; aoff < len(os.Args) && os.Args[aoff][0] == '-'; aoff++ {
		switch os.Args[aoff] {
		case "-v":
			g_verbosity++
			break

		default:
			terr("Unknown option: '" + os.Args[aoff] + "'")
			os.Exit(1)
			break
		}
	}

	if len(os.Args) < (aoff + 3) {
		terr("Usage: " + tname + " [-v] [-v] foswiki <dir> <dstfile>")
		os.Exit(1)
		return
	}

	wikitype := os.Args[aoff]
	dir := os.Args[aoff+1]
	dst := os.Args[aoff+2]

	if wikitype != "foswiki" {
		terr("Unknown sourcetype '" + wikitype + "', currently only 'foswiki' is supported")
		os.Exit(1)
		return
	}

	if len(dir) == 0 {
		terr("Directory name given is empty!?")
		os.Exit(1)
		return
	}

	if dir[len(dir)-1:] == "/" {
		dir = dir[0 : len(dir)-1]
	}

	/* Check that the source exists */
	_, err = os.Stat(dir + "/data/Main")
	if err != nil {
		terr("Missing pub/Data from source Wiki, please point at the correct directory")
		os.Exit(1)
	}

	info("Starting, storing collected files into " + dst)

	dfile, err := os.Create(dst)
	if err != nil {
		terr("Could not open file %s: %s", dst, err.Error())
		os.Exit(1)
		return
	}

	/* Create a gzipped tarball */
	gw := gzip.NewWriter(dfile)
	tw := tar.NewWriter(gw)

	/* Add all wiki pages */
	numfiles, numskipped := add_wiki(tw, dir+"/data/Main")

	/* Add all files ("attachments") */
	numfiles += add_files(tw, dir+"/pub/Main", "")

	/* Close the tarball */
	err = tw.Close()
	if err != nil {
		terr("Closing Tarball failed: %s", err.Error())
		os.Exit(2)
		return
	}

	err = gw.Close()
	if err != nil {
		terr("Closing Gzip failed: %s", err.Error())
		os.Exit(2)
		return
	}

	err = dfile.Close()
	if err != nil {
		terr("Closing file failed: %s", err.Error())
		os.Exit(2)
		return
	}

	info("Done: %d stored in archive (%d ignored)", numfiles, numskipped)
	os.Exit(0)
}

func add_wiki(tw *tar.Writer, dname string) (numfiles int, numskipped int) {
	ign_files := []string{
		".changes",
		"AdminUser.txt",
		"AdminUserLeftBar.txt",
		"GroupTemplate.txt",
		"GroupViewTemplate.txt",
		"NobodyGroup.txt",
		"PatternSkinUserViewTemplate.txt",
		"ProjectContributor.txt",
		"RegistrationAgent.txt",
		"SitePreferences.txt",
		"UnknownUser.txt",
		"Unprocessed(.*)",
		"UserHomepageHeader.txt",
		"UserList.txt",
		"UserListByDateJoined.txt",
		"UserListByLocation.txt",
		"UserListHeader.txt",
		"WebAtom.txt",
		"WebChanges.txt",
		"WebCreateNewTopic.txt",
		"WebIndex.txt",
		"WebLeftBarExample.txt",
		"WebNotify.txt",
		"WebPreferences.txt",
		"WebRss.txt",
		"WebSearch.txt",
		"WebSearchAdvanced.txt",
		"WebTopicList.txt",
		"WikiGroups.txt",
		"WikiGuest.txt",
		"WikiUsers.txt",
		"WikiUsers.txt,v",
		"(.*).lease",
	}

	files, err := ioutil.ReadDir(dname)
	if err != nil {
		terr(err.Error())
		os.Exit(1)
		return
	}

	for _, f := range files {
		fname := f.Name()
		skip := false

		if f.IsDir() {
			info("Unexpected directory in Wiki: " + fname)
			continue
		}

		for _, ign := range ign_files {
			ok, err := regexp.MatchString(ign, fname)
			if err != nil {
				terr(err.Error())
				os.Exit(2)
				return
			}

			if !ok {
				continue
			}

			skip = true
			break
		}

		if skip {
			vrb2("Skipping: " + fname)
			numskipped++
			continue
		}

		vrb1("Wiki Adding: " + fname)
		ok := addfile(tw, dname+"/"+fname, "wiki/"+fname)
		if !ok {
			terr("Failed")
			os.Exit(2)
			return
		}
		numfiles++
	}

	return
}

func add_files(tw *tar.Writer, dname string, path string) (numfiles int) {
	numfiles = 0

	files, err := ioutil.ReadDir(dname)
	if err != nil {
		terr("Could not open File directory %s: %s", dname, err.Error())
		return
	}

	for _, f := range files {
		fname := f.Name()

		if f.IsDir() {
			add_files(tw, dname+"/"+fname, path+"/"+fname)
			continue
		}

		vrb1("File Adding: " + path + fname)
		ok := addfile(tw, dname+"/"+fname, "file"+path+"/"+fname)
		if !ok {
			terr("Failed")
			os.Exit(2)
			return
		}
		numfiles++
	}

	return
}
