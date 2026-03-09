package distance

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetDistance_Walking(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req routeMatrixRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.TravelMode != "WALK" {
			t.Errorf("expected WALK mode, got %s", req.TravelMode)
		}

		json.NewEncoder(w).Encode([]routeMatrixElement{{
			Condition:      "ROUTE_EXISTS",
			DistanceMeters: 1609, // ~1 mile
			Duration:       "1200s", // 20 minutes
		}})
	}))
	defer srv.Close()

	c := &Client{apiKey: "test", http: srv.Client(), baseURL: srv.URL}
	result, err := c.GetDistance(context.Background(), "A", "B", "WALK")
	if err != nil {
		t.Fatal(err)
	}
	if result.Minutes != 20 {
		t.Fatalf("expected 20 minutes, got %d", result.Minutes)
	}
	if result.DistanceMi < 0.99 || result.DistanceMi > 1.01 {
		t.Fatalf("expected ~1 mile, got %f", result.DistanceMi)
	}
}

func TestGetDistance_Driving(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req routeMatrixRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.TravelMode != "DRIVE" {
			t.Errorf("expected DRIVE mode, got %s", req.TravelMode)
		}

		json.NewEncoder(w).Encode([]routeMatrixElement{{
			Condition:      "ROUTE_EXISTS",
			DistanceMeters: 160934, // ~100 miles
			Duration:       "5400s", // 90 minutes
		}})
	}))
	defer srv.Close()

	c := &Client{apiKey: "test", http: srv.Client(), baseURL: srv.URL}
	result, err := c.GetDistance(context.Background(), "A", "B", "DRIVE")
	if err != nil {
		t.Fatal(err)
	}
	if result.Minutes != 90 {
		t.Fatalf("expected 90 minutes, got %d", result.Minutes)
	}
}

func TestGetDistance_NoRoute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]routeMatrixElement{{
			Condition: "ROUTE_NOT_FOUND",
		}})
	}))
	defer srv.Close()

	c := &Client{apiKey: "test", http: srv.Client(), baseURL: srv.URL}
	_, err := c.GetDistance(context.Background(), "A", "B", "WALK")
	if err == nil {
		t.Fatal("expected error for no route")
	}
}

func TestParseDurationSeconds(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"480s", 480},
		{"0s", 0},
		{"3600s", 3600},
	}
	for _, tt := range tests {
		got := parseDurationSeconds(tt.input)
		if got != tt.want {
			t.Errorf("parseDurationSeconds(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
