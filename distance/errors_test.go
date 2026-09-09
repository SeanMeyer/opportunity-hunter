package distance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRouteMatrixErrorEnvelopes(t *testing.T) {
	for _, body := range []string{`{"error":{"message":"Routes API blocked"}}`, `[{"error":{"message":"Routes API blocked"}}]`} {
		t.Run(body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(403); w.Write([]byte(body)) }))
			defer srv.Close()
			c := &Client{apiKey: "secret", http: srv.Client(), baseURL: srv.URL}
			_, err := c.GetDistance(context.Background(), "A", "B", "WALK")
			if err == nil || !strings.Contains(err.Error(), "Routes API blocked") {
				t.Fatalf("lost provider diagnostic: %v", err)
			}
		})
	}
}

func TestRouteMatrixErrorDoesNotEchoCredential(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`[{"error":{"message":"Invalid key secret"}}]`))
	}))
	defer srv.Close()
	c := &Client{apiKey: "secret", http: srv.Client(), baseURL: srv.URL}
	_, err := c.GetDistance(context.Background(), "A", "B", "WALK")
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("credential in error: %v", err)
	}
}
