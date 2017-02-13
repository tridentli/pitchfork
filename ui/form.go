package pitchforkui

import (
	"errors"
	"html/template"
	"reflect"
	"strconv"
	"strings"

	"trident.li/keyval"
	pf "trident.li/pitchfork/lib"
)

func init() {
	pf.Template_FuncAdd("pfform", pfform)
}

const PFFORM_READONLY = "readonly"

/*
 * pftype = string (default) | text | tel | email | bool (stored as string) | note
 *
 * 'note' = not a real string, just renders as non-input text
 */

func TFErr(cui PfUI, str string) (o string, buttons string, neditable int, subs []string, multipart bool) {
	cui.Errf("pfform: %s", str)
	return pf.HE("Problem encountered while rendering template"), "", 0, []string{}, false
}

func pfform_keyval(kvs keyval.KeyVals, val string) string {
	if kvs == nil {
		return val
	}

	for _, kv := range kvs {
		/* Gotcha */
		k := pf.ToString(kv.Key)
		if k == val {
			return pf.ToString(kv.Value)
		}
	}

	/* Not found, thus just return the val */
	return val
}

func pfform_select(kvs keyval.KeyVals, def, idpfx, fname, opts string) (t string) {
	t += "<select"
	t += " id=\"" + idpfx + fname + "\""
	t += " name=\"" + fname + "\""
	t += opts
	t += ">\n"

	for _, kv := range kvs {
		key := pf.ToString(kv.Key)
		val := pf.ToString(kv.Value)

		key = pf.HE(key)
		val = pf.HE(val)

		t += "<option value=\"" + key + "\""
		if def == key {
			t += " selected"

		}
		t += ">" + val + "</option>\n"
	}

	t += "</select>\n"

	return
}

func pfform_hidden(idpfx, fname, val string) (t string) {
	t += "<input"
	t += " id=\"" + idpfx + fname + "\""
	t += " name=\"" + fname + "\""
	t += " type=\"hidden\""
	t += " value=\"" + val + "\""
	t += " />"
	return
}

func pfform_mask(masknum int, idpfx string, fname string) (t string) {
	if masknum == 0 {
		return
	}

	id := idpfx + fname + ".hide"
	if masknum != 1 {
		id += strconv.Itoa(masknum)
	}

	t += "<input type=\"checkbox\" id=\"" + id + "\" class=\"hidebox\" />\n"
	t += "<label for=\"" + id + "\" class=\"hidebox fakebutton noselect\"></label>\n"
	t += "<div class=\"hidebox\">\n"
	return
}

func pfform_masktail(masknum int) (t string) {
	if masknum == 0 {
		return
	}

	t += "</div>\n"
	return
}

func pfform_string(kvs keyval.KeyVals, val, idpfx, fname, ttype, opts, min, max string, allowedit bool, masknum int) (t string) {
	/* Multiple items? Then it it should be a select box */
	if len(kvs) > 1 {
		if allowedit {
			t += pfform_select(kvs, val, idpfx, fname, opts)
			return
		} else {
			t += pfform_hidden(idpfx, fname, val)
			fname += ".readonly"
		}
	}

	/* If there is only item, it is just a prettifier */
	val = pfform_keyval(kvs, val)
	val = pf.HE(val)

	/* Detect multi-line content */
	if ttype == "string" {
		if strings.Index(val, "\n") != -1 {
			ttype = "text"
		}
	}

	t += pfform_mask(masknum, idpfx, fname)

	/* Multiline -> Render as textarea */
	if ttype == "text" {
		t += "<textarea"
		t += " id=\"" + idpfx + fname + "\""
		t += " name=\"" + fname + "\""
		t += opts
		t += ">" + val + "</textarea>\n"

		t += pfform_masktail(masknum)

		return
	}

	t += "<input"
	t += " id=\"" + idpfx + fname + "\""
	t += " name=\"" + fname + "\""

	if ttype == "string" {
		t += " type=\"text\" "
	} else {
		t += " type=\"" + ttype + "\" "
	}
	t += " value=\"" + val + "\" "

	t += opts

	if min != "" || max != "" {
		if min == "" {
			min = "0"
		} else if strings.HasPrefix(min, "CFG_") {
			r := reflect.ValueOf(pf.Config)
			f := reflect.Indirect(r).FieldByName(min)
			min = string(f.String())
		}
		if max == "" {
		} else if strings.HasPrefix(max, "CFG_") {
			r := reflect.ValueOf(pf.Config)
			f := reflect.Indirect(r).FieldByName(max)
			max = string(f.String())
		}
		t += " pattern=\".{" + min + "," + max + "}\""
	}

	t += " />\n"

	t += pfform_masktail(masknum)

	return
}

func pfform_number(kvs keyval.KeyVals, val, idpfx, fname, ttype, opts, mint, maxt, minmax, hint string, allowedit bool, masknum int) (t string) {
	/* Multiple items? Then it it should be a select box */
	if len(kvs) > 1 {
		if allowedit {
			t += pfform_select(kvs, val, idpfx, fname, opts)
			return
		} else {
			t += pfform_hidden(idpfx, fname, val)
			fname += ".readonly"
		}
	}

	/* If there is only item, it is just a prettifier */
	val = pfform_keyval(kvs, val)
	val = pf.HE(val)

	/* If not range, default to number */
	if len(kvs) > 1 {
		ttype = "string"
	} else if ttype != "range" {
		ttype = "number"
	}

	if mint == "" {
		mint = "min=0 "
	}

	t += pfform_mask(masknum, idpfx, fname)

	t += "<input "
	t += "id=\"" + idpfx + fname + "\" "
	t += "name=\"" + fname + "\" "
	t += "type=\"" + ttype + "\" "
	t += "value=\"" + val + "\" "
	t += mint
	t += maxt
	t += opts
	t += " />"
	if minmax != "" {
		if hint == "" {
			hint = minmax
		} else {
			hint += " (" + minmax + ")"
		}
	}
	t += "\n"

	t += pfform_masktail(masknum)

	return
}

func pfform_bool(val bool, fname, idpfx, opts string, allowedit bool) (t string) {
	/* Checkboxes are special, when you want them readonly, you need to disable them too */
	t += "<input "
	if allowedit {
		t += "name=\"" + fname + "\" "
		t += "id=\"" + idpfx + fname + "\""
	}
	t += "type=\"checkbox\" "
	if val {
		t += " checked=\"checked\""
	}
	if !allowedit {
		t += " disabled=\"disabled\""
	}
	t += opts
	t += " />\n"

	/* Add a hidden field, so that when this form gets submitted we still transmit the right thing */
	if !allowedit {
		t += "<input "
		t += "name=\"" + fname + "\" "
		t += "id=\"" + idpfx + fname + "\""
		t += "type=\"hidden\" "
		t += "value=\""
		if val {
			t += "on"
		}
		t += "off"
		t += "\" />\n"
	}

	return
}

func pfform_submit(val string, class string) (t string) {
	t += "<input "
	t += "id=\"submit\" "
	t += "name=\"submit\" "
	t += "type=\"submit\" value=\"" + pf.HE(val) + "\""
	if class != "" {
		t += " class=\"" + class + "\""
	}
	t += " />\n"
	return
}

func pfform_label(idpfx string, fname string, label string, ttype string) (t string) {
	t += "<label for=\"" + idpfx + fname + "\">"

	l := len(label)

	if l > 0 && ttype != "submit" && ttype != "note" {
		t += label

		if label[l-1] != '?' {
			t += ":"
		}
	} else {
		t += "&nbsp;"
	}

	t += "</label>\n"

	return
}

/*
 * s = struct where the fields are scanned from and that build up the form
 * m = struct where a 'message' and 'error' field should be present
 */
func pfformA(cui PfUI, section *string, idpfx string, objtrail []interface{}, obj interface{}, m interface{}, ptype pf.PType) (o string, buttons string, neditable int, subs []string, multipart bool) {
	var err error

	/* Prepend this object to the trail */
	objtrail = append([]interface{}{obj}, objtrail...)

	s, va := pf.StructReflect(obj)

	if s.Kind() != reflect.Struct {
		return TFErr(cui, "Error: parameter is not a struct but '"+s.String()+"'")
	}

	/* Prefix before the ids to make them unique-ish */
	idpfx += s.Name() + "-"

	/* Nothing generated here yet */
	o = ""
	buttons = ""

	oname := pf.StructNameObjTrail(objtrail)

	hides := make(map[string]string)

	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		v := va.Field(i)

		// cui.Dbg("%s FIELD[%d] %s (%s)", oname, i, f.Name)
		if f.PkgPath != "" {
			// cui.Dbg("%s FIELD[%d] %s (pkg: %#v) - unexported, skipping", oname, i, f.Name, f.PkgPath)
			continue
		}

		ttype, dorecurse, compound := pf.PfType(f, v, false)
		if ttype == "ignore" {
			continue
		}

		if dorecurse {
			tt, bb, ne, s, mp := pfformA(cui, section, idpfx, objtrail, pf.StructRecurse(v), m, ptype)
			o += tt
			buttons += bb
			neditable += ne
			subs = append(subs, s...)

			/* Enable multipart when needed */
			if mp {
				multipart = true
			}
		}

		if compound {
			continue
		}

		/* Per-field ptype */
		lptype := ptype

		/* Not a sub-form yet */
		issubform := false

		/* No options yet */
		opts := ""

		/* Column/fieldname in SQL Table */
		fname := f.Tag.Get("pfcol")
		if fname == "" {
			fname = strings.ToLower(f.Name)
		}

		/* Do we determine field visibility with a custom function? */
		vis_func := f.Tag.Get("isvisible")
		if vis_func != "" {
			ok, obj := pf.ObjHasFunc(objtrail, vis_func)
			if ok {
				res, err2 := pf.ObjFunc(obj, vis_func, f.Name)
				if err2 != nil {
					err = errors.New(oname + " Field '" + f.Name + "' function " + vis_func + "() failed: " + err2.Error())
					return
				}

				ok, ok2 := res[0].Interface().(bool)
				if !ok2 {
					err = errors.New(oname + " Field '" + f.Name + "' function " + vis_func + "() return failed: not a bool")
					return
				}

				if !ok {
					/* The field is not visible */
					continue
				}
			} else {
				cui.Errf("Object %s Field %s has isvisible function %s defined but object does not have that function", oname, f.Name, vis_func)
			}
		}

		/* Permissions check */
		var ok bool
		var allowedit bool

		ok, allowedit, err = pf.StructPermCheck(cui, lptype, objtrail, pf.PTypeWrap(f))
		if err != nil {
			skipfailperm := f.Tag.Get("pfskipfailperm")
			if skipfailperm != "" {
				/* Skip the field when permission check failed */
				continue
			}

			return TFErr(cui, "Error: Field '"+oname+":"+fname+"' has invalid permissions: "+err.Error())
		}

		if !ok {
			/* Don't even show the field */
			// cui.Dbgf("NOT SHOWING: field = %s, ttype = %s", oname+":"+fname, ttype)
			continue
		}

		/* The label */
		label := f.Tag.Get("label")
		if label == "" {
			/* No label -> ignore the field */
			// cui.Dbgf("SKIPPING field = %s, ttype = %s", oname+":"+fname, ttype)
			continue
		}

		/* Try to translate it */
		label = pf.TranslateObj(cui, objtrail, label)

		/* "options" function for mapping key -> value */
		var kvs keyval.KeyVals
		keyval_func := f.Tag.Get("options")
		kvs = nil

		if keyval_func != "" {
			var kvs_i interface{} = nil
			var context interface{} = nil

			oc_func := "ObjectContext"
			ok, obj := pf.ObjHasFunc(objtrail, oc_func)
			if ok {
				context, err = pf.ObjFuncIface(obj, oc_func)
				if err != nil {
					err = errors.New("Field '" + f.Name + "' function " + oc_func + "() failed: " + err.Error())
					return
				}
			}

			ok, obj = pf.ObjHasFunc(objtrail, keyval_func)
			if !ok {
				err = errors.New("Keyval function " + pf.StructNameObj(obj) + " " + keyval_func + "() not found")
				return
			}

			kvs_i, err = pf.ObjFuncIface(obj, keyval_func, context)
			if err == nil {
				kvs = kvs_i.(keyval.KeyVals)
			} else {
				err = errors.New("Field '" + f.Name + "' keyvals function " + keyval_func + "() failed: " + err.Error())
				return
			}

			if err != nil {
				cui.Errf("Keyvals failed: %s", err.Error())
			}
		}

		/* Optional Hint and placeholder */
		hint := f.Tag.Get("hint")
		hint = pf.TranslateObj(cui, objtrail, hint)
		placeholder := f.Tag.Get("placeholder")
		placeholder = pf.TranslateObj(cui, objtrail, placeholder)

		/* Other options */
		min := f.Tag.Get("min")
		max := f.Tag.Get("max")
		class := f.Tag.Get("htmlclass")
		req := f.Tag.Get("pfreq")
		sec := f.Tag.Get("pfsection")
		omitempty := pf.IsTrue(f.Tag.Get("pfomitempty"))
		checkboxmode := pf.IsTrue(f.Tag.Get("pfcheckboxmode"))

		/* Some fields should not be editable from a form (eg username) */
		formedit := f.Tag.Get("pfformedit")
		if formedit == "no" {
			allowedit = false
		}

		/* Mask the field (eg passwords / keys) */
		mask := f.Tag.Get("mask")
		masknum := 0
		if mask == "read" {
			masknum = 1
		}

		/* Single item, then make it readonly */
		if len(kvs) == 1 {
			allowedit = false
		}

		/* If not allowed to edit, then mark the inputs readonly */
		if !allowedit {
			opts += " " + PFFORM_READONLY
		}

		if pf.IsTrue(req) {
			opts += " required"
		}

		if class != "" {
			opts += " class=\"" + class + "\""
		}

		if placeholder != "" {
			if strings.HasPrefix(placeholder, "CFG_") {
				r := reflect.ValueOf(pf.Config)
				f := reflect.Indirect(r).FieldByName(placeholder)
				placeholder = string(f.String())
			}

			opts += " placeholder=\"" + pf.HE(placeholder) + "\""
		}

		minmax := ""

		mint := ""
		if min != "" {
			mint = "min=" + min + " "
			minmax += "minimum: " + min
		}

		maxt := ""
		if max != "" {
			maxt = "max=" + max + " "

			if minmax != "" {
				minmax += ", "
			}

			minmax += "maximum: " + max
		}

		/* Section */
		if *section != sec {
			/* Close old section? */
			if *section != "" {
				o += "</ul>\n"
				o += "</li>\n"
			}

			/* Open new section? */
			*section = sec
			if *section != "" {
				o += "<li>\n"
				o += sec
				o += "<ul>\n"
			}
		}

		if ttype == "hidden" {
			val := v.Interface().(string)

			/* Will be appended at the end */
			if val != "" {
				hides[fname] = val
			} else {
				hides[fname] = label
			}

			/* next */
			continue
		}

		/* Do not add buttons to forms that cannot be edited */
		if ttype == "submit" && neditable == 0 {
			continue
		}

		tlabel := label

		/* This part of the HTML output for this variable */
		t := "<li>\n"

		t += pfform_label(idpfx, fname, tlabel, ttype)

		pftype := f.Type.Kind()

		switch pftype {
		case reflect.String:
			switch ttype {
			case "email", "password", "string", "submit", "tel", "text":
				val := label

				if ttype != "submit" {
					val = v.Interface().(string)
				}

				if val == "" && omitempty {
					t = ""
					break
				}

				t += pfform_string(kvs, val, idpfx, fname, ttype, opts, min, max, allowedit, masknum)
				break

			case "bool":
				val := pf.IsTrue(v.Interface().(string))
				t += "<input "
				t += "name=\"" + fname + "\" "
				t += "id=\"" + idpfx + fname + "\" "
				t += "type=\"checkbox\" "
				if val {
					t += " checked=\"checked\""
				}
				t += opts
				t += " />\n"
				break

			case "note":
				val := v.Interface().(string)
				val = pfform_keyval(kvs, val)
				val = pf.HE(val)

				if val == "" && omitempty {
					t = ""
					break
				}

				t += "<span "
				t += "id=\"" + idpfx + fname + "\""
				t += opts
				t += ">"
				t += val
				t += "</span>\n"
				break

			case "file":
				if allowedit {
					val := v.Interface().(string)
					val = pfform_keyval(kvs, val)
					val = pf.HE(val)

					t += "<input type=\"file\" "
					t += "id=\"" + idpfx + fname + "\" "
					t += "name=\"" + fname + "\" "
					t += opts
					t += ">\n"

					/* Need multipart */
					multipart = true
				} else {
					/* No File uploader when not editable */
					t = ""
				}
				break

			default:
				return TFErr(cui, "Field '"+fname+"', unknown pftype: '"+ttype+"'")
			}
			break

		case reflect.Bool:
			val := v.Interface().(bool)
			t += pfform_bool(val, fname, idpfx, opts, allowedit)
			break

		case reflect.Int, reflect.Int64, reflect.Float64, reflect.Uint, reflect.Uint64:
			val := pf.ToString(v.Interface())
			t += pfform_number(kvs, val, idpfx, fname, ttype, opts, mint, maxt, minmax, hint, allowedit, masknum)
			break

		/* <input type="date" min="2012-01-01" max="2013-01-01" type="date"> */
		case reflect.Struct:
			ty := pf.StructNameT(v.Type())

			switch ty {
			case "time.Time":
				val := pf.ToString(v.Interface())
				vals := strings.Split(val, ":")
				val = vals[0] + ":" + vals[1]
				val = strings.Replace(val, " ", "T", -1)
				val = pf.HE(val)
				t += "<input "
				t += "id=\"" + idpfx + fname + "\" "
				t += "name=\"" + fname + "\" "
				t += "type=\"datetime-local\" "
				t += "value=\"" + val + "\""
				t += opts
				t += " />\n"
				break

			case "database/sql.NullString":
				val := pf.ToString(v.Interface())
				t += pfform_string(kvs, val, idpfx, fname, ttype, opts, min, max, allowedit, masknum)
				break

			case "database/sql.NullInt64", "database/NullFloat64":
				val := pf.ToString(v.Interface())
				t += pfform_number(kvs, val, idpfx, fname, ttype, opts, mint, maxt, minmax, hint, allowedit, masknum)
				break

			case "database/sql.NullBool":
				val := pf.IsTrue(pf.ToString(v.Interface()))
				t += pfform_bool(val, fname, idpfx, opts, allowedit)
				break

			default:
				/* Handle it as a string? */
				if ttype == "string" {
					val := pf.ToString(v.Interface())
					t += pfform_string(kvs, val, idpfx, fname, ttype, opts, min, max, allowedit, masknum)
					break
				}

				/* Don't understand it */
				return TFErr(cui, "Field '"+fname+"' is of unsupported struct type: "+ty)
			}
			break

		case reflect.Map:
			if checkboxmode {
				t += "<ul>\n"

				for _, kv := range kvs {
					key := pf.ToString(kv.Key)
					val := pf.ToString(kv.Value)

					key = pf.HE(key)
					val = pf.HE(val)

					t += "<li>\n"
					t += "<input "
					t += "name=\"" + fname + "[]\" "
					t += "id=\"" + idpfx + fname + "\" "
					t += "type=\"checkbox\" "
					t += "value=\"" + key + "\" "
					t += opts
					t += " />"
					t += " " + val
					t += "</li>\n"
				}
				t += "<ul>\n"
			} else {
				/* Has no default, all are already selected */
				t += "<select "
				t += "id=\"" + idpfx + fname + "\" "
				t += "name=\"" + fname + "\" "
				t += "multiple "
				t += ">\n"

				for _, k := range v.MapKeys() {
					key := pf.ToString(k.Interface())
					ov := v.MapIndex(k)
					val := pf.ToString(ov.Interface())

					key = pf.HE(key)
					val = pf.HE(val)

					t += "<option value=\"" + key + "\""
					t += " selected"
					t += ">" + val + "</option>\n"
				}

				/* Add the options from keyvals -- these are unselected */
				if kvs != nil {
					for _, kv := range kvs {
						key := pf.ToString(kv.Key)
						val := pf.ToString(kv.Value)

						key = pf.HE(key)
						val = pf.HE(val)

						t += "<option value=\"" + key + "\""
						t += ">" + val + "</option>\n"
					}
				}

				t += "</select>\n"
			}
			break

		case reflect.Slice:
			t = "<div class=\"styled_form\">\n" + t
			vn := v.Type().String()

			/* RMB, not Right Mouse Button, but ReMove Button */
			rmb_pre := ""
			rmb_post := ""
			if allowedit {
				rmb_pre, err = pfform_head(cui, false)
				if err != nil {
					return TFErr(cui, "pfform head failed")
				}

				rmb_post = pfform_submit("Remove", "deny")
				rmb_post += pfform_tail()
			}

			/* Put these in a sub-form */
			issubform = true
			t += "<ul>\n"

			for k := 0; k < v.Len(); k += 1 {
				masknumber := masknum
				if masknum > 0 {
					masknumber += k
				}

				switch vn {
				case "[]string":
					ttype = "string"
					val := v.Index(k).Interface().(string)

					t += "<li>"
					t += rmb_pre
					t += pfform_string(kvs, val, idpfx, fname, ttype, opts+PFFORM_READONLY, min, max, false, masknumber)
					t += rmb_post
					t += "</li>\n"
					break

				case "[]int":
					ttype = "number"
					val := pf.ToString(v.Index(k).Interface())

					t += "<li>"
					t += rmb_pre
					t += pfform_number(kvs, val, idpfx, fname, ttype, opts+PFFORM_READONLY, mint, maxt, minmax, hint, false, masknumber)
					t += rmb_post
					t += "</li>\n"
					break

				default:
					return TFErr(cui, "Field '"+fname+"' is of unsupported Slice type: "+vn)
				}

				t += "\n"
			}

			/* When allowed to edit, add an add button */
			if allowedit {
				t += "<li >\n"
				tt, e := pfform_head(cui, false)
				if e != nil {
					return TFErr(cui, "pfform head failed")
				}

				t += tt
				switch vn {
				case "[]string":
					t += pfform_string(kvs, "", idpfx, fname, ttype, opts, min, max, true, masknum)
					break

				case "[]int":
					t += pfform_number(kvs, "", idpfx, fname, ttype, opts, mint, maxt, minmax, hint, true, masknum)
					break

				default:
					return TFErr(cui, "unhandled type: "+vn)
				}
				t += pfform_submit("Add", "allow")
				t += pfform_tail()
				t += "</li>\n"
				t += "\n"
			}

			t += "</ul>\n"
			t += "</div>\n"
			break

		case reflect.Interface:
			tt, bb, ne, s, mp := pfformA(cui, section, idpfx, objtrail, pf.StructRecurse(v), m, ptype)
			t += tt
			buttons += bb
			neditable += ne
			subs = append(subs, s...)

			/* Enable multipart where needed */
			if mp {
				multipart = true
			}
			break

		default:
			return TFErr(cui, "Field '"+fname+"' is an unknown type "+ftype.String()+": "+pf.StructNameT(f.Type))
		}

		if t != "" {
			if ok {
				if hint != "" {
					h := pf.HE(hint)
					t += "<span class=\"form_hint\">" + h + "</span>\n"
				}
				t += "</li>\n\n"
			}

			/* Group all the submit buttons at the end */
			if ttype == "submit" {
				buttons += t
			} else {
				if allowedit {
					neditable++
				}

				if issubform {
					subs = append(subs, t)
				} else {
					/* Main form */
					o += t
				}
			}
		}
	}

	for fname, val := range hides {
		o += pfform_hidden(idpfx, fname, val)
	}
	return
}

func pfform_head(cui PfUI, multipart bool) (o string, err error) {
	o += "<form class=\"styled_form\" method=\"post\""
	if multipart {
		o += " enctype=\"multipart/form-data\""
	}
	o += ">\n"

	o += "<fieldset>\n"
	o += csrf_input(cui, "", "post")
	o += "<ul>\n"
	return
}

func pfform_tail() (o string) {
	o += "</ul>\n"
	o += "</fieldset>\n"
	o += "</form>\n"
	return
}

func pfform(cui PfUI, obj interface{}, m interface{}, editable bool) template.HTML {
	/* Section - initially none */
	section := ""

	/* Check that we really have an object */
	if obj == nil {
		return pf.HEB("No object provided")
	}

	/* Prefix */
	s := reflect.TypeOf(obj)
	idpfx := s.Name() + "-"

	/* TODO Change to be option to pfform() instead of editable, would require updating templates */
	ptype := pf.PTYPE_READ
	if editable {
		ptype = pf.PTYPE_UPDATE
	}

	/* Call our recursive function */
	objtrail := []interface{}{}
	o, buttons, neditable, subs, multipart := pfformA(cui, &section, "", objtrail, obj, m, ptype)

	if editable && buttons == "" && neditable > 0 {
		buttons = pfform_string(nil, "Update", "update", "submit", "submit", "", "", "", false, 0)
	}

	o += buttons

	/* Close section if it is still open */
	if section != "" {
		o += "</ul>\n"
		o += "</li>\n"
	}

	_, _, fvalue, err := pf.StructDetails(cui, m, "message", pf.SD_None)
	if err == nil && fvalue != "" {
		/* Found it */
		id := pf.HE(idpfx + "message")
		o += "<li class=\"okay\">\n"
		o += "<label for=\"" + id + "\">&nbsp;</label>\n"
		o += "<span id=\"" + id + "\">\n"
		o += pf.HE(fvalue)
		o += "</span>\n"
		o += "</li>\n"
	}

	_, _, fvalue, err = pf.StructDetails(cui, m, "error", pf.SD_None)
	if err == nil && fvalue != "" {
		/* Found it */
		id := pf.HE(idpfx + "error")
		o += "<li class=\"error\">\n"
		o += "<label for=\"" + id + "\">&nbsp;</label>\n"
		o += "<span id=\"" + id + "\">\n"
		o += pf.HE(fvalue)
		o += "</span>\n"
		o += "</li>\n"
	}

	o += pfform_tail()

	/* Any sub-forms -- used for slices which have separate add/remove buttons */
	for _, s := range subs {
		/* Separator line between the forms */
		o += "<hr />\n"
		o += s
	}

	/* Prefix the head with optional multipart option */
	oo, err := pfform_head(cui, multipart)
	if err != nil {
		return pf.HEB("pfform head failed")
	}

	/* Prepend the header */
	o = oo + o

	return pf.HEB(o)
}
