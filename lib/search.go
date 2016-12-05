package pitchfork

import (
	"encoding/json"
	"errors"
	"time"
)

type PfSearcherI func(ctx PfCtx, c chan PfSearchResult, search string, abort <-chan bool) (err error)

var searchers []PfSearcherI

type PfSearchResult struct {
	Source  string `json:"source"`
	Title   string `json:"title"`
	Link    string `json:"link"`
	Summary string `json:"summary"`
}

func SearcherRegister(f PfSearcherI) {
	searchers = append(searchers, f)
}

func SearchResult(c chan PfSearchResult, source string, title string, link string, summary string) {
	c <- PfSearchResult{source, title, link, summary}
}

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
