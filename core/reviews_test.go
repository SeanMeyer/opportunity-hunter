package core

import "testing"

func TestYouTubeVideoOnlyAcceptsVideoLinks(t *testing.T) {
	for _, raw := range []string{"https://www.youtube.com/watch?v=abcdefghijk", "https://youtu.be/abcdefghijk?t=31", "https://www.youtube.com/shorts/abcdefghijk"} {
		if !YouTubeVideo(raw) {
			t.Errorf("rejected %s", raw)
		}
	}
	for _, raw := range []string{"javascript:alert(1)", "http://youtu.be/abcdefghijk", "https://youtube.com.evil.test/watch?v=abcdefghijk", "https://www.youtube.com/results?search_query=comedy", "https://www.youtube.com/@comedian", "https://youtube.com/watch?v=bad", "https://user@youtube.com/watch?v=abcdefghijk"} {
		if YouTubeVideo(raw) {
			t.Errorf("accepted %s", raw)
		}
	}
}

func TestReviewEvidenceLegacyAndUnsafeAttributes(t *testing.T) {
	for _, raw := range []string{"", "null", `{"sell_out_risk":"low"}`, `{"review_evidence":[{"kind":"review","title":"Bad","subject":"show","summary":"text","url":"javascript:alert(1)"}]}`} {
		if len(ReadReviewEvidence(Attributes(raw))) != 0 {
			t.Fatal("legacy or unsafe attributes exposed evidence")
		}
	}
}
