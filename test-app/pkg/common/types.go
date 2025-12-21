// Package common provides shared types and utilities for all test-app components.
package common

import "time"

// ComponentStatus represents the status of a test component.
type ComponentStatus struct {
	Name        string            `json:"name"`
	Ready       bool              `json:"ready"`
	Mode        string            `json:"mode"`
	Config      map[string]any    `json:"config"`
	Metrics     ComponentMetrics  `json:"metrics"`
	LastUpdated time.Time         `json:"lastUpdated"`
}

// ComponentMetrics contains resource usage metrics for a component.
type ComponentMetrics struct {
	CPUUsagePercent   float64 `json:"cpuUsagePercent"`
	MemoryUsageMB     int     `json:"memoryUsageMB"`
	RequestsPerSecond float64 `json:"requestsPerSecond,omitempty"`
	ErrorRate         float64 `json:"errorRate,omitempty"`
}

// TestScenario defines a test scenario configuration.
type TestScenario struct {
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	Duration     time.Duration    `json:"duration"`
	Phases       []ScenarioPhase  `json:"phases"`
	PassCriteria []PassCriterion  `json:"passCriteria"`
}

// ScenarioPhase defines a phase within a test scenario.
type ScenarioPhase struct {
	StartsAt  time.Duration          `json:"startsAt"`
	Component string                 `json:"component"`
	Action    string                 `json:"action"`
	Config    map[string]interface{} `json:"config"`
}

// PassCriterion defines a pass/fail criterion for validation.
type PassCriterion struct {
	Name     string `json:"name"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Pass     bool   `json:"pass"`
}

// MemoryMode represents memory allocation modes.
type MemoryMode string

const (
	MemoryModeSteady MemoryMode = "steady"
	MemoryModeLeak   MemoryMode = "leak"
	MemoryModeSpike  MemoryMode = "spike"
)
