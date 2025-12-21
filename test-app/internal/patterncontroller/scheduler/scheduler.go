// Package scheduler provides scenario orchestration and time-based scheduling.
package scheduler

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// ComponentClient defines the interface for communicating with components.
type ComponentClient interface {
	Configure(ctx context.Context, name string, config map[string]interface{}) error
	Start(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
	GetStatus(ctx context.Context, name string) (map[string]interface{}, error)
}

// Scheduler orchestrates test scenarios and manages time-based scheduling.
type Scheduler struct {
	mu              sync.RWMutex
	client          ComponentClient
	timeConfig      *TimeConfig
	currentScenario *Scenario
	status          ScenarioStatus
	ctx             context.Context
	cancel          context.CancelFunc
	events          []ScenarioEvent
	onEvent         func(event ScenarioEvent)
}

// ScenarioEvent represents an event during scenario execution.
type ScenarioEvent struct {
	Timestamp     time.Time     `json:"timestamp"`
	SimulatedTime time.Duration `json:"simulatedTime"`
	Type          string        `json:"type"`
	Component     string        `json:"component,omitempty"`
	Action        string        `json:"action,omitempty"`
	Message       string        `json:"message"`
	Error         string        `json:"error,omitempty"`
}

// New creates a new Scheduler.
func New(client ComponentClient) *Scheduler {
	return &Scheduler{
		client:     client,
		timeConfig: NewTimeConfig(),
		status: ScenarioStatus{
			Running: false,
		},
		events: make([]ScenarioEvent, 0),
	}
}

// SetOnEvent sets a callback for scenario events.
func (s *Scheduler) SetOnEvent(fn func(event ScenarioEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onEvent = fn
}

// emitEvent records and emits a scenario event.
func (s *Scheduler) emitEvent(eventType, component, action, message, errMsg string) {
	event := ScenarioEvent{
		Timestamp:     time.Now(),
		SimulatedTime: s.timeConfig.SimulatedElapsed(),
		Type:          eventType,
		Component:     component,
		Action:        action,
		Message:       message,
		Error:         errMsg,
	}
	s.events = append(s.events, event)
	if s.onEvent != nil {
		s.onEvent(event)
	}
	if errMsg != "" {
		log.Printf("[Scheduler] %s: %s - %s (error: %s)", eventType, component, message, errMsg)
	} else {
		log.Printf("[Scheduler] %s: %s - %s", eventType, component, message)
	}
}

// StartScenario starts a predefined scenario.
func (s *Scheduler) StartScenario(ctx context.Context, name ScenarioName) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status.Running {
		return fmt.Errorf("scenario %s is already running", s.status.Name)
	}

	scenario, ok := GetScenario(name)
	if !ok {
		return fmt.Errorf("unknown scenario: %s", name)
	}

	return s.startScenarioLocked(ctx, &scenario)
}

// StartCustomScenario starts a custom scenario.
func (s *Scheduler) StartCustomScenario(ctx context.Context, scenario *Scenario) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status.Running {
		return fmt.Errorf("scenario %s is already running", s.status.Name)
	}

	return s.startScenarioLocked(ctx, scenario)
}

func (s *Scheduler) startScenarioLocked(ctx context.Context, scenario *Scenario) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.currentScenario = scenario
	s.events = make([]ScenarioEvent, 0)
	s.timeConfig.Reset()

	s.status = ScenarioStatus{
		Name:        scenario.Name,
		Running:     true,
		StartedAt:   time.Now(),
		TotalPhases: len(scenario.Phases),
	}

	s.emitEvent("scenario_start", "", "", fmt.Sprintf("Starting scenario: %s", scenario.Name), "")

	// Configure initial components
	for _, comp := range scenario.Components {
		if err := s.configureComponent(s.ctx, comp); err != nil {
			s.emitEvent("component_error", comp.Name, "configure", "Failed to configure component", err.Error())
			// Continue with other components
		}
	}

	// Start phase scheduler
	go s.runPhases()

	return nil
}

// configureComponent configures a component with the given config.
func (s *Scheduler) configureComponent(ctx context.Context, comp ComponentConfig) error {
	config := make(map[string]interface{})
	config["mode"] = comp.Mode
	for k, v := range comp.Config {
		config[k] = v
	}

	s.emitEvent("component_configure", comp.Name, "configure",
		fmt.Sprintf("Configuring %s with mode=%s", comp.Name, comp.Mode), "")

	return s.client.Configure(ctx, comp.Name, config)
}

// runPhases executes scenario phases based on simulated time.
func (s *Scheduler) runPhases() {
	if s.currentScenario == nil || len(s.currentScenario.Phases) == 0 {
		s.waitForCompletion()
		return
	}

	// Sort phases by start time
	phases := make([]Phase, len(s.currentScenario.Phases))
	copy(phases, s.currentScenario.Phases)
	sort.Slice(phases, func(i, j int) bool {
		return phases[i].StartsAt < phases[j].StartsAt
	})

	phaseIndex := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.completeScenario("cancelled")
			return
		case <-ticker.C:
			simElapsed := s.timeConfig.SimulatedElapsed()

			// Update status
			s.mu.Lock()
			s.status.ElapsedTime = time.Since(s.status.StartedAt)
			s.status.SimulatedTime = simElapsed
			if s.currentScenario.Duration > 0 {
				s.status.Progress = float64(simElapsed) / float64(s.currentScenario.Duration) * 100
				if s.status.Progress > 100 {
					s.status.Progress = 100
				}
			}
			s.mu.Unlock()

			// Execute phases that should start
			for phaseIndex < len(phases) && simElapsed >= phases[phaseIndex].StartsAt {
				phase := phases[phaseIndex]
				s.executePhase(phase)
				s.mu.Lock()
				s.status.CurrentPhase = phaseIndex + 1
				s.mu.Unlock()
				phaseIndex++
			}

			// Check if scenario is complete
			if simElapsed >= s.currentScenario.Duration {
				s.completeScenario("completed")
				return
			}
		}
	}
}

// waitForCompletion waits for scenario duration without phases.
func (s *Scheduler) waitForCompletion() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.completeScenario("cancelled")
			return
		case <-ticker.C:
			simElapsed := s.timeConfig.SimulatedElapsed()

			s.mu.Lock()
			s.status.ElapsedTime = time.Since(s.status.StartedAt)
			s.status.SimulatedTime = simElapsed
			if s.currentScenario.Duration > 0 {
				s.status.Progress = float64(simElapsed) / float64(s.currentScenario.Duration) * 100
			}
			s.mu.Unlock()

			if simElapsed >= s.currentScenario.Duration {
				s.completeScenario("completed")
				return
			}
		}
	}
}

// executePhase executes a single phase.
func (s *Scheduler) executePhase(phase Phase) {
	s.emitEvent("phase_start", phase.Component, phase.Action,
		fmt.Sprintf("Executing phase at %v", phase.StartsAt), "")

	var err error
	switch phase.Action {
	case "configure":
		err = s.configureComponent(s.ctx, phase.Config)
	case "start":
		err = s.client.Start(s.ctx, phase.Component)
	case "stop":
		err = s.client.Stop(s.ctx, phase.Component)
	default:
		err = fmt.Errorf("unknown action: %s", phase.Action)
	}

	if err != nil {
		s.emitEvent("phase_error", phase.Component, phase.Action, "Phase execution failed", err.Error())
	} else {
		s.emitEvent("phase_complete", phase.Component, phase.Action, "Phase completed", "")
	}
}

// completeScenario marks the scenario as complete.
func (s *Scheduler) completeScenario(reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.status.Running = false
	s.status.Progress = 100
	s.emitEvent("scenario_complete", "", "", fmt.Sprintf("Scenario %s: %s", s.status.Name, reason), "")
}

// StopScenario stops the currently running scenario.
func (s *Scheduler) StopScenario() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.status.Running {
		return fmt.Errorf("no scenario is running")
	}

	if s.cancel != nil {
		s.cancel()
	}

	return nil
}

// GetStatus returns the current scenario status.
func (s *Scheduler) GetStatus() ScenarioStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// GetEvents returns all scenario events.
func (s *Scheduler) GetEvents() []ScenarioEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := make([]ScenarioEvent, len(s.events))
	copy(events, s.events)
	return events
}

// GetTimeConfig returns the time configuration.
func (s *Scheduler) GetTimeConfig() *TimeConfig {
	return s.timeConfig
}

// SetTimeAcceleration sets the time acceleration factor.
func (s *Scheduler) SetTimeAcceleration(factor float64) error {
	if factor <= 0 {
		return fmt.Errorf("acceleration factor must be positive")
	}
	if factor > 100 {
		return fmt.Errorf("acceleration factor cannot exceed 100")
	}
	s.timeConfig.SetAcceleration(factor)
	s.emitEvent("time_acceleration", "", "", fmt.Sprintf("Time acceleration set to %.1fx", factor), "")
	return nil
}

// GetSimulatedTime returns the current simulated time info.
func (s *Scheduler) GetSimulatedTime() map[string]interface{} {
	return map[string]interface{}{
		"realTime":           time.Now(),
		"simulatedTime":      s.timeConfig.SimulatedNow(),
		"simulatedElapsed":   s.timeConfig.SimulatedElapsed().String(),
		"accelerationFactor": s.timeConfig.GetAcceleration(),
	}
}
