// Package e2e provides end-to-end tests for validating Kubewise functionality.
package e2e

import (
	"context"
	"testing"
	"time"
)

// testFullValidationReport tests the metrics validator full validation cycle.
// Requirements: 7.4, 8.4, 8.5
func testFullValidationReport(t *testing.T, ctx context.Context, helper *TestHelper) {
	t.Log("Testing full validation report...")

	// Start the full-validation scenario via pattern controller
	if err := helper.StartScenario(ctx, "full-validation"); err != nil {
		t.Logf("Could not start full-validation scenario: %v", err)
		t.Log("Running validation with current state instead...")
	}
	defer helper.StopScenario(ctx)

	// Trigger validation cycle on metrics-validator
	t.Log("Triggering validation cycle...")
	if err := helper.TriggerValidation(ctx); err != nil {
		t.Logf("Warning: Could not trigger validation: %v", err)
	}

	// Wait for validation to complete
	validationWait := 5 * time.Minute
	if *shortTest {
		validationWait = 2 * time.Minute
	}
	t.Logf("Waiting %v for validation to complete...", validationWait)
	time.Sleep(validationWait)

	// Get validation report
	report, err := helper.GetValidationReport(ctx)
	if err != nil {
		t.Fatalf("Failed to get validation report: %v", err)
	}
	
	if report == nil {
		t.Fatal("No validation report available")
	}

	// Log report details
	t.Logf("Validation Report:")
	t.Logf("  Generated: %s", report.GeneratedAt)
	t.Logf("  Duration: %s", report.TestDuration)
	t.Logf("  Scenario: %s", report.ScenarioName)
	t.Logf("  Total Predictions: %d", report.TotalPredictions)
	t.Logf("  Accurate Predictions: %d", report.AccuratePredictions)
	t.Logf("  CPU Accuracy: %.1f%%", report.OverallCPUAccuracy)
	t.Logf("  Memory Accuracy: %.1f%%", report.OverallMemAccuracy)
	t.Logf("  Confidence Correlation: %.2f", report.ConfidenceCorrelation)

	// Log anomaly stats
	t.Logf("  Anomaly Stats:")
	t.Logf("    Total: %d", report.AnomalyStats.Total)
	t.Logf("    Detected: %d", report.AnomalyStats.Detected)
	t.Logf("    Detection Rate: %.1f%%", report.AnomalyStats.DetectionRate*100)
	t.Logf("    False Positive Rate: %.1f%%", report.AnomalyStats.FalsePositiveRate*100)

	// Log cost stats
	t.Logf("  Cost Stats:")
	t.Logf("    Total Validations: %d", report.CostStats.TotalValidations)
	t.Logf("    Average Accuracy: %.1f%%", report.CostStats.AverageAccuracy*100)

	// Log pass criteria
	t.Logf("  Pass Criteria:")
	for _, criterion := range report.PassCriteria {
		status := "✓"
		if !criterion.Pass {
			status = "✗"
		}
		t.Logf("    %s %s: expected %s, actual %s", status, criterion.Name, criterion.Expected, criterion.Actual)
	}

	t.Logf("  Overall Pass: %v", report.OverallPass)

	// Validate report meets pass criteria
	// Only validate if we have enough data
	if report.TotalPredictions > 0 {
		// CPU prediction accuracy should be >= 70%
		AssertGreaterOrEqual(t, report.OverallCPUAccuracy, 70.0,
			"CPU prediction accuracy should be >= 70%")

		// Memory prediction accuracy should be >= 70%
		AssertGreaterOrEqual(t, report.OverallMemAccuracy, 70.0,
			"Memory prediction accuracy should be >= 70%")
	} else {
		t.Log("Note: No predictions to validate - this may be expected on first run")
	}

	// Anomaly detection rate should be >= 90% (if anomalies were triggered)
	if report.AnomalyStats.Total > 0 {
		AssertGreaterOrEqual(t, report.AnomalyStats.DetectionRate*100, 90.0,
			"Anomaly detection rate should be >= 90%")
	}

	// False positive rate should be <= 10%
	AssertLessOrEqual(t, report.AnomalyStats.FalsePositiveRate*100, 10.0,
		"False positive rate should be <= 10%")

	// Cost estimation accuracy should be >= 80% (if cost validations exist)
	if report.CostStats.TotalValidations > 0 {
		AssertGreaterOrEqual(t, report.CostStats.AverageAccuracy*100, 80.0,
			"Cost estimation accuracy should be >= 80%")
	}

	t.Log("Full validation report test completed")
}
