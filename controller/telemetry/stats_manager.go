package telemetry

import (
	"container/ring"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/reef-pi/reef-pi/controller/storage"
)

type Metric interface {
	Rollup(Metric) (Metric, bool)
	Before(Metric) bool
}

// swagger:model statsResponse
type StatsResponse struct {
	Current    []Metric `json:"current"`
	Historical []Metric `json:"historical"`
}

type Stats struct {
	Current    *ring.Ring
	Historical *ring.Ring
}

// On-disk telemetry schema (arrays). NOTE: you already saw some old data is not arrays.
type StatsOnDisk struct {
	Current    []json.RawMessage `json:"current"`
	Historical []json.RawMessage `json:"historical"`
}

type StatsManager interface {
	Get(string) (StatsResponse, error)
	IsLoaded(string) bool
	Initialize(string) error
	Load(string, func(json.RawMessage) interface{}) error
	Save(string) error
	Update(string, Metric)
	Delete(string) error
}

type mgr struct {
	mu sync.Mutex

	inMemory        map[string]Stats
	bucket          string
	CurrentLimit    int
	HistoricalLimit int
	store           storage.Store

	// Debug / instrumentation
	debug        bool
	slowLockWarn time.Duration
	slowHoldWarn time.Duration

	// Breadcrumb (best-effort): last lock owner info
	lastOp string
	lastID string
	lastAt time.Time
}

var statsBannerOnce sync.Once

func envBool(name string, def bool) bool {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "TRUE", "yes", "YES":
		return true
	case "0", "false", "FALSE", "no", "NO":
		return false
	default:
		return def
	}
}

func envMS(name string, defMS int) time.Duration {
	v := os.Getenv(name)
	if v == "" {
		return time.Duration(defMS) * time.Millisecond
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return time.Duration(defMS) * time.Millisecond
	}
	if i < 0 {
		i = 0
	}
	return time.Duration(i) * time.Millisecond
}

// IMPORTANT: make sure the codebase uses this constructor.
// If your code instantiates mgr directly somewhere, you must fix that call site.
func NewStatsManager(store storage.Store, bucket string, currentLimit, historicalLimit int) StatsManager {
	m := &mgr{
		inMemory:        make(map[string]Stats),
		bucket:          bucket,
		CurrentLimit:    currentLimit,
		HistoricalLimit: historicalLimit,
		store:           store,

		debug:        envBool("REEFPI_TELEMETRY_DEBUG", false),
		slowLockWarn: envMS("REEFPI_TELEMETRY_SLOW_LOCK_MS", 250),
		slowHoldWarn: envMS("REEFPI_TELEMETRY_SLOW_HOLD_MS", 250),
	}

	// Print once per process (not once per StatsManager instance)
	statsBannerOnce.Do(func() {
		log.Printf("telemetry.stats: ENABLED debug=%v slowLockWarn=%s slowHoldWarn=%s (env debug=%q lock_ms=%q hold_ms=%q)",
			m.debug, m.slowLockWarn, m.slowHoldWarn,
			os.Getenv("REEFPI_TELEMETRY_DEBUG"),
			os.Getenv("REEFPI_TELEMETRY_SLOW_LOCK_MS"),
			os.Getenv("REEFPI_TELEMETRY_SLOW_HOLD_MS"),
		)
	})

	return m
}

// debug-only helper
func (m *mgr) dbg(format string, args ...any) {
	if !m.debug {
		return
	}
	log.Printf("telemetry.stats: "+format, args...)
}

func (m *mgr) lockWithTiming(op, id string) (unlock func()) {
	startWait := time.Now()
	m.mu.Lock()
	waited := time.Since(startWait)

	// Capture previous breadcrumb BEFORE overwrite (so it’s meaningful)
	prevOp, prevID, prevAt := m.lastOp, m.lastID, m.lastAt
	m.lastOp, m.lastID, m.lastAt = op, id, time.Now()

	// FIX: slow lock logging is UNCONDITIONAL (not gated by debug)
	if m.slowLockWarn > 0 && waited > m.slowLockWarn {
		log.Printf("telemetry.stats: %s id=%s waited=%s to acquire lock (prev=%s/%s at %s)",
			op, id, waited, prevOp, prevID, prevAt.Format(time.RFC3339Nano))
	}

	startHold := time.Now()
	return func() {
		held := time.Since(startHold)

		// FIX: slow hold logging is UNCONDITIONAL (not gated by debug)
		if m.slowHoldWarn > 0 && held > m.slowHoldWarn {
			log.Printf("telemetry.stats: %s id=%s held lock=%s", op, id, held)
		}

		m.mu.Unlock()
	}
}

func (m *mgr) IsLoaded(id string) bool {
	unlock := m.lockWithTiming("IsLoaded", id)
	defer unlock()
	_, ok := m.inMemory[id]
	return ok
}

func (m *mgr) NewStats() Stats {
	return Stats{
		Current:    ring.New(m.CurrentLimit),
		Historical: ring.New(m.HistoricalLimit),
	}
}

// snapshotLocked builds a response WITHOUT re-locking (caller must hold lock).
func snapshotLocked(stats Stats) StatsResponse {
	resp := StatsResponse{
		Current:    []Metric{},
		Historical: []Metric{},
	}

	stats.Current.Do(func(i interface{}) {
		if i != nil {
			resp.Current = append(resp.Current, i.(Metric))
		}
	})
	sort.Slice(resp.Current, func(i, j int) bool {
		return resp.Current[i].Before(resp.Current[j])
	})

	stats.Historical.Do(func(i interface{}) {
		if i != nil {
			resp.Historical = append(resp.Historical, i.(Metric))
		}
	})
	sort.Slice(resp.Historical, func(i, j int) bool {
		return resp.Historical[i].Before(resp.Historical[j])
	})

	return resp
}

func (m *mgr) Get(id string) (StatsResponse, error) {
	unlock := m.lockWithTiming("Get", id)
	defer unlock()

	stats, ok := m.inMemory[id]
	if !ok {
		return StatsResponse{Current: []Metric{}, Historical: []Metric{}},
			fmt.Errorf("stats for id: '%s' not found", id)
	}
	return snapshotLocked(stats), nil
}

// Load loads stats from disk (outside lock), then installs in memory (inside lock).
func (m *mgr) Load(id string, fn func(json.RawMessage) interface{}) error {
	var resp StatsOnDisk
	if err := m.store.Get(m.bucket, id, &resp); err != nil {
		return err
	}

	stats := m.NewStats()
	for _, c := range resp.Current {
		stats.Current.Value = fn(c)
		stats.Current = stats.Current.Next()
	}
	stats.Current = stats.Current.Prev()

	for _, h := range resp.Historical {
		stats.Historical.Value = fn(h)
		stats.Historical = stats.Historical.Next()
	}
	stats.Historical = stats.Historical.Prev()

	unlock := m.lockWithTiming("Load", id)
	m.inMemory[id] = stats
	unlock()
	return nil
}

// Save persists current snapshot to disk.
// NOTE: no lock held across store.Update
func (m *mgr) Save(id string) error {
	// Lock only to snapshot
	unlock := m.lockWithTiming("Save-snapshot", id)
	stats, ok := m.inMemory[id]
	var snap StatsResponse
	if ok {
		snap = snapshotLocked(stats)
	}
	unlock()

	if !ok {
		return fmt.Errorf("stats for id: '%s' not found", id)
	}

	if m.debug {
		m.dbg("Save id=%s calling store.Update (no lock)", id)
	}
	t0 := time.Now()
	err := m.store.Update(m.bucket, id, snap)
	if m.debug {
		m.dbg("Save id=%s store.Update done in %s err=%v", id, time.Since(t0), err)
	}
	return err
}

// Update updates ring buffers.
// If a rollup move happens, we snapshot under lock then persist after unlock.
// This removes disk IO under mutex and avoids self-deadlock patterns.
func (m *mgr) Update(id string, metric Metric) {
	var (
		needSave bool
		snap     StatsResponse
	)

	unlock := m.lockWithTiming("Update", id)

	stats, ok := m.inMemory[id]
	if !ok {
		stats = m.NewStats()
		stats.Historical.Value = metric
		stats.Current.Value = metric
		stats.Current = stats.Current.Next()
		m.inMemory[id] = stats
		unlock()
		return
	}

	stats.Current.Value = metric
	stats.Current = stats.Current.Next()

	if stats.Historical.Value == nil {
		stats.Historical.Value = metric
	} else {
		m1, moved := stats.Historical.Value.(Metric).Rollup(metric)
		if moved {
			stats.Historical = stats.Historical.Next()
			needSave = true
		}
		stats.Historical.Value = m1
	}

	m.inMemory[id] = stats

	if needSave {
		// snapshot while locked, then persist after unlock
		snap = snapshotLocked(stats)
	}
	unlock()

	if needSave {
		if m.debug {
			m.dbg("Update id=%s rollup moved -> store.Update (no lock)", id)
		}
		t0 := time.Now()
		err := m.store.Update(m.bucket, id, snap)
		if m.debug {
			m.dbg("Update id=%s store.Update done in %s err=%v", id, time.Since(t0), err)
		}
	}
}

func (m *mgr) Initialize(id string) error {
	unlock := m.lockWithTiming("Initialize", id)
	m.inMemory[id] = m.NewStats()
	unlock()
	return nil
}

func (m *mgr) Delete(id string) error {
	// remove from memory under lock
	unlock := m.lockWithTiming("Delete", id)
	delete(m.inMemory, id)
	unlock()

	// do disk delete without holding mutex
	return m.store.Delete(m.bucket, id)
}
