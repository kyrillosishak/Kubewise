// Package rest provides REST API handlers
package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetNamespaceConfigHandler(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("GET", "/api/v1/safety/config/default", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response NamespaceConfig
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Namespace != "default" {
		t.Errorf("Expected namespace 'default', got '%s'", response.Namespace)
	}
	// Default config should have dry-run disabled
	if response.DryRunEnabled {
		t.Error("Expected dry-run to be disabled by default")
	}
	// Default thresholds
	if response.HighRiskThresholdMemoryReduction != 0.30 {
		t.Errorf("Expected memory threshold 0.30, got %f", response.HighRiskThresholdMemoryReduction)
	}
	if response.HighRiskThresholdCPUReduction != 0.50 {
		t.Errorf("Expected CPU threshold 0.50, got %f", response.HighRiskThresholdCPUReduction)
	}
}

func TestUpdateNamespaceConfigHandler(t *testing.T) {
	router := setupRouter()

	body := `{"dry_run_enabled": true, "auto_approve_enabled": false}`
	req, _ := http.NewRequest("PUT", "/api/v1/safety/config/production", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response NamespaceConfig
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Namespace != "production" {
		t.Errorf("Expected namespace 'production', got '%s'", response.Namespace)
	}
	if !response.DryRunEnabled {
		t.Error("Expected dry-run to be enabled")
	}
	if response.AutoApproveEnabled {
		t.Error("Expected auto-approve to be disabled")
	}
}

func TestListNamespaceConfigsHandler(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("GET", "/api/v1/safety/config", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response []NamespaceConfig
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Should have at least the global config
	if len(response) == 0 {
		t.Error("Expected at least one namespace config")
	}
}

func TestDryRunRecommendationHandler(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("POST", "/api/v1/recommendation/test-id/dry-run", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response DryRunResult
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.RecommendationID != "test-id" {
		t.Errorf("Expected recommendation ID 'test-id', got '%s'", response.RecommendationID)
	}
	if !response.WouldApply {
		t.Error("Expected would_apply to be true")
	}
	if response.YamlPatch == "" {
		t.Error("Expected YAML patch to be non-empty")
	}
	// Mock response should have changes
	if len(response.Changes) == 0 {
		t.Error("Expected changes to be non-empty")
	}
	// Mock response should have warnings for significant reductions
	if len(response.Warnings) == 0 {
		t.Error("Expected warnings to be non-empty for significant reductions")
	}
}

func TestDryRunResultContainsResourceChanges(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("POST", "/api/v1/recommendation/test-id/dry-run", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response DryRunResult
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Check that changes contain expected fields
	for _, change := range response.Changes {
		if change.Resource == "" {
			t.Error("Expected resource to be non-empty")
		}
		if change.CurrentValue == "" {
			t.Error("Expected current_value to be non-empty")
		}
		if change.NewValue == "" {
			t.Error("Expected new_value to be non-empty")
		}
	}
}

func TestGetApprovalHistoryHandler(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("GET", "/api/v1/recommendation/test-id/approval-history", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response []ApprovalHistory
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Without store, should return empty list
	if len(response) != 0 {
		t.Errorf("Expected empty approval history, got %d items", len(response))
	}
}

func TestGetRecommendationOutcomeHandler(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("GET", "/api/v1/recommendation/test-id/outcome", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response RecommendationOutcome
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.RecommendationID != "test-id" {
		t.Errorf("Expected recommendation ID 'test-id', got '%s'", response.RecommendationID)
	}
	if response.OutcomeStatus != "monitoring" {
		t.Errorf("Expected outcome status 'monitoring', got '%s'", response.OutcomeStatus)
	}
}

func TestListRollbackEventsHandler(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("GET", "/api/v1/safety/rollbacks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response []RollbackEvent
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Without store, should return empty list
	if len(response) != 0 {
		t.Errorf("Expected empty rollback events, got %d items", len(response))
	}
}

func TestListRollbackEventsWithNamespaceFilter(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("GET", "/api/v1/safety/rollbacks?namespace=production", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response []RollbackEvent
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestApproveRecommendationWithApprover(t *testing.T) {
	router := setupRouter()

	body := `{"approver": "admin@example.com", "reason": "Approved after review"}`
	req, _ := http.NewRequest("POST", "/api/v1/recommendation/test-id/approve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response ApproveRecommendationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got '%s'", response.ID)
	}
	if response.Status != "approved" {
		t.Errorf("Expected status 'approved', got '%s'", response.Status)
	}
	if response.Approver != "admin@example.com" {
		t.Errorf("Expected approver 'admin@example.com', got '%s'", response.Approver)
	}
}
