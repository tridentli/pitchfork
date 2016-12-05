package pitchfork

func Wiki_search(ctx PfCtx, pathroot string, c chan PfSearchResult, search string, abort <-chan bool) (err error) {
	/* XXX: Namespace limiter (Groups) */

	q := "SELECT DISTINCT ON (r.page_id) t.path, r.title, r.markdown " +
		"FROM wiki_page_rev r " +
		"INNER JOIN wiki_namespace t ON r.page_id = t.page_id " +
		"INNER JOIN member ON r.member = member.ident " +
		"WHERE t.path LIKE $1 " +
		"AND r.markdown ILIKE $2 " +
		"ORDER BY r.page_id, path DESC " +
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
		SearchResult(c, "Wiki", title, path, snippet)
	}

	return nil
}
