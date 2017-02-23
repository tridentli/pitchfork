// Pitchfork's Main Menu
package pitchfork

// MainMenu is the Main CLI Menu for Pitchfork.
var MainMenu = NewPfMenu([]PfMEntry{
	{"user", user_menu, 0, -1, nil, PERM_NONE, "User commands"},
	{"group", group_menu, 0, -1, nil, PERM_USER, "Group commands"},
	{"ml", ml_menu, 0, -1, nil, PERM_USER, "Mailing List commands"},
	{"system", system_menu, 0, -1, nil, PERM_NONE, "System commands"},
})
