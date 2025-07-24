package marketdata

import "time"

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
	statuses map[time.Time]dayStatus // keyed by UTC date (00:00:00)
	cutoff   time.Time               // largest date for which *all* earlier days are settled
	maxDay   time.Time               // highest day included in this run
}

// newDayStatusTracker initialises the tracker with the set of days that will be
// processed in this run. The cutoff is initialCutoff – typically the
// last_loaded_at value already stored in the DB (or one day before the first
// pending day).
func newDayStatusTracker(days []time.Time, initialCutoff time.Time) *dayStatusTracker {
	// Normalise all dates to YYYY-MM-DD UTC.
	m := make(map[time.Time]dayStatus, len(days))
	var max time.Time
	for _, d := range days {
		nd := truncateDate(d)
		m[nd] = statusPending
		if nd.After(max) {
			max = nd
		}
	}
	return &dayStatusTracker{statuses: m, cutoff: truncateDate(initialCutoff), maxDay: max}
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
		d.advanceCutoff()
	}
}

// CurrentCutoff returns the largest date such that every day ≤ cutoff is either
// loaded or permanently failed.
func (d *dayStatusTracker) CurrentCutoff() time.Time {
	return d.cutoff
}

// advanceCutoff moves the cutoff forward while the next contiguous day is no
// longer pending.
func (d *dayStatusTracker) advanceCutoff() {
	for {
		next := d.cutoff.AddDate(0, 0, 1)
		if next.After(d.maxDay) {
			return // we have advanced as far as we can for this run
		}
		st, ok := d.statuses[next]
		if ok {
			if st == statusPending {
				return
			}
			// loaded or failed -> advance
			d.cutoff = next
			continue
		}
		// Day not in map (e.g., weekend) – treat as implicitly completed.
		d.cutoff = next
	}
}

// truncateDate strips the time component, returning the date at 00:00:00 UTC.
func truncateDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
