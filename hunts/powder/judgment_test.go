package powder

import (
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder/weather"
	"strings"
	"testing"
)

func TestPromptKeepsPreferencesAlongsideProfile(t *testing.T) {
	p := buildPrompt(core.EvalContext{Profile: &core.UserProfile{HomeBase: "Denver", Preferences: "Trees over bowls"}, Preferences: "No flights this month"})
	for _, want := range []string{"Denver", "Trees over bowls", "No flights this month"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt lost user context: %s", want)
		}
	}
}

func TestVerdictLeadsCardAndSort(t *testing.T) {
	r := &powderCardRenderer{}
	for _, tc := range []struct {
		tier, label string
		style       core.ScoreTier
		score       float64
	}{
		{"DROP_EVERYTHING", "Drop Everything", core.ScoreHigh, .95},
		{"RECOMMENDED", "Recommended", core.ScoreMedium, .75},
		{"WATCH", "Watch", core.ScoreLow, .5},
		{"SKIP", "Skip", core.ScoreNone, 0},
		{"WORTH_A_LOOK", "Recommended", core.ScoreMedium, .75},
		{"ON_THE_RADAR", "Watch", core.ScoreLow, .5},
	} {
		t.Run(tc.tier, func(t *testing.T) {
			c := r.RenderCard(core.Opportunity{Attributes: PowderAttrs{SnowfallIn: 40}.Encode()}, core.Pick{DisplayScore: tc.tier, Reason: "Road closures make this a bad trip.", Attributes: core.Attributes(`{"summary":"40 inches Friday"}`)}, core.Venue{})
			if c.Reason != "Road closures make this a bad trip." || c.Score != tc.label || c.ScoreTier != tc.style || c.SortScore != tc.score {
				t.Fatalf("verdict lost: %+v", c)
			}
		})
	}
}

func TestSkipNotificationPolicy(t *testing.T) {
	f := &powderNotifyFormatter{}
	for _, tc := range []struct {
		change string
		want   bool
	}{{"new", false}, {"minor", false}, {"downgrade", true}} {
		actions := f.FormatPicks(core.NotifyContext{Picks: []core.Pick{{DisplayScore: "SKIP", Reason: "Don't book: resort closed", Attributes: core.Attributes(`{"change_class":"` + tc.change + `","summary":"Deep snow"}`)}}})
		if (len(actions) > 0) != tc.want {
			t.Errorf("%s: got %d actions", tc.change, len(actions))
		}
		for _, a := range actions {
			if a.Ping {
				t.Error("skip must never ping")
			}
			if !strings.Contains(a.Message.Content, "Don't book") {
				t.Error("downgrade lost judgment")
			}
		}
	}
	if actions := f.FormatReminder(core.Opportunity{}, core.Pick{DisplayScore: "SKIP"}, ""); len(actions) != 0 {
		t.Fatal("skip generated reminder")
	}
}

func TestNewAndLegacyTiersCompare(t *testing.T) {
	if Compare(weather.Tier("WORTH_A_LOOK"), weather.Tier("RECOMMENDED"), 10, 10) != weather.ChangeMinor {
		t.Fatal("legacy recommended changed rank")
	}
	if Compare(weather.Tier("RECOMMENDED"), weather.Tier("SKIP"), 10, 10) != weather.ChangeDowngrade {
		t.Fatal("missed skip downgrade")
	}
}
