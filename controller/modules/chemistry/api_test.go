package chemistry

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/reef-pi/hal"
	"github.com/reef-pi/reef-pi/controller/device_manager/connectors"
	"github.com/reef-pi/reef-pi/controller/storage"
	"github.com/reef-pi/reef-pi/controller"
	"github.com/reef-pi/reef-pi/controller/utils"
)

const chemistryProbesAPI = "/api/chemistryprobes"

func f64ptr(v float64) *float64 { return &v }

func TestPhAPI(t *testing.T) {
	t.Parallel()
	r, err := controller.TestController()
	if err != nil {
		t.Fatal("Failed to create test controller. Error:", err)
	}
	// chemistry calibration persistence writes into analog_inputs bucket.
	// TestController doesn't create it for this module test, so we create it here.
	if err := r.Store().CreateBucket(storage.AnalogInputBucket); err != nil {
		t.Fatal(err)
	}

	// Create a minimal analog input record that the probe references.
	// (Pin/Driver aren't used in dev mode, but the record must exist to persist config.)
	ai := connectors.AnalogInput{
		ID:     "1",
		Name:   "AI1",
		Pin:    0,
		Driver: "rpi",
	}
	if err := r.Store().Create(storage.AnalogInputBucket, func(id string) interface{} {
		ai.ID = id
		return &ai
	}); err != nil {
		t.Fatal(err)
	}	
	c := New(true, r, nil)
	tr := utils.NewTestRouter()
	if err := c.Setup(); err != nil {
		t.Error(err)
	}
	
	c.LoadAPI(tr.Router)

	body := new(bytes.Buffer)
	enc := json.NewEncoder(body)
	p := &Probe{Name: "Foo", Period: 1, Enable: true, AnalogInput: "1"}
	p.Notify.Enable = true
	enc.Encode(p)
	if err := tr.Do("PUT", chemistryProbesAPI, body, nil); err != nil {
		t.Fatal("Failed to create ph probe using api. Error:", err)
	}

	c.Start()

	body.Reset()
	enc.Encode(p)
	if err := tr.Do("PUT", chemistryProbesAPI, body, nil); err != nil {
		t.Fatal("Failed to create ph probe using api. Error:", err)
	}

	if err := tr.Do("GET", chemistryProbesAPI+"/1", new(bytes.Buffer), nil); err != nil {
		t.Fatal("Failed to get ph probe using api. Error:", err)
	}
	if err := tr.Do("GET", chemistryProbesAPI, new(bytes.Buffer), nil); err != nil {
		t.Fatal("failed to list ph probe using api. error:", err)
	}
	tr.Do("GET", chemistryProbesAPI+"/1/readings", new(bytes.Buffer), nil)
	body.Reset()
	p.Enable = false
	enc.Encode(p)
	if err := tr.Do("POST", chemistryProbesAPI+"/1", body, nil); err != nil {
		t.Fatal("Failed to update ph probe using api. Error:", err)
	}
	p.Enable = true
	if err := c.Update("1", *p); err != nil {
		t.Error(err)
	}
	if err := c.On("1", false); err != nil {
		t.Error(err)
	}
	if err := c.On("1", true); err != nil {
		t.Error(err)
	}
	if err := c.On("-1", false); err == nil {
		t.Error("Enabling invalid probe id should fail")
	}
	p.Enable = false
	p.Control = true
	if err := c.Update("1", *p); err != nil {
		t.Error(err)
	}
	p.loadHomeostasis(r)
	c.checkAndControl(*p)
	ms := []hal.Measurement{
		hal.Measurement{
			Observed: 7.8,
			Expected: 8.1,
		},
	}
	if err := c.Calibrate("1", ms); err != nil {
		t.Error(err)
	}
	cp := CalibrationPoint{
		Observed: f64ptr(7.8),
		Expected: 8.1,
		Type:     "low",
	}
	body.Reset()
	if err := c.CalibratePoint("1", cp); err != nil {
		t.Error(err)
	}
	body.Reset()
	if err := json.NewEncoder(body).Encode(&ms); err != nil {
		t.Error(err)
	}
	if err := tr.Do("POST", chemistryProbesAPI+"/1/calibrate", body, nil); err != nil {
		t.Fatal("Failed to calibrate ph probe using api. Error:", err)
	}
	body.Reset()
	if err := json.NewEncoder(body).Encode(&cp); err != nil {
		t.Error(err)
	}
	if err := tr.Do("POST", chemistryProbesAPI+"/1/calibratepoint", body, nil); err != nil {
		t.Fatal("Failed to calibratepoint ph probe using api. Error:", err)
	}
	if err := tr.Do("GET", chemistryProbesAPI+"/1/read", new(bytes.Buffer), nil); err != nil {
		t.Fatal("Failed to read ph probe using api. Error:", err)
	}

	if err := tr.Do("DELETE", chemistryProbesAPI+"/1", new(bytes.Buffer), nil); err != nil {
		t.Fatal("Failed to delete ph probe using api. Error:", err)
	}
	c.Stop()
}
