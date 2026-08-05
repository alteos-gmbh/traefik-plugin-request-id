package traefik_plugin_request_id

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func newHandler(t *testing.T, config *Config) http.Handler {
	t.Helper()

	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {})
	handler, err := New(context.Background(), next, config, "test")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	return handler
}

func serve(handler http.Handler, request *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder
}

func TestGeneratedIDIsUUIDv7(t *testing.T) {
	handler := newHandler(t, CreateConfig())

	// UUIDv7 leads with a millisecond timestamp, so successive IDs must also sort
	// in generation order as strings.
	previous := ""
	for i := 0; i < 200; i++ {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		recorder := serve(handler, request)

		value := request.Header.Get(defaultHeader)
		if got := recorder.Header().Get(defaultHeader); got != value {
			t.Fatalf("response header %q does not match request header %q", got, value)
		}

		id, err := uuid.Parse(value)
		if err != nil {
			t.Fatalf("generated %q is not a valid UUID: %v", value, err)
		}
		if id.Version() != 7 {
			t.Fatalf("generated %q has version %d, want 7", value, id.Version())
		}
		if id.Variant() != uuid.RFC4122 {
			t.Fatalf("generated %q has variant %v, want RFC4122", value, id.Variant())
		}
		if value <= previous {
			t.Fatalf("IDs are not monotonically increasing: %q followed by %q", previous, value)
		}

		previous = value
	}
}

func TestExistingHeaderIsPreserved(t *testing.T) {
	handler := newHandler(t, CreateConfig())

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(defaultHeader, "upstream-id")
	recorder := serve(handler, request)

	if got := request.Header.Get(defaultHeader); got != "upstream-id" {
		t.Errorf("request header is %q, want it left as %q", got, "upstream-id")
	}
	if got := recorder.Header().Get(defaultHeader); got != "" {
		t.Errorf("response header is %q, want it unset", got)
	}
}

func TestDisabledGeneratesNothing(t *testing.T) {
	handler := newHandler(t, &Config{HeaderName: defaultHeader, Enabled: false})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := serve(handler, request)

	if got := request.Header.Get(defaultHeader); got != "" {
		t.Errorf("request header is %q, want it unset", got)
	}
	if got := recorder.Header().Get(defaultHeader); got != "" {
		t.Errorf("response header is %q, want it unset", got)
	}
}

func TestCustomHeaderName(t *testing.T) {
	const headerName = "X-Correlation-ID"

	handler := newHandler(t, &Config{HeaderName: headerName, Enabled: true})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := serve(handler, request)

	value := request.Header.Get(headerName)
	if _, err := uuid.Parse(value); err != nil {
		t.Fatalf("%s is %q, want a valid UUID: %v", headerName, value, err)
	}
	if got := recorder.Header().Get(headerName); got != value {
		t.Errorf("response %s is %q, want %q", headerName, got, value)
	}
	if got := request.Header.Get(defaultHeader); got != "" {
		t.Errorf("%s is %q, want it unset", defaultHeader, got)
	}
}

func TestNextHandlerSeesGeneratedID(t *testing.T) {
	var seen string
	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		seen = request.Header.Get(defaultHeader)
	})

	handler, err := New(context.Background(), next, CreateConfig(), "test")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	serve(handler, httptest.NewRequest(http.MethodGet, "/", nil))

	if _, err := uuid.Parse(seen); err != nil {
		t.Fatalf("next handler saw %q, want a valid UUID: %v", seen, err)
	}
}

func TestCreateConfigDefaults(t *testing.T) {
	config := CreateConfig()

	if config.HeaderName != defaultHeader {
		t.Errorf("HeaderName is %q, want %q", config.HeaderName, defaultHeader)
	}
	if !config.Enabled {
		t.Error("Enabled is false, want true")
	}
}
