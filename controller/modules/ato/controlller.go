package ato

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/reef-pi/reef-pi/controller"
	"github.com/reef-pi/reef-pi/controller/device_manager/connectors"
	"github.com/reef-pi/reef-pi/controller/storage"
	"github.com/reef-pi/reef-pi/controller/telemetry"
)

const Bucket = storage.ATOBucket
const UsageBucket = storage.ATOUsageBucket

type Controller struct {
	statsMgr telemetry.StatsManager
	devMode  bool
	quitters map[string]chan struct{}
	mu       *sync.Mutex
	inlets   *connectors.Inlets
	c        controller.Controller
}

func New(devMode bool, c controller.Controller) (*Controller, error) {
	con := &Controller{
		devMode:  devMode,
		mu:       &sync.Mutex{},
		inlets:   c.DM().Inlets(),
		quitters: make(map[string]chan struct{}),
		statsMgr: c.Telemetry().NewStatsManager(UsageBucket),
		c:        c,
	}
	return con, nil
}

func (c *Controller) sub(a ATO) (controller.Subsystem, error) {
	if a.IsMacro {
		return c.c.Subsystem(storage.MacroBucket)
	}
	return c.c.Subsystem(storage.EquipmentBucket)
}

func (c *Controller) Setup() error {
	// Create the ATO config bucket
	if err := c.c.Store().CreateBucket(Bucket); err != nil {
		return err
	}
	// Create the ATO usage bucket
	return c.c.Store().CreateBucket(UsageBucket)
}

func (c *Controller) Start() {
	c.mu.Lock()
	defer c.mu.Unlock()

	atos, err := c.List()
	if err != nil {
		log.Println("ERROR: ato subsystem: Failed to list sensors. Error:", err)
		return
	}

	// Decoder used by the stats manager when loading persisted usage data
	decodeUsage := func(d json.RawMessage) interface{} {
		u := Usage{}
		_ = json.Unmarshal(d, &u)
		return u
	}

	for _, a := range atos {
		// Always attempt to load usage for every ATO (enabled or not).
		// This ensures /api/atos/{id}/usage can return a consistent shape even
		// before the first ATO run or when the ATO is currently disabled.
		if err := c.statsMgr.Load(a.ID, decodeUsage); err != nil {
			// "not found" is normal for brand new ATOs that have never recorded usage.
			// Anything else is a real error worth logging loudly.
			if strings.Contains(err.Error(), "not found") {
				log.Println("INFO: ato controller: No usage data yet for ATO:", a.ID)
			} else {
				log.Println("ERROR: ato controller: Failed to load usage. Error:", err)
			}
		}

		// Only start the run loop for enabled ATOs
		if !a.Enable {
			continue
		}

		quit := make(chan struct{})
		c.quitters[a.ID] = quit
		go c.Run(a, quit)
	}
}

func (c *Controller) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for id, quit := range c.quitters {
		close(quit)

		// Persist usage history to storage
		if err := c.statsMgr.Save(id); err != nil {
			log.Println("ERROR: ato controller. Failed to save usage. Error:", err)
		}

		log.Println("ato sub-system: Saved usage data of sensor:", id)
		delete(c.quitters, id)
	}
}

func (c *Controller) Control(a ATO, reading int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if a.Pump == "" {
		log.Println("ato-subsystem: control enabled but pump not set. Skipping")
		return nil
	}

	sub, err := c.sub(a)
	if err != nil {
		return err
	}

	// Note: reef-pi semantics here appear inverted (depending on relay logic):
	// reading == 1 -> turn pump ON (false passed to sub.On)
	// reading != 1 -> turn pump OFF (true passed to sub.On)
	switch reading {
	case 1:
		return sub.On(a.Pump, false)
	default:
		return sub.On(a.Pump, true)
	}
}

func (c *Controller) InUse(depType, id string) ([]string, error) {
	var deps []string

	switch depType {
	case storage.EquipmentBucket:
		atos, err := c.List()
		if err != nil {
			return deps, err
		}
		for _, a := range atos {
			if a.Pump == id && !a.IsMacro {
				deps = append(deps, a.Name)
			}
		}
		return deps, nil

	case storage.InletBucket:
		atos, err := c.List()
		if err != nil {
			return deps, err
		}
		for _, a := range atos {
			if a.Inlet == id {
				deps = append(deps, a.Name)
			}
		}
		return deps, nil

	case storage.MacroBucket:
		atos, err := c.List()
		if err != nil {
			return deps, err
		}
		for _, a := range atos {
			if a.IsMacro && a.Pump == id {
				deps = append(deps, a.Name)
			}
		}
		return deps, nil

	default:
		return deps, fmt.Errorf("unknown dependency type:%s", depType)
	}
}

func (c *Controller) GetEntity(id string) (controller.Entity, error) {
	return c.Get(id)
}
