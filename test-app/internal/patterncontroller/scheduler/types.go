// Package scheduler provides time-based scheduling and scenario orchestration.
package scheduler

import (
	"sync"
	"time"
)

// ScenarioName represents predefined scenario names.
type ScenarioName string

const (
	ScenarioBaseline       ScenarioName = "baseline"
	ScenarioStress         ScenarioName = "stress"
	ScenarioAnomaly        ScenarioName = "anomaly"
	ScenarioFullValidation ScenarioName = "full-validation"
)

// ComponentConfig defines configuration for a component within a scenario.
type ComponentConfig struct {
	Name   string                 `json:"name"`
	Mode   string                 `json:"mode"`
	Config map[string]interface{} `json:"config"`
}

// Phase defines a phase within a scenario.
type Phase struct {
	StartsAt  time.Duration   `json:"startsAt"`
	Component string          `json:"component"`
	Action    string          `json:"action"`
	Config    ComponentConfig `json:"config"`
}

// Scenario defines a test scenario.
type Scenario struct {
	Name        ScenarioName      `json:"name"`
	Description string            `json:"description"`
	Duration    time.Duration     `json:"duration"`
	Components  []ComponentConfig `json:"components"`
	Phases      []Phase           `json:"phases"`
}

// ScenarioStatus represents the current status of a running scenario.
type ScenarioStatus struct {
	Name          ScenarioName  `json:"name"`
	Running       bool          `json:"running"`
	StartedAt     time.Time     `json:"startedAt,omitempty"`
	ElapsedTime   time.Duration `json:"elapsedTime"`
	SimulatedTime time.Duration `json:"simulatedTime"`
	CurrentPhase  int           `json:"currentPhase"`
	TotalPhases   int           `json:"totalPhases"`
	Progress      float64       `json:"progress"`
}

// TimeConfig holds time acceleration settings.
type TimeConfig struct {
	mu                sync.RWMutex
	accelerationFactor float64
	startTime         time.Time
	simulatedStart    time.Time
}

// NewTimeConfig creates a new TimeConfig with default settings.
func NewTimeConfig() *TimeConfig {
	return &TimeConfig{
		accelerationFactor: 1.0,
		startTime:         time.Now(),
		simulatedStart:    time.Now(),
	}
}

// SetAcceleration sets the time acceleration factor.
func (tc *TimeConfig) SetAcceleration(factor float64) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.accelerationFactor = factor
	tc.startTime = time.Now()
	tc.simulatedStart = time.Now()
}

// GetAcceleration returns the current acceleration factor.
func (tc *TimeConfig) GetAcceleration() float64 {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.accelerationFactor
}

// SimulatedNow returns the current simulated time.
func (tc *TimeConfig) SimulatedNow() time.Time {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	elapsed := time.Since(tc.startTime)
	acceleratedElapsed := time.Duration(float64(elapsed) * tc.accelerationFactor)
	return tc.simulatedStart.Add(acceleratedElapsed)
}

// SimulatedElapsed returns the simulated elapsed time since start.
func (tc *TimeConfig) SimulatedElapsed() time.Duration {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	elapsed := time.Since(tc.startTime)
	return time.Duration(float64(elapsed) * tc.accelerationFactor)
}

// Reset resets the time configuration.
func (tc *TimeConfig) Reset() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.startTime = time.Now()
	tc.simulatedStart = time.Now()
}
