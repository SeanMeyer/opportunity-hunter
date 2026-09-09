package web_test

import (
	"context"
	"github.com/seanmeyer/opportunity-hunter/storage"
	"github.com/seanmeyer/opportunity-hunter/testutil"
	"github.com/seanmeyer/opportunity-hunter/web"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOverdueScheduleAndRunOutcome(t *testing.T) {
	for _, status := range []string{"ok", "error", "running"} {
		t.Run(status, func(t *testing.T) {
			db := testutil.NewTestDB(t)
			ctx := context.Background()
			if err := db.SeedScheduleIfNotExists(ctx, "comedy", 60, time.Now().AddDate(0, -5, 0)); err != nil {
				t.Fatal(err)
			}
			id, err := db.InsertRun(ctx, "comedy", "startup")
			if err != nil {
				t.Fatal(err)
			}
			if status != "running" {
				if err := db.FinishRun(ctx, id, storage.RunResult{Status: status, Scanned: 3}); err != nil {
					t.Fatal(err)
				}
			}
			s, err := web.New(db, []web.HuntInfo{{Name: "comedy"}}, "")
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{"/", "/status"} {
				r := httptest.NewRecorder()
				s.Handler().ServeHTTP(r, httptest.NewRequest("GET", path, nil))
				body := r.Body.String()
				if strings.Contains(body, "Next:") || !strings.Contains(body, "Scan due") {
					t.Errorf("%s misrepresents overdue schedule", path)
				}
				if strings.Contains(body, " scanned") || strings.Contains(body, ">Scanned<") {
					t.Errorf("%s misleading new-record label", path)
				}
				if !strings.Contains(body, "Discord notifications off") {
					t.Errorf("%s missing opt-out status", path)
				}
				if path == "/" && status == "error" && (!strings.Contains(body, "Last run had errors") || !strings.Contains(body, "href=\"/status?hunt=comedy\"")) {
					t.Error("missing linked error explanation")
				}
				if path == "/" && status == "ok" && (!strings.Contains(body, "Last scan") || !strings.Contains(body, "3 new opportunities")) {
					t.Error("missing scan explanation")
				}
			}
		})
	}
}
