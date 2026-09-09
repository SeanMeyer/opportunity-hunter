package notify_test

import (
	"context"
	"encoding/json"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder"
	"github.com/seanmeyer/opportunity-hunter/notify"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPowderCanPostToExistingThread(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("thread_id") != "saved-thread" {
			t.Errorf("wrong destination: %s", r.URL)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	h := &powder.PowderHunt{}
	actions := h.NotifyFormatter().FormatPicks(core.NotifyContext{ExistingThreadID: "saved-thread", Picks: []core.Pick{{DisplayScore: "RECOMMENDED", Reason: "Worth a day trip"}}})
	_, err := notify.NewClient(srv.URL).ExecuteActions(context.Background(), actions)
	if err != nil || calls != 1 {
		t.Fatalf("existing thread failed: calls=%d err=%v", calls, err)
	}
}

func TestPowderExceptionalPingsOnce(t *testing.T) {
	mentions := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		json.NewDecoder(r.Body).Decode(&payload)
		if content, ok := payload["content"].(string); ok {
			mentions += strings.Count(content, "@here")
		}
		if r.URL.Query().Get("wait") == "true" {
			json.NewEncoder(w).Encode(map[string]string{"channel_id": "new-thread"})
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()
	h := &powder.PowderHunt{}
	actions := h.NotifyFormatter().FormatPicks(core.NotifyContext{Picks: []core.Pick{{DisplayScore: "DROP_EVERYTHING", Reason: "Exceptional local day"}}})
	if _, err := notify.NewClient(srv.URL).ExecuteActions(context.Background(), actions); err != nil {
		t.Fatal(err)
	}
	if mentions != 1 {
		t.Fatalf("got %d mentions", mentions)
	}
}
