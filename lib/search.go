// Pitchfork Search interface
//
// The search interface is rendered in the UI at the top of the screen
// in the menu bar. It allows searching through a variety of components
// of pitchfork and other parts that register to the system.
//
// Typically an init function of a piece of code will register
// a search interface with the searcher code.
// This happens using the SearcherRegister function.
// From then on, when something is entered in the search box
// the searcher code will call Search which then asks each
// searcher if they have something that matches the given string.
//
// The Searcher then performs a search and when it finds a matching
// item it calls SearchResult() to add that search result to the
// found items list, which gets returned to the client, either
// in the form of a type-ahead result (JSON).
//
// To allow type-ahead, the abort channel, which is given down
// from the HTTP handler, is used to cancel the request when a new
// request comes in with a different queries.
//
// Each handler will return as many results as it can, though
// one will typically limit it to 20 per module.
//
// Each result consists of a source (typically the module returning the search item), the title of the item, a link to the item and a summary of the item.
//
// In the rendered results we hilight the parts of the summary that matches the searched query.
//
// This search code primarily facilitates the searching.
// The actual searching happens in the module providing the search.
package pitchfork

import (
	"encoding/json"
	"errors"
	"time"
)

// PfSearcherI is a prototype of a function providing a Searcher
type PfSearcherI func(ctx PfCtx, c chan PfSearchResult, search string, abort <-chan bool) (err error)

// searchers describes the list of registered searcher functions, see SearcherRegister.
var searchers []PfSearcherI

// PfSearchResult is a single result of a search
type PfSearchResult struct {
	Source  string `json:"source"`  // Which module provided this result
	Title   string `json:"title"`   // Title of the result
	Link    string `json:"link"`    // Link to the result
	Summary string `json:"summary"` // Summary of the result
}

// SearcherRegister allows an application to register a searcher function.
func SearcherRegister(f PfSearcherI) {
	searchers = append(searchers, f)
}

// SearchResult is called by a Searcher when a result has been found.
func SearchResult(c chan PfSearchResult, source string, title string, link string, summary string) {
	c <- PfSearchResult{source, title, link, summary}
}

// Search is the start of a search - it calls the searchers.
//
// This gets called from the UI when somebody types a new search query.
// It serves both that autocomplete and the full result that can come
// out of the searches as an answer.
func Search(ctx PfCtx, async bool, search string) (results []PfSearchResult, te time.Duration, err error) {
	if len(searchers) == 0 {
		err = errors.New("No Searchers available")
		return
	}

	/* Abort signals other goroutine that any work needs not to be completed */
	abort := make(chan bool)
	done := make(chan error)
	result := make(chan PfSearchResult)

	busy := 0

	/* Start Tracking time */
	t1 := TrackStart()

	for s := 0; s < len(searchers); s++ {
		busy++
		sf := searchers[s]
		go func() {
			done <- sf(ctx, result, search, abort)
		}()
	}

	for busy > 0 {
		select {
		case <-ctx.GetAbort():
			/* Client disconnected, abort all searchers */
			close(abort)
			break

		case err = <-done:
			/* Search completed */
			if err != nil {
				ctx.Errf("Search failed: %s", err)
				ctx.OutLn("Search failed")
			}
			busy--
			break

		case res := <-result:
			if async {
				jsn, e := json.Marshal(res)
				if e != nil {
					err = errors.New("JSON encoding failed: " + e.Error())
					return
				}
				ctx.OutLn("%s", string(jsn))
			} else {
				results = append(results, res)
			}
			break
		}
	}

	te = TrackTime(t1, "Search")

	return
}
