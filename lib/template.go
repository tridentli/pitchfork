// Pitchfork Template functions - used from inside the templates
//
// These functions are for the templates, not to be otherwise called.
package pitchfork

import (
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Number of items per page, for pagers
const PAGER_PERPAGE = 10

// The Templates, stored here cached and loaded along with functions.
//
// Can be retrieved with Template_Get so that one can then operate
// on these templates.
//
// Template_Load does the setup of this variable.
// There is no mutex protecting this variable as no modifications
// after the Template_Load are done on it.
var g_tmp *template.Template

// Template Functions to make a variety of tasks
// in templates easier or more standardized.
//
// See the specific functions for more details about each of them.
//
// Extra, typically application specific, functions can be
// added using the Template_FuncAdd call.
var template_funcs = template.FuncMap{
	"pager_less_ok":     tmpPagerLessOk,
	"pager_less":        tmpPagerLess,
	"pager_more_ok":     tmpPagerMoreOk,
	"pager_more":        tmpPagerMore,
	"var_pager_less_ok": tmpVarPagerLessOk,
	"var_pager_less":    tmpVarPagerLess,
	"var_pager_more_ok": tmpVarPagerMoreOk,
	"var_pager_more":    tmpVarPagerMore,
	"group_home_link":   tmpGroupHomeLink,
	"user_home_link":    tmpUserHomeLink,
	"user_image_link":   tmpUserImageLink,
	"fmt_date":          tmpFmtDate,
	"fmt_datemin":       tmpFmtDateMin,
	"fmt_time":          tmpFmtTime,
	"fmt_string":        tmpFmtString,
	"str_tolower":       tmpStrToLower,
	"str_emboss":        tmpStrEmboss,
	"inc_file":          tmpIncFile,
	"dumpvar":           tmpDumpVar,
	"dict":              tmpDict,
}

// Template_FuncAdd allows adding a application specific template function
//
// After adding, the function is available to all templates in the system.
//
// Noting that a template function has to be available before we load the
// template, thus typically Template_FuncAdd is called from a init() function.
func Template_FuncAdd(name string, f interface{}) {
	template_funcs[name] = f
}

// Template_Get retrieves the custom template cache along with configured custom template functions.
//
// After which ExecuteTemplate can be called passing the template name and the data to be rendered.
//
// Primarily used by UI's page_render.
func Template_Get() *template.Template {
	return g_tmp
}

// tmpPagerLessOk returns true if there can be a 'previous' page.
func tmpPagerLessOk(cur int) bool {
	return cur >= PAGER_PERPAGE
}

// tmpPagerLess returns the offset of the 'previous' page.
func tmpPagerLess(cur int) int {
	return cur - PAGER_PERPAGE
}

// tmpPagerMoreOk returns true if there can be a 'next' page.
func tmpPagerMoreOk(cur int, max int) bool {
	return cur < (max - PAGER_PERPAGE)
}

// tmpPagerMore returns the offset of the 'next' page.
func tmpPagerMore(cur int, max int) int {
	return cur + PAGER_PERPAGE
}

// tmpVarPagerLessOk returns true if there can be a 'previous' page.
func tmpVarPagerLessOk(page int, cur int) bool {
	return cur >= page
}

// tmpVarPagerLess returns the offset of the 'previous' page.
func tmpVarPagerLess(page int, cur int) int {
	return cur - page
}

// tmpVarPagerMoreOk returns true if there can be a 'next' page.
func tmpVarPagerMoreOk(page int, cur int, max int) bool {
	return cur < (max - page)
}

// tmpVarPagerMore returns the offset of the 'next' page.
func tmpVarPagerMore(page int, cur int, max int) int {
	return cur + page
}

// tmpGroupHomeLink returns the HTML formatted link to the group's home
// though only returns the fullname when the configuration to link to them is disabled.
func tmpGroupHomeLink(ctx PfCtx, groupname string, username string, fullname string) template.HTML {
	html := ""

	/* In case the user has no full name use the username */
	if fullname == "" {
		fullname = username
	}

	if Config.UserHomeLinks || ctx.IsSysAdmin() || username == ctx.TheUser().GetUserName() {
		html = "<a href=\"/group/" + groupname + "/member/" + HE(username) + "/\">" + HE(fullname) + "</a>"
	} else {
		html = HE(fullname)
	}

	return HEB(html)
}

// tmpUserHomeLink returns the HTML formatted link to the user's home
// though only returns the fullname when the configuration to link to them is disabled.
func tmpUserHomeLink(ctx PfCtx, username string, fullname string) template.HTML {
	html := ""

	/* In case the user has no full name use the username */
	if fullname == "" {
		fullname = username
	}

	if Config.UserHomeLinks || ctx.IsSysAdmin() || username == ctx.TheUser().GetUserName() {
		html = "<a href=\"/user/" + HE(username) + "/\">" + HE(fullname) + "</a>"
	} else {
		html = HE(fullname)
	}

	return HEB(html)
}

// tmpUserImageLink returns the HTML formatted link to the user's image.
//
// username and fullname provide the details about the user.
// extraclass can optionally be used to specify an extra HTML class to include.
func tmpUserImageLink(ctx PfCtx, username string, fullname string, extraclass string) template.HTML {
	link := false
	if Config.UserHomeLinks || ctx.IsSysAdmin() || username == ctx.TheUser().GetUserName() {
		link = true
	}

	html := ""

	if link {
		html += "<a href=\"/user/" + HE(username) + "/\" title=\"" + HE(fullname) + "\" class=\"nolines\">"
		html += "<img src=\"/user/" + HE(username) + "/image.png\" "
	} else {
		html += "<img src=\"" + System_Get().UnknownImg + "\" "
	}

	html += "class=\"userimage"

	if extraclass != "" {
		html += " " + extraclass
	}

	html += "\" alt=\"Profile Image\" />"

	if link {
		html += "</a>"
	}

	return HEB(html)
}

// tmpFmtDate returns a formatted date stamp.
//
// The format depends on the system configuration.
func tmpFmtDate(t time.Time) string {
	return t.Format(Config.DateFormat)
}

// tmpFmtDateMin displays a time in a minimum way.
func tmpFmtDateMin(a time.Time, b time.Time, skipstamp string) (s string) {
	if skipstamp == "now" {
		/* This causes the year to be skipped if it is the same */
		skipstamp = time.Now().Format("2006")
	}
	ssl := len(skipstamp)

	/* Show just the date */
	s = a.Format("2006-01-02")

	/* Shrink the date, skipping what is already a given? */
	if skipstamp != "" && ssl < len(s) && s[0:ssl] == skipstamp {
		/* 2006 ? */
		skip := 5

		/* 2006-01 ? */
		if ssl == 7 {
			skip = 8
		}

		s = s[skip:]
	}

	/* Same start & end, nothing more to show */
	if a == b {
		return
	}

	if b.Year() != a.Year() {
		/* Year change, show full end date */
		s += " - "
		s += b.Format("2006-01-02")
		return
	}

	if b.Month() != a.Month() {
		/* Month change, show Month + Day */
		s += " - "
		s += b.Format("01-02")
		return
	}

	/* Show only the day */
	s += "..."
	s += b.Format("02")

	return
}

/* tmp_fmt_time returns a standardized time format */
func tmpFmtTime(t time.Time) string {
	return Fmt_Time(t)
}

/* tmpFmtString returns a object's String rendering */
func tmpFmtString(obj interface{}) string {
	return ToString(obj)
}

/* tmp_str_tolower returns a lowercase version of the string */
func tmpStrToLower(str string) string {
	return strings.ToLower(str)
}

/* tmp_str_emboss embosses part of a string */
func tmpStrEmboss(in string, em string) (o template.HTML) {
	inlen := len(in)
	inlow := strings.ToLower(in)

	emlen := len(em)
	emlow := strings.ToLower(em)

	for i := 0; i < inlen; {
		f := strings.Index(inlow[i:], emlow)
		if f == -1 {
			/* Remainder */
			o += HEB(HE(in[i:]))
			break
		}

		o += HEB(HE(in[i : i+f]))
		o += HEB("<em>" + in[i+f:i+f+emlen] + "</em>")
		i += f + emlen
	}
	return
}

/* tmp_inc_file includes a file */
func tmpIncFile(fn string) template.HTML {
	b, err := ioutil.ReadFile(fn)
	if err != nil {
		return HEB("Invalid File")
	}

	return HEB(string(b))
}

/*
 * tmpDumpVar dump a variable from a template.
 *
 * Useful for debugging so that one can check
 * the entire structure of a variable that is
 * passed in to a template.
 */
func tmpDumpVar(v interface{}) template.HTML {
	str := fmt.Sprintf("%#v", v)
	str = template.HTMLEscapeString(str)
	str = strings.Replace(str, "\n", "<br />", -1)
	return template.HTML("<pre><code>" + str + "</code></pre>")
}

// tmpDict is a golang template trick to pass multiple values
// along to another template that one is including.
func tmpDict(values ...interface{}) (map[string]interface{}, error) {
	if len(values)%2 != 0 {
		return nil, errors.New("invalid dict call")
	}

	dict := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, errors.New("dict keys must be strings")
		}
		dict[key] = values[i+1]
	}

	return dict, nil
}

// template_loader loads a template from a file.
func template_loader(root string, path string) error {
	/* Name is the 'short' name without the root of the template dir */
	if !strings.HasSuffix(path, ".tmpl") {
		return nil
	}

	/* We want just the name, not the whole path */
	name := path[len(root)+1:]

	/* Do we already have a version of this template? */
	if g_tmp.Lookup(name) != nil {
		Dbgf("Skipping overruled template %s", name)
		return nil
	}

	/* Load the file */
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	s := string(b)

	/* New Template */
	t := g_tmp.New(name)
	Dbgf("Loaded template %s", name)

	/* Add functions */
	t.Funcs(template_funcs)

	/* Parse the template */
	_, err = t.Parse(s)
	return err
}

// Template_Load loads our templates from the multiple configured roots, overriding where needed.
func Template_Load() (err error) {
	g_tmp = template.New("Pitchfork Templates")

	/* Pre-load the templates from multiple roots */
	for _, root := range Config.File_roots {
		root = filepath.Join(root, "templates/")

		Dbgf("Probing root %s for templates", root)

		err = filepath.Walk(root, func(path string, fi os.FileInfo, _ error) error {
			/* fi can be nil when the dir is not found... */
			if fi == nil {
				return errors.New("Path " + path + " not found")
			} else if fi.IsDir() {
				return nil
			}

			return template_loader(root, path)
		})

		if err != nil {
			err = errors.New(err.Error() + " [root: " + root + "]")
			Dbgf("Template loading caused an error: %s", err.Error())
			break
		}

		Dbgf("Probing root %s for templates - done", root)
	}

	Dbg("Loading templates... done")

	return err
}

// HE escapes a string as HTML.
func HE(str string) string {
	return template.HTMLEscapeString(str)
}

// HEB escapes a string as HTML and Blesses it as proper HTML (use only for strings that one controls).
func HEB(str string) template.HTML {
	return template.HTML(str)
}
