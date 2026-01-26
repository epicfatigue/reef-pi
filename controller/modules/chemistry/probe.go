package chemistry

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/Knetic/govaluate"

	"github.com/reef-pi/hal"

	"github.com/reef-pi/reef-pi/controller"
	"github.com/reef-pi/reef-pi/controller/storage"
	"github.com/reef-pi/reef-pi/controller/telemetry"
	"github.com/reef-pi/reef-pi/controller/utils"
)

const ReadingsBucket = storage.PhReadingsBucket

type Notify struct {
	Enable bool    `json:"enable"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
}

type ChartConfig struct {
	YMin  float64 `json:"ymin"`
	YMax  float64 `json:"ymax"`
	Color string  `json:"color"`
	Unit  string  `json:"unit"`
}

// --- TEMP COMP ---
// Something that can read temperature by reef-pi temp sensor ID
type TempReader interface {
	ReadByID(id int) (float64, error)
}

// Optional: if your analog-input subsystem can expose the underlying pin,
// we can set temperature on it before measuring.
type AnalogPinProvider interface {
	AnalogInputPin(id string) (hal.AnalogInputPin, error)
}

// Optional: implemented by pins/drivers that accept live temperature updates
type TemperatureSetter interface {
	SetTemperatureC(tempC float64)
}

// swagger:model phProbe
type Probe struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Enable       bool          `json:"enable"`
	Period       time.Duration `json:"period"`
	AnalogInput  string        `json:"analog_input"`
	Control      bool          `json:"control"`
	Notify       Notify        `json:"notify"`
	UpperEq      string        `json:"upper_eq"`
	DownerEq     string        `json:"downer_eq"`
	Min          float64       `json:"min"`
	Max          float64       `json:"max"`
	Hysteresis   float64       `json:"hysteresis"`
	IsMacro      bool          `json:"is_macro"`
	OneShot      bool          `json:"one_shot"`
	Chart        ChartConfig   `json:"chart"`
	Transformer  string        `json:"transformer"`
	TempSensorID int           `json:"temp_sensor_id"` // reef-pi temperature sensor ID, -1 = disabled

	h *controller.Homeostasis
}

func (p *Probe) loadHomeostasis(c controller.Controller) {
	hConf := controller.HomeoStasisConfig{
		Name:       p.Name,
		Upper:      p.UpperEq,
		Downer:     p.DownerEq,
		Min:        p.Min,
		Max:        p.Max,
		Period:     int(p.Period),
		IsMacro:    p.IsMacro,
		Hysteresis: p.Hysteresis,
	}
	p.h = controller.NewHomeostasis(c, hConf)
}

// swagger:model calibrationPoint
type CalibrationPoint struct {
	Type     string   `json:"type"`
	Expected float64  `json:"expected"`
	Observed *float64 `json:"observed,omitempty"` // nil means "auto-pick from snapshot"
}

func (c *Controller) Get(id string) (Probe, error) {
	var p Probe
	return p, c.c.Store().Get(Bucket, id, &p)
}

func (c *Controller) List() ([]Probe, error) {
	probes := []Probe{}
	fn := func(_ string, v []byte) error {
		var p Probe
		if err := json.Unmarshal(v, &p); err != nil {
			return err
		}
		probes = append(probes, p)
		return nil
	}
	return probes, c.c.Store().List(Bucket, fn)
}

func (p Probe) Validate() error {
	if p.Period <= 0 {
		return fmt.Errorf("Period should be positive. Supplied: %d", p.Period)
	}
	if p.Transformer != "" {
		expr, err := govaluate.NewEvaluableExpression(p.Transformer)
		parameters := make(map[string]interface{}, 1)
		parameters["v"] = 0.0
		result, err := expr.Evaluate(parameters)
		if err != nil {
			return fmt.Errorf("invalid transformer expresssion '%s'. Failed to parse:%w", p.Transformer, err)
		}
		_, ok := result.(float64)
		if !ok {
			return fmt.Errorf("invalid transformer expression '%s'. failed to typecast result '%v' into float64", p.Transformer, result)
		}
	}
	// TempSensorID can be -1 (disabled) or a valid temp sensor id (>=0)
	if p.TempSensorID < -1 {
		return fmt.Errorf("TempSensorID must be -1 (disabled) or >= 0. Supplied: %d", p.TempSensorID)
	}
	return nil
}

func (c *Controller) Create(p Probe) error {
	if err := p.Validate(); err != nil {
		return err
	}
	fn := func(id string) interface{} {
		p.ID = id
		return &p
	}
	if err := c.c.Store().Create(Bucket, fn); err != nil {
		return err
	}
	c.statsMgr.Initialize(p.ID)
	if p.Enable {
		p.CreateFeed(c.c.Telemetry())

		// IMPORTANT: apply saved calibration whenever we start a probe
		c.applySavedCalibration(p)

		quit := make(chan struct{})
		c.quitters[p.ID] = quit
		go c.Run(p, quit)
	}
	return nil
}

func (c *Controller) Update(id string, p Probe) error {
	p.ID = id
	if err := p.Validate(); err != nil {
		return err
	}
	if err := c.c.Store().Update(Bucket, id, p); err != nil {
		return err
	}
	quit, ok := c.quitters[p.ID]
	if ok {
		close(quit)
		delete(c.quitters, p.ID)
	}
	if p.Enable {
		p.CreateFeed(c.c.Telemetry())

		// IMPORTANT: apply saved calibration whenever we (re)start a probe
		c.applySavedCalibration(p)

		quit := make(chan struct{})
		c.quitters[p.ID] = quit
		go c.Run(p, quit)
	}
	return nil
}

func (c *Controller) Delete(id string) error {
	c.Lock()
	defer c.Unlock()
	if err := c.c.Store().Delete(Bucket, id); err != nil {
		return err
	}
	if err := c.statsMgr.Delete(id); err != nil {
		log.Println("ERROR: chemistry sub-system: Failed to deleted readings for probe:", id)
	}

	_ = c.c.Store().Delete(CalibrationBucket, id)

	quit, ok := c.quitters[id]
	if ok {
		close(quit)
		delete(c.quitters, id)
	}
	return nil
}

// --- TEMP COMP ---
// Tries to push the latest temperature into the analog input pin (if supported).
// This is safe/no-op for non-salinity probes or when subsystems don't support the hooks.
func (c *Controller) applyTemperatureIfSupported(p Probe) {
	if p.TempSensorID < 0 {
		log.Printf("chemistry tempcomp: disabled for probe=%s (temp_sensor_id=%d)", p.Name, p.TempSensorID)
		return
	}
	if c.tcs == nil {
		log.Printf("chemistry tempcomp: no temp reader wired (c.tcs is nil)")
		return
	}
	pp, ok := any(c.ais).(AnalogPinProvider)
	if !ok {
		log.Printf("chemistry tempcomp: analog input subsystem does not expose AnalogInputPin()")
		return
	}
	pin, err := pp.AnalogInputPin(p.AnalogInput)
	if err != nil {
		log.Printf("chemistry tempcomp: failed to resolve pin=%s err=%v", p.AnalogInput, err)
		return
	}
	ts, ok := any(pin).(TemperatureSetter)
	if !ok {
		log.Printf("chemistry tempcomp: pin=%s type=%T does not implement TemperatureSetter", p.AnalogInput, pin)
		return
	}
	tempC, err := c.tcs.ReadByID(p.TempSensorID)
	if err != nil {
		log.Printf("chemistry tempcomp: failed to read temp sensor id=%d err=%v", p.TempSensorID, err)
		return
	}

	log.Printf("chemistry tempcomp: applying %.2fC to pin=%s", tempC, p.AnalogInput)
	ts.SetTemperatureC(tempC)
}

func (c *Controller) Read(p Probe) (float64, error) {
	var v float64
	if c.devMode {
		v = 8 + rand.Float64()*2
	} else {
		v1, err := c.ais.Read(p.AnalogInput)
		if err != nil {
			return 0, nil
		}
		v = v1
	}
	if p.Transformer != "" {
		expr, err := govaluate.NewEvaluableExpression(p.Transformer)
		parameters := make(map[string]interface{}, 1)
		parameters["v"] = v
		result, err := expr.Evaluate(parameters)
		if err != nil {
			return -1, err
		}
		log.Println("chemistry subsystem executing transform:", p.Transformer, "with value:", v, "result:", result)
		v1, ok := result.(float64)
		if !ok {
			return -1, fmt.Errorf("failed to typecast '%v' into float64", result)
		}
		v = v1
	}
	return utils.RoundToTwoDecimal(v), nil
}

func (c *Controller) Run(p Probe, quit chan struct{}) error {
	if p.Period <= 0 {
		log.Printf("ERROR:chemistry sub-system. Invalid period set for probe:%s. Expected positive, found:%d\n", p.Name, p.Period)
		return fmt.Errorf("invalid period: %d for probe: %s", p.Period, p.Name)
	}
	if p.Control {
		p.loadHomeostasis(c.c)
	}
	p.CreateFeed(c.c.Telemetry())
	ticker := time.NewTicker(p.Period * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			reading, err := c.checkAndControl(p)
			if p.OneShot {
				if err != nil {
					return err
				}
				if p.WithinRange(reading) {
					p.Enable = false
					return c.Update(p.ID, p)
				}
			}
		case <-quit:
			return nil
		}
	}
}

func (c *Controller) checkAndControl(p Probe) (float64, error) {
	// Push the latest temperature into the driver/pin before reading.
	c.applyTemperatureIfSupported(p)

	reading, err := c.Read(p)
	if err != nil {
		log.Println("chemistry sub-system: ERROR: Failed to read probe:", p.Name, ". Error:", err)
		c.c.LogError("ph-"+p.ID, "chemistry subsystem: Failed read probe:"+p.Name+"Error:"+err.Error())
		return 0, err
	}
	c.Lock()
	defer c.Unlock()

	log.Println("chemistry sub-system: Probe:", p.Name, "Reading:", reading)
	notifyIfNeeded(c.c.Telemetry(), p, reading)
	u := controller.NewObservation(reading)
	if p.Control {
		if err := p.h.Sync(&u); err != nil {
			log.Println("ERROR: Failed to execute ph control logic. Error:", err)
		}
	}
	c.statsMgr.Update(p.ID, u)
	c.c.Telemetry().EmitMetric("ph", p.Name, reading)
	return reading, nil
}

func (c *Controller) Calibrate(id string, ms []hal.Measurement) error {
	p, err := c.Get(id)
	if err != nil {
		return err
	}
	if p.Enable {
		return fmt.Errorf("Probe must be disabled from automatic polling before running calibration")
	}

	// Make sure temp is applied before calibration (important for temp-comp probes)
	c.applyTemperatureIfSupported(p)

	pin, err := c.analogPinForProbe(p)
	if err != nil {
		return err
	}

	// Driver-owned calibration
	if err := pin.Calibrate(ms); err != nil {
		return err
	}

	// Persist to ph_calibration bucket (this is what we replay on reboot)
	if err := c.upsertCalibration(p.ID, ms); err != nil {
		return err
	}

	return nil
}

func (c *Controller) CalibratePoint(id string, point CalibrationPoint) error {
	p, err := c.Get(id)
	if err != nil {
		return err
	}
	if p.Enable {
		return fmt.Errorf("Probe must be disabled from automatic polling before running calibration")
	}

	// Apply temp before snapshot/calibration
	c.applyTemperatureIfSupported(p)

	pin, err := c.analogPinForProbe(p)
	if err != nil {
		return err
	}

	// Determine observed:
	var observed float64
	if point.Observed != nil {
		observed = *point.Observed
	} else {
		if sc, ok := any(pin).(hal.SnapshotCapable); ok {
			snap, err := sc.Snapshot()
			if err != nil {
				return err
			}
			observed = snap.Value
			if snap.Meta != nil {
				if k, ok := snap.Meta["calibration_observed_key"].(string); ok && k != "" {
					if sig, ok := snap.Signals[k]; ok {
						observed = sig.Now
					}
				}
			}
		} else {
			v, err := pin.Value()
			if err != nil {
				return err
			}
			observed = v
		}
	}

	newPoint := hal.Measurement{Expected: point.Expected, Observed: observed}

	// Merge with existing saved calibration (so fresh + std both persist)
	var existing []hal.Measurement
	_ = c.c.Store().Get(CalibrationBucket, p.ID, &existing)
	merged := mergeCalibration(existing, newPoint)

	// Apply ONLY the new point right now (fast)
	if err := pin.Calibrate([]hal.Measurement{newPoint}); err != nil {
		return err
	}

	// Persist merged set for reboot
	if err := c.upsertCalibration(p.ID, merged); err != nil {
		return err
	}

	return nil
}

func mergeCalibration(existing []hal.Measurement, add hal.Measurement) []hal.Measurement {
	// Replace by Expected bucket:
	// - Expected == 0 -> fresh point
	// - Expected > 0  -> standard point (we only keep one standard in this simple model)
	out := make([]hal.Measurement, 0, 2)

	// Keep everything except the point we're replacing
	for _, m := range existing {
		if add.Expected == 0 {
			if m.Expected == 0 {
				continue
			}
		} else { // add.Expected > 0
			if m.Expected > 0 {
				continue
			}
		}
		out = append(out, m)
	}

	out = append(out, add)
	return out
}

// helper to resolve the pin
func (c *Controller) analogPinForProbe(p Probe) (hal.AnalogInputPin, error) {
	pp, ok := any(c.ais).(AnalogPinProvider)
	if !ok {
		return nil, fmt.Errorf("analog input subsystem does not expose AnalogInputPin()")
	}
	return pp.AnalogInputPin(p.AnalogInput)
}

func (p Probe) CreateFeed(t telemetry.Telemetry) {
	t.CreateFeedIfNotExist("ph-" + p.Name)
}

func notifyIfNeeded(t telemetry.Telemetry, p Probe, reading float64) {
	if !p.Notify.Enable {
		return
	}
	subject := fmt.Sprintf("sensor '%s' is out of range", p.Name)
	format := "Current value of probe '%s' (%s) is out of acceptable range ( %s -%s )"
	body := fmt.Sprintf(format, p.Name, utils.FormatFloat(reading), utils.FormatFloat(p.Notify.Min), utils.FormatFloat(p.Notify.Max))
	if reading >= p.Notify.Max {
		t.Alert(subject, p.Name+" is high. "+body)
		return
	}
	if reading <= p.Notify.Min {
		t.Alert(subject, p.Name+" is low. "+body)
		return
	}
}

func (p Probe) WithinRange(v float64) bool {
	return v >= p.Min && v <= p.Max
}
