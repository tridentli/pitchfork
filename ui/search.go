package pitchforkui

import (
	"errors"
	"fmt"
	pf "trident.li/pitchfork/lib"
)

// h_search provides the search capabilities of the UI
func h_search(cui PfUI) {
	async := false

	/* Async search parameter? */
	q := cui.GetArgCSRF("qa")
	if q != "" {
		async = true
	} else {
		/* Normal HTML form based search */
		q, _ = cui.FormValue("q")
	}

	if q == "" {
		H_errmsg(cui, errors.New("No search parameters given"))
		return
	}

	/* Prepare */
	if async {
		/* Switch Content-Type to Sequential JSON */
		cui.SetContentType("application/json-seq")

		/* Switch to unbuffered mode */
		cui.OutBuffered(false)

		/* Flush whatever is there already (should only be headers) */
		cui.Flush()
	}

	results, te, err := pf.Search(cui, async, q)

	if err != nil {
		cui.Errf("Search Failed: %s", err.Error())
		/* Not returned further to user, they will just see a blank response */
	}

	if async {
		/* All done as results have been returned already */
	} else {
		message := fmt.Sprintf("Returned %d results in %s", len(results), te)

		/* Output the page */
		type popt struct {
			Q      string `label:"Search" hint:"What to search for" htmlclass:"search" placeholder:"Search"`
			Button string `label:"Search" pftype:"submit"`
		}

		type Page struct {
			*PfPage
			Opt     popt
			Results []pf.PfSearchResult
			Message string
		}

		p := Page{cui.Page_def(), popt{q, ""}, results, message}
		cui.Page_show("misc/search.tmpl", p)
	}
}
