package web_test

import (
	"context"
	"github.com/seanmeyer/opportunity-hunter/testutil"
	"github.com/seanmeyer/opportunity-hunter/web"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunningScanIsNotPresentedAsCompleted(t *testing.T) {
	db := testutil.NewTestDB(t)
	if _, err := db.InsertRun(context.Background(), "comedy", "manual"); err != nil {
		t.Fatal(err)
	}
	s, err := web.New(db, []web.HuntInfo{{Name: "comedy"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "/status"} {
		r := httptest.NewRecorder()
		s.Handler().ServeHTTP(r, httptest.NewRequest("GET", path, nil))
		body := r.Body.String()
		if !strings.Contains(body, "Counts are available when the run finishes.") {
			t.Errorf("%s lacks running explanation", path)
		}
		if strings.Contains(body, "Ran just now") || strings.Contains(body, "0 scanned") {
			t.Errorf("%s implies completed counts", path)
		}
	}
}
