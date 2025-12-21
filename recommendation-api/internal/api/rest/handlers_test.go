// Package rest provides REST API handlers
package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupRouter() *gin.Engine {
	router := gin.New()
	RegisterRoutes(router)
	return router
}

func TestHealthzHandler(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", response["status"])
	}
}

func TestReadyzHandler(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["status"] != "ready" {
		t.Errorf("Expected status 'ready', got '%s'", response["status"])
	}
}

func TestListRecommendationsHandler(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("GET", "/api/v1/recommendations", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response RecommendationList
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Without store, should return empty list
	if response.Total != 0 {
		t.Errorf("Expected 0 recommendations, got %d", response.Total)
	}
}

func TestListNamespaceRecommendationsHandler(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("GET", "/api/v1/recommendations/default", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response RecommendationList
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestGetRecommendationHandler(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("GET", "/api/v1/recommendations/default/my-deployment", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response Recommendation
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Mock response should have the deployment name
	if response.Deployment != "my-deployment" {
		t.Errorf("Expected deployment 'my-deployment', got '%s'", response.Deployment)
	}
	if response.Namespace != "default" {
		t.Errorf("Expected namespace 'default', got '%s'", response.Namespace)
	}
}

func TestApplyRecommendationHandler(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("POST", "/api/v1/recommendation/test-id/apply", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response ApplyRecommendationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got '%s'", response.ID)
	}
	if response.Status != "applied" {
		t.Errorf("Expected status 'applied', got '%s'", response.Status)
	}
	if response.YamlPatch == "" {
		t.Error("Expected YAML patch to be non-empty")
	}
}

func TestApproveRecommendationHandler(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("POST", "/api/v1/recommendation/test-id/approve", nil)
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
}

func TestGetClusterCostsHandler(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("GET", "/api/v1/costs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response CostAnalysis
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Currency != "USD" {
		t.Errorf("Expected currency 'USD', got '%s'", response.Currency)
	}
	// Mock data should have positive values
	if response.CurrentMonthlyCost <= 0 {
		t.Error("Expected positive current monthly cost")
	}
}

func TestGetNamespaceCostsHandler(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("GET", "/api/v1/costs/production", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response CostAnalysis
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Namespace != "production" {
		t.Errorf("Expected namespace 'production', got '%s'", response.Namespace)
	}
}

func TestGetSavingsHandler(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("GET", "/api/v1/savings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response SavingsReport
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Currency != "USD" {
		t.Errorf("Expected currency 'USD', got '%s'", response.Currency)
	}
	if len(response.SavingsByMonth) == 0 {
		t.Error("Expected savings by month to be non-empty")
	}
}

func TestListModelsHandler(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("GET", "/api/v1/models", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response ModelList
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Mock data should have models
	if response.Total == 0 {
		t.Error("Expected models to be non-empty")
	}
}

func TestGetModelHandler(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("GET", "/api/v1/models/v1.0.0", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response ModelVersion
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Version != "v1.0.0" {
		t.Errorf("Expected version 'v1.0.0', got '%s'", response.Version)
	}
}

func TestRollbackModelHandler(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("POST", "/api/v1/models/rollback/v1.0.0", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response RollbackResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Version != "v1.0.0" {
		t.Errorf("Expected version 'v1.0.0', got '%s'", response.Version)
	}
	if response.Status != "rolled_back" {
		t.Errorf("Expected status 'rolled_back', got '%s'", response.Status)
	}
}

func TestGetPredictionHistoryHandler(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("GET", "/api/v1/debug/predictions/my-deployment", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response PredictionHistory
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Deployment != "my-deployment" {
		t.Errorf("Expected deployment 'my-deployment', got '%s'", response.Deployment)
	}
	if len(response.Predictions) == 0 {
		t.Error("Expected predictions to be non-empty")
	}
}
