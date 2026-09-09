package weather

import "math"

// Equal-hour circular mean of meteorological FROM bearings. A weak resultant
// indicates substantially changing/opposing winds, not a meaningful mean.
type directionAccumulator struct {
	sin, cos float64
	count    int
}

func (a *directionAccumulator) add(degrees *float64) {
	if degrees == nil || math.IsNaN(*degrees) || math.IsInf(*degrees, 0) || *degrees < 0 || *degrees > 360 {
		return
	}
	radians := *degrees * math.Pi / 180
	a.sin += math.Sin(radians)
	a.cos += math.Cos(radians)
	a.count++
}

func (a directionAccumulator) summary() (*float64, bool) {
	if a.count == 0 {
		return nil, false
	}
	if math.Hypot(a.sin, a.cos)/float64(a.count) < 0.5 {
		return nil, true
	}
	degrees := math.Mod(math.Atan2(a.sin, a.cos)*180/math.Pi+360, 360)
	return &degrees, false
}

func directionMean(a directionAccumulator) *float64 { v, _ := a.summary(); return v }
func directionVariable(a directionAccumulator) bool { _, v := a.summary(); return v }
