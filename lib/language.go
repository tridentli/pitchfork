// Pitchfork User Language settings
package pitchfork

// The language name.
type PfLanguage struct {
	Name string // Name of the language
	Code string // Language code ('en', 'de', etc) in ISO 639-1
}

// ToString displays the name of the language.
func (tl *PfLanguage) ToString() (out string) {
	out = tl.Code + "\t" + tl.Name
	return
}

// LanguageList lists the possible languages.
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

// language_list lists the possible languages (CLI).
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

// language_menu provides the CLI menu for languages (CLI).
func language_menu(ctx PfCtx, args []string) (err error) {
	menu := NewPfMenu([]PfMEntry{
		{"list", language_list, 0, -1, nil, PERM_USER, "List details types"},
	})

	err = ctx.Menu(args, menu)
	return
}
