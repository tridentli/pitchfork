package pitchfork

import (
	"github.com/aryann/difflib"
	"strings"
)

type PfDiff struct {
	Common string
	Left   string
	Right  string
}

func diff_do(a string, b string) (diff []difflib.DiffRecord) {
	tA := strings.Split(a, "\n")
	tB := strings.Split(b, "\n")

	return difflib.Diff(tA, tB)
}

func Diff_Out(ctx PfCtx, a string, b string) {
	df := diff_do(a, b)

	for _, d := range df {
		switch d.Delta {
		case difflib.Common:
			ctx.OutLn(" %s", d.Payload)
			break

		case difflib.LeftOnly:
			ctx.OutLn("-%s", d.Payload)
			break

		case difflib.RightOnly:
			ctx.OutLn("+%s", d.Payload)
			break
		}
	}
}

func DoDiff(a string, b string) (diff []PfDiff) {
	df := diff_do(a, b)

	for _, d := range df {
		var t PfDiff

		switch d.Delta {
		case difflib.Common:
			t.Common = d.Payload
			break

		case difflib.LeftOnly:
			t.Left = d.Payload
			break

		case difflib.RightOnly:
			t.Right = d.Payload
			break
		}

		diff = append(diff, t)
	}

	return
}
