// Package e2e provides end-to-end tests for validating Kubewise functionality.
package e2e

import (
	"math"
	"testing"
)

// AssertWithinPercent asserts that actual is within percent% of expected.
func AssertWithinPercent(t *testing.T, actual, expected float64, percent float64, msg string) {
	t.Helper()
	
	if expected == 0 {
		if actual == 0 {
			return // Both zero is acceptable
		}
		t.Errorf("%s: expected 0, got %v", msg, actual)
		return
	}
	
	deviation := math.Abs(actual-expected) / expected * 100
	if deviation > percent {
		t.Errorf("%s: %v is not within %.1f%% of %v (deviation: %.1f%%)", 
			msg, actual, percent, expected, deviation)
	}
}

// AssertGreaterOrEqual asserts that actual >= expected.
func AssertGreaterOrEqual(t *testing.T, actual, expected float64, msg string) {
	t.Helper()
	if actual < expected {
		t.Errorf("%s: %v is not >= %v", msg, actual, expected)
	}
}

// AssertLessOrEqual asserts that actual <= expected.
func AssertLessOrEqual(t *testing.T, actual, expected float64, msg string) {
	t.Helper()
	if actual > expected {
		t.Errorf("%s: %v is not <= %v", msg, actual, expected)
	}
}

// AssertGreater asserts that actual > expected.
func AssertGreater(t *testing.T, actual, expected float64, msg string) {
	t.Helper()
	if actual <= expected {
		t.Errorf("%s: %v is not > %v", msg, actual, expected)
	}
}

// AssertNotNil asserts that value is not nil.
func AssertNotNil(t *testing.T, value interface{}, msg string) {
	t.Helper()
	if value == nil {
		t.Errorf("%s: expected non-nil value", msg)
	}
}

// AssertTrue asserts that condition is true.
func AssertTrue(t *testing.T, condition bool, msg string) {
	t.Helper()
	if !condition {
		t.Errorf("%s: expected true", msg)
	}
}

// AssertNoError asserts that err is nil.
func AssertNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Errorf("%s: unexpected error: %v", msg, err)
	}
}
