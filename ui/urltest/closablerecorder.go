package urltest

import (
	"net/http/httptest"
)

type ClosableRecorder struct {
	*httptest.ResponseRecorder
	closer chan bool
}

func (r *ClosableRecorder) CloseNotify() <-chan bool {
	return r.closer
}

func NewClosableRecorder() *ClosableRecorder {
	r := httptest.NewRecorder()
	closer := make(chan bool)
	return &ClosableRecorder{r, closer}
}
