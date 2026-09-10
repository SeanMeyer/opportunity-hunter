package storage

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/seanmeyer/opportunity-hunter/core"
	"sort"
	"time"
)

type comedySeries struct {
	opp   core.Opportunity
	venue string
}

func (d *DB) comedySeries(ctx context.Context) ([]comedySeries, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT o.id, COALESCE(v.name,''), COALESCE(v.address,'') FROM opportunities o LEFT JOIN venues v ON v.id=o.venue_id WHERE o.hunt_name='comedy' ORDER BY o.id`)
	if err != nil {
		return nil, err
	}
	var series []comedySeries
	for rows.Next() {
		var id int64
		var name, address string
		if err := rows.Scan(&id, &name, &address); err != nil {
			rows.Close()
			return nil, err
		}
		venue := ""
		if name != "" {
			venue = core.EventVenueKey(name, address)
		}
		series = append(series, comedySeries{opp: core.Opportunity{ID: id}, venue: venue})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range series {
		o, err := d.GetOpportunity(ctx, series[i].opp.ID)
		if err != nil {
			return nil, err
		}
		series[i].opp = o
	}
	return series, nil
}

func sameSource(a, b core.Opportunity) bool {
	return a.Source != "" && a.SourceID != "" && a.Source == b.Source && a.SourceID == b.SourceID
}
func seriesDates(o core.Opportunity) []time.Time {
	if len(o.ShowDates) > 0 {
		return o.ShowDates
	}
	if o.StartTime.IsZero() {
		return nil
	}
	return []time.Time{o.StartTime}
}
func overlappingDates(a, b core.Opportunity) bool {
	for _, x := range seriesDates(a) {
		for _, y := range seriesDates(b) {
			if core.EventLocalDate(x) == core.EventLocalDate(y) {
				return true
			}
		}
	}
	return false
}
func sameComedySeries(a, b comedySeries) bool {
	if sameSource(a.opp, b.opp) {
		return true
	}
	return a.venue != "" && a.venue == b.venue && core.NormalizeTitleForDedup(a.opp.Title) == core.NormalizeTitleForDedup(b.opp.Title) && overlappingDates(a.opp, b.opp)
}

// combineSeriesDates retains different actual showtimes, replacing a midnight
// date placeholder only when a source supplies an actual time on that local day.
func combineSeriesDates(a, b core.Opportunity) []time.Time {
	dates := append(append([]time.Time{}, seriesDates(a)...), seriesDates(b)...)
	actualDays := make(map[string]bool)
	for _, t := range dates {
		if !core.IsDatePlaceholder(t) {
			actualDays[core.EventLocalDate(t)] = true
		}
	}
	unique := make(map[int64]bool)
	var out []time.Time
	for _, t := range dates {
		if core.IsDatePlaceholder(t) && actualDays[core.EventLocalDate(t)] {
			continue
		}
		if !unique[t.Unix()] {
			unique[t.Unix()] = true
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out
}

// ReconcileComedySeries retains the earliest card ID and all historical records.
// Aliases are flattened so feedback and evaluation history have one identity.
// Matching is deliberately conservative: provider identity, or normalized title
// at the same verified venue with at least one overlapping local performance day.
func (d *DB) ReconcileComedySeries(ctx context.Context) error {
	series, err := d.comedySeries(ctx)
	if err != nil {
		return err
	}
	// Build components from the immutable observations before changing any
	// schedules. A bridging observation connects its neighbors in this pass;
	// accumulated dates never manufacture additional matching edges.
	parents := make([]int, len(series))
	for i := range parents {
		parents[i] = i
	}
	root := func(i int) int {
		for parents[i] != i {
			i = parents[i]
		}
		return i
	}
	for i := range series {
		if series[i].opp.SupersededBy != nil {
			continue
		}
		for j := i + 1; j < len(series); j++ {
			if series[j].opp.SupersededBy != nil || !sameComedySeries(series[i], series[j]) {
				continue
			}
			a, b := root(i), root(j)
			if a > b {
				a, b = b, a
			}
			parents[b] = a
		}
	}
	components := make(map[int][]int)
	for i := range series {
		if series[i].opp.SupersededBy == nil {
			r := root(i)
			components[r] = append(components[r], i)
		}
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i := range series {
		members := components[i]
		if len(members) < 2 {
			continue
		}
		canonical := series[i].opp
		combined := canonical
		newest := canonical
		oneIdentity := sameSource(canonical, canonical) && len(seriesDates(canonical)) == 1
		for _, j := range members {
			o := series[j].opp
			combined.ShowDates = combineSeriesDates(combined, o)
			oneIdentity = oneIdentity && sameSource(canonical, o) && len(seriesDates(o)) == 1 && !hasOtherSeriesIdentity(series, o.ID, canonical)
			if o.DiscoveredAt.After(newest.DiscoveredAt) || (o.DiscoveredAt.Equal(newest.DiscoveredAt) && o.ID > newest.ID) {
				newest = o
			}
		}
		dates := combined.ShowDates
		// Replacement is safe only if the entire component and its previous
		// aliases represent one provider event, never a different provider's date.
		if oneIdentity {
			dates = seriesDates(newest)
		}
		if len(dates) == 0 {
			continue
		}
		if err := writeSeriesSchedule(ctx, tx, canonical.ID, dates); err != nil {
			return err
		}
		for _, j := range members[1:] {
			alias := series[j].opp
			if canonical.State == core.Expired && alias.State != core.Expired {
				if _, err := tx.ExecContext(ctx, `UPDATE opportunities SET state=?, evaluated_at=? WHERE id=?`, alias.State, nullTimeStr(alias.EvaluatedAt), canonical.ID); err != nil {
					return err
				}
				canonical.State = alias.State
			}
			if _, err := tx.ExecContext(ctx, `UPDATE opportunities SET superseded_by=? WHERE id=? OR superseded_by=?`, canonical.ID, alias.ID, alias.ID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func hasOtherSeriesIdentity(series []comedySeries, canonicalID int64, identity core.Opportunity) bool {
	for _, s := range series {
		if s.opp.SupersededBy != nil && *s.opp.SupersededBy == canonicalID && (!sameSource(s.opp, identity) || len(seriesDates(s.opp)) != 1) {
			return true
		}
	}
	return false
}

func writeSeriesSchedule(ctx context.Context, tx *sql.Tx, id int64, dates []time.Time) error {
	if len(dates) == 0 {
		return fmt.Errorf("series %d has no performance dates", id)
	}
	_, err := tx.ExecContext(ctx, `UPDATE opportunities SET start_time=?, show_dates=? WHERE id=?`, dates[0].Format(time.RFC3339), marshalShowDates(dates), id)
	return err
}

// UpsertComedySeries refreshes a scan's merged observations before date-based
// deduplication can discard the overlap needed to attach additional show dates.
// Different provider IDs are retained as aliases for subsequent scans.
func (d *DB) UpsertComedySeries(ctx context.Context, incoming core.Opportunity) (bool, error) {
	if incoming.HuntName != "comedy" {
		return false, fmt.Errorf("comedy series received hunt %q", incoming.HuntName)
	}
	series, err := d.comedySeries(ctx)
	if err != nil {
		return false, err
	}
	venue := ""
	if incoming.VenueID != nil {
		v, err := d.GetVenue(ctx, *incoming.VenueID)
		if err != nil {
			return false, err
		}
		venue = core.EventVenueKey(v.Name, v.Address)
	}
	match := -1
	identityKnown := false
	for i, s := range series {
		if sameSource(s.opp, incoming) {
			match = i
			identityKnown = true
			break
		}
	}
	if match < 0 {
		for i, s := range series {
			if s.opp.SupersededBy == nil && sameComedySeries(s, comedySeries{opp: incoming, venue: venue}) {
				match = i
				break
			}
		}
	}
	if match < 0 {
		_, err := d.InsertOpportunity(ctx, incoming)
		return err == nil, err
	}
	canonical := series[match].opp
	if canonical.SupersededBy != nil {
		canonical, err = d.GetOpportunity(ctx, *canonical.SupersededBy)
		if err != nil {
			return false, err
		}
	}
	dates := combineSeriesDates(canonical, incoming)
	if sameSource(canonical, incoming) && len(seriesDates(canonical)) == 1 && !hasOtherSeriesIdentity(series, canonical.ID, canonical) {
		dates = seriesDates(incoming)
	}
	if len(dates) == 0 {
		return false, nil
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if err := writeSeriesSchedule(ctx, tx, canonical.ID, dates); err != nil {
		return false, err
	}
	if canonical.State == core.Expired && dates[len(dates)-1].After(time.Now()) {
		if _, err := tx.ExecContext(ctx, `UPDATE opportunities SET state='discovered' WHERE id=?`, canonical.ID); err != nil {
			return false, err
		}
	}
	if !identityKnown && incoming.Source != "" && incoming.SourceID != "" {
		alias, err := insertOpportunity(ctx, tx, incoming)
		if err != nil {
			return false, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE opportunities SET superseded_by=? WHERE id=?`, canonical.ID, alias); err != nil {
			return false, err
		}
	}
	return false, tx.Commit()
}

// ResolveOpportunityID is for user interactions. Delivery deliberately reads
// the original row instead, so an obsolete alias can never resend its judgment.
func (d *DB) ResolveOpportunityID(ctx context.Context, id int64) (int64, error) {
	var canonical int64
	err := d.db.QueryRowContext(ctx, `SELECT COALESCE(superseded_by,id) FROM opportunities WHERE id=?`, id).Scan(&canonical)
	if err == sql.ErrNoRows {
		return 0, ErrNotFound
	}
	return canonical, err
}
