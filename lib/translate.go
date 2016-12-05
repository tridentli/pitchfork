package pitchfork

func TranslateObj(ctx PfCtx, objtrail []interface{}, label string) string {
	/* No need to attempt to translate empty strings */
	if label == "" {
		return label
	}

	tfunc := "Translate"

	// ctx.Dbg("Translate trying %d objects", len(objtrail))

	ok, obj := ObjHasFunc(objtrail, tfunc)
	if ok {
		/* Try it */
		res, err := ObjFuncStr(obj, tfunc, label, ctx.GetLanguage())
		if err == nil {
			if res != label {
				// ctx.Dbg("%s-Translate() translated %s to %s", sname, label, res)
				/* Got a translation */
				return res
			} else {
				// ctx.Dbg("%s-Translate() labels the same: %s", sname, res)
			}
		} else {
			/* Translation Function failed */
			ctx.Errf("Translate function failed >>>%s<<<: %s", label, err.Error())
		}
	}

	/* Note that a translation is needed */
	oname := StructNameObjTrail(objtrail)
	ctx.Errf("Translation missing %s >>>%s<<<", oname, label)
	return label
}
