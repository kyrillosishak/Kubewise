// Package storage provides storage for validation history.
package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/container-resource-predictor/test-app/internal/metricsvalidator/reporter"
	"github.com/container-resource-predictor/test-app/internal/metricsvalidator/validator"
)

// ValidationHistory stores validation results for trend analysis.
type ValidationHistory struct {
	mu           sync.RWMutex
	dataDir      string
	results      []validator.ValidationResult
	reports      []reporter.ValidationReport
	maxResults   int
	maxReports   int
}

// NewValidationHistory creates a new ValidationHistory.
func NewValidationHistory(dataDir string) *ValidationHistory {
	return &ValidationHistory{
		dataDir:    dataDir,
		results:    make([]validator.ValidationResult, 0),
		reports:    make([]reporter.ValidationReport, 0),
		maxResults: 10000,
		maxReports: 1000,
	}
}

// AddResult adds a validation result to history.
func (h *ValidationHistory) AddResult(result validator.ValidationResult) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.results = append(h.results, result)
	if len(h.results) > h.maxResults {
		h.results = h.results[len(h.results)-h.maxResults:]
	}
}

// AddResults adds multiple validation results to history.
func (h *ValidationHistory) AddResults(results []validator.ValidationResult) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.results = append(h.results, results...)
	if len(h.results) > h.maxResults {
		h.results = h.results[len(h.results)-h.maxResults:]
	}
}

// AddReport adds a validation report to history.
func (h *ValidationHistory) AddReport(report reporter.ValidationReport) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.reports = append(h.reports, report)
	if len(h.reports) > h.maxReports {
		h.reports = h.reports[len(h.reports)-h.maxReports:]
	}
}

// GetResults returns all stored results.
func (h *ValidationHistory) GetResults() []validator.ValidationResult {
	h.mu.RLock()
	defer h.mu.RUnlock()

	results := make([]validator.ValidationResult, len(h.results))
	copy(results, h.results)
	return results
}

// GetResultsSince returns results since a given time.
func (h *ValidationHistory) GetResultsSince(since time.Time) []validator.ValidationResult {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var results []validator.ValidationResult
	for _, r := range h.results {
		if r.Timestamp.After(since) {
			results = append(results, r)
		}
	}
	return results
}

// GetReports returns all stored reports.
func (h *ValidationHistory) GetReports() []reporter.ValidationReport {
	h.mu.RLock()
	defer h.mu.RUnlock()

	reports := make([]reporter.ValidationReport, len(h.reports))
	copy(reports, h.reports)
	return reports
}

// GetTrend calculates accuracy trend over time.
func (h *ValidationHistory) GetTrend(duration time.Duration) AccuracyTrend {
	h.mu.RLock()
	defer h.mu.RUnlock()

	since := time.Now().Add(-duration)
	var cpuAccuracies, memAccuracies []float64
	var timestamps []time.Time

	for _, r := range h.results {
		if r.Timestamp.After(since) {
			cpuAccuracies = append(cpuAccuracies, r.CPUAccuracy)
			memAccuracies = append(memAccuracies, r.MemoryAccuracy)
			timestamps = append(timestamps, r.Timestamp)
		}
	}

	return AccuracyTrend{
		Period:        duration.String(),
		DataPoints:   len(cpuAccuracies),
		CPUAccuracies: cpuAccuracies,
		MemAccuracies: memAccuracies,
		Timestamps:   timestamps,
	}
}

// AccuracyTrend represents accuracy trend data.
type AccuracyTrend struct {
	Period        string      `json:"period"`
	DataPoints    int         `json:"data_points"`
	CPUAccuracies []float64   `json:"cpu_accuracies"`
	MemAccuracies []float64   `json:"mem_accuracies"`
	Timestamps    []time.Time `json:"timestamps"`
}

// Save persists the history to disk.
func (h *ValidationHistory) Save() error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.dataDir == "" {
		return nil
	}

	if err := os.MkdirAll(h.dataDir, 0755); err != nil {
		return err
	}

	// Save results
	resultsPath := filepath.Join(h.dataDir, "results.json")
	resultsData, err := json.Marshal(h.results)
	if err != nil {
		return err
	}
	if err := os.WriteFile(resultsPath, resultsData, 0644); err != nil {
		return err
	}

	// Save reports
	reportsPath := filepath.Join(h.dataDir, "reports.json")
	reportsData, err := json.Marshal(h.reports)
	if err != nil {
		return err
	}
	return os.WriteFile(reportsPath, reportsData, 0644)
}

// Load loads history from disk.
func (h *ValidationHistory) Load() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.dataDir == "" {
		return nil
	}

	// Load results
	resultsPath := filepath.Join(h.dataDir, "results.json")
	if data, err := os.ReadFile(resultsPath); err == nil {
		if err := json.Unmarshal(data, &h.results); err != nil {
			return err
		}
	}

	// Load reports
	reportsPath := filepath.Join(h.dataDir, "reports.json")
	if data, err := os.ReadFile(reportsPath); err == nil {
		if err := json.Unmarshal(data, &h.reports); err != nil {
			return err
		}
	}

	return nil
}
