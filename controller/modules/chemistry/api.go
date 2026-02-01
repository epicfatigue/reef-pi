package chemistry

import (
	"net/http"

	"github.com/reef-pi/hal"

	"github.com/gorilla/mux"

	"github.com/reef-pi/reef-pi/controller/utils"
)

func (e *Controller) LoadAPI(r *mux.Router) {

	// -------------------------------------------------------------------------
	// Legacy endpoints (keep for backward compatibility): /api/phprobes...
	// -------------------------------------------------------------------------

	// swagger:operation GET /api/phprobes/{id} PhProbes phProbeGet
	// Get an ph probe by id.
	// Get an existing ph probe by id.
	// ---
	// parameters:
	//  - in: path
	//    name: id
	//    description: The Id of the ph probe
	//    required: true
	//    schema:
	//     type: integer
	// responses:
	//  200:
	//   description: OK
	//   schema:
	//    $ref: '#/definitions/phProbe'
	//  404:
	//   description: Not Found
	r.HandleFunc("/api/phprobes/{id}", e.getProbe).Methods("GET")

	// swagger:route GET /api/phprobes PhProbes phProbeList
	// List all ph probes.
	// List all ph probes in reef-pi.
	// responses:
	// 	200: body:[]phProbe
	r.HandleFunc("/api/phprobes", e.listProbes).Methods("GET")

	// swagger:operation PUT /api/phprobes PhProbes phProbeCreate
	// Create a ph probe.
	// Create a new ph probe.
	// ---
	// parameters:
	//  - in: body
	//    name: phProbe
	//    description: The ph probe to create
	//    required: true
	//    schema:
	//     $ref: '#/definitions/phProbe'
	// responses:
	//  200:
	//   description: OK
	r.HandleFunc("/api/phprobes", e.createProbe).Methods("PUT")

	// swagger:operation POST /api/phprobes/{id} PhProbes phProbeUpdate
	// Update a ph probe.
	// Update an existing ph probe.
	//---
	//parameters:
	// - in: path
	//   name: id
	//   description: The Id of the ph probe to update
	//   required: true
	//   schema:
	//    type: integer
	// - in: body
	//   name: phProbe
	//   description: The ph probe to update
	//   required: true
	//   schema:
	//    $ref: '#/definitions/phProbe'
	//responses:
	// 200:
	//  description: OK
	// 404:
	//  description: Not Found
	r.HandleFunc("/api/phprobes/{id}", e.updateProbe).Methods("POST")

	// swagger:operation DELETE /api/phprobes/{id} PhProbes phProbeDelete
	// Delete a ph probe.
	// Delete an existing ph probe.
	// ---
	// parameters:
	//  - in: path
	//    name: id
	//    description: The Id of the ph probe to delete
	//    required: true
	//    schema:
	//     type: integer
	// responses:
	//  200:
	//   description: OK
	r.HandleFunc("/api/phprobes/{id}", e.deleteProbe).Methods("DELETE")

	// swagger:operation GET /api/phprobes/{id}/readings PhProbes phProbeReadings
	// Get ph probe readings.
	// Get ph probe readings.
	// ---
	// parameters:
	//  - in: path
	//    name: id
	//    description: The Id of the ph probe to delete
	//    required: true
	//    schema:
	//     type: integer
	// responses:
	//  200:
	//   description: OK
	//   schema:
	//    $ref: '#/definitions/statsResponse'
	r.HandleFunc("/api/phprobes/{id}/readings", e.getReadings).Methods("GET")
	
	// swagger:operation POST /api/phprobes/{id}/readings/clear PhProbes phProbeReadingsClear
	// Clear ph probe readings (usage history).
	// Deletes stored readings so graphs and history start fresh.
	// Legacy alias of /api/chemistryprobes/{id}/readings/clear.
	// ---
	// parameters:
	//  - in: path
	//    name: id
	//    description: The Id of the ph probe
	//    required: true
	//    schema:
	//     type: integer
	// responses:
	//  200:
	//   description: OK
	r.HandleFunc("/api/phprobes/{id}/readings/clear", e.clearReadings).Methods("POST")

	// swagger:operation POST /api/chemistryprobes/{id}/readings/clear ChemistryProbes chemistryProbeReadingsClear
	// Clear chemistry probe readings (usage history).
	// Deletes stored readings so graphs and history start fresh.
	// ---
	// parameters:
	//  - in: path
	//    name: id
	//    description: The Id of the chemistry probe
	//    required: true
	//    schema:
	//     type: integer
	// responses:
	//  200:
	//   description: OK
	r.HandleFunc("/api/chemistryprobes/{id}/readings/clear", e.clearReadings).Methods("POST")


	// swagger:operation POST /api/phprobes/{id}/calibrate PhProbes phProbeCalibrate
	// Calibrate a ph probe.
	// Set calibration points for one or two point calibration
	// ---
	// parameters:
	//  - in: path
	//    name: id
	//    description: The Id of the ph probe to calibrate
	//    required: true
	//    schema:
	//     type: integer
	//  - in: body
	//    name: measurements
	//    description: The calibration measurements
	//    required: true
	//    schema:
	//     type: array
	//     items:
	//      type: object
	//      properties:
	//       expected:
	//        type: float64
	//        format: float
	//        description: The expected value
	//       observed:
	//        type: float64
	//        format: float
	//        description: The actual value observed
	// responses:
	//  200:
	//   description: OK
	r.HandleFunc("/api/phprobes/{id}/calibrate", e.calibrate).Methods("POST")

	// swagger:operation GET /api/phprobes/{id}/read PhProbes phProbeRead
	// Get ph probe reading.
	// Get ph probe reading.
	// ---
	// parameters:
	//  - in: path
	//    name: id
	//    description: The Id of the ph probe to read
	//    required: true
	//    schema:
	//     type: integer
	// responses:
	//  200:
	//   description: OK
	r.HandleFunc("/api/phprobes/{id}/read", e.read).Methods("GET")

	// swagger:operation POST /api/phprobes/{id}/calibratepoint PhProbes phProbeCalibrateSingle
	// Calibrate a ph probe.
	// Set a calibration point for one or two point calibration
	// ---
	// parameters:
	//  - in: path
	//    name: id
	//    description: The Id of the ph probe to calibrate
	//    required: true
	//    schema:
	//     type: integer
	//  - in: body
	//    name: measurements
	//    description: The calibration measurement
	//    required: true
	//    schema:
	//     $ref: '#/definitions/calibrationPoint'
	// responses:
	//  200:
	//   description: OK
	r.HandleFunc("/api/phprobes/{id}/calibratepoint", e.calibratePoint).Methods("POST")

	// -------------------------------------------------------------------------
	// New endpoints (preferred): /api/chemistryprobes...
	// NOTE: These are aliases of the legacy handlers for clean naming.
	// -------------------------------------------------------------------------

	// swagger:operation GET /api/chemistryprobes/{id} ChemistryProbes chemistryProbeGet
	// Get a chemistry probe by id.
	// Get an existing chemistry probe by id.
	// ---
	// parameters:
	//  - in: path
	//    name: id
	//    description: The Id of the chemistry probe
	//    required: true
	//    schema:
	//     type: integer
	// responses:
	//  200:
	//   description: OK
	//   schema:
	//    $ref: '#/definitions/phProbe'
	//  404:
	//   description: Not Found
	r.HandleFunc("/api/chemistryprobes/{id}", e.getProbe).Methods("GET")
		
	// swagger:operation GET /api/phprobes/{id}/snapshot PhProbes phProbeSnapshot
	// Get a ph probe snapshot (raw + derived + rolling averages).
	// Legacy alias of /api/chemistryprobes/{id}/snapshot.
	// ---
	// parameters:
	//  - in: path
	//    name: id
	//    description: The Id of the ph probe
	//    required: true
	//    schema:
	//     type: integer
	// responses:
	//  200:
	//   description: OK
	//   schema:
	//    $ref: '#/definitions/snapshotResponse'
	//  404:
	//   description: Not Found
	r.HandleFunc("/api/phprobes/{id}/snapshot", e.snapshot).Methods("GET")

	// swagger:operation GET /api/chemistryprobes/{id}/snapshot ChemistryProbes chemistryProbeSnapshot
	// Get a chemistry probe snapshot (raw + derived + rolling averages).
	// Returns a structured snapshot for calibration/debug UI: current value,
	// plus raw/intermediate signals (driver-defined), each with now + optional 1m/5m averages.
	// ---
	// parameters:
	//  - in: path
	//    name: id
	//    description: The Id of the chemistry probe
	//    required: true
	//    schema:
	//     type: integer
	// responses:
	//  200:
	//   description: OK
	//   schema:
	//    $ref: '#/definitions/snapshotResponse'
	//  404:
	//   description: Not Found
	r.HandleFunc("/api/chemistryprobes/{id}/snapshot", e.snapshot).Methods("GET")

	// swagger:route GET /api/chemistryprobes ChemistryProbes chemistryProbeList
	// List all chemistry probes.
	// List all chemistry probes in reef-pi.
	// responses:
	// 	200: body:[]phProbe
	r.HandleFunc("/api/chemistryprobes", e.listProbes).Methods("GET")

	// swagger:operation PUT /api/chemistryprobes ChemistryProbes chemistryProbeCreate
	// Create a chemistry probe.
	// Create a new chemistry probe.
	// ---
	// parameters:
	//  - in: body
	//    name: phProbe
	//    description: The chemistry probe to create
	//    required: true
	//    schema:
	//     $ref: '#/definitions/phProbe'
	// responses:
	//  200:
	//   description: OK
	r.HandleFunc("/api/chemistryprobes", e.createProbe).Methods("PUT")

	// swagger:operation POST /api/chemistryprobes/{id} ChemistryProbes chemistryProbeUpdate
	// Update a chemistry probe.
	// Update an existing chemistry probe.
	//---
	//parameters:
	// - in: path
	//   name: id
	//   description: The Id of the chemistry probe to update
	//   required: true
	//   schema:
	//    type: integer
	// - in: body
	//   name: phProbe
	//   description: The chemistry probe to update
	//   required: true
	//   schema:
	//    $ref: '#/definitions/phProbe'
	//responses:
	// 200:
	//  description: OK
	// 404:
	//  description: Not Found
	r.HandleFunc("/api/chemistryprobes/{id}", e.updateProbe).Methods("POST")

	// swagger:operation DELETE /api/chemistryprobes/{id} ChemistryProbes chemistryProbeDelete
	// Delete a chemistry probe.
	// Delete an existing chemistry probe.
	// ---
	// parameters:
	//  - in: path
	//    name: id
	//    description: The Id of the chemistry probe to delete
	//    required: true
	//    schema:
	//     type: integer
	// responses:
	//  200:
	//   description: OK
	r.HandleFunc("/api/chemistryprobes/{id}", e.deleteProbe).Methods("DELETE")

	// swagger:operation GET /api/chemistryprobes/{id}/readings ChemistryProbes chemistryProbeReadings
	// Get chemistry probe readings.
	// Get chemistry probe readings.
	// ---
	// parameters:
	//  - in: path
	//    name: id
	//    description: The Id of the chemistry probe to delete
	//    required: true
	//    schema:
	//     type: integer
	// responses:
	//  200:
	//   description: OK
	//   schema:
	//    $ref: '#/definitions/statsResponse'
	r.HandleFunc("/api/chemistryprobes/{id}/readings", e.getReadings).Methods("GET")

	// swagger:operation POST /api/chemistryprobes/{id}/calibrate ChemistryProbes chemistryProbeCalibrate
	// Calibrate a chemistry probe.
	// Set calibration points for one or two point calibration
	// ---
	// parameters:
	//  - in: path
	//    name: id
	//    description: The Id of the chemistry probe to calibrate
	//    required: true
	//    schema:
	//     type: integer
	//  - in: body
	//    name: measurements
	//    description: The calibration measurements
	//    required: true
	//    schema:
	//     type: array
	//     items:
	//      type: object
	//      properties:
	//       expected:
	//        type: float64
	//        format: float
	//        description: The expected value
	//       observed:
	//        type: float64
	//        format: float
	//        description: The actual value observed
	// responses:
	//  200:
	//   description: OK
	r.HandleFunc("/api/chemistryprobes/{id}/calibrate", e.calibrate).Methods("POST")

	// swagger:operation GET /api/chemistryprobes/{id}/read ChemistryProbes chemistryProbeRead
	// Get chemistry probe reading.
	// Get chemistry probe reading.
	// ---
	// parameters:
	//  - in: path
	//    name: id
	//    description: The Id of the chemistry probe to read
	//    required: true
	//    schema:
	//     type: integer
	// responses:
	//  200:
	//   description: OK
	r.HandleFunc("/api/chemistryprobes/{id}/read", e.read).Methods("GET")

	// swagger:operation POST /api/chemistryprobes/{id}/calibratepoint ChemistryProbes chemistryProbeCalibrateSingle
	// Calibrate a chemistry probe.
	// Set a calibration point for one or two point calibration
	// ---
	// parameters:
	//  - in: path
	//    name: id
	//    description: The Id of the chemistry probe to calibrate
	//    required: true
	//    schema:
	//     type: integer
	//  - in: body
	//    name: measurements
	//    description: The calibration measurement
	//    required: true
	//    schema:
	//     $ref: '#/definitions/calibrationPoint'
	// responses:
	//  200:
	//   description: OK
	r.HandleFunc("/api/chemistryprobes/{id}/calibratepoint", e.calibratePoint).Methods("POST")
}

func (c *Controller) calibrate(w http.ResponseWriter, r *http.Request) {
	var ms []hal.Measurement
	fn := func(id string) error {
		return c.Calibrate(id, ms)
	}
	utils.JSONUpdateResponse(&ms, fn, w, r)
}

func (c *Controller) calibratePoint(w http.ResponseWriter, r *http.Request) {
	var calibrationPoint CalibrationPoint
	fn := func(id string) error {
		return c.CalibratePoint(id, calibrationPoint)
	}
	utils.JSONUpdateResponse(&calibrationPoint, fn, w, r)
}

func (c *Controller) getProbe(w http.ResponseWriter, r *http.Request) {
	fn := func(id string) (interface{}, error) {
		return c.Get(id)
	}
	utils.JSONGetResponse(fn, w, r)
}

func (c *Controller) getReadings(w http.ResponseWriter, r *http.Request) {
	fn := func(id string) (interface{}, error) { return c.statsMgr.Get(id) }
	utils.JSONGetResponse(fn, w, r)
}

func (c *Controller) clearReadings(w http.ResponseWriter, r *http.Request) {
	fn := func(id string) (interface{}, error) {
		// Delete the stored stats series for this probe id.
		if err := c.statsMgr.Delete(id); err != nil {
			return nil, err
		}

		// Re-create in-memory entry so next Update/Save behaves predictably.
		// (Optional, but nice.)
		c.statsMgr.Initialize(id)

		return map[string]string{"status": "ok"}, nil
	}
	utils.JSONGetResponse(fn, w, r)
}

func (c *Controller) read(w http.ResponseWriter, r *http.Request) {
	fn := func(id string) (interface{}, error) {
		probe, err := c.Get(id)
		if err != nil {
			return nil, err
		}
		return c.Read(probe)
	}
	utils.JSONGetResponse(fn, w, r)
}

func (c *Controller) listProbes(w http.ResponseWriter, r *http.Request) {
	fn := func() (interface{}, error) {
		return c.List()
	}
	utils.JSONListResponse(fn, w, r)
}

func (c *Controller) createProbe(w http.ResponseWriter, r *http.Request) {
	var p Probe
	fn := func() error {
		return c.Create(p)
	}
	utils.JSONCreateResponse(&p, fn, w, r)
}

func (c *Controller) updateProbe(w http.ResponseWriter, r *http.Request) {
	var p Probe
	fn := func(id string) error {
		return c.Update(id, p)
	}
	utils.JSONUpdateResponse(&p, fn, w, r)
}

func (c *Controller) deleteProbe(w http.ResponseWriter, r *http.Request) {
	fn := func(id string) error {
		return c.Delete(id)
	}
	utils.JSONDeleteResponse(fn, w, r)
}


func (c *Controller) snapshot(w http.ResponseWriter, r *http.Request) {
	fn := func(id string) (interface{}, error) {
		probe, err := c.Get(id)
		if err != nil {
			return nil, err
		}
		return c.ReadSnapshot(probe)
	}
	utils.JSONGetResponse(fn, w, r)
}