//snapshot_models.go
package chemistry

import (
	"encoding/json"
	"time"
)

// swagger:model chemistryProbeSnapshot
type SnapshotResponse struct {
	// Probe ID
	ProbeID string `json:"probe_id"`

	// Time the snapshot was captured
	At time.Time `json:"at"`

	// Primary (calibrated/converted) reading for this probe
	Value float64 `json:"value"`

	// Unit for the primary reading (e.g. "pH", "mV", "uS/cm", "ppt")
	Unit string `json:"unit"`

	// Signals are driver-defined raw/intermediate readings with now + optional rolling averages.
	// Key examples: "U", "V", "abs_d", "voltage", "adc", "tempC", etc.
	Signals map[string]SignalStats `json:"signals,omitempty"`

	// Meta is optional driver-defined context (calibration coefficients, temp-comp settings, etc.)
	// Stored as raw JSON to keep swagger generators happy.
	Meta json.RawMessage `json:"meta,omitempty"`

	// Notes are optional human-readable messages (e.g. missing calibration, stale temperature).
	Notes []string `json:"notes,omitempty"`
}

// swagger:model chemistryProbeSignalStats
type SignalStats struct {
	// Current value
	Now float64 `json:"now"`

	// Unit for this signal (e.g. "mV", "V", "counts", "C")
	Unit string `json:"unit,omitempty"`

	// 1 minute rolling average (if enough samples exist)
	Avg1m *float64 `json:"avg_1m,omitempty"`

	// 5 minute rolling average (if enough samples exist)
	Avg5m *float64 `json:"avg_5m,omitempty"`
}
