package pitchfork

/* BUG(https://github.com/tridentli/pitchfork/issues/77) -- needs to be restored */

import (
	"errors"

	"github.com/nicksnyder/go-i18n/i18n"
)

func SetupTranslation() (err error) {

	/* Load the translation languages */
	for _, f := range Config.TransLanguages {
		fn := System_findfile("languages/", f)
		if fn == "" {
			err = errors.New("Could not find languages file :" + f)
			return
		}
		Dbgf("Loading Language %q...", fn)
		i18n.MustLoadTranslationFile(fn)
	}
	return
}

func TranslateObj(ctx PfCtx, objtrail []interface{}, label string) string {
	/* No need to attempt to translate empty strings */
	if label == "" {
		return label
	}

	return ctx.GetTfunc()(label)
}
