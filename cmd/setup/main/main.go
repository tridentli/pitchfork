package main

import (
	pf "trident.li/pitchfork/lib"
	pf_cmd_setup "trident.li/pitchfork/cmd/setup"
)

func newctx() pf.PfCtx {
	return pf.NewPfCtx(nil, nil, nil, nil, nil)
}

func main() {
	pf_cmd_setup.Setup(
		"pitchfork-setup", // tname
		"pitchfork",       // ldname
		"Pitchfork",       // appname
		"0.0.1",           // version
		"Trident Project", // copyright
		"https://trident.li", // website
		0,                 // app_schema_version
		"",                // env_server
		"",                // server
		newctx,            // newctx
	)
}
