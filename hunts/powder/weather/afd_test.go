package weather

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAFDUsesIssuingOfficeForAlaskaGrids(t *testing.T) {
	for _, grid := range []string{"AER", "ALU", "AFC", "BOU"} {
		t.Run(grid, func(t *testing.T) {
			office := grid
			if grid == "AER" || grid == "ALU" {
				office = "AFC"
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/products/types/AFD/locations/" + office:
					fmt.Fprint(w, `{"@graph":[{"id":"latest","issuanceTime":"2026-09-10T00:05:00Z"}]}`)
				case "/products/latest":
					fmt.Fprint(w, `{"productText":"Current regional forecast discussion"}`)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			old := nwsBaseURL
			nwsBaseURL = server.URL
			defer func() { nwsBaseURL = old }()
			got, err := NewNWSClient(server.Client()).FetchAFD(context.Background(), grid)
			if err != nil || got.WFO != office || got.Text == "" {
				t.Fatalf("discussion=%+v err=%v", got, err)
			}
		})
	}
}
