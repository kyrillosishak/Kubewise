// Package model provides model storage and management functionality
package model

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStorageConfig(t *testing.T) {
	cfg := DefaultStorageConfig()

	if cfg.Backend != StorageBackendLocal {
		t.Errorf("Expected backend %s, got %s", StorageBackendLocal, cfg.Backend)
	}
	if cfg.LocalPath == "" {
		t.Error("Expected non-empty local path")
	}
	if cfg.Region == "" {
		t.Error("Expected non-empty region")
	}
	if cfg.Bucket == "" {
		t.Error("Expected non-empty bucket")
	}
}

func TestLocalStorage(t *testing.T) {
	// Create temp directory for testing
	tempDir, err := os.MkdirTemp("", "model-storage-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &StorageConfig{
		Backend:   StorageBackendLocal,
		LocalPath: tempDir,
	}

	storage, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	ctx := context.Background()

	// Test storing a model
	testWeights := []byte("test model weights data for testing purposes")
	version := "v1.0.0-test"

	storagePath, checksum, err := storage.StoreModel(ctx, version, testWeights)
	if err != nil {
		t.Fatalf("Failed to store model: %v", err)
	}

	if storagePath == "" {
		t.Error("Expected non-empty storage path")
	}
	if checksum == "" {
		t.Error("Expected non-empty checksum")
	}

	// Verify file exists
	if _, err := os.Stat(storagePath); os.IsNotExist(err) {
		t.Errorf("Model file does not exist at %s", storagePath)
	}

	// Test retrieving the model
	retrievedWeights, err := storage.GetModel(ctx, storagePath)
	if err != nil {
		t.Fatalf("Failed to get model: %v", err)
	}

	if string(retrievedWeights) != string(testWeights) {
		t.Errorf("Retrieved weights don't match: got %s, want %s", retrievedWeights, testWeights)
	}

	// Test checksum verification
	if !storage.VerifyChecksum(retrievedWeights, checksum) {
		t.Error("Checksum verification failed")
	}

	// Test with wrong checksum
	if storage.VerifyChecksum(retrievedWeights, "wrong-checksum") {
		t.Error("Checksum verification should have failed with wrong checksum")
	}

	// Test listing models
	models, err := storage.ListModels(ctx)
	if err != nil {
		t.Fatalf("Failed to list models: %v", err)
	}

	if len(models) != 1 {
		t.Errorf("Expected 1 model, got %d", len(models))
	}

	// Test deleting the model
	err = storage.DeleteModel(ctx, storagePath)
	if err != nil {
		t.Fatalf("Failed to delete model: %v", err)
	}

	// Verify file is deleted
	if _, err := os.Stat(storagePath); !os.IsNotExist(err) {
		t.Error("Model file should have been deleted")
	}
}

func TestCalculateChecksum(t *testing.T) {
	data := []byte("test data for checksum")
	checksum := calculateChecksum(data)

	if checksum == "" {
		t.Error("Expected non-empty checksum")
	}

	// Same data should produce same checksum
	checksum2 := calculateChecksum(data)
	if checksum != checksum2 {
		t.Error("Same data should produce same checksum")
	}

	// Different data should produce different checksum
	differentData := []byte("different test data")
	differentChecksum := calculateChecksum(differentData)
	if checksum == differentChecksum {
		t.Error("Different data should produce different checksum")
	}
}

func TestExtractS3Key(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "S3 path with bucket",
			path:     "s3://my-bucket/models/model_v1.0.0.onnx",
			expected: "models/model_v1.0.0.onnx",
		},
		{
			name:     "S3 path with nested key",
			path:     "s3://bucket/path/to/model.onnx",
			expected: "path/to/model.onnx",
		},
		{
			name:     "Local path",
			path:     "/var/lib/models/model.onnx",
			expected: "/var/lib/models/model.onnx",
		},
		{
			name:     "Empty path",
			path:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractS3Key(tt.path)
			if result != tt.expected {
				t.Errorf("extractS3Key(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestModelInfo(t *testing.T) {
	info := ModelInfo{
		Version:            "v1.0.0",
		StoragePath:        "/path/to/model.onnx",
		Checksum:           "abc123",
		SizeBytes:          98304,
		CreatedAt:          time.Now(),
		ValidationAccuracy: 0.92,
		IsActive:           true,
		Metadata: map[string]interface{}{
			"training_samples": 10000,
		},
	}

	if info.Version != "v1.0.0" {
		t.Errorf("Expected version v1.0.0, got %s", info.Version)
	}
	if info.SizeBytes != 98304 {
		t.Errorf("Expected size 98304, got %d", info.SizeBytes)
	}
	if !info.IsActive {
		t.Error("Expected model to be active")
	}
}

func TestStorageBackendConstants(t *testing.T) {
	if StorageBackendLocal != "local" {
		t.Errorf("Expected 'local', got %s", StorageBackendLocal)
	}
	if StorageBackendS3 != "s3" {
		t.Errorf("Expected 's3', got %s", StorageBackendS3)
	}
	if StorageBackendMinIO != "minio" {
		t.Errorf("Expected 'minio', got %s", StorageBackendMinIO)
	}
}

func TestStorageWithInvalidBackend(t *testing.T) {
	cfg := &StorageConfig{
		Backend: "invalid",
	}

	_, err := NewStorage(cfg)
	if err == nil {
		t.Error("Expected error for invalid backend")
	}
}

func TestStorageLocalPathCreation(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "model-storage-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Use a nested path that doesn't exist
	nestedPath := filepath.Join(tempDir, "nested", "path", "models")

	cfg := &StorageConfig{
		Backend:   StorageBackendLocal,
		LocalPath: nestedPath,
	}

	_, err = NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage with nested path: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(nestedPath); os.IsNotExist(err) {
		t.Error("Nested directory should have been created")
	}
}

// Tests for federated learning

func TestParseGradients(t *testing.T) {
	// Create test gradients
	original := []float32{1.0, 2.0, 3.0, 4.0}
	serialized := serializeGradients(original)

	// Parse them back
	parsed, err := parseGradients(serialized)
	if err != nil {
		t.Fatalf("Failed to parse gradients: %v", err)
	}

	if len(parsed) != len(original) {
		t.Errorf("Expected %d gradients, got %d", len(original), len(parsed))
	}

	for i, v := range original {
		if parsed[i] != v {
			t.Errorf("Gradient[%d] = %f, want %f", i, parsed[i], v)
		}
	}
}

func TestSerializeGradients(t *testing.T) {
	gradients := []float32{0.5, -0.5, 1.5, -1.5}
	serialized := serializeGradients(gradients)

	// Should be 4 bytes per float32
	expectedLen := len(gradients) * 4
	if len(serialized) != expectedLen {
		t.Errorf("Expected %d bytes, got %d", expectedLen, len(serialized))
	}
}

func TestParseGradientsInvalidLength(t *testing.T) {
	// Invalid length (not multiple of 4)
	invalidData := []byte{1, 2, 3}
	_, err := parseGradients(invalidData)
	if err == nil {
		t.Error("Expected error for invalid gradient data length")
	}
}

func TestFederatedConfig(t *testing.T) {
	cfg := DefaultFederatedConfig()

	if cfg.MinAgentsForAggregation <= 0 {
		t.Error("Expected positive min agents for aggregation")
	}
	if cfg.AggregationInterval <= 0 {
		t.Error("Expected positive aggregation interval")
	}
}

func TestGradientUpdate(t *testing.T) {
	update := GradientUpdate{
		ID:           "test-id",
		AgentID:      "agent-1",
		ModelVersion: "v1.0.0",
		Gradients:    []byte{1, 2, 3, 4},
		SampleCount:  100,
		CreatedAt:    time.Now(),
		Aggregated:   false,
	}

	if update.AgentID != "agent-1" {
		t.Errorf("Expected agent ID 'agent-1', got '%s'", update.AgentID)
	}
	if update.SampleCount != 100 {
		t.Errorf("Expected sample count 100, got %d", update.SampleCount)
	}
	if update.Aggregated {
		t.Error("Expected aggregated to be false")
	}
}

func TestAggregationResult(t *testing.T) {
	result := AggregationResult{
		ModelVersion:        "v1.0.0",
		AggregatedGradients: []byte{1, 2, 3, 4},
		TotalSamples:        1000,
		NumAgents:           5,
		AggregatedAt:        time.Now(),
	}

	if result.NumAgents != 5 {
		t.Errorf("Expected 5 agents, got %d", result.NumAgents)
	}
	if result.TotalSamples != 1000 {
		t.Errorf("Expected 1000 samples, got %d", result.TotalSamples)
	}
}

func TestAggregationStats(t *testing.T) {
	stats := AggregationStats{
		ModelVersion:        "v1.0.0",
		TotalUpdates:        10,
		UniqueAgents:        5,
		TotalSamples:        5000,
		AggregatedCount:     7,
		PendingCount:        3,
		MinAgentsRequired:   3,
		ReadyForAggregation: true,
	}

	if !stats.ReadyForAggregation {
		t.Error("Expected ready for aggregation")
	}
	if stats.PendingCount != 3 {
		t.Errorf("Expected 3 pending, got %d", stats.PendingCount)
	}
}

// Tests for model distribution

func TestDeploymentStatusConstants(t *testing.T) {
	if DeploymentStatusPending != "pending" {
		t.Errorf("Expected 'pending', got %s", DeploymentStatusPending)
	}
	if DeploymentStatusDeployed != "deployed" {
		t.Errorf("Expected 'deployed', got %s", DeploymentStatusDeployed)
	}
	if DeploymentStatusFailed != "failed" {
		t.Errorf("Expected 'failed', got %s", DeploymentStatusFailed)
	}
	if DeploymentStatusRolledBack != "rolled_back" {
		t.Errorf("Expected 'rolled_back', got %s", DeploymentStatusRolledBack)
	}
}

func TestModelDeployment(t *testing.T) {
	passed := true
	prevVersion := "v0.9.0"

	deployment := ModelDeployment{
		ID:               "deploy-1",
		ModelVersion:     "v1.0.0",
		AgentID:          "agent-1",
		DeployedAt:       time.Now(),
		Status:           DeploymentStatusDeployed,
		ValidationPassed: &passed,
		PreviousVersion:  &prevVersion,
	}

	if deployment.Status != DeploymentStatusDeployed {
		t.Errorf("Expected status 'deployed', got '%s'", deployment.Status)
	}
	if *deployment.ValidationPassed != true {
		t.Error("Expected validation passed to be true")
	}
	if *deployment.PreviousVersion != "v0.9.0" {
		t.Errorf("Expected previous version 'v0.9.0', got '%s'", *deployment.PreviousVersion)
	}
}

func TestModelUpdate(t *testing.T) {
	update := ModelUpdate{
		Version:            "v1.0.0",
		Weights:            []byte{1, 2, 3, 4},
		Checksum:           "abc123",
		SizeBytes:          98304,
		ValidationAccuracy: 0.92,
		CreatedAt:          time.Now(),
	}

	if update.Version != "v1.0.0" {
		t.Errorf("Expected version 'v1.0.0', got '%s'", update.Version)
	}
	if update.ValidationAccuracy != 0.92 {
		t.Errorf("Expected accuracy 0.92, got %f", update.ValidationAccuracy)
	}
}

func TestDeploymentSummary(t *testing.T) {
	summary := DeploymentSummary{
		ModelVersion:     "v1.0.0",
		Total:            10,
		Pending:          2,
		Deployed:         7,
		Failed:           1,
		RolledBack:       0,
		ValidationPassed: 6,
		ValidationFailed: 1,
	}

	if summary.Total != 10 {
		t.Errorf("Expected total 10, got %d", summary.Total)
	}
	if summary.Deployed != 7 {
		t.Errorf("Expected deployed 7, got %d", summary.Deployed)
	}
}

func TestIncrementalUpdate(t *testing.T) {
	update := IncrementalUpdate{
		FromVersion: "v0.9.0",
		ToVersion:   "v1.0.0",
		Delta:       []byte{1, 2, 3, 4},
		Checksum:    "abc123",
	}

	if update.FromVersion != "v0.9.0" {
		t.Errorf("Expected from version 'v0.9.0', got '%s'", update.FromVersion)
	}
	if update.ToVersion != "v1.0.0" {
		t.Errorf("Expected to version 'v1.0.0', got '%s'", update.ToVersion)
	}
}

func TestCalculateDelta(t *testing.T) {
	oldWeights := []byte{0x00, 0x01, 0x02, 0x03}
	newWeights := []byte{0x01, 0x01, 0x03, 0x03}

	delta, err := calculateDelta(oldWeights, newWeights)
	if err != nil {
		t.Fatalf("Failed to calculate delta: %v", err)
	}

	// XOR delta
	expected := []byte{0x01, 0x00, 0x01, 0x00}
	for i, v := range expected {
		if delta[i] != v {
			t.Errorf("Delta[%d] = %x, want %x", i, delta[i], v)
		}
	}
}

func TestCalculateDeltaSizeMismatch(t *testing.T) {
	oldWeights := []byte{0x00, 0x01}
	newWeights := []byte{0x00, 0x01, 0x02}

	_, err := calculateDelta(oldWeights, newWeights)
	if err == nil {
		t.Error("Expected error for size mismatch")
	}
}

// Tests for rollback

func TestRollbackConfig(t *testing.T) {
	cfg := DefaultRollbackConfig()

	if cfg.MaxVersionsToKeep != 5 {
		t.Errorf("Expected max versions to keep 5, got %d", cfg.MaxVersionsToKeep)
	}
}

func TestRollbackResult(t *testing.T) {
	result := RollbackResult{
		PreviousVersion: "v1.1.0",
		RolledBackTo:    "v1.0.0",
		RollbackTime:    time.Now(),
		Reason:          "Validation failure",
		AgentsNotified:  10,
	}

	if result.PreviousVersion != "v1.1.0" {
		t.Errorf("Expected previous version 'v1.1.0', got '%s'", result.PreviousVersion)
	}
	if result.RolledBackTo != "v1.0.0" {
		t.Errorf("Expected rolled back to 'v1.0.0', got '%s'", result.RolledBackTo)
	}
	if result.AgentsNotified != 10 {
		t.Errorf("Expected 10 agents notified, got %d", result.AgentsNotified)
	}
}

func TestRollbackEvent(t *testing.T) {
	event := RollbackEvent{
		ID:           "rollback-1",
		FromVersion:  "v1.1.0",
		ToVersion:    "v1.0.0",
		Reason:       "Auto-rollback due to validation failures",
		RolledBackAt: time.Now(),
	}

	if event.FromVersion != "v1.1.0" {
		t.Errorf("Expected from version 'v1.1.0', got '%s'", event.FromVersion)
	}
	if event.ToVersion != "v1.0.0" {
		t.Errorf("Expected to version 'v1.0.0', got '%s'", event.ToVersion)
	}
}

func TestGenerateVersionString(t *testing.T) {
	version := generateVersionString("v1.0.0")

	if version == "" {
		t.Error("Expected non-empty version string")
	}
	if version == "v1.0.0" {
		t.Error("Expected different version string from base")
	}
	// Should contain "fed" suffix
	if len(version) < 4 || version[len(version)-3:] != "fed" {
		t.Errorf("Expected version to end with 'fed', got '%s'", version)
	}
}

// Tests for model version

func TestModelVersion(t *testing.T) {
	model := ModelVersion{
		Version:                 "v1.0.0",
		CreatedAt:               time.Now(),
		Description:             "Initial model",
		WeightsPath:             "/path/to/weights",
		StoragePath:             "/path/to/weights",
		StorageBackend:          "local",
		Checksum:                "abc123",
		ValidationAccuracy:      0.92,
		SizeBytes:               98304,
		IsActive:                true,
		RollbackCount:           0,
		TrainingSamples:         10000,
		TrainingDurationSeconds: 3600,
		Metadata: map[string]interface{}{
			"framework": "pytorch",
		},
	}

	if model.Version != "v1.0.0" {
		t.Errorf("Expected version 'v1.0.0', got '%s'", model.Version)
	}
	if model.ValidationAccuracy != 0.92 {
		t.Errorf("Expected accuracy 0.92, got %f", model.ValidationAccuracy)
	}
	if !model.IsActive {
		t.Error("Expected model to be active")
	}
	if model.SizeBytes != 98304 {
		t.Errorf("Expected size 98304, got %d", model.SizeBytes)
	}
}

func TestCreateModelInput(t *testing.T) {
	input := CreateModelInput{
		Version:                 "v1.0.0",
		Description:             "Test model",
		Weights:                 []byte{1, 2, 3, 4},
		ValidationAccuracy:      0.90,
		TrainingSamples:         5000,
		TrainingDurationSeconds: 1800,
		Metadata: map[string]interface{}{
			"test": true,
		},
	}

	if input.Version != "v1.0.0" {
		t.Errorf("Expected version 'v1.0.0', got '%s'", input.Version)
	}
	if len(input.Weights) != 4 {
		t.Errorf("Expected 4 bytes of weights, got %d", len(input.Weights))
	}
}

func TestValidationInput(t *testing.T) {
	agentID := "agent-1"
	precision := float32(0.91)
	recall := float32(0.89)
	f1 := float32(0.90)

	input := ValidationInput{
		ModelVersion:   "v1.0.0",
		AgentID:        &agentID,
		Accuracy:       0.90,
		Precision:      &precision,
		Recall:         &recall,
		F1Score:        &f1,
		SampleCount:    1000,
		ValidationType: "holdout",
		Passed:         true,
		Details: map[string]interface{}{
			"test_set_size": 200,
		},
	}

	if input.ModelVersion != "v1.0.0" {
		t.Errorf("Expected model version 'v1.0.0', got '%s'", input.ModelVersion)
	}
	if !input.Passed {
		t.Error("Expected validation to pass")
	}
	if *input.AgentID != "agent-1" {
		t.Errorf("Expected agent ID 'agent-1', got '%s'", *input.AgentID)
	}
}

func TestValidationResult(t *testing.T) {
	precision := float32(0.91)
	recall := float32(0.89)
	f1 := float32(0.90)
	agentID := "agent-1"

	result := ValidationResult{
		ID:             "val-1",
		ModelVersion:   "v1.0.0",
		AgentID:        &agentID,
		ValidatedAt:    time.Now(),
		Accuracy:       0.90,
		Precision:      &precision,
		Recall:         &recall,
		F1Score:        &f1,
		SampleCount:    1000,
		ValidationType: "holdout",
		Passed:         true,
		Details: map[string]interface{}{
			"test_set_size": 200,
		},
	}

	if result.Accuracy != 0.90 {
		t.Errorf("Expected accuracy 0.90, got %f", result.Accuracy)
	}
	if !result.Passed {
		t.Error("Expected validation to pass")
	}
}
