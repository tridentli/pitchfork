package pitchforkui

import (
	"encoding/base32"
	"trident.li/go/rsc/qr"
)

// h_qr renders a string as a QR code image
func h_qr(cui PfUI) {
	path := cui.GetPath()
	if len(path) != 1 {
		H_error(cui, StatusNotFound)
		return
	}

	str := path[0]

	b, err := base32.StdEncoding.DecodeString(str)
	if err != nil {
		cui.Errf("Could not base32 decode incoming QR code")
		H_error(cui, StatusNotFound)
		return
	}

	str = string(b)

	code, err := qr.Encode(str, qr.H)
	if err != nil {
		cui.Errf("QR generation failed: %s", err.Error())
		H_error(cui, StatusNotFound)
		return
	}

	img := code.PNG()

	cui.SetContentType("image/png")
	cui.SetRaw(img)
}
