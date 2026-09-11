package core

import (
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// ReviewEvidence is editorial context, separate from the personalized pick score.
// Subject identifies the work/production reviewed, not necessarily the local show.
type ReviewEvidence struct {
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	Subject string `json:"subject"`
	Summary string `json:"summary"`
	URL     string `json:"url"`
}

var videoID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

func YouTubeVideo(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "youtu.be":
		return videoID.MatchString(strings.TrimPrefix(u.Path, "/"))
	case "youtube.com", "www.youtube.com", "m.youtube.com":
		if strings.HasPrefix(u.Path, "/shorts/") {
			return videoID.MatchString(strings.TrimPrefix(u.Path, "/shorts/"))
		}
		return u.Path == "/watch" && videoID.MatchString(u.Query().Get("v"))
	}
	return false
}

func (e ReviewEvidence) Valid() bool {
	u, err := url.Parse(e.URL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return false
	}
	if strings.TrimSpace(e.Title) == "" || strings.TrimSpace(e.Subject) == "" || strings.TrimSpace(e.Summary) == "" {
		return false
	}
	switch e.Kind {
	case "clip":
		return YouTubeVideo(e.URL)
	case "review", "award":
		return true
	}
	return false
}

func WithReviewEvidence(attrs Attributes, evidence []ReviewEvidence) Attributes {
	m := map[string]any{}
	if len(attrs) > 0 {
		_ = json.Unmarshal(attrs, &m)
	}
	if m == nil {
		m = map[string]any{}
	}
	if len(evidence) > 0 {
		m["review_evidence"] = evidence
	}
	b, _ := json.Marshal(m)
	return b
}

func ReadReviewEvidence(attrs Attributes) []ReviewEvidence {
	var saved struct {
		Evidence []ReviewEvidence `json:"review_evidence"`
	}
	if json.Unmarshal(attrs, &saved) != nil {
		return nil
	}
	var result []ReviewEvidence
	seen := map[string]bool{}
	for _, e := range saved.Evidence {
		if e.Valid() && !seen[e.URL] {
			result = append(result, e)
			seen[e.URL] = true
			if len(result) == 3 {
				break
			}
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Kind == "clip" && result[j].Kind != "clip" })
	return result
}
