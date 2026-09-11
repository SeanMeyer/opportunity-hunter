// Run three isolated evaluator samples. No source scans, database writes or notifications.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/comedy"
	"github.com/seanmeyer/opportunity-hunter/hunts/movies"
	"github.com/seanmeyer/opportunity-hunter/hunts/performing"
	"os"
	"path/filepath"
	"time"
)

func main() {
	if len(os.Args) < 2 || len(os.Args) > 3 {
		panic("usage: review-evidence OUTPUT_DIRECTORY [HUNT]")
	}
	dir := os.Args[1]
	if e := os.MkdirAll(dir, 0700); e != nil {
		panic(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	cases := []struct {
		h            core.Hunt
		title, prefs string
	}{{&comedy.ComedyHunt{}, "Nate Bargatze", "I enjoy Nate Bargatze's understated observational humor."}, {&movies.MoviesHunt{}, "The Grand Budapest Hotel (2014)", "I love Wes Anderson, visual craft and dry humor. Repertory screenings are welcome."}, {&performing.PerformingHunt{}, "Hamilton (touring production)", "I love ambitious musical theatre, including Hamilton; I have not seen it live."}}
	for _, tc := range cases {
		if len(os.Args) == 3 && tc.h.Name() != os.Args[2] {
			continue
		}
		lookup := func(k string) string {
			if k == "TICKETMASTER_API_KEY" || k == "TMDB_API_KEY" {
				return "unused-no-scanning"
			}
			return os.Getenv(k)
		}
		if e := tc.h.Init(ctx, lookup); e != nil {
			panic(e)
		}
		result, e := tc.h.Evaluator().Evaluate(ctx, core.EvalContext{Opportunities: []core.Opportunity{{ID: 1, HuntName: tc.h.Name(), Title: tc.title, Subtitle: "Synthetic evaluation fixture; not an actual bookable listing", StartTime: time.Now().Add(7 * 24 * time.Hour), State: core.Discovered}}, Preferences: tc.prefs})
		if e != nil {
			fmt.Fprintf(os.Stderr, "%s evaluation failed: %v\n", tc.h.Name(), e)
			os.Exit(1)
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		if e := os.WriteFile(filepath.Join(dir, tc.h.Name()+".json"), b, 0600); e != nil {
			panic(e)
		}
		fmt.Printf("%s: %d picks, $%.4f\n", tc.h.Name(), len(result.Picks), result.Evaluation.CostUSD)
		for _, p := range result.Picks {
			fmt.Printf("  evidence: %+v\n", core.ReadReviewEvidence(p.Attributes))
		}
	}
}
