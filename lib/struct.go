package pitchfork

import (
	"database/sql"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type PTypeField struct {
	reflect.StructField
}

func PTypeWrap(f reflect.StructField) PTypeField {
	return PTypeField{f}
}

type PType int

/* CRUD */
const (
	PTYPE_CREATE PType = iota /* Create */
	PTYPE_READ                /* Read */
	PTYPE_UPDATE              /* Update */
	PTYPE_DELETE              /* Delete */
)

/*
 * Get the datatype from either the forced version
 * or the actual type of the field using reflection
 *
 * If 'doignore' is set return 'ignore'
 */
func PfType(f reflect.StructField, v reflect.Value, doignore bool) (ttype string, dorecurse bool, compound bool) {
	/* Forced type */
	ttype = f.Tag.Get("pftype")

	/* Detected type */
	if ttype == "" {
		/* Always ignore functions */
		if f.Type.Kind() == reflect.Func {
			ttype = "ignore"
			return
		}

		/* When the package path is not empty, we ignore the field as it is not exported */
		if f.PkgPath != "" {
			// Dbg("Skipping %s (pkg: %#v) - unexported", f.Name, f.PkgPath)
			ttype = "ignore"
			return
		}

		switch f.Type.Kind() {
		case reflect.String:
			ttype = "string"
			break

		case reflect.Bool:
			ttype = "bool"
			break

		/* We consider everything just a number, we call it a 'int' out of convienience */
		case reflect.Int, reflect.Int64, reflect.Float64, reflect.Uint, reflect.Uint64:
			ttype = "int"
			break

		case reflect.Struct:
			ty := StructNameT(f.Type)

			switch ty {
			case "time.Time":
				ttype = "time"
				break

			case "database/sql.NullString":
				ttype = "string"
				break

			case "database/sql.NullInt64", "database/sql.NullFloat64":
				ttype = "int"
				break

			case "database/sql.NullBool":
				ttype = "bool"
				break

			default:
				/* Generic struct */
				ttype = "struct"

				o := StructRecurse(v)

				tfunc := "TreatAsString"
				objtrail := []interface{}{o}
				ok, _ := ObjHasFunc(objtrail, tfunc)
				if ok {
					/* Really, it is a string, believe me */
					ttype = "string"
				}
				break
			}

			break

		case reflect.Interface:
			ttype = "interface"
			break

		case reflect.Slice:
			ttype = "slice"
			break

		case reflect.Map:
			ttype = "map"
			break

		case reflect.Ptr:
			ttype = "ptr"
			break

		case reflect.Func:
			ttype = "ignore"
			break

		default:
			panic("Unsupported Reflection Type " + f.Type.Kind().String() + ": " + StructNameT(f.Type))
		}
	}

	if doignore {
		/* Ignore submit buttons and notes */
		if ttype == "submit" || ttype == "note" {
			ttype = "ignore"
		}
	}

	/* Recurse if it is a interface or a generic struct */
	if ttype == "interface" || ttype == "struct" {
		compound = true

		if ttype != "struct" || v.NumField() > 0 {
			dorecurse = true
		}
	}

	return
}

/*
 * Check CanAddr() so that we do a recurse while
 * we can with ability to set, but recurse otherwise
 * in readonly version
 */
func StructRecurse(v reflect.Value) interface{} {
	if v.Kind() != reflect.Interface && v.CanAddr() {
		return v.Addr().Interface()
	}

	return v.Interface()
}

func StructNameT(t reflect.Type) string {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	n := t.PkgPath() + "." + t.Name()
	if n == "." {
		Dbgf("StructNameT() = %s", n)
		panic("StructNameT() could not find a name")
	}
	return n
}

func StructNameObj(obj interface{}) string {
	s, _ := StructReflect(obj)
	n := s.PkgPath() + "." + s.Name()
	if n == "." {
		Dbgf("StructNameObj(%s) obj = %#v", n, obj)
		panic("StructNameObj() could not find a name")
	}
	return n
}

func StructNameObjTrail(objtrail []interface{}) (oname string) {
	for _, obj := range objtrail {
		if oname != "" {
			oname = oname + "->"
		}
		oname = StructNameObj(obj) + oname
	}

	return
}

func StructReflect(obj interface{}) (s reflect.Type, va reflect.Value) {
	s = reflect.TypeOf(obj)

	if s.Kind() == reflect.Ptr {
		// Dereference the pointer
		s = reflect.ValueOf(obj).Type().Elem()
	}

	/* Values (Indirect() takes care of pointer to structs) */
	va = reflect.Indirect(reflect.ValueOf(obj))

	return s, va
}

func StructFetchFields(obj interface{}, table string, q *string, ifs *[]interface{}) (err error) {
	fun := "StructFetchFields() "

	s, va := StructReflect(obj)

	if s.Kind() == reflect.Interface {
		return StructFetchFields(StructRecurse(va), table, q, ifs)
	}

	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		v := va.Field(i)

		ttype, dorecurse, compound := PfType(f, v, true)
		if ttype == "ignore" {
			continue
		}

		if dorecurse {
			err = StructFetchFields(StructRecurse(v), table, q, ifs)
			if err != nil {
				return
			}
		}

		if compound {
			continue
		}

		/* No tags, then ignore it */
		if f.Tag == "" {
			continue
		}

		/* Column/fieldname in SQL Table */
		fname := f.Tag.Get("pfcol")
		if fname == "" {
			fname = strings.ToLower(f.Name)
		}

		/* Custom table to take it from? */
		tname := f.Tag.Get("pftable")
		if tname == "" {
			tname = table
		}

		fname = tname + "." + fname

		if !v.CanSet() {
			err = errors.New("Can't set field '" + fname + "' (" + fun + ")")
			return
		}

		/* Start or continue the SELECT statement */
		if *q == "" {
			*q = "SELECT "
		} else {
			*q += ", "
		}

		coalesce := f.Tag.Get("coalesce")

		ftype := f.Type.Kind()

		/* Handle 'nil's in the database */
		switch ftype {
		case reflect.String:
			*q += "COALESCE(" + fname + ", '" + coalesce + "')"
			break

		case reflect.Int, reflect.Int64, reflect.Float64:
			*q += "COALESCE(" + fname + ", 0)"
			break

		default:
			/* Don't COALESCE as we do not know the type */
			*q += fname
			break
		}

		var vr interface{}

		switch ftype {
		case reflect.String:
			vr = new(string)
			break

		case reflect.Bool:
			vr = new(bool)
			break

		case reflect.Int, reflect.Int64, reflect.Float64:
			vr = new(int64)
			break

		case reflect.Struct:
			ty := StructNameT(f.Type)

			switch ty {
			case "time.Time":
				vr = new(time.Time)
				break

			case "database/sql.NullString":
				vr = new(sql.NullString)
				break

			case "database/sql.NullInt64":
				vr = new(sql.NullInt64)
				break

			case "database/sql.NullFloat64":
				vr = new(sql.NullFloat64)
				break

			case "database/sql.NullBool":
				vr = new(sql.NullBool)
				break

			default:
				if ttype == "string" {
					vr = new(string)
					break
				}

				return errors.New(fun + "Variable '" + fname + "' is an unknown struct: " + ty)
			}
			break

		default:
			var k reflect.Kind
			k = f.Type.Kind()
			return errors.New(fun + "Variable " + fname + " Unknown type: " + k.String())
		}

		*ifs = append(*ifs, vr)
	}

	return nil
}

func StructFetchStore(obj interface{}, ifs []interface{}, ifs_n *int) (err error) {
	fun := "StructFetch() "

	s, va := StructReflect(obj)

	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		v := va.Field(i)

		ttype, dorecurse, compound := PfType(f, v, true)
		if ttype == "ignore" {
			continue
		}

		if dorecurse {
			err = StructFetchStore(StructRecurse(v), ifs, ifs_n)
			if err != nil {
				return
			}
		}

		if compound {
			continue
		}

		/* No tags, then ignore it */
		if f.Tag == "" {
			continue
		}

		/* Column/fieldname in SQL Table */
		fname := f.Tag.Get("pfcol")
		if fname == "" {
			fname = strings.ToLower(f.Name)
		}

		n := *ifs_n

		switch f.Type.Kind() {
		case reflect.String:
			v.SetString(*(ifs[n].(*string)))
			break

		case reflect.Bool:
			v.SetBool(*(ifs[n].(*bool)))
			break

		case reflect.Int, reflect.Int64:
			v.SetInt(*(ifs[n].(*int64)))
			break

		case reflect.Float64:
			v.SetFloat(*(ifs[n].(*float64)))
			break

		case reflect.Struct:
			ty := StructNameT(f.Type)

			switch ty {
			case "time.Time":
				v.Set(reflect.ValueOf(*(ifs[n].(*time.Time))))
				break

			case "database/sql.NullString":
				v.Set(reflect.ValueOf(*(ifs[n].(*sql.NullString))))
				return

			case "database/sql.NullInt64":
				v.Set(reflect.ValueOf(*(ifs[n].(*sql.NullInt64))))
				return

			case "database/sql.NullFloat64":
				v.Set(reflect.ValueOf(*(ifs[n].(*sql.NullFloat64))))
				return

			case "database/sql.NullBool":
				v.Set(reflect.ValueOf(*(ifs[n].(*sql.NullBool))))
				return

			default:
				return errors.New(fun + "Variable '" + fname + "' is an unknown struct: " + ty)
			}
			break

		default:
			var k reflect.Kind
			k = f.Type.Kind()
			return errors.New(fun + "Variable " + fname + " Unknown type: " + k.String())
		}

		/* Next Field */
		n++
		*ifs_n = n
	}

	return nil
}

func StructFetchWhere(qi string, table string, join string, andor DB_AndOr, params []string, matchopts []DB_Op, matches []interface{}, order string) (q string, vals []interface{}) {
	q = qi

	/* From which table */
	q += " FROM " + DB.QI(table)

	if join != "" {
		q += " " + join
	}

	where := ""
	vals = nil

	for n, p := range params {
		if where == "" {
			where += " WHERE "
		} else {
			switch andor {
			case DB_OP_AND:
				where += " AND "
				break

			case DB_OP_OR:
				where += " OR "
				break

			default:
				panic("Invalid andor")
			}
		}

		pp := strings.Split(p, ".")
		if len(pp) == 2 {
			where += DB.QI(pp[0]) + "." + DB.QI(pp[1])
		} else {
			where += DB.QI(p)
		}

		switch matchopts[n] {
		case DB_OP_LIKE:
			where += " LIKE "
			break

		case DB_OP_ILIKE:
			where += " ILIKE "
			break

		case DB_OP_EQ:
			where += " = "
			break

		case DB_OP_NE:
			where += " <> "
			break

		case DB_OP_LE:
			where += " <= "
			break

		case DB_OP_GE:
			where += " >= "
			break

		default:
			panic("Unsupported Match option")
		}

		where += "$" + strconv.Itoa(n+1)
		vals = append(vals, matches[n])
	}

	/* Append the WHERE portion */
	q += where

	q += " " + strings.TrimSpace(order)

	return
}

func StructFetchMulti(newobject func() interface{}, table string, join string, andor DB_AndOr, params []string, matchopts []DB_Op, matches []interface{}, order string, limit int, offset int) (objs []interface{}, err error) {
	var ifs []interface{} = nil

	q := ""
	objs = nil

	obj := newobject()

	err = StructFetchFields(obj, table, &q, &ifs)
	if err != nil {
		return
	}

	if q == "" {
		return nil, errors.New("No fields to retrieve")
	}

	q, vals := StructFetchWhere(q, table, join, andor, params, matchopts, matches, order)

	if limit != 0 {
		q += " LIMIT "
		DB.Q_AddArg(&q, &vals, limit)
	}

	if offset != 0 {
		q += " OFFSET "
		DB.Q_AddArg(&q, &vals, offset)
	}

	/* Execute the query & scan it */
	var rows *Rows
	rows, err = DB.Query(q, vals...)
	if err != nil {
		return
	}

	defer rows.Close()

	/* There should be one */
	for rows.Next() {
		err = rows.Scan(ifs...)
		if err != nil {
			return
		}

		o := newobject()
		n := 0

		err = StructFetchStore(o, ifs, &n)
		objs = append(objs, o)
	}

	return objs, nil
}

func StructFetchA(obj interface{}, table string, join string, params []string, matches []string, order string, notfoundok bool) (err error) {
	q := ""

	var ifs []interface{} = nil

	err = StructFetchFields(obj, table, &q, &ifs)
	if err != nil {
		return
	}

	if q == "" {
		err = errors.New("No fields to retrieve")
		return
	}

	var matchopts []DB_Op
	for _, _ = range params {
		matchopts = append(matchopts, DB_OP_EQ)
	}

	var imatches []interface{}
	for _, m := range matches {
		imatches = append(imatches, m)
	}

	q, vals := StructFetchWhere(q, table, join, DB_OP_AND, params, matchopts, imatches, order)

	/* Only want one back */
	q += " LIMIT 1"

	/* Execute the query & scan it */
	var rows *Rows
	rows, err = DB.Query(q, vals...)
	if err != nil {
		return
	}

	defer rows.Close()

	/* There should be one */
	if !rows.Next() {
		if !notfoundok {
			err = errors.New("No entry in " + table + " with that ID")
			return
		}

		return ErrNoRows
	}

	err = rows.Scan(ifs...)
	if err != nil {
		return
	}

	n := 0
	err = StructFetchStore(obj, ifs, &n)

	return
}

func StructFetch(obj interface{}, table string, params []string, matches []string) (err error) {
	return StructFetchA(obj, table, "", params, matches, "", false)
}

type StructOp uint

const (
	STRUCTOP_SET    StructOp = iota /* Set the item */
	STRUCTOP_ADD                    /* Add the item */
	STRUCTOP_REMOVE                 /* Remove the item */
)

func StructFieldMod(op StructOp, fname string, f reflect.StructField, v reflect.Value, value interface{}) (err error) {
	fun := "StructFieldMod() "

	/* What kind of object is this? */
	kind := f.Type.Kind()

	/* Check that this type of operand is actually allowed */
	switch op {
	case STRUCTOP_SET:
		if kind == reflect.Slice {
			return errors.New("Can't 'set' a slice type: " + StructNameT(f.Type))
		}
		break

	case STRUCTOP_ADD:
		if kind != reflect.Slice {
			return errors.New("Can't add to non-slice type: " + StructNameT(f.Type))
		}
		break

	case STRUCTOP_REMOVE:
		if kind != reflect.Slice {
			return errors.New("Can't remove from non-slice type: " + StructNameT(f.Type))
		}
		break

	default:
		return errors.New("Unknown STRUCTOP")
	}

	vo := reflect.ValueOf(value)

	switch kind {
	case reflect.String:
		v.SetString(value.(string))
		return nil

	case reflect.Bool:
		switch vo.Kind() {
		case reflect.String:
			v.SetBool(IsTrue(value.(string)))
			break

		case reflect.Bool:
			v.SetBool(value.(bool))
			break

		default:
			return errors.New(fun + "Variable " + fname + " Unknown source type: " + vo.Kind().String())
		}
		return nil

	case reflect.Int, reflect.Int64:
		switch vo.Kind() {
		case reflect.String:
			number, err := strconv.ParseInt(value.(string), 10, 64)
			if err != nil {
				return errors.New(fun + "Variable " + fname + " Invalid number encountered: '" + value.(string) + "'")
			}
			v.SetInt(number)
			break

		case reflect.Int, reflect.Int64:
			v.SetInt(value.(int64))
			break

		default:
			return errors.New(fun + "Variable " + fname + " Invalid Type")
		}
		return nil

	case reflect.Uint, reflect.Uint64:
		switch vo.Kind() {
		case reflect.String:
			number, err := strconv.Atoi(value.(string))
			if err != nil {
				return errors.New(fun + "Variable " + fname + " Invalid number encountered: '" + value.(string) + "'")
			}
			v.SetUint(uint64(number))
			break

		case reflect.Int, reflect.Int64:
			v.SetUint(value.(uint64))
			break

		default:
			return errors.New(fun + "Variable " + fname + " Invalid Type")
		}
		return nil

	case reflect.Float64:
		switch vo.Kind() {
		case reflect.String:
			number, err := strconv.ParseFloat(value.(string), 64)
			if err != nil {
				return errors.New(fun + "Variable " + fname + " Invalid floating number encountered: '" + value.(string) + "'")
			}
			v.SetFloat(number)
			break

		case reflect.Float64:
			v.SetFloat(value.(float64))
			break

		default:
			return errors.New(fun + "Variable " + fname + " Invalid Type")
		}
		return nil

	case reflect.Struct:
		ty := StructNameT(f.Type)
		switch ty {
		case "time.Time":
			var no time.Time
			no, err = time.Parse(Config.TimeFormat, value.(string))
			if err != nil {
				return
			}
			v.Set(reflect.ValueOf(no))
			return

		case "database/sql.NullString":
			switch vo.Kind() {
			case reflect.String:
				no := sql.NullString{String: value.(string), Valid: true}
				v.Set(reflect.ValueOf(no))
				break

			default:
				return errors.New(fun + "Variable " + fname + " Invalid Type")
			}
			return

		case "database/sql.NullInt64":
			switch vo.Kind() {
			case reflect.String:
				valid := true
				var number int64 = 0
				if value.(string) == "" {
					valid = false
				} else {
					number, err = strconv.ParseInt(value.(string), 10, 64)
					if err != nil {
						return errors.New(fun + "Variable " + fname + " Invalid number encountered: '" + value.(string) + "'")
					}
				}

				no := sql.NullInt64{Int64: number, Valid: valid}
				v.Set(reflect.ValueOf(no))
				break

			case reflect.Int, reflect.Int64:
				no := NI64(value.(int64))
				v.Set(reflect.ValueOf(no))
				break

			default:
				return errors.New(fun + "Variable " + fname + " Invalid Type")
			}
			return

		case "database/sql.NullFloat64":
			switch vo.Kind() {
			case reflect.String:
				valid := true
				var number float64
				if value.(string) == "" {
					valid = false
				} else {
					number, err = strconv.ParseFloat(value.(string), 64)
				}
				if err != nil {
					return errors.New(fun + "Variable " + fname + " Invalid floating number encountered: '" + value.(string) + "'")
				}
				no := sql.NullFloat64{Float64: number, Valid: valid}
				v.Set(reflect.ValueOf(no))
				break

			case reflect.Float64:
				no := sql.NullFloat64{Float64: value.(float64), Valid: true}
				v.Set(reflect.ValueOf(no))
				break

			default:
				return errors.New(fun + "Variable " + fname + " Invalid Type")
			}
			return

		case "database/sql.NullBool":
			switch vo.Kind() {
			case reflect.String:
				yesno := IsTrue(value.(string))
				no := sql.NullBool{Bool: yesno, Valid: true}
				v.Set(reflect.ValueOf(no))
				break

			case reflect.Bool:
				no := sql.NullBool{Bool: value.(bool), Valid: true}
				v.Set(reflect.ValueOf(no))
				break

			default:
				return errors.New(fun + "Variable " + fname + " Invalid Type")
			}

			return
		}

		/* Check if the object supports the Scan interface */
		o := StructRecurse(v)
		tfunc := "Scan"
		objtrail := []interface{}{o}
		ok, obj := ObjHasFunc(objtrail, tfunc)
		if ok {
			/* Scan() the value in */
			res, err2 := ObjFunc(obj, tfunc, value)
			if err2 == nil {
				err2, ok := res[0].Interface().(error)
				if ok {
					err = err2
				}

				return
			}
		}

		return errors.New(fun + "Variable '" + fname + "' is an unknown struct: " + ty)

	case reflect.Slice:
		switch op {
		case STRUCTOP_ADD:
			/* What do we store here? */
			vn := v.Type().String()

			switch vn {
			case "[]string":
				break

			case "[]int":
				/* Input a string or a int? */
				switch vo.Kind() {
				case reflect.String:
					number, err := strconv.Atoi(value.(string))
					if err != nil {
						return errors.New(fun + "Variable " + fname + " Invalid number encountered: '" + value.(string) + "'")
					}
					vo = reflect.ValueOf(number)
					break

				case reflect.Uint, reflect.Uint64:
					vo = reflect.ValueOf(value.(uint64))
					break

				default:
					return errors.New(fun + " detected a unsupported type for " + fname)
				}
				break
			}

			n := reflect.Append(v, vo)
			v.Set(n)
			return nil

		case STRUCTOP_REMOVE:
			/* What do we store here? */
			vn := v.Type().String()

			/* Found it? */
			found := -1

			/* First, find the item we want to remove */
			for k := 0; found == -1 && k < v.Len(); k += 1 {
				switch vn {
				case "[]string":
					ov := v.Index(k).Interface().(string)
					if ov == value.(string) {
						found = k
					}
					break

				case "[]int", "[]uint64":
					var ov uint64

					switch vn {
					case "[]int":
						ov = uint64(v.Index(k).Interface().(int))
						break

					case "[]uint64":
						ov = v.Index(k).Interface().(uint64)
						break

					default:
						return errors.New("Unsupported integer?")
					}

					/* Input a string or a int? */
					switch vo.Kind() {
					case reflect.String:
						number, err := strconv.Atoi(value.(string))
						if err != nil {
							return errors.New(fun + "Variable " + fname + " invalid number encountered: '" + value.(string) + "'")
						}

						if uint64(number) == ov {
							found = k
						}
						break

					case reflect.Uint:
						number := value.(int)

						if uint64(number) == ov {
							found = k
						}

						break

					case reflect.Uint64:
						number := value.(uint64)
						if number == ov {
							found = k
						}
						break

					default:
						return errors.New(fun + " detected a unsupported type for " + fname)
					}
					break

				default:
					return errors.New("Do not support removing from slice of type " + vn)
				}
			}

			if found == -1 {
				return errors.New("Item not found, thus cannot remove")
			}

			/* Create a new slice with all elements except the found one */
			n := v.Slice(0, found)
			n = reflect.AppendSlice(n, v.Slice(found+1, v.Len()))

			/* Set the slice to the new one, which does not have the item */
			v.Set(n)
			return nil
		}

		/* Handled nicer above */
		panic("Cannot apply STRUCTOP_SET to a Slice")

	/* TODO support reflect.Map */

	default:
		var k reflect.Kind
		k = f.Type.Kind()
		return errors.New(fun + "Variable " + fname + " Unknown type: " + k.String())
	}
}

func StructModA(op StructOp, obj interface{}, field string, value interface{}) (done bool, err error) {
	fun := "StructMod() "

	done = false

	field = strings.ToLower(field)

	s, va := StructReflect(obj)

	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		v := va.Field(i)

		ttype, dorecurse, compound := PfType(f, v, true)
		if ttype == "ignore" {
			continue
		}

		if dorecurse {
			done, err = StructModA(op, StructRecurse(v), field, value)
			if done || err != nil {
				return
			}
		}

		if compound {
			continue
		}

		/* No tags, then ignore it */
		if f.Tag == "" {
			continue
		}

		/* Column/fieldname in SQL Table */
		fname := f.Tag.Get("pfcol")
		if fname == "" {
			fname = strings.ToLower(f.Name)
		}

		/* Not this field? */
		if fname != field {
			continue
		}

		if !v.CanSet() {
			err = errors.New(fun + "Can't set field '" + fname + "'")
			return
		}

		done = true
		err = StructFieldMod(op, fname, f, v, value)
		return
	}

	return
}

func StructMod(op StructOp, obj interface{}, field string, value interface{}) (err error) {
	done, err := StructModA(op, obj, field, value)
	if err == nil && !done {
		err = ErrNoRows
		return
	}

	return
}

/*
 * Return all fields of a struct that can be retrieved or modified
 */
func StructVars(ctx PfCtx, obj interface{}, ptype PType, doignore bool) (vars map[string]string, err error) {
	objtrail := []interface{}{}
	vars = make(map[string]string)
	err = StructVarsA(ctx, objtrail, obj, ptype, doignore, vars)
	return vars, err
}

func StructVarsA(ctx PfCtx, objtrail []interface{}, obj interface{}, ptype PType, doignore bool, vars map[string]string) (err error) {
	s, va := StructReflect(obj)

	objtrail = append([]interface{}{obj}, objtrail...)

	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		v := va.Field(i)

		// ctx.Dbgf("StructVars: %s [%s]", f.Name, f.Type.Kind().String())

		ttype, dorecurse, compound := PfType(f, v, true)
		if ttype == "ignore" {
			continue
		}

		if dorecurse {
			err = StructVarsA(ctx, objtrail, StructRecurse(v), ptype, doignore, vars)
			if err != nil {
				return
			}
		}

		if compound {
			continue
		}

		/* No tags, then ignore it */
		if f.Tag == "" {
			continue
		}

		/* Column/fieldname in SQL Table */
		fname := f.Tag.Get("pfcol")
		if fname == "" {
			fname = strings.ToLower(f.Name)
		}

		var ok bool

		ok, _, err = StructPermCheck(ctx, ptype, objtrail, PTypeWrap(f))
		// ctx.Dbgf("StructVars: %s - permcheck: %s, err: %v", f.Name, YesNo(ok), err)
		if err != nil {
			skipfailperm := f.Tag.Get("pfskipfailperm")
			if skipfailperm == "" {
				ctx.Dbgf("StructVars: %s - permcheck: %s, err: %s", f.Name, YesNo(ok), err.Error())
			}
			continue
		}

		if !ok && ttype != "ptr" && ttype != "struct" {
			// oname := StructNameObjTrail(objtrail)
			// ctx.Dbg("NOT SHOWING: field = %s, ttype = %s", oname+":"+fname, ttype)
			continue
		}

		vars[fname] = ttype
	}

	err = nil
	return
}

type StructDetails_Options int

const (
	SD_None                              = 0
	SD_Perms_Check StructDetails_Options = 0 << iota
	SD_Perms_Ignore
	SD_Tags_Require
	SD_Tags_Ignore
)

func StructDetailsA(ctx PfCtx, obj interface{}, field string, opts StructDetails_Options) (ftype string, fname string, fvalue string, err error) {
	checkperms := false
	if opts&SD_Perms_Check > 0 {
		checkperms = true
	}

	requiretags := false
	if opts&SD_Tags_Require > 0 {
		requiretags = true
	}

	s, va := StructReflect(obj)

	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		v := va.Field(i)

		/* Column/fieldname in SQL Table */
		fname = f.Tag.Get("pfcol")
		if fname == "" {
			fname = strings.ToLower(f.Name)
		}

		/* Ignore the field completely? */
		ttype, dorecurse, compound := PfType(f, v, true)
		if ttype == "ignore" {
			if fname == field {
				return "ignore", "", "", errors.New("Field is ignored")
			}
			continue
		}

		if dorecurse {
			ftype, fname, fvalue, err = StructDetailsA(ctx, StructRecurse(v), field, opts)
			if ftype != "" || err != nil {
				return
			}
		}

		if compound {
			continue
		}

		/* No tags, then ignore it */
		if requiretags && f.Tag == "" {
			continue
		}

		/* Wrong field, skip it */
		if fname != field {
			continue
		}

		if checkperms {
			ok := true
			permstr := f.Tag.Get("pfset")
			ok, err = ctx.CheckPermsT("StructDetails("+fname+")", permstr)
			if !ok {
				return "", "", "", err
			}
		}

		return "string", fname, ToString(v.Interface()), nil
	}

	return "", "", "", nil
}

func StructDetails(ctx PfCtx, obj interface{}, field string, opts StructDetails_Options) (ftype string, fname string, fvalue string, err error) {
	field = strings.ToLower(field)

	ftype, fname, fvalue, err = StructDetailsA(ctx, obj, field, opts)
	if err == nil && ftype == "" {
		return "unknown", "", "", errors.New("Unknown Field: " + field + " (StructDetails)")
	}

	return
}

func StructTagA(obj interface{}, field string, tag string) (val string, err error) {
	s, va := StructReflect(obj)

	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		v := va.Field(i)

		ttype, dorecurse, compound := PfType(f, v, true)
		if ttype == "ignore" {
			continue
		}

		if dorecurse {
			val, err = StructTagA(StructRecurse(v), field, tag)
			if err != nil || val != "" {
				return
			}
		}

		if compound {
			continue
		}

		/* No tags, then ignore it */
		if f.Tag == "" {
			continue
		}

		/* Column/fieldname in SQL Table */
		fname := f.Tag.Get("pfcol")
		if fname == "" {
			fname = strings.ToLower(f.Name)
		}

		if fname != field {
			continue
		}

		val = f.Tag.Get(tag)
		return
	}

	return "", nil
}

func StructTag(obj interface{}, field string, tag string) (val string, err error) {
	field = strings.ToLower(field)

	val, err = StructTagA(obj, field, tag)
	if err == nil && val == "" {
		return "", errors.New("Unknown Field: " + field + " (StructTag)")
	}

	return
}

/* Create a "get" or "set" menu from a struct */
func StructMenu(ctx PfCtx, subjects []string, obj interface{}, onlyslices bool, fun PfFunc) (menu PfMenu, err error) {
	var isedit bool

	/* Select the Object */
	ctx.SelectObject(&obj)

	/* Number of subjects */
	nargs := len(subjects)

	/* Edit or not? */
	if fun != nil {
		isedit = true

		/* Edit's require one more argument */
		nargs++
	} else {
		fun = structGet
	}

	/* Recursive call */
	objtrail := []interface{}{}
	return StructMenuA(ctx, subjects, objtrail, obj, onlyslices, fun, isedit, nargs)
}

func StructMenuA(ctx PfCtx, subjects []string, objtrail []interface{}, obj interface{}, onlyslices bool, fun PfFunc, isedit bool, nargs int) (menu PfMenu, err error) {
	/* Prepend this object to the trail */
	objtrail = append([]interface{}{obj}, objtrail...)

	s, va := StructReflect(obj)

	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		v := va.Field(i)

		ttype, dorecurse, compound := PfType(f, v, true)
		if ttype == "ignore" {
			continue
		}

		if dorecurse {
			m, err := StructMenuA(ctx, subjects, objtrail, StructRecurse(v), onlyslices, fun, isedit, nargs)
			if err != nil {
				return PfMenu{}, err
			}

			menu.Add(m.M...)
		}

		if compound {
			continue
		}

		/* No tags, then ignore it */
		if f.Tag == "" {
			continue
		}

		/* Ignore slices when we don't want them, others if we only want slices */
		if (ttype == "slice" && onlyslices == false) || (ttype != "slice" && onlyslices == true) {
			continue
		}

		/* Column/fieldname in SQL Table */
		fname := f.Tag.Get("pfcol")
		if fname == "" {
			fname = strings.ToLower(f.Name)
		}

		/* Options from the Tag of the structure */
		label := f.Tag.Get("label")
		if label != "" {
			/* Only translate when the label is specifically set */
			label = TranslateObj(ctx, objtrail, label)
		} else {
			label = f.Name
		}

		hint := f.Tag.Get("hint")
		if hint != "" {
			/* Only translate when the hint is specifically set */
			hint = TranslateObj(ctx, objtrail, hint)
		}

		/* Default description to the label */
		desc := label

		/* Append the hint to the description */
		if hint != "" {
			desc += " - " + hint
		}

		/* Ignore the field completely? */
		ignore := f.Tag.Get("pfignore")
		if ignore == "yes" {
			continue
		}

		var perms Perm
		var tag string

		if isedit {
			tag = "pfset"
		} else {
			tag = "pfget"
		}

		set := f.Tag.Get(tag)
		perms, err = ctx.ConvertPerms(set)
		if err != nil {
			return
		}

		if perms == PERM_NOTHING {
			/* Default permissions is to allow getting/setting of anything */
			perms = PERM_NONE
		}

		var ok bool
		ok, _ = ctx.CheckPerms("StructMenu("+fname+")", perms)
		if !ok {
			/* Also change to 'ok, err' above */
			/* Dbgf("StructMenu(%s) Skipping (tag: %s), err: %s", fname, tag, err.Error()) */
			continue
		}

		/* Initial subjects */
		subj := subjects

		if isedit {
			otype := ""

			switch ttype {
			case "bool":
				otype = "#bool"
				break

			case "int":
				otype = "#int"
				break

			case "file":
				otype = "#file"
				otype += "#" + f.Tag.Get("pfmaximagesize")

				b64 := f.Tag.Get("pfb64")
				otype += "#" + NormalizeBoolean(b64)
				break

			case "string", "text", "tel":
				otype = "#string"
				break

			case "time":
				otype = "#time"
				break

			case "struct":
				break

			case "slice":
				break

			case "map":
				break

			case "ptr":
				break

			default:
				panic("Unknown Type for field " + fname + ", type " + ttype)
			}

			subj = append(subj, fname+otype)
		}

		var m PfMEntry
		m.Cmd = fname
		m.Fun = fun
		m.Args_min = nargs
		m.Args_max = nargs
		m.Args = subj
		m.Perms = perms
		m.Desc = desc

		menu.Add(m)
	}

	return menu, nil
}

func structGetA(ctx PfCtx, obj interface{}, field string) (done bool, err error) {
	s, va := StructReflect(obj)

	done = false

	if s.Kind() == reflect.Interface {
		return structGetA(ctx, StructRecurse(va), field)
	}

	if s.Kind() != reflect.Struct {
		err = errors.New("Error: parameter is not a struct/interface but " + s.String() + " (structGet)")
		return
	}

	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		v := va.Field(i)

		ttype, dorecurse, compound := PfType(f, v, true)
		if ttype == "ignore" {
			continue
		}

		if dorecurse {
			done, err = structGetA(ctx, StructRecurse(v), field)
			if done || err != nil {
				return
			}
		}

		if compound {
			continue
		}

		/* No tags, then ignore it */
		if f.Tag == "" {
			continue
		}

		/* Column/fieldname in SQL Table */
		fname := f.Tag.Get("pfcol")
		if fname == "" {
			fname = strings.ToLower(f.Name)
		}

		/* Wrong field -> next! */
		if fname != field {
			continue
		}

		/* Ignore the field completely? */
		ignore := f.Tag.Get("pfignore")
		if ignore == "yes" {
			continue
		}

		/*
		 * Note: structGet does not check permissions,
		 * it is only used by StructMenu() which does
		 * check for permissions
		 */
		str := ToString(v.Interface())
		ctx.OutLn(str)

		done = true
		err = nil
		return
	}

	return
}

/* Create a "get" or "set" menu from a struct -- called from StructMenuA() */
func structGet(ctx PfCtx, args []string) (err error) {
	obj := ctx.SelectedObject()

	if obj == nil {
		return errors.New("No object selected")
	}

	field := ctx.GetLastPart()

	done, err := structGetA(ctx, obj, field)
	if err == nil && !done {
		err = errors.New("Unknown property")
	}

	return
}

func ToString(v interface{}) (str string) {
	s, _ := StructReflect(v)

	switch s.Kind() {

	case reflect.String:
		return v.(string)

	case reflect.Bool:
		return YesNo(v.(bool))

	case reflect.Int:
		return strconv.Itoa(v.(int))

	case reflect.Uint:
		return strconv.FormatUint(uint64(v.(uint)), 10)

	case reflect.Int64:
		return strconv.FormatInt(v.(int64), 10)

	case reflect.Uint64:
		return strconv.FormatUint(v.(uint64), 10)

	case reflect.Float64:
		return strconv.FormatFloat(v.(float64), 'E', -1, 64)

	case reflect.Struct:
		ty := StructNameT(s)

		switch ty {
		case "time.Time":
			no := v.(time.Time)
			return no.Format(Config.TimeFormat)

		case "database/sql.NullString":
			no := v.(sql.NullString)
			if !no.Valid {
				return ""
			}
			return ToString(no.String)

		case "database/sql.NullInt64":
			no := v.(sql.NullInt64)
			if !no.Valid {
				return ""
			}
			return ToString(no.Int64)

		case "database/sql.NullFloat64":
			no := v.(sql.NullFloat64)
			if !no.Valid {
				return ""
			}
			return ToString(no.Float64)

		case "database/sql.NullBool":
			no := v.(sql.NullBool)
			if !no.Valid {
				return ""
			}
			return ToString(no.Bool)

		default:
			/* Try if the object has a String() function */
			tfunc := "String"
			objtrail := []interface{}{v}
			ok, obj := ObjHasFunc(objtrail, tfunc)
			if ok {
				s, err := ObjFuncStr(obj, tfunc)
				if err == nil {
					return s
				}
			}

			panic("ToString() Unhandled Struct Type '" + ty + "' : " + s.String())
		}
	}

	panic("ToString() Unhandled Type: " + s.String())
}

type ObjFuncI struct {
	obj interface{}
}

func ObjHasFunc(objtrail []interface{}, fun string) (ok bool, obj ObjFuncI) {
	ok = false

	for _, ob := range objtrail {
		o := reflect.ValueOf(ob)

		if o.IsValid() {
			f := o.MethodByName(fun)
			if f.IsValid() {
				ok = true
				obj.obj = ob
				return
			}
		} else {
			Errf("Not a valid object: %#v", obj)
		}
	}

	return
}

func ObjFunc(obj ObjFuncI, fun string, params ...interface{}) (result []reflect.Value, err error) {
	result = nil
	err = nil

	o := reflect.ValueOf(obj.obj)
	if !o.IsValid() {
		err = errors.New("Not a valid object")
		return
	}

	f := o.MethodByName(fun)
	if !f.IsValid() {
		err = errors.New("Unknown Function " + fun)
		return
	}

	pnum := f.Type().NumIn()
	if (f.Type().IsVariadic() && len(params) < pnum) || (!f.Type().IsVariadic() && len(params) != pnum) {
		vtxt := ""
		if f.Type().IsVariadic() {
			vtxt = " [note: variadic]"
		}
		err = errors.New("Wrong amount of parameters, got: " + strconv.Itoa(len(params)) + ", need: " + strconv.Itoa(pnum) + vtxt)
		panic("Need more")
	}

	in := make([]reflect.Value, len(params))

	for k, param := range params {
		/* Avoid a null Value */
		if param == nil {
			in[k] = reflect.ValueOf(&param).Elem()
		} else {
			in[k] = reflect.ValueOf(param)
		}
	}

	result = f.Call(in)
	return
}

func ObjFuncIface(obj ObjFuncI, fun string, params ...interface{}) (iface interface{}, err error) {
	res, err := ObjFunc(obj, fun, params...)

	if err == nil {
		iface = res[0].Interface()
	} else {
		iface = nil
	}

	return
}

func ObjFuncStr(obj ObjFuncI, fun string, params ...interface{}) (str string, err error) {
	res, err := ObjFunc(obj, fun, params...)

	if err == nil {
		if res[0].Kind() == reflect.String {
			str = res[0].String()
		} else {
			str = fun + "()-not-a-string"
		}
	} else {
		str = fun + "()-failed"
	}

	return
}

func ObjPermCheck(ctx PfCtx, obj ObjFuncI, ptype PType, f PTypeField) (ok bool, allowedit bool, err error) {
	res, err := ObjFunc(obj, "PermCheck", ctx, ptype, f)

	if err == nil {
		var varok bool

		ok = res[0].Interface().(bool)
		allowedit = res[1].Interface().(bool)
		err, varok = res[2].Interface().(error)
		if !varok {
			err = nil
		}
	} else {
		ok = false
		allowedit = false
	}

	return
}

func StructPermCheck(ctx PfCtx, ptype PType, objtrail []interface{}, f PTypeField) (ok bool, allowedit bool, err error) {
	switch ptype {
	case PTYPE_CREATE, PTYPE_UPDATE:
		allowedit = true
		break

	case PTYPE_READ, PTYPE_DELETE:
		allowedit = false
		break

	default:
		panic("Unknown ptype")
	}

	/* Check Application specific permissions */
	app_perms, obj := ObjHasFunc(objtrail, "PermCheck")
	if app_perms {
		ok, allowedit, err = ObjPermCheck(ctx, obj, ptype, f)

		if err == nil && !ok && allowedit {
			/* Retry in read mode */
			ptype = PTYPE_READ
			ok, allowedit, err = ObjPermCheck(ctx, obj, ptype, f)
		}

		/* Errors or denies give a direct answer */
		if err != nil || !ok {
			return
		}
	}

	/* If there is a Pitchfork tag it also gets to make a decision */
	tag := "pfget"
	if allowedit {
		tag = "pfset"
	}

	permstr := f.Tag.Get(tag)

	if !app_perms || permstr != "" {
		ok, err = ctx.CheckPermsT("StructPermCheck("+f.Name+"/"+tag+"/"+permstr+")", permstr)
		if !ok && allowedit {
			allowedit = false
			tag = "pfget"
			permstr := f.Tag.Get(tag)

			/* Use the fail for pfset, if no pfget is defined and pfset errored */
			if permstr == "" && err != nil {
				return
			}

			/* Fall back */
			ok, err = ctx.CheckPermsT("StructPermCheck("+f.Name+"/get/"+permstr+")", permstr)
			if err != nil {
				return
			}
		}
	}

	return
}
