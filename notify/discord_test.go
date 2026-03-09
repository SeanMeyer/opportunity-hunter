package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestPostMessage(t *testing.T) {
	var received discordPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	d := NewDiscord(srv.URL)
	err := d.PostMessage(context.Background(), discordPayload{Content: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if received.Content != "hello" {
		t.Fatalf("expected 'hello', got %q", received.Content)
	}
}

func TestPostThread_ReturnsThreadID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "wait=true" {
			t.Errorf("expected ?wait=true, got %q", r.URL.RawQuery)
		}
		json.NewEncoder(w).Encode(threadResponse{ChannelID: "thread-123"})
	}))
	defer srv.Close()

	d := NewDiscord(srv.URL)
	id, err := d.PostThread(context.Background(), discordPayload{
		Content:    "briefing",
		ThreadName: "PNW Cascades — Jan 15",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != "thread-123" {
		t.Fatalf("expected 'thread-123', got %q", id)
	}
}

func TestPostToThread_SendsThreadID(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	d := NewDiscord(srv.URL)
	err := d.PostToThread(context.Background(), "thread-456", discordPayload{Content: "detail"})
	if err != nil {
		t.Fatal(err)
	}
	if gotQuery != "thread_id=thread-456" {
		t.Fatalf("expected 'thread_id=thread-456', got %q", gotQuery)
	}
}

func TestRetryOn5xx(t *testing.T) {
	var mu sync.Mutex
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		a := attempts
		mu.Unlock()
		if a < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	d := NewDiscord(srv.URL)
	d.maxRetries = 3
	err := d.PostMessage(context.Background(), discordPayload{Content: "retry me"})
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	mu.Lock()
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
	mu.Unlock()
}

func TestNoRetryOn4xx(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer srv.Close()

	d := NewDiscord(srv.URL)
	err := d.PostMessage(context.Background(), discordPayload{Content: "bad"})
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt (no retry on 4xx), got %d", attempts)
	}
}
