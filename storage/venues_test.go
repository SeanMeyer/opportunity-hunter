package storage_test

import (
	"context"
	"testing"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/storage"
)

func TestUpsertVenue_Insert(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	id, err := db.UpsertVenue(ctx, core.Venue{
		Name:    "Comedy Works Downtown",
		Address: "1226 15th St, Denver, CO",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatal("expected non-zero ID")
	}

	v, err := db.GetVenue(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if v.Name != "Comedy Works Downtown" {
		t.Fatalf("expected 'Comedy Works Downtown', got %q", v.Name)
	}
}

func TestUpsertVenue_UpdateExisting(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	id1, _ := db.UpsertVenue(ctx, core.Venue{
		Name:      "Comedy Works Downtown",
		Address:   "1226 15th St, Denver, CO",
		Latitude:  39.747,
		Longitude: -104.999,
	})
	id2, _ := db.UpsertVenue(ctx, core.Venue{
		Name:      "Comedy Works Downtown",
		Address:   "1226 15th St, Denver, CO",
		Latitude:  39.748,
		Longitude: -105.000,
	})

	if id1 != id2 {
		t.Fatalf("expected same ID on upsert, got %d and %d", id1, id2)
	}

	v, _ := db.GetVenue(ctx, id2)
	if v.Latitude != 39.748 {
		t.Fatalf("expected updated latitude 39.748, got %f", v.Latitude)
	}
}

func TestUpsertVenue_Normalization(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	id1, _ := db.UpsertVenue(ctx, core.Venue{Name: "Comedy Works - Downtown", Address: "123 Main St"})
	id2, _ := db.UpsertVenue(ctx, core.Venue{Name: "Comedy Works Downtown", Address: "123 Main St"})

	if id1 != id2 {
		t.Fatalf("normalization should match: got IDs %d and %d", id1, id2)
	}
}

func TestGetVenueByName(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	db.UpsertVenue(ctx, core.Venue{Name: "Paramount Theatre", Address: "1621 Glenarm Pl"})

	v, err := db.GetVenueByName(ctx, "Paramount Theatre")
	if err != nil {
		t.Fatal(err)
	}
	if v.Name != "Paramount Theatre" {
		t.Fatalf("expected 'Paramount Theatre', got %q", v.Name)
	}
}

func TestGetVenueByName_NotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_, err := db.GetVenueByName(ctx, "Nonexistent")
	if err != storage.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
