package chemistry

import "time"

// snapSample is one snapshot sample at a point in time.
// We store a map of numeric signals so different drivers can expose different keys
// (e.g. "U", "V", "abs_d", "voltage", "_value", etc).
type snapSample struct {
	ts      time.Time
	signals map[string]float64 // key -> value
}

// snapCache holds recent samples for averaging.
// We only keep a rolling 5-minute window (enough to compute 1m + 5m averages).
type snapCache struct {
	window5m []snapSample
}

// add appends a new sample and prunes anything older than 5 minutes.
func (sc *snapCache) add(ts time.Time, signals map[string]float64) {
	// Make a shallow copy of the signals map so callers can reuse/modify theirs safely.
	cp := make(map[string]float64, len(signals))
	for k, v := range signals {
		cp[k] = v
	}

	sc.window5m = append(sc.window5m, snapSample{
		ts:      ts,
		signals: cp,
	})

	// Prune older than 5 minutes (keep samples with ts >= cut)
	cut := ts.Add(-5 * time.Minute)

	// Find first index with ts >= cut
	i := 0
	for ; i < len(sc.window5m); i++ {
		if !sc.window5m[i].ts.Before(cut) {
			break
		}
	}
	if i > 0 {
		sc.window5m = sc.window5m[i:]
	}
}

// avgSince computes the average of "key" over samples with ts >= since.
func (sc *snapCache) avgSince(since time.Time, key string) (float64, bool) {
	var sum float64
	var n int

	for _, s := range sc.window5m {
		if s.ts.Before(since) {
			continue
		}
		v, ok := s.signals[key]
		if !ok {
			continue
		}
		sum += v
		n++
	}

	if n == 0 {
		return 0, false
	}
	return sum / float64(n), true
}
