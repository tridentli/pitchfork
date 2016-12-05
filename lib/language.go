package pitchfork

type PfLanguage struct {
	Name string
	Code string
}

func (tl *PfLanguage) ToString() (out string) {
	out = tl.Code + "\t" + tl.Name
	return
}

func LanguageList() (languages []PfLanguage, err error) {
	q := "SELECT " +
		"name, " +
		"iso_639_1 " +
		"FROM languages " +
		"ORDER BY iso_639_1"
	rows, err := DB.Query(q)

	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		var language PfLanguage

		err = rows.Scan(&language.Name, &language.Code)
		if err != nil {
			return
		}

		languages = append(languages, language)
	}
	return
}

func language_list(ctx PfCtx, args []string) (err error) {
	languages, err := LanguageList()
	if err != nil {
		return
	}

	ctx.OutLn("Detail\tDescription\n")

	for _, language := range languages {
		ctx.OutLn("%s", language.ToString())
	}

	return
}

func language_menu(ctx PfCtx, args []string) (err error) {
	menu := NewPfMenu([]PfMEntry{
		{"list", language_list, 0, -1, nil, PERM_USER, "List details types"},
	})

	err = ctx.Menu(args, menu)
	return
}
