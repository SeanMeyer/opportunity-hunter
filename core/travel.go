package core

import "fmt"

// VenueTravel describes a useful route without presenting an impractical walk.
func VenueTravel(v Venue) CardField {
	f := CardField{Label: "Distance"}
	miles := 0.0
	switch {
	case v.WalkingMinutes > 0 && v.WalkingMinutes <= 30:
		f.Icon = "🚶"
		f.Value = fmt.Sprintf("%d min walk", v.WalkingMinutes)
		miles = v.DistanceMi
	case v.DrivingMinutes > 0:
		f.Icon = "🚗"
		f.Value = fmt.Sprintf("%d min drive", v.DrivingMinutes)
		miles = v.DrivingDistanceMi
	case v.WalkingMinutes > 30:
		f.Icon = "🚗"
		f.Value = "Drive · time unavailable"
	default:
		return f
	}
	if miles > 0 {
		f.Value += fmt.Sprintf(" · %.1f mi", miles)
	}
	return f
}
