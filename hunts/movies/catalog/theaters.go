package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed data/theaters.json
var theatersJSON []byte

// Theater represents a local movie theater with rich metadata.
type Theater struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Address  string            `json:"address"`
	Coords   Coords            `json:"coords"`
	Metadata map[string]string `json:"metadata"`
}

// Coords holds geographic coordinates.
type Coords struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type theatersFile struct {
	Theaters []Theater `json:"theaters"`
}

// Theaters loads the embedded theater catalog.
// Panics on parse failure so misconfigured seed data is caught at startup.
func Theaters() []Theater {
	var f theatersFile
	if err := json.Unmarshal(theatersJSON, &f); err != nil {
		panic(fmt.Sprintf("seed: parse theaters.json: %v", err))
	}
	return f.Theaters
}

// TheatersByName returns a map of theater name → *Theater for matching.
func TheatersByName() map[string]*Theater {
	theaters := Theaters()
	m := make(map[string]*Theater, len(theaters))
	for i := range theaters {
		m[theaters[i].Name] = &theaters[i]
	}
	return m
}
