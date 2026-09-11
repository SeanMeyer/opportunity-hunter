package distance

import (
	"context"
	"github.com/seanmeyer/opportunity-hunter/core"
	"log/slog"
)

// EnrichVenues keeps nearby trips walkable and requests driving for longer trips.
func (c *Client) EnrichVenues(ctx context.Context, origin string, venues map[int64]core.Venue) {
	if c == nil || origin == "" {
		return
	}
	for id, v := range venues {
		if v.Address == "" {
			continue
		}
		if v.WalkingMinutes == 0 {
			r, err := c.GetDistance(ctx, origin, v.Address, "WALK")
			if err != nil {
				slog.Warn("walking distance lookup failed", "venue", v.Name, "err", err)
			} else {
				v.WalkingMinutes = r.Minutes
				v.DistanceMi = r.DistanceMi
			}
		}
		if (v.WalkingMinutes > 30 || v.WalkingMinutes == 0) && v.DrivingMinutes == 0 {
			r, err := c.GetDistance(ctx, origin, v.Address, "DRIVE")
			if err != nil {
				slog.Warn("driving distance lookup failed", "venue", v.Name, "err", err)
			} else {
				v.DrivingMinutes = r.Minutes
				v.DrivingDistanceMi = r.DistanceMi
			}
		}
		venues[id] = v
	}
}
