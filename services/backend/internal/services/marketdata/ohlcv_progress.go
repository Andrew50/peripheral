package marketdata

import (
	"log"
	"time"
)

// dayStatus represents the load status for a single trading day.
type dayStatus int

const (
	statusPending dayStatus = iota
	statusLoaded
	statusFailed
)

// dayStatusTracker keeps an in-memory view of which days are done and which are
// still pending so we can always emit a conservative, crash-safe cut-off date.
type dayStatusTracker struct {
	statuses  map[time.Time]dayStatus // keyed by UTC date (00:00:00)
	cutoff    time.Time               // current cutoff (moves forward or backward based on direction)
	maxDay    time.Time               // highest day included in this run
	minDay    time.Time               // lowest day included in this run
	direction bool                    // true = forward, false = backward
}

// newDayStatusTracker initialises the tracker with the set of days that will be
// processed in this run. The cutoff is initialCutoff – typically the
// last_loaded_at value already stored in the DB (or one day before the first
// pending day). Direction is explicitly provided by the caller.
func newDayStatusTracker(days []time.Time, initialCutoff time.Time, direction bool) *dayStatusTracker {
	// Normalise all dates to YYYY-MM-DD UTC.
	m := make(map[time.Time]dayStatus, len(days))
	var max, min time.Time
	for _, d := range days {
		nd := truncateDate(d)
		m[nd] = statusPending
		if max.IsZero() || nd.After(max) {
			max = nd
		}
		if min.IsZero() || nd.Before(min) {
			min = nd
		}
	}

	log.Printf("🔍 Progress tracker initialized: days=%d, min=%s, max=%s, initialCutoff=%s, direction=%s",
		len(days), min.Format("2006-01-02"), max.Format("2006-01-02"),
		initialCutoff.Format("2006-01-02"), map[bool]string{true: "forward", false: "backward"}[direction])

	return &dayStatusTracker{
		statuses:  m,
		cutoff:    truncateDate(initialCutoff),
		maxDay:    max,
		minDay:    min,
		direction: direction,
	}
}

// MarkLoaded records a successful load for the given day.
func (d *dayStatusTracker) MarkLoaded(day time.Time) { d.mark(day, statusLoaded) }

// MarkFailed records a permanent failure for the given day (it is guaranteed
// that it has been written to ohlcv_failed_files).
func (d *dayStatusTracker) MarkFailed(day time.Time) { d.mark(day, statusFailed) }

func (d *dayStatusTracker) mark(day time.Time, s dayStatus) {
	day = truncateDate(day)
	if prev, ok := d.statuses[day]; ok && prev == statusPending {
		d.statuses[day] = s
		// statusName := map[dayStatus]string{statusPending: "pending", statusLoaded: "loaded", statusFailed: "failed"}[s]
		// log.Printf("🔍 Progress tracker: marked %s as %s (direction=%s)",
		//	day.Format("2006-01-02"), statusName, map[bool]string{true: "forward", false: "backward"}[d.direction])
		d.advanceCutoff()
	} else {
		// log.Printf("🔍 Progress tracker: skipped marking %s (not pending or not in map)", day.Format("2006-01-02"))
	}
}

// CurrentCutoff returns the largest date such that every day ≤ cutoff is either
// loaded or permanently failed.
func (d *dayStatusTracker) CurrentCutoff() time.Time {
	return d.cutoff
}

// advanceCutoff moves the cutoff forward or backward while the next contiguous day
// is no longer pending, based on the tracker's direction.
func (d *dayStatusTracker) advanceCutoff() {
	//oldCutoff := d.cutoff
	if d.direction {
		// Forward direction
		for {
			next := d.cutoff.AddDate(0, 0, 1)
			if next.After(d.maxDay) {
				break // we have advanced as far as we can for this run
			}
			st, ok := d.statuses[next]
			if ok {
				if st == statusPending {
					break
				}
				// loaded or failed -> advance
				d.cutoff = next
				continue
			}
			// Day not in map (e.g., weekend) – treat as implicitly completed.
			d.cutoff = next
		}
	} else {
		// Backward direction
		for {
			prev := d.cutoff.AddDate(0, 0, -1)
			if prev.Before(d.minDay) {
				break // we have retreated as far as we can for this run
			}
			st, ok := d.statuses[prev]
			if ok {
				if st == statusPending {
					break
				}
				// loaded or failed -> retreat
				d.cutoff = prev
				continue
			}
			// Day not in map (e.g., weekend) – treat as implicitly completed.
			d.cutoff = prev
		}
	}

	/*if !d.cutoff.Equal(oldCutoff) {
		 log.Printf("🔍 Progress tracker: cutoff moved from %s to %s (direction=%s)",
			oldCutoff.Format("2006-01-02"), d.cutoff.Format("2006-01-02"),
			map[bool]string{true: "forward", false: "backward"}[d.direction])
	}*/
}

// truncateDate strips the time component, returning the date at 00:00:00 UTC.
func truncateDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
