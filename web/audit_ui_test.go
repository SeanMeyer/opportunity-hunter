package web_test

import (
	"context"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/storage"
	"github.com/seanmeyer/opportunity-hunter/testutil"
	"github.com/seanmeyer/opportunity-hunter/web"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStatusExplainsFailureWithoutCredentials(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()
	id, _ := db.InsertRun(ctx, "comedy", "manual")
	db.FinishRun(ctx, id, storage.RunResult{Status: "error", ErrorSummary: "forum requires thread_id https://discord.com/api/webhooks/123/secret-token AIzaFakeSecret123 <script>bad</script>"})
	s, _ := web.New(db, []web.HuntInfo{{Name: "comedy"}}, "")
	r := httptest.NewRecorder()
	s.Handler().ServeHTTP(r, httptest.NewRequest("GET", "/status", nil))
	body := r.Body.String()
	if !strings.Contains(body, "forum requires thread_id") || strings.Contains(body, "secret-token") || strings.Contains(body, "AIzaFakeSecret123") || strings.Contains(body, "<script>bad") {
		t.Fatal("missing explanation or unsafe error output")
	}
	r = httptest.NewRecorder()
	s.Handler().ServeHTTP(r, httptest.NewRequest("GET", "/?hunt=comedy", nil))
	if !strings.Contains(r.Body.String(), "The last scan had errors") {
		t.Fatal("empty state does not explain failure")
	}
}

func TestOldAssessmentDoesNotClaimCurrentUrgency(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()
	o := core.Opportunity{HuntName: "comedy", Title: "Future show", State: core.Evaluated, StartTime: time.Now().AddDate(0, 1, 0)}
	id, _ := db.InsertOpportunity(ctx, o)
	o.ID = id
	db.SaveEvaluatedGroup(ctx, core.Evaluation{HuntName: "comedy", EvaluatedAt: time.Now().AddDate(0, -4, 0)}, []core.Pick{{OpportunityID: id, DisplayScore: "8", Reason: "Good act", Urgency: "STALE BOOK IN THREE MONTHS"}}, []core.Opportunity{o})
	s, _ := web.New(db, []web.HuntInfo{{Name: "comedy"}}, "")
	r := httptest.NewRecorder()
	s.Handler().ServeHTTP(r, httptest.NewRequest("GET", "/?hunt=comedy", nil))
	if strings.Contains(r.Body.String(), "STALE BOOK IN THREE MONTHS") || !strings.Contains(r.Body.String(), "Check current details") {
		t.Fatal("stale urgency presented as current")
	}
}
