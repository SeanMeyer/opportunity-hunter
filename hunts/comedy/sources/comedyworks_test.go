package sources

import (
	"context"
	"github.com/seanmeyer/opportunity-hunter/core"
	"io"
	"net/http"
	"strings"
	"testing"
)

type fixtureTransport func(*http.Request) (*http.Response, error)

func (f fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestComedyWorksCurrentMarkupAndVenueIdentity(t *testing.T) {
	var urls []string
	s := NewComedyWorks()
	s.client = &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
		urls = append(urls, r.URL.String())
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(strings.NewReader(`<ul class="comedian-boxes"><li class="comedian-box"><h2 class="comedian-box-title"><a href="/comedians/example">Example Artist</a></h2><p class="comedian-box-date"><a>Sep 12, 2026</a></p><a href="/comedians/example">Buy Tickets</a></li></ul>`)), Request: r}, nil
	})}
	items, err := s.Scan(context.Background(), core.ScanRegion{})
	if err != nil || len(items) != 2 {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	if items[0].SourceID == items[1].SourceID {
		t.Fatal("same-day different venues collide")
	}
	if !strings.Contains(urls[1], "landmark=1") {
		t.Fatalf("wrong south URL: %s", urls[1])
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Scan(ctx, core.ScanRegion{}); err == nil {
		t.Fatal("ignored cancellation")
	}
}
