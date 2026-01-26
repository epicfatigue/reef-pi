//snapshot.go
package chemistry

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
	"github.com/reef-pi/hal"
)

// ReadSnapshot returns a structured snapshot (raw + derived) and augments it with
// rolling averages (1m + 5m) maintained by Chemistry.
func (c *Controller) ReadSnapshot(p Probe) (SnapshotResponse, error) {
	// Ensure the driver/pin has latest temperature before measuring (your existing hook)
	c.applyTemperatureIfSupported(p)

	pp, ok := any(c.ais).(AnalogPinProvider)
	if !ok {
		// No pin access: fall back to plain Read()
		v, err := c.Read(p)
		if err != nil {
			return SnapshotResponse{}, err
		}
		unit := p.Chart.Unit
		if unit == "" {
			unit = "value"
		}
		return SnapshotResponse{
			ProbeID: p.ID,
			At:      time.Now(),
			Value:   v,
			Unit:    unit,
			Signals: map[string]SignalStats{
				"value": {Now: v, Unit: unit},
			},
			Notes: []string{"snapshot not supported by analog input subsystem (no AnalogInputPin provider)"},
		}, nil
	}

	pin, err := pp.AnalogInputPin(p.AnalogInput)
	if err != nil {
		return SnapshotResponse{}, err
	}

	now := time.Now()

	// Prefer structured snapshot if supported by the pin/driver
	if sc, ok := any(pin).(hal.SnapshotCapable); ok {
		snap, err := sc.Snapshot()
		if err != nil {
			return SnapshotResponse{}, err
		}

		// Flatten numeric values for averaging.
		// "_value" is reserved for the primary reading.
		nums := make(map[string]float64, len(snap.Signals)+1)
		nums["_value"] = snap.Value
		for k, sig := range snap.Signals {
			nums[k] = sig.Now
		}

		// Update cache
		c.snapMu.Lock()
		cache := c.snaps[p.ID]
		if cache == nil {
			cache = &snapCache{}
			c.snaps[p.ID] = cache
		}
		cache.add(now, nums)
		c.snapMu.Unlock()

		// Helper to set avg pointers
		setAvgs := func(key string) (a1, a5 *float64) {
			c.snapMu.Lock()
			cache := c.snaps[p.ID]
			c.snapMu.Unlock()
			if cache == nil {
				return nil, nil
			}
			if v, ok := cache.avgSince(now.Add(-1*time.Minute), key); ok {
				vv := v
				a1 = &vv
			}
			if v, ok := cache.avgSince(now.Add(-5*time.Minute), key); ok {
				vv := v
				a5 = &vv
			}
			return
		}

		// Build output first (signals + avgs)
		out := SnapshotResponse{
			ProbeID: p.ID,
			At:      now,
			Value:   snap.Value,
			Unit:    snap.Unit,
			Signals: map[string]SignalStats{},
			Notes:   snap.Notes,
		}

		// Primary reading averages under key "value"
		a1, a5 := setAvgs("_value")
		out.Signals["value"] = SignalStats{Now: snap.Value, Unit: snap.Unit, Avg1m: a1, Avg5m: a5}

		// Raw/intermediate signals
		for k, sig := range snap.Signals {
			aa1, aa5 := setAvgs(k)
			out.Signals[k] = SignalStats{Now: sig.Now, Unit: sig.Unit, Avg1m: aa1, Avg5m: aa5}
		}

		// ---------------------------------------------------------------------
		// Meta: pass through driver meta, but add generic UI "display contract"
		// keys IF the driver hasn't provided them.
		// ---------------------------------------------------------------------
		metaMap := map[string]any{}
		if snap.Meta != nil {
			// Copy to avoid mutating driver's map instance
			for k, v := range snap.Meta {
				metaMap[k] = v
			}
		}

		// Helper getters
		getStr := func(k string) string {
			if v, ok := metaMap[k]; ok {
				if s, ok := v.(string); ok {
					return s
				}
			}
			return ""
		}

		// NEW: allow meta["calibration"] to be map OR JSON (RawMessage/[]byte/string)
		parseObj := func(v any) (map[string]any, bool) {
			switch t := v.(type) {
			case map[string]any:
				return t, true
			case json.RawMessage:
				if len(t) == 0 {
					return nil, false
				}
				var m map[string]any
				if err := json.Unmarshal(t, &m); err == nil && m != nil {
					return m, true
				}
				return nil, false
			case []byte:
				if len(t) == 0 {
					return nil, false
				}
				var m map[string]any
				if err := json.Unmarshal(t, &m); err == nil && m != nil {
					return m, true
				}
				return nil, false
			case string:
				if t == "" {
					return nil, false
				}
				var m map[string]any
				if err := json.Unmarshal([]byte(t), &m); err == nil && m != nil {
					return m, true
				}
				return nil, false
			default:
				return nil, false
			}
		}

		// Supports BOTH legacy flat key AND new nested contract meta.calibration.observed_key
		getObservedKey := func() string {
			// 1) legacy flat key
			if rk := getStr("calibration_observed_key"); rk != "" {
				return rk
			}

			// 2) new contract: meta.calibration.observed_key
			if v, ok := metaMap["calibration"]; ok && v != nil {
				if m, ok := parseObj(v); ok {
					if s, ok := m["observed_key"].(string); ok && s != "" {
						return s
					}
				}
			}
			return ""
		}

		// 1) raw_signal_key: prefer driver's observed key (legacy or new contract)
		if _, ok := metaMap["raw_signal_key"]; !ok {
			if rk := getObservedKey(); rk != "" {
				metaMap["raw_signal_key"] = rk
			}
		}

		// 2) primary_signal_key: always "value" in our SnapshotResponse Signals map
		// (driver can override this if it wants)
		if _, ok := metaMap["primary_signal_key"]; !ok {
			metaMap["primary_signal_key"] = "value"
		}

		// 3) secondary_signal_keys: if driver didn't provide, compute GENERIC list:
		// include signals that are not raw, and that are not redundant with primary.
		if _, ok := metaMap["secondary_signal_keys"]; !ok {
			rawKey := ""
			if v, ok := metaMap["raw_signal_key"]; ok {
				if s, ok := v.(string); ok {
					rawKey = s
				}
			}

			// Redundant = same unit as primary AND numerically almost equal to snap.Value
			almostEqual := func(a, b float64) bool {
				// relative-ish tolerance: good enough for UI redundancy filtering
				return math.Abs(a-b) <= 1e-9*math.Max(1.0, math.Max(math.Abs(a), math.Abs(b)))
			}

			secs := make([]string, 0, len(snap.Signals))
			for k, sig := range snap.Signals {
				if k == rawKey {
					continue
				}
				if sig.Unit == snap.Unit && almostEqual(sig.Now, snap.Value) {
					// don't show duplicate of primary
					continue
				}
				secs = append(secs, k)
			}
			metaMap["secondary_signal_keys"] = secs
		}

		// Marshal meta for swagger-friendliness
		if len(metaMap) > 0 {
			if b, err := json.Marshal(metaMap); err == nil {
				out.Meta = b
			}
		}

		return out, nil
	}

	// Pin doesn't support Snapshot(): fallback to Value only
	v, err := c.Read(p)
	if err != nil {
		return SnapshotResponse{}, err
	}
	unit := p.Chart.Unit
	if unit == "" {
		unit = "value"
	}
	return SnapshotResponse{
		ProbeID: p.ID,
		At:      now,
		Value:   v,
		Unit:    unit,
		Signals: map[string]SignalStats{
			"value": {Now: v, Unit: unit},
		},
		Notes: []string{fmt.Sprintf("pin %T does not implement hal.SnapshotCapable", pin)},
	}, nil
}
