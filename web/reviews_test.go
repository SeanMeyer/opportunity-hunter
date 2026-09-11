package web

import (
	"context"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/comedy"
	"github.com/seanmeyer/opportunity-hunter/hunts/movies"
	"github.com/seanmeyer/opportunity-hunter/hunts/performing"
	"github.com/seanmeyer/opportunity-hunter/testutil"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReviewEvidencePersistsAndRendersAcrossHunts(t *testing.T) {
	for _, h := range []core.Hunt{&comedy.ComedyHunt{}, &movies.MoviesHunt{}, &performing.PerformingHunt{}} {
		t.Run(h.Name(), func(t *testing.T) {
			ctx := context.Background()
			db := testutil.NewTestDB(t)
			wh := h.(core.WebHunt)
			s, err := New(db, []HuntInfo{{Name: h.Name(), CardRenderer: wh.CardRenderer()}}, "")
			if err != nil {
				t.Fatal(err)
			}
			id, err := db.InsertOpportunity(ctx, core.Opportunity{HuntName: h.Name(), SourceID: "fixture", Title: "Fixture", State: core.Evaluated, StartTime: time.Now().Add(24 * time.Hour), DiscoveredAt: time.Now()})
			if err != nil {
				t.Fatal(err)
			}
			evidence := []core.ReviewEvidence{{Kind: "review", Title: "Critic <script>bad()</script>", Subject: "Original production (2015), not the touring cast", Summary: "Beautiful staging, but uneven pacing.", URL: "https://critic.example/review"}, {Kind: "clip", Title: "An actual set", Subject: "Past special (2020)", Summary: "Observational humor.", URL: "https://www.youtube.com/watch?v=abcdefghijk"}}
			_, err = db.SaveEvaluationWithPicks(ctx, core.Evaluation{HuntName: h.Name(), EvaluatedAt: time.Now()}, []core.Pick{{OpportunityID: id, Score: .8, Reason: "Personal taste match", Attributes: core.WithReviewEvidence(nil, evidence)}})
			if err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/?hunt="+h.Name(), nil))
			body := w.Body.String()
			if w.Code != 200 {
				t.Fatalf("HTTP %d", w.Code)
			}
			for _, want := range []string{"Get a feel for it", "Watch a bit", "Original production (2015), not the touring cast", "https://www.youtube.com/watch?v=abcdefghijk", "More reviews &amp; context", "Critic &lt;script&gt;"} {
				if !strings.Contains(body, want) {
					t.Errorf("missing %q", want)
				}
			}
			if strings.Contains(body, "<script>bad()</script>") {
				t.Fatal("unescaped evidence")
			}
			if strings.Index(body, "Watch a bit") > strings.Index(body, "Reviews &amp; recommendation details") {
				t.Fatal("clip not featured outside details")
			}
		})
	}
}
