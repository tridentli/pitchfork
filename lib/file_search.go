package pitchfork

// File_search provides a search interface for searching files.
//
// It searches for the given text in the description and the path name.
func File_search(ctx PfCtx, pathroot string, c chan PfSearchResult, search string, abort <-chan bool) (err error) {
	/* XXX: Namespace limiter (Groups) */
	q := "SELECT n.path, n.path, r.description " +
		"FROM file_rev r " +
		"INNER JOIN member ON r.member = member.ident " +
		"INNER JOIN file f ON r.file_id = f.id " +
		"INNER JOIN file_namespace n ON r.file_id = n.file_id " +
		"WHERE PATH LIKE $1 " +
		"AND (" +
		"r.description ILIKE $2 " +
		"OR n.path ILIKE $2 " +
		")" +
		"ORDER BY r.file_id, path DESC " +
		"LIMIT 20"

	pathq := pathroot + "%"
	searchq := "%" + search + "%"

	rows, err := DB.Query(q, pathq, searchq)
	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		/* First check if we need to abort */
		select {
		case <-abort:
			return nil

		default:
			/* Nothing there, do work */
			break
		}

		path := ""
		title := ""
		snippet := ""
		err = rows.Scan(&path, &title, &snippet)
		if err != nil {
			return
		}

		/* The result */
		SearchResult(c, "File", title, path, snippet)
	}

	return nil
}
