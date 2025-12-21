// Package validator provides validation logic for Kubewise predictions.
package validator

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
)

// RegisterAnomaly registers an anomaly that was triggered for validation.
func (v *Validator) RegisterAnomaly(anomalyType, component string) string {
	v.mu.Lock()
	defer v.mu.Unlock()

	id := uuid.New().String()
	var timeout time.Duration
	switch anomalyType {
	case AnomalyTypeMemoryLeak:
		timeout = v.config.LeakDetectionTimeout
	case AnomalyTypeCPUSpike:
		timeout = v.config.SpikeDetectionTimeout
	default:
		timeout = 30 * time.Minute
	}

	validation := &AnomalyValidation{
		ID:          id,
		AnomalyType: anomalyType,
		Component:   component,
		TriggeredAt: time.Now(),
		WasDetected: false,
		ExpectedBy:  time.Now().Add(timeout),
	}

	v.anomalyValidations[id] = validation
	log.Printf("[validator] Registered anomaly %s: type=%s, component=%s, expected_by=%v",
		id, anomalyType, component, validation.ExpectedBy)

	return id
}

// MarkAnomalyDetected marks an anomaly as detected by Kubewise.
func (v *Validator) MarkAnomalyDetected(id string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if validation, ok := v.anomalyValidations[id]; ok {
		now := time.Now()
		validation.DetectedAt = &now
		validation.WasDetected = true
		validation.TimeToDetect = now.Sub(validation.TriggeredAt)
		log.Printf("[validator] Anomaly %s detected in %v", id, validation.TimeToDetect)
	}
}

// MarkFalsePositive marks an anomaly detection as a false positive.
func (v *Validator) MarkFalsePositive(id string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if validation, ok := v.anomalyValidations[id]; ok {
		validation.FalsePositive = true
		log.Printf("[validator] Anomaly %s marked as false positive", id)
	}
}

func (v *Validator) checkAnomalyDetections(ctx context.Context) {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()
	for id, validation := range v.anomalyValidations {
		// Skip already detected or expired
		if validation.WasDetected {
			continue
		}

		// Check if detection window has expired
		if now.After(validation.ExpectedBy) {
			log.Printf("[validator] Anomaly %s not detected within timeout (false negative)", id)
			continue
		}

		// TODO: Query Kubewise for anomaly alerts and match with registered anomalies
		// This would require an anomaly/alerts endpoint in Kubewise API
	}
}

// GetAnomalyValidations returns all anomaly validations.
func (v *Validator) GetAnomalyValidations() []*AnomalyValidation {
	v.mu.RLock()
	defer v.mu.RUnlock()

	validations := make([]*AnomalyValidation, 0, len(v.anomalyValidations))
	for _, val := range v.anomalyValidations {
		validations = append(validations, val)
	}
	return validations
}

// GetAnomalyValidation returns a specific anomaly validation.
func (v *Validator) GetAnomalyValidation(id string) *AnomalyValidation {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.anomalyValidations[id]
}

// CalculateAnomalyStats calculates anomaly detection statistics.
func (v *Validator) CalculateAnomalyStats() AnomalyStats {
	v.mu.RLock()
	defer v.mu.RUnlock()

	stats := AnomalyStats{
		ByType: make(map[string]AnomalyTypeStats),
	}

	for _, val := range v.anomalyValidations {
		stats.Total++
		
		typeStats := stats.ByType[val.AnomalyType]
		typeStats.Total++

		if val.WasDetected {
			stats.Detected++
			typeStats.Detected++
			typeStats.TotalDetectionTime += val.TimeToDetect
		}

		if val.FalsePositive {
			stats.FalsePositives++
			typeStats.FalsePositives++
		}

		// Check for false negatives (not detected within timeout)
		if !val.WasDetected && time.Now().After(val.ExpectedBy) {
			stats.FalseNegatives++
			typeStats.FalseNegatives++
		}

		stats.ByType[val.AnomalyType] = typeStats
	}

	// Calculate rates
	if stats.Total > 0 {
		stats.DetectionRate = float64(stats.Detected) / float64(stats.Total)
		stats.FalsePositiveRate = float64(stats.FalsePositives) / float64(stats.Total)
		stats.FalseNegativeRate = float64(stats.FalseNegatives) / float64(stats.Total)
	}

	// Calculate per-type stats
	for anomalyType, typeStats := range stats.ByType {
		if typeStats.Total > 0 {
			typeStats.DetectionRate = float64(typeStats.Detected) / float64(typeStats.Total)
			if typeStats.Detected > 0 {
				typeStats.AvgDetectionTime = typeStats.TotalDetectionTime / time.Duration(typeStats.Detected)
			}
		}
		stats.ByType[anomalyType] = typeStats
	}

	return stats
}

// AnomalyStats holds statistics about anomaly detection.
type AnomalyStats struct {
	Total             int                        `json:"total"`
	Detected          int                        `json:"detected"`
	FalsePositives    int                        `json:"false_positives"`
	FalseNegatives    int                        `json:"false_negatives"`
	DetectionRate     float64                    `json:"detection_rate"`
	FalsePositiveRate float64                    `json:"false_positive_rate"`
	FalseNegativeRate float64                    `json:"false_negative_rate"`
	ByType            map[string]AnomalyTypeStats `json:"by_type"`
}

// AnomalyTypeStats holds statistics for a specific anomaly type.
type AnomalyTypeStats struct {
	Total              int           `json:"total"`
	Detected           int           `json:"detected"`
	FalsePositives     int           `json:"false_positives"`
	FalseNegatives     int           `json:"false_negatives"`
	DetectionRate      float64       `json:"detection_rate"`
	TotalDetectionTime time.Duration `json:"-"`
	AvgDetectionTime   time.Duration `json:"avg_detection_time"`
}
