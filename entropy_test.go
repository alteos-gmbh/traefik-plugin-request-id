// This test substitutes the entropy source with a failing one, which means
// handing an interpreter-defined io.Reader to uuid.SetRand. Yaegi cannot make
// that assignment and panics with "reflect.Set: value of type []uint8 is not
// assignable to type func(string) error", so the test is excluded from the
// interpreter run via the yaegi build tag while still running under go test.
//
// The code path it covers is interpreter-safe; only this way of forcing the
// failure is not.
//
// Both constraint forms are present on purpose: Yaegi does not honour the
// //go:build form, only the legacy one.
//go:build !yaegi
// +build !yaegi

package traefik_plugin_request_id

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

// failingReader stands in for an entropy source that has stopped working.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("entropy source unavailable")
}

// A failure to generate an ID must not cost the caller their request: the
// header is diagnostic, so the request is forwarded without it.
func TestEntropyFailureForwardsRequestUntagged(t *testing.T) {
	uuid.SetRand(failingReader{})
	t.Cleanup(func() { uuid.SetRand(nil) })

	called := false
	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		called = true
	})

	handler, err := New(context.Background(), next, CreateConfig(), "test")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := serve(handler, request)

	if !called {
		t.Error("next handler was not called")
	}
	if recorder.Code != http.StatusOK {
		t.Errorf("status is %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := request.Header.Get(defaultHeader); got != "" {
		t.Errorf("request header is %q, want it unset", got)
	}
	if got := recorder.Header().Get(defaultHeader); got != "" {
		t.Errorf("response header is %q, want it unset", got)
	}
}
