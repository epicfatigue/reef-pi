// Homeostasis.go
package controller

import (
	"fmt"
	"log"
	"math"
	"time"

	"github.com/reef-pi/reef-pi/controller/storage"
	"github.com/reef-pi/reef-pi/controller/telemetry"
	"github.com/reef-pi/reef-pi/controller/utils"
)

type target uint

const (
	noTarget target = iota
	upperTarget
	downerTarget
)

type Observation struct {
	Value  float64            `json:"value"`
	Upper  int                `json:"up"`
	Downer int                `json:"down"`
	Time   telemetry.TeleTime `json:"time"`
	total  float64
	len    int
}

func (o1 Observation) Rollup(o telemetry.Metric) (telemetry.Metric, bool) {
	o2 := o.(Observation)
	if o1.Time.Hour() != o2.Time.Hour() {
		return o, true
	}
	return Observation{
		Upper:  o1.Upper + o2.Upper,
		Downer: o1.Downer + o2.Downer,
		Time:   o1.Time,
		Value:  utils.RoundToTwoDecimal((o1.total + o2.Value) / float64(o1.len+1)),
		total:  o1.total + o2.Value,
		len:    o1.len + 1,
	}, false
}

func NewObservation(v float64) Observation {
	return Observation{
		Value: v,
		total: v,
		len:   1,
		Time:  telemetry.TeleTime(time.Now()),
	}
}

func (o1 Observation) Before(o2 telemetry.Metric) bool {
	o, ok := o2.(Observation)
	if !ok {
		return false
	}
	return o1.Time.Before(o.Time)
}

// HomeoStasisConfig defines control behavior.
// Historically reef-pi used IsMacro + Upper/Downer IDs.
// We keep IsMacro for backward compatibility, but introduce ControlType for extensibility.
type HomeoStasisConfig struct {
	Name       string
	Upper      string
	Downer     string
	Min        float64
	Max        float64
	Period     int
	Hysteresis float64

	// Legacy:
	// If ControlType is empty, IsMacro decides which subsystem to toggle:
	//   IsMacro=true  -> macro subsystem
	//   IsMacro=false -> equipment subsystem
	IsMacro bool

	// New:
	//   "equipment" | "macro" | "ato"
	ControlType string

	// ATO mode-switch support:
	// If ControlType == "ato" and ATOInRangeDisable == false,
	// we DO NOT disable both "actuators" when the value returns in range.
	// This lets ATO mode behave like a selector: keep last chosen ATO enabled.
	ATORaiseID        int
	ATOLowerID        int
	ATOInRangeDisable bool
}

// effectiveControlType resolves which control target type to use, keeping old configs working.
func (c HomeoStasisConfig) effectiveControlType() string {
	if c.ControlType != "" {
		return c.ControlType
	}
	if c.IsMacro {
		return "macro"
	}
	return "equipment"
}

// shouldDisableInRange decides whether we switch everything off when the value returns in range.
func (c HomeoStasisConfig) shouldDisableInRange() bool {
	// Only special-case ATO mode:
	// - If ATOInRangeDisable=false: latch (keep last selection enabled)
	// - If ATOInRangeDisable=true : disable like normal
	if c.effectiveControlType() == "ato" {
		return c.ATOInRangeDisable
	}
	// Equipment/macro keep classic reef-pi behavior: disable in-range
	return true
}

type Homeostasis struct {
	config     HomeoStasisConfig
	t          telemetry.Telemetry
	eqs        Subsystem
	macros     Subsystem
	atos       Subsystem
	pastTarget target
}

func NewHomeostasis(c Controller, config HomeoStasisConfig) *Homeostasis {
	h := Homeostasis{
		config:     config,
		t:          c.Telemetry(),
		eqs:        NoopSubsystem(),
		macros:     NoopSubsystem(),
		atos:       NoopSubsystem(),
		pastTarget: noTarget,
	}
	if sub, err := c.Subsystem(storage.MacroBucket); err == nil {
		h.macros = sub
	}
	if sub, err := c.Subsystem(storage.EquipmentBucket); err == nil {
		h.eqs = sub
	}
	if sub, err := c.Subsystem(storage.ATOBucket); err == nil {
		h.atos = sub
	}
	return &h
}

// a very basic equivalent of errors.Join, but works for go < 1.20
func BasicErrJoin(prevErr error, newErr error) error {
	if prevErr != nil {
		return fmt.Errorf("%w; %v", newErr, prevErr.Error())
	}
	return newErr
}

// Sub returns the subsystem to use for control actions.
func (h *Homeostasis) Sub() Subsystem {
	switch h.config.effectiveControlType() {
	case "macro":
		return h.macros
	case "ato":
		return h.atos
	default:
		return h.eqs
	}
}

func (h *Homeostasis) EmitMetric(m string, v float64) {
	h.t.EmitMetric(h.config.Name, m, v)
}

func (h *Homeostasis) Sync(o *Observation) error {
	switch {
	case (o.Value > h.config.Max) && (h.config.Downer != ""):
		log.Printf("Current value of '%s' is above maximum threshold. Executing down routine\n", h.config.Name)
		if err := h.down(); err != nil {
			return err
		}
		o.Downer += h.config.Period
		h.pastTarget = downerTarget

	case (o.Value < h.config.Min) && (h.config.Upper != ""):
		log.Printf("Current value of '%s' is below minimum threshold. Executing up routine\n", h.config.Name)
		if err := h.up(); err != nil {
			return err
		}
		o.Upper += int(h.config.Period)
		h.pastTarget = upperTarget

	case h.pastTarget == downerTarget && math.Abs(o.Value-h.config.Max) < h.config.Hysteresis:
		log.Printf("Current value of '%s' is within max threshold hysteresis, continue executing down routine\n", h.config.Name)
		if h.pastTarget == downerTarget {
			o.Downer += int(h.config.Period)
		}

	case h.pastTarget == upperTarget && math.Abs(o.Value-h.config.Min) < h.config.Hysteresis:
		log.Printf("Current value of '%s' is within min threshold hysteresis, continue executing up routine\n", h.config.Name)
		if h.pastTarget == upperTarget {
			o.Upper += int(h.config.Period)
		}

	default:
		// In-range behavior:
		// - equipment/macro: switch off both (classic reef-pi behavior)
		// - ato:
		//     - if ATOInRangeDisable=true  -> switch off both
		//     - if ATOInRangeDisable=false -> latch (do nothing; keep last selection enabled)
		if h.config.shouldDisableInRange() {
			log.Printf("Current value of '%s' within range switching off control targets\n", h.config.Name)
			_ = h.switchOffAll()
			h.pastTarget = noTarget
		} else {
			log.Printf("Current value of '%s' within range (ato latch enabled): leaving control targets as-is\n", h.config.Name)
			// Keep pastTarget as-is so hysteresis continuation still makes sense
			// and we keep the last selected ATO enabled.
		}
	}

	// NOTE: these metric keys are historical/quirky in the existing codebase.
	// Keep as-is to avoid breaking dashboards.
	if h.config.Upper != "" {
		h.EmitMetric("down", float64(o.Downer))
	}
	if h.config.Downer != "" {
		h.EmitMetric("up", float64(o.Upper))
	}
	return nil
}

func (h *Homeostasis) up() error {
	var result error

	// When executing "up", we turn OFF the downer and turn ON the upper.
	if h.config.Downer != "" {
		if err := h.Sub().On(h.config.Downer, false); err != nil {
			result = BasicErrJoin(result, err)
		}
	}
	if h.config.Upper != "" {
		if err := h.Sub().On(h.config.Upper, true); err != nil {
			result = BasicErrJoin(result, err)
		}
	}
	return result
}

func (h *Homeostasis) down() error {
	var result error

	// When executing "down", we turn OFF the upper and turn ON the downer.
	if h.config.Upper != "" {
		if err := h.Sub().On(h.config.Upper, false); err != nil {
			result = BasicErrJoin(result, err)
		}
	}
	if h.config.Downer != "" {
		if err := h.Sub().On(h.config.Downer, true); err != nil {
			result = BasicErrJoin(result, err)
		}
	}
	return result
}

func (h *Homeostasis) switchOffAll() error {
	var result error

	if h.config.Upper != "" {
		if err := h.Sub().On(h.config.Upper, false); err != nil {
			result = BasicErrJoin(result, err)
		}
	}
	if h.config.Downer != "" {
		if err := h.Sub().On(h.config.Downer, false); err != nil {
			result = BasicErrJoin(result, err)
		}
	}
	return result
}
