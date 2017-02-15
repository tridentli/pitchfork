package urltest

import (
	"net/http/httptest"
)

// ClosableRecorder is a HTTP recorder that can be closed
type ClosableRecorder struct {
	*httptest.ResponseRecorder
	closer chan bool
}

// CloseNotify handles the notification that the recorder should be closed
func (r *ClosableRecorder) CloseNotify() <-chan bool {
	return r.closer
}

// NewClosableRecorder constructs a new Closeable Recorder
func NewClosableRecorder() *ClosableRecorder {
	r := httptest.NewRecorder()
	closer := make(chan bool)
	return &ClosableRecorder{r, closer}
}
