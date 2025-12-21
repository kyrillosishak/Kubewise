// Package reporter provides report generation for validation results.
package reporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// WebhookNotifier sends notifications via webhooks.
type WebhookNotifier struct {
	webhookURL string
	httpClient *http.Client
}

// WebhookPayload represents the payload sent to webhooks.
type WebhookPayload struct {
	Event     string           `json:"event"`
	Timestamp time.Time        `json:"timestamp"`
	Report    *ValidationReport `json:"report,omitempty"`
	Message   string           `json:"message,omitempty"`
}

// NewWebhookNotifier creates a new WebhookNotifier.
func NewWebhookNotifier(webhookURL string) *WebhookNotifier {
	return &WebhookNotifier{
		webhookURL: webhookURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// NotifyTestComplete sends a notification when a test completes.
func (w *WebhookNotifier) NotifyTestComplete(ctx context.Context, report ValidationReport) error {
	if w.webhookURL == "" {
		return nil
	}

	payload := WebhookPayload{
		Event:     "test_complete",
		Timestamp: time.Now(),
		Report:    &report,
		Message:   fmt.Sprintf("Test '%s' completed. Overall pass: %v", report.ScenarioName, report.OverallPass),
	}

	return w.send(ctx, payload)
}

// NotifyAnomalyDetected sends a notification when an anomaly is detected.
func (w *WebhookNotifier) NotifyAnomalyDetected(ctx context.Context, anomalyType, component string, detectionTime time.Duration) error {
	if w.webhookURL == "" {
		return nil
	}

	payload := WebhookPayload{
		Event:     "anomaly_detected",
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("Anomaly '%s' detected in component '%s' after %v", anomalyType, component, detectionTime),
	}

	return w.send(ctx, payload)
}

// NotifyCriticalEvent sends a notification for critical events.
func (w *WebhookNotifier) NotifyCriticalEvent(ctx context.Context, message string) error {
	if w.webhookURL == "" {
		return nil
	}

	payload := WebhookPayload{
		Event:     "critical",
		Timestamp: time.Now(),
		Message:   message,
	}

	return w.send(ctx, payload)
}

func (w *WebhookNotifier) send(ctx context.Context, payload WebhookPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	log.Printf("[webhook] Sent notification: event=%s", payload.Event)
	return nil
}

// IsConfigured returns true if a webhook URL is configured.
func (w *WebhookNotifier) IsConfigured() bool {
	return w.webhookURL != ""
}
