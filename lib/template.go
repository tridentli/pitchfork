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

/* Constants */
const PAGER_PERPAGE = 10

/* Templates */
var g_tmp *template.Template

var template_funcs = template.FuncMap{
	"pager_less_ok":     tmp_pager_less_ok,
	"pager_less":        tmp_pager_less,
	"pager_more_ok":     tmp_pager_more_ok,
	"pager_more":        tmp_pager_more,
	"var_pager_less_ok": tmp_var_pager_less_ok,
	"var_pager_less":    tmp_var_pager_less,
	"var_pager_more_ok": tmp_var_pager_more_ok,
	"var_pager_more":    tmp_var_pager_more,
	"group_home_link":   tmp_group_home_link,
	"user_home_link":    tmp_user_home_link,
	"user_image_link":   tmp_user_image_link,
	"fmt_date":          tmp_fmt_date,
	"fmt_datemin":       tmp_fmt_datemin,
	"fmt_time":          tmp_fmt_time,
	"fmt_string":        tmp_fmt_string,
	"str_tolower":       tmp_str_tolower,
	"str_emboss":        tmp_str_emboss,
	"inc_file":          tmp_inc_file,
	"dumpvar":           tmp_dumpvar,
	"dict":              tmp_dict,
}

func Template_FuncAdd(name string, f interface{}) {
	template_funcs[name] = f
}

func Template_Get() *template.Template {
	return g_tmp
}

/* Template Functions - used from inside the templates */
func tmp_pager_less_ok(cur int) bool {
	return cur >= PAGER_PERPAGE
}

func tmp_pager_less(cur int) int {
	return cur - PAGER_PERPAGE
}

func tmp_pager_more_ok(cur int, max int) bool {
	return cur < (max - PAGER_PERPAGE)
}

func tmp_pager_more(cur int, max int) int {
	return cur + PAGER_PERPAGE
}

/* Variable size pager function.
func tmp_var_pager_less_ok(page int, cur int) bool {
        return cur >= page
}

func tmp_var_pager_less(page int, cur int) int {
        return cur - page
}

func tmp_var_pager_more_ok(page int, cur int, max int) bool {
        return cur < (max - page)
}

func tmp_var_pager_more(page int, cur int, max int) int {
        return cur + PAGE
}




func tmp_group_home_link(ctx PfCtx, groupname string, username string, fullname string) template.HTML {
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

func tmp_user_home_link(ctx PfCtx, username string, fullname string) template.HTML {
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

func tmp_user_image_link(ctx PfCtx, username string, fullname string, extraclass string) template.HTML {
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

func tmp_fmt_date(t time.Time) string {
	return t.Format(Config.DateFormat)
}

/* Minimum time display */
func tmp_fmt_datemin(a time.Time, b time.Time, skipstamp string) (s string) {
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

func tmp_fmt_time(t time.Time) string {
	return Fmt_Time(t)
}

func tmp_fmt_string(obj interface{}) string {
	return ToString(obj)
}

func tmp_str_tolower(str string) string {
	return strings.ToLower(str)
}

func tmp_str_emboss(in string, em string) (o template.HTML) {
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

func tmp_inc_file(fn string) template.HTML {
	b, err := ioutil.ReadFile(fn)
	if err != nil {
		return HEB("Invalid File")
	}

	return HEB(string(b))
}

/*
 * Dump a variable from a template
 *
 * Useful for debugging so that one can check
 * the entire structure of a variable that is
 * passed in to a template
 */
func tmp_dumpvar(v interface{}) template.HTML {
	str := fmt.Sprintf("%#v", v)
	str = template.HTMLEscapeString(str)
	str = strings.Replace(str, "\n", "<br />", -1)
	return template.HTML("<pre><code>" + str + "</code></pre>")
}

func tmp_dict(values ...interface{}) (map[string]interface{}, error) {
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

func HE(str string) string {
	return template.HTMLEscapeString(str)
}

func HEB(str string) template.HTML {
	return template.HTML(str)
}
