package web_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/testutil"
	"github.com/seanmeyer/opportunity-hunter/web"
)

func newTestServer(t *testing.T) (*web.Server, *httptest.Server) {
	t.Helper()
	db := testutil.NewTestDB(t)
	hunts := []web.HuntInfo{
		{Name: "comedy", FeedbackOptions: []core.FeedbackOption{{Value: "loved", Label: "Loved"}}},
		{Name: "powder"},
	}
	srv, err := web.New(db, hunts, "")
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return srv, ts
}

func TestIndex_Returns200(t *testing.T) {
	_, ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestIndex_ShowsHuntTabs(t *testing.T) {
	_, ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "comedy") {
		t.Fatal("expected 'comedy' tab in response")
	}
	if !strings.Contains(body, "powder") {
		t.Fatal("expected 'powder' tab in response")
	}
}

func TestIndex_HuntFilter(t *testing.T) {
	_, ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/?hunt=powder")
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, resp)
	// Powder tab should be active.
	if !strings.Contains(body, `class="active"`) {
		t.Fatal("expected active tab class")
	}
}

func TestSavePreferences(t *testing.T) {
	_, ts := newTestServer(t)
	resp, err := http.PostForm(ts.URL+"/preferences", url.Values{
		"hunt":        {"comedy"},
		"preferences": {"I like dark humor"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should redirect back.
	if resp.StatusCode != 200 { // follows redirect
		t.Fatalf("expected 200 after redirect, got %d", resp.StatusCode)
	}
}

func TestSaveFeedback(t *testing.T) {
	_, ts := newTestServer(t)
	resp, err := http.PostForm(ts.URL+"/feedback", url.Values{
		"hunt":   {"comedy"},
		"title":  {"Nate Bargatze"},
		"rating": {"loved"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 after redirect, got %d", resp.StatusCode)
	}
}

func TestSmartDistanceDisplay(t *testing.T) {
	tests := []struct {
		name     string
		walking  int
		driving  int
		wantText string
	}{
		{"close venue", 8, 3, "8 min walk"},
		{"far venue", 90, 25, "25 min drive"},
		{"boundary", 30, 12, "30 min walk"},
		{"just over", 31, 13, "13 min drive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := web.FormatDistance(tt.walking, tt.driving)
			if got != tt.wantText {
				t.Errorf("got %q, want %q", got, tt.wantText)
			}
		})
	}
}

func TestHandleRun(t *testing.T) {
	db := testutil.NewTestDB(t)

	var triggered string
	hunts := []web.HuntInfo{{Name: "comedy"}, {Name: "powder"}}
	srv, err := web.New(db, hunts, "", func(ctx context.Context, hunt string) {
		triggered = hunt
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.PostForm(ts.URL+"/run", url.Values{"hunt": {"comedy"}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 after redirect, got %d", resp.StatusCode)
	}

	time.Sleep(50 * time.Millisecond)
	if triggered != "comedy" {
		t.Errorf("expected comedy triggered, got %q", triggered)
	}
}

func TestHandleRunInvalidHunt(t *testing.T) {
	db := testutil.NewTestDB(t)
	hunts := []web.HuntInfo{{Name: "comedy"}}
	srv, err := web.New(db, hunts, "")
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.PostForm(ts.URL+"/run", url.Values{"hunt": {"nonexistent"}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	buf := new(strings.Builder)
	if _, err := strings.NewReader("").WriteTo(buf); err != nil {
		t.Fatal(err)
	}
	// Read the actual body.
	b := make([]byte, 10000)
	n, _ := resp.Body.Read(b)
	return string(b[:n])
}
