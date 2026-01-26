package chemistry

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/reef-pi/hal"
	"github.com/reef-pi/reef-pi/controller"
	"github.com/reef-pi/reef-pi/controller/device_manager/connectors"
	"github.com/reef-pi/reef-pi/controller/storage"
	"github.com/reef-pi/reef-pi/controller/telemetry"
)


const Bucket = storage.PhBucket
const CalibrationBucket = storage.PhCalibrationBucket

type Controller struct {
	sync.Mutex
	c        controller.Controller
	quitters map[string]chan struct{}
	statsMgr telemetry.StatsManager
	devMode  bool
	ais      *connectors.AnalogInputs

	tcs TempReader // TEMP COMP: temperature subsystem reader (inject latest temp into driver)

	// Snapshot rolling averages (1m/5m)
	snapMu sync.Mutex
	snaps  map[string]*snapCache // probeID -> cache
}

func New(devMode bool, c controller.Controller, tr TempReader) *Controller {
	return &Controller{
		quitters: make(map[string]chan struct{}),
		c:        c,
		devMode:  devMode,
		ais:      c.DM().AnalogInputs(),
		statsMgr: c.Telemetry().NewStatsManager(ReadingsBucket),
		tcs:      tr,
		snaps:    make(map[string]*snapCache),
	}
}

func (c *Controller) Setup() error {
	// Probe configs
	if err := c.c.Store().CreateBucket(Bucket); err != nil {
		return err
	}

	// Keep bucket for audit/history/backward compatibility.
	// NOTE: we no longer load/apply hal.CalibratorFactory-based calibration at runtime.
	if err := c.c.Store().CreateBucket(CalibrationBucket); err != nil {
		return err
	}

	// Readings stats bucket
	return c.c.Store().CreateBucket(ReadingsBucket)
}

func (c *Controller) Start() {
	probes, err := c.List()
	if err != nil {
		log.Println("ERROR: chemistry subsystem: Failed to list probes. Error:", err)
		return
	}
	for _, p := range probes {
		if !p.Enable {
			continue
		}
		fn := func(d json.RawMessage) interface{} {
			u := controller.Observation{}
			json.Unmarshal(d, &u)
			return u
		}
		if err := c.statsMgr.Load(p.ID, fn); err != nil {
			log.Println("ERROR: chemistry controller. Failed to load usage. Error:", err)
		}
		c.applySavedCalibration(p)
		quit := make(chan struct{})
		c.quitters[p.ID] = quit
		go c.Run(p, quit)
	}
}

func (c *Controller) Stop() {
	for id, quit := range c.quitters {
		close(quit)
		if err := c.statsMgr.Save(id); err != nil {
			log.Println("ERROR: chemistry controller. Failed to save usage. Error:", err)
		}
		log.Println("chemistry sub-system: Saved usaged data of sensor:", id)
		delete(c.quitters, id)
	}
}

// upsertCalibration persists calibration points under the probe ID.
// Update() fails if the key doesn't exist yet, so we fall back to CreateWithID().
func (c *Controller) upsertCalibration(probeID string, ms []hal.Measurement) error {
	// Try update first (normal case)
	if err := c.c.Store().Update(CalibrationBucket, probeID, ms); err == nil {
		return nil
	}

	// If it doesn't exist yet, create it with the same key as probeID
	// (this is exactly what we need for "persist after reboot")
	if err := c.c.Store().CreateWithID(CalibrationBucket, probeID, &ms); err != nil {
		return err
	}
	return nil
}


func (c *Controller) applySavedCalibration(p Probe) {
    // Load saved measurements (if any)
    var ms []hal.Measurement
    if err := c.c.Store().Get(CalibrationBucket, p.ID, &ms); err != nil {
        return // none saved -> nothing to apply
    }
    if len(ms) == 0 {
        return
    }

    // Temperature first (important for temp-comp probes)
    c.applyTemperatureIfSupported(p)

    pin, err := c.analogPinForProbe(p)
    if err != nil {
        log.Printf("chemistry: failed to resolve pin for probe=%s analog_input=%s err=%v", p.Name, p.AnalogInput, err)
        return
    }

    // Re-apply driver-owned calibration
    if err := pin.Calibrate(ms); err != nil {
        log.Printf("chemistry: failed to re-apply calibration for probe=%s ms=%v err=%v", p.Name, ms, err)
        return
    }

    log.Printf("chemistry: re-applied calibration on startup for probe=%s (points=%d)", p.Name, len(ms))
}


func (c *Controller) On(id string, b bool) error {
	p, err := c.Get(id)
	if err != nil {
		return err
	}
	p.Enable = b
	if b && p.OneShot {
		q := make(chan struct{})
		defer close(q)
		return c.Run(p, q)
	}
	return c.Update(id, p)
}

func (c *Controller) InUse(depType, id string) ([]string, error) {
	var deps []string
	switch depType {
	case storage.EquipmentBucket:
		probes, err := c.List()
		if err != nil {
			return deps, err
		}
		for _, p := range probes {
			if p.UpperEq == id && !p.IsMacro {
				deps = append(deps, p.Name)
			}
		}
		for _, p := range probes {
			if p.DownerEq == id && !p.IsMacro {
				deps = append(deps, p.Name)
			}
		}
		return deps, nil

	case storage.AnalogInputBucket:
		probes, err := c.List()
		if err != nil {
			return deps, err
		}
		for _, p := range probes {
			if p.AnalogInput == id {
				deps = append(deps, p.Name)
			}
		}
		return deps, nil

	case storage.MacroBucket:
		probes, err := c.List()
		if err != nil {
			return deps, err
		}
		for _, p := range probes {
			if p.UpperEq == id && p.IsMacro {
				deps = append(deps, p.Name)
			}
		}
		for _, p := range probes {
			if p.DownerEq == id && p.IsMacro {
				deps = append(deps, p.Name)
			}
		}
		return deps, nil

	default:
		return deps, fmt.Errorf("unknown dependency type:%s", depType)
	}
}

func (c *Controller) GetEntity(id string) (controller.Entity, error) {
	return nil, fmt.Errorf("chemistry subsystem does not support 'GetEntity' interface")
}
