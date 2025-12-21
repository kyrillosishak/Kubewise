// Package e2e provides end-to-end tests for validating Kubewise functionality.
package e2e

import (
	"context"
	"flag"
	"os"
	"testing"
	"time"
)

var (
	// Test configuration flags
	kubewiseURL      = flag.String("kubewise-url", getEnvOrDefault("KUBEWISE_URL", "http://kubewise-api.kubewise-system:8080"), "Kubewise API URL")
	testNamespace    = flag.String("test-namespace", getEnvOrDefault("TEST_NAMESPACE", "kubewise-test"), "Test application namespace")
	kubeconfig       = flag.String("kubeconfig", getEnvOrDefault("KUBECONFIG", ""), "Path to kubeconfig file")
	skipCleanup      = flag.Bool("skip-cleanup", false, "Skip cleanup after tests")
	shortTest        = flag.Bool("short", false, "Run shorter test durations")
	
	// Test timeouts
	dataCollectionWait = 10 * time.Minute
	leakDetectionWait  = 30 * time.Minute
	spikeDetectionWait = 5 * time.Minute
)

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func TestMain(m *testing.M) {
	flag.Parse()
	
	// Adjust timeouts for short tests
	if *shortTest {
		dataCollectionWait = 2 * time.Minute
		leakDetectionWait = 5 * time.Minute
		spikeDetectionWait = 2 * time.Minute
	}
	
	os.Exit(m.Run())
}

// TestKubewiseE2E runs the full E2E test suite.
func TestKubewiseE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()
	
	// Initialize test helper
	helper, err := NewTestHelper(ctx, TestHelperConfig{
		KubewiseURL:   *kubewiseURL,
		TestNamespace: *testNamespace,
		Kubeconfig:    *kubeconfig,
	})
	if err != nil {
		t.Fatalf("Failed to create test helper: %v", err)
	}
	defer func() {
		if !*skipCleanup {
			helper.Cleanup(ctx)
		}
	}()
	
	// Verify prerequisites
	t.Run("Prerequisites", func(t *testing.T) {
		if err := helper.VerifyPrerequisites(ctx); err != nil {
			t.Fatalf("Prerequisites check failed: %v", err)
		}
	})
	
	// Run individual test cases
	t.Run("BaselinePredictionAccuracy", func(t *testing.T) {
		testBaselinePredictionAccuracy(t, ctx, helper)
	})
	
	t.Run("MemoryLeakDetection", func(t *testing.T) {
		testMemoryLeakDetection(t, ctx, helper)
	})
	
	t.Run("CPUSpikeDetection", func(t *testing.T) {
		testCPUSpikeDetection(t, ctx, helper)
	})
	
	t.Run("TimeAwareRecommendations", func(t *testing.T) {
		testTimeAwareRecommendations(t, ctx, helper)
	})
	
	t.Run("TrafficDrivenWorkloads", func(t *testing.T) {
		testTrafficDrivenWorkloads(t, ctx, helper)
	})
	
	t.Run("FullValidationReport", func(t *testing.T) {
		testFullValidationReport(t, ctx, helper)
	})
}
