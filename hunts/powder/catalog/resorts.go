package catalog

// Resort represents a ski resort.
type Resort struct {
	ID               string
	RegionID         string
	Name             string
	Latitude         float64
	Longitude        float64
	SummitElevFt     int
	BaseElevFt       int
	PassAffiliations []string
}

// DefaultResorts returns a set of example resorts.
func DefaultResorts() []Resort {
	return []Resort{
		{ID: "abasin", RegionID: "co-front-range", Name: "Arapahoe Basin", Latitude: 39.6425, Longitude: -105.8719, SummitElevFt: 13050, BaseElevFt: 10780, PassAffiliations: []string{"ikon"}},
		{ID: "loveland", RegionID: "co-front-range", Name: "Loveland", Latitude: 39.6800, Longitude: -105.8978, SummitElevFt: 13010, BaseElevFt: 10800},
		{ID: "breck", RegionID: "co-summit", Name: "Breckenridge", Latitude: 39.4817, Longitude: -106.0384, SummitElevFt: 12998, BaseElevFt: 9600, PassAffiliations: []string{"epic"}},
		{ID: "keystone", RegionID: "co-summit", Name: "Keystone", Latitude: 39.6069, Longitude: -105.9498, SummitElevFt: 12408, BaseElevFt: 9280, PassAffiliations: []string{"epic"}},
		{ID: "vail", RegionID: "co-vail", Name: "Vail", Latitude: 39.6061, Longitude: -106.3550, SummitElevFt: 11570, BaseElevFt: 8120, PassAffiliations: []string{"epic"}},
		{ID: "snowbird", RegionID: "ut-cottonwood", Name: "Snowbird", Latitude: 40.5830, Longitude: -111.6569, SummitElevFt: 11000, BaseElevFt: 7760, PassAffiliations: []string{"ikon"}},
		{ID: "stevens", RegionID: "pnw-cascades", Name: "Stevens Pass", Latitude: 47.7448, Longitude: -121.0890, SummitElevFt: 5845, BaseElevFt: 4061, PassAffiliations: []string{"epic"}},
	}
}
