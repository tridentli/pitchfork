package pitchforkui

// h_logout renders a logout page
func h_logout(cui PfUI) {
	/* Log Out */
	cui.Logout()

	/* Redirect to the login page */
	url := "/login/"
	cui.SetRedirect(url, StatusSeeOther)
}
