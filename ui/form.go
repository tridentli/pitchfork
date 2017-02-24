// Pitchfork's pfform function
//
// pfform is a powerful template function that can render HTML5 forms from
// a golang structure using the tags in the structure to influence how the
// form gets rendered and the options the input elements receive.
//
// It avoids the need for maintaining HTML and adheres to the
// security/permission model that is embedded in Pitchfork.
//
// A struct's fields are rendered in order.
//
// Translation of all fields is attempted; a custom translation function
// can be specified by having the object have a Translate() function that
// accepts two arguments typed string, the first being the label to be
// translated, the second the requested target language.
//
// Tag pfcol describes the column/fieldname in a SQL table, used also as
// part of the HTML 'id' field.
// The default, when not specified, is the lowercase version of the golang
// field name.
//
// Tag isvisible, when defined provides the name of a custom function for
// determining if a field should be visible or not.
// It is called with the first parameter being the string describing the
// fieldname (see: pfcol)
//
// Tag label indicates the human readable label shown before the input field.
// When not specified the field is ignored from rendering.
//
// Tag options indicates a function to be called to retrieve a keyval to be
// used for generating a select style input.
// The ObjectContext function in addition allows specifiying a context
// for that call.
//
// Tag hint indicates the HTML hint to be included for the given input.
//
// Tag placeholder indicates the HTML placeholder to be included for the
// given input.
//
// Tag min can be used to specify a minimum string length or a minimum number.
//
// Tag max can be used to specify a maximum string length or a maximum number.
//
// Tag htmlclass can be used to specify a custom HTML class (CSS) for the
// input.
//
// Tag pfreq indicates, when set, that the field is required and thus that the
// input should be marked and decorated as such.
//
// Tag pfsection indicates the name of a section. Every field with the same
// section is grouped together. The name of the section is shown as a HTML
// legend item above the group of fields that are put in a fieldset.
//
// Tag pfomitempty indicates that the input field should be omitted when the
// value is empty.
//
// Tag mask indicates that we wrap a HTML mask around the field, thus requiring
// a user to first click on the expand button to reveal the field.
//
// Tag content is used for notes and widenotes, it allows the content/value of
// the item to be included in the tag instead of having to be separately set
// in the struct.
//
// Strings are rendered as strings, strings with keyvals are rendered as
// select boxes, with the default being the value of the string.
//
// Numbers are rendered as integers that can be increased/decreased.
//
// Dates and Time objects are rendered as dates allowing a HTML5 calendar
// control for selecting the details.
//
// SQL Nullstrings are treated like their native types.
//
// Maps are rendered as multi select options.
//
// Slices are rendered as multiple options with an add/remove button when
// editing is allowed.
//
// pftype = string (default) |
//	    text |
//          tel |
//          email |
//          bool (stored as string) |
//          note |
//          widenote
//
// 'note' and 'widenote' are not a real string, just renders as non-input text
// 'note' renders inline with the input boxes, while 'widenote' uses the full
// width that also uses the space for the labels.

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

// init registers the pfform function as a template function
func init() {
	pf.Template_FuncAdd("pfform", pfform)
}

// PFFORM_READONLY is the marker used to indicate a readonly HTML input
const PFFORM_READONLY = "readonly"

// TFErr renders a Template error and reports it in the error log
func TFErr(cui PfUI, str string) (o string, buttons string, neditable int, subs []string, multipart bool) {
	cui.Errf("pfform: %s", str)
	return pf.HE("Problem encountered while rendering template"), "", 0, []string{}, false
}

// pfform_keyval returns the value associated with the given key
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

// pfform_select renders a HTML select form from a keyval
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

// pfform_hidden renders a hidden HTML input option
func pfform_hidden(idpfx, fname, val string) (t string) {
	t += "<input"
	t += " id=\"" + idpfx + fname + "\""
	t += " name=\"" + fname + "\""
	t += " type=\"hidden\""
	t += " value=\"" + val + "\""
	t += " />"
	return
}

// pfform_mask renders a maskbox prefix
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

// pfform_masktail renders a maskbox tail.
func pfform_masktail(masknum int) (t string) {
	if masknum == 0 {
		return
	}

	t += "</div>\n"
	return
}

// pfform_string renders an input selector from a string.
//
// In case a kvs is provided with multiple options, a select box
// is rendered if it is allowed to be edited or a hidden input when not.
//
// The val argument is used to select a default item from a keyval.
//
// idpfx provides a prefix for the id of the HTML object.
// fname indicates the fieldname.
// ttype indicates the type of the field (text or string).
// min/max indicate minimal/maximum lengths of the content.
// allowedit is used to indicate if the field is editable or not.
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

// pfform_number renders a number with minmax when provided
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

// pfform_bool renders a boolean toggle box
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

// pfform_submit renders a submit box
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

// pfform_label renders a label for an input box
func pfform_label(idpfx string, fname string, label string, ttype string) (t string) {
	t += "<label for=\"" + idpfx + fname + "\">"

	l := len(label)

	if l > 0 && ttype != "submit" && ttype != "note" && ttype != "widenote" {
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

// pfformA is the recursively called variant of pfform, called by pfform.
//
// It renders the form from a structure.
//
// s = struct where the fields are scanned from and that build up the form
// m = struct where a 'message' and 'error' field should be present
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
				o += "</fieldset>\n"
				o += "</ul>\n"
				o += "</li>\n"
			}

			/* Open new section? */
			*section = sec
			if *section != "" {
				o += "<li>\n"
				o += "<fieldset>\n"
				o += "<legend>" + sec + "</legend>\n"
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

		ftype := f.Type.Kind()

		switch ftype {
		case reflect.String:
			switch ttype {
			case "string", "tel", "email", "submit", "password", "text":
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

			case "note", "widenote":
				val := v.Interface().(string)
				val = pfform_keyval(kvs, val)
				val = pf.HE(val)

				// Fallback to 'content' Tag when no value is given
				if val == "" {
					val = f.Tag.Get("content")
				}

				if val == "" && omitempty {
					t = ""
					break
				}

				// widenotes are fullwidth, thus without label
				if ttype == "widenote" {
					// Thus start over building the HTML
					t = "<li>"
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

// pfform_head renders the head of a HTML form
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

// pfform_tail renders the tail of a HTML form
func pfform_tail() (o string) {
	o += "</ul>\n"
	o += "</fieldset>\n"
	o += "</form>\n"
	return
}

// pfform is the function that gets called from a template
//
// Arguments are the context (cui), the object to render (obj),
// the structure where to retrieve error messages from (m) and
// whether the form is editable (PTYPE_UPDATE) or not (PTYPE_READ).
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
