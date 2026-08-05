package traefik_plugin_request_id

import (
	"context"
	"log"
	"net/http"

	"github.com/google/uuid"
)

const defaultHeader = "X-Request-ID"
const defaultEnabled = true

type Config struct {
	HeaderName string `json:"headerName,omitempty"`
	Enabled    bool   `json:"enabled,omitempty"`
}

func CreateConfig() *Config {
	return &Config{
		HeaderName: defaultHeader,
		Enabled:    defaultEnabled,
	}
}

func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if config.Enabled && request.Header.Get(config.HeaderName) == "" {
			id, err := uuid.NewV7()
			if err != nil {
				// Only reachable if the entropy source fails. Tagging requests is
				// a diagnostic concern, so forward the request untagged rather
				// than turning it into an error, and log so the gap is visible.
				log.Printf("%s: could not generate a request ID: %v", name, err)
			} else {
				value := id.String()
				request.Header.Add(config.HeaderName, value)
				writer.Header().Add(config.HeaderName, value)
			}
		}
		next.ServeHTTP(writer, request)
	}), nil
}
