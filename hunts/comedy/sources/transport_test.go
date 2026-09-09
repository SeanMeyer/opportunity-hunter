package sources

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestScanTransportPreservesRequestDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(300 * time.Millisecond):
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: scanTransport{ctx: context.Background(), base: http.DefaultTransport}}
	start := time.Now()
	resp, err := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	if err == nil || time.Since(start) > 200*time.Millisecond {
		t.Fatalf("request deadline lost: duration=%v err=%v", time.Since(start), err)
	}
}

func TestScanTransportClientTimeoutAndBodyCancellation(t *testing.T) {
	for _, headers := range []bool{false, true} {
		t.Run(map[bool]string{false: "client-timeout", true: "scan-cancels-body"}[headers], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if headers {
					w.WriteHeader(http.StatusOK)
					w.(http.Flusher).Flush()
				}
				select {
				case <-r.Context().Done():
				case <-time.After(300 * time.Millisecond):
				}
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client := &http.Client{Timeout: 40 * time.Millisecond, Transport: scanTransport{ctx: ctx, base: http.DefaultTransport}}
			start := time.Now()
			resp, err := client.Get(server.URL)
			if headers {
				if err != nil {
					t.Fatal(err)
				}
				cancel()
				_, err = io.ReadAll(resp.Body)
				resp.Body.Close()
			}
			if err == nil || time.Since(start) > 200*time.Millisecond {
				t.Fatalf("cancellation lost: duration=%v err=%v", time.Since(start), err)
			}
		})
	}
}
