package catalog

// Region represents a ski region with detection thresholds.
type Region struct {
	ID                 string
	Name               string
	Latitude           float64
	Longitude          float64
	FrictionTier       string
	NearThresholdIn    float64
	ExtendedThresholdIn float64
	Country            string
	Timezone           string
	StormGroup         string
}

// DefaultRegions returns a set of example ski regions.
// In production, this would be loaded from config or a data file.
func DefaultRegions() []Region {
	return []Region{
		{ID: "co-front-range", Name: "Front Range", Latitude: 39.6, Longitude: -105.8, FrictionTier: "local_drive", NearThresholdIn: 6, ExtendedThresholdIn: 12, Country: "US", Timezone: "America/Denver", StormGroup: "co_front_range"},
		{ID: "co-summit", Name: "Summit County", Latitude: 39.5, Longitude: -106.0, FrictionTier: "local_drive", NearThresholdIn: 6, ExtendedThresholdIn: 12, Country: "US", Timezone: "America/Denver", StormGroup: "co_central"},
		{ID: "co-vail", Name: "Vail Valley", Latitude: 39.6, Longitude: -106.4, FrictionTier: "regional_drive", NearThresholdIn: 14, ExtendedThresholdIn: 20, Country: "US", Timezone: "America/Denver", StormGroup: "co_central"},
		{ID: "ut-cottonwood", Name: "Cottonwood Canyons", Latitude: 40.6, Longitude: -111.6, FrictionTier: "regional_drive", NearThresholdIn: 14, ExtendedThresholdIn: 20, Country: "US", Timezone: "America/Denver", StormGroup: "ut_wasatch"},
		{ID: "pnw-cascades", Name: "PNW Cascades", Latitude: 47.4, Longitude: -121.4, FrictionTier: "flight", NearThresholdIn: 24, ExtendedThresholdIn: 36, Country: "US", Timezone: "America/Los_Angeles", StormGroup: "pnw_cascades"},
	}
}
