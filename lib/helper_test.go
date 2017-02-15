package pitchfork

// testingctx is a helper function for testing to set up a Pitchfork Context for testing.
func testingctx() PfCtx {
	return NewPfCtx(nil, nil, nil, nil, nil)
}
