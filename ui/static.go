package pitchforkui

import (
	pf "trident.li/pitchfork/lib"
)

/*
 * Note that ServeFile calls path.clean() to undo '/../' and other
 * such tricks, thus we do not have to take care of that
 */
func h_static_file(cui PfUI, path string) {
	/* Do not allow directory listings */
	if path[len(path)-1:] == "/" {
		H_error(cui, StatusForbidden)
		return
	}

	fn := pf.System_findfile("webroot/", path)
	if fn == "" {
		cui.Logf("Could not find webroot file %s", path)
		H_error(cui, StatusNotFound)
		return
	}

	/* Let it be in cache for an hour */
	cui.SetExpires(1 * 60)

	/* Actual output happens when we are done (cui.Flush()) */
	cui.SetStaticFile(fn)
}

func h_static(cui PfUI) {
	h_static_file(cui, cui.GetFullPath()[1:])
}
