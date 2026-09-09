package weather

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type failedWeatherTransport struct{}

func (failedWeatherTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 400, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("unavailable")), Request: r}, nil
}
func TestAllProviderFailuresAreReported(t *testing.T) {
	c := &http.Client{Transport: failedWeatherTransport{}}
	s := NewService(NewOpenMeteoClient(c), NewNWSClient(c))
	_, err := s.FetchAll(context.Background(), Region{ID: "test", Country: "US", Timezone: "America/Denver"}, []Resort{{ID: "test"}})
	if err == nil {
		t.Fatal("all weather failures reported as empty success")
	}
}
