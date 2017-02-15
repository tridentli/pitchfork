// Pitchfork Language Module
package pitchfork

import (
	"time"
)

// PfUserLanguage describes a user language setting
type PfUserLanguage struct {
	Language PfLanguage // The language
	Skill    string     // The attained skill
	Entered  time.Time  // When it was added
}

// ToString returns a textual rendering of the language detail
func (ul *PfUserLanguage) ToString() (out string) {
	out = ul.Language.ToString() + " " +
		ul.Skill + " " +
		"Entered:" + ul.Entered.Format(time.UnixDate)
	return
}

// GetLanguages gets the possible languages
func (user *PfUserS) GetLanguages() (output []PfUserLanguage, err error) {
	q := "SELECT " +
		"mls.language, " +
		"l.name, " +
		"mls.entered, " +
		"mls.skill " +
		"FROM member_language_skill mls " +
		"INNER JOIN languages l ON mls.language = l.iso_639_1 " +
		"AND mls.member = $1"
	rows, err := DB.Query(q, user.GetUserName())
	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		var user_lang PfUserLanguage
		var lang PfLanguage

		err = rows.Scan(&lang.Code, &lang.Name, &user_lang.Entered, &user_lang.Skill)
		if err != nil {
			return
		}

		user_lang.Language = lang

		output = append(output, user_lang)
	}

	return
}

// LanguageSkillList lists the skills for a language.
func LanguageSkillList() (languageskill []string) {
	q := "SELECT skill " +
		"FROM language_skill " +
		"ORDER BY seq"
	rows, err := DB.Query(q)

	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		var t_skill string

		err = rows.Scan(&t_skill)
		if err != nil {
			languageskill = nil
			return
		}

		languageskill = append(languageskill, t_skill)
	}

	return
}

// user_lang_list lists the languages for a user (CLI).
func user_lang_list(ctx PfCtx, args []string) (err error) {
	username := args[0]

	err = ctx.SelectUser(username, PERM_USER_SELF)
	if err != nil {
		return
	}

	user := ctx.SelectedUser()

	languages, err := user.GetLanguages()
	var ls PfUserLanguage

	for _, ls = range languages {
		ctx.OutLn(ls.ToString())
	}

	return
}

// user_lang_skill shows the language skill levels (CLI).
func user_lang_skill(ctx PfCtx, args []string) (err error) {
	types := LanguageSkillList()

	ctx.OutLn("Skill Level\n")

	for _, t_skill := range types {
		ctx.OutLn("%s", t_skill)
	}

	return
}

// user_lang_set sets a language skill level (CLI).
func user_lang_set(ctx PfCtx, args []string) (err error) {
	username := args[0]

	err = ctx.SelectUser(username, PERM_USER_SELF)
	if err != nil {
		return
	}

	user := ctx.SelectedUser()
	lang := args[1]
	skill := args[2]

	q := "INSERT INTO member_language_skill " +
		"(member, language, skill, entered) " +
		"VALUES($1, $2, $3, now())"
	err = DB.Exec(ctx,
		"Added new member_language_skill ($1,$2,$3)",
		1, q,
		user.GetUserName(), lang, skill)
	if err != nil {
		return
	}

	return
}

// user_lang_delete removes a language (CLI).
func user_lang_delete(ctx PfCtx, args []string) (err error) {
	username := args[0]

	err = ctx.SelectUser(username, PERM_USER_SELF)
	if err != nil {
		return
	}

	user := ctx.SelectedUser()
	lang := args[1]

	q := "DELETE FROM member_language_skill " +
		"WHERE member = $1 " +
		"AND language = $2 "
	err = DB.Exec(ctx,
		"Delete member_language_skill ($1,$2)",
		1, q,
		user.GetUserName(), lang)
	if err != nil {
		return
	}

	return
}

// user_language is the user's language menu (CLI).
func user_language(ctx PfCtx, args []string) (err error) {
	menu := NewPfMenu([]PfMEntry{
		{"list", user_lang_list, 1, 1, []string{"username"}, PERM_USER, "List user language skills"},
		{"list_skill", user_lang_skill, 0, 0, nil, PERM_NONE, "List possible language skill levels"},
		{"set", user_lang_set, 3, 3, []string{"username", "language", "skill"}, PERM_USER_SELF, "Set a language skill"},
		{"delete", user_lang_delete, 2, 2, []string{"username", "language"}, PERM_USER_SELF, "Delete a language skill"},
	})

	err = ctx.Menu(args, menu)
	return
}
