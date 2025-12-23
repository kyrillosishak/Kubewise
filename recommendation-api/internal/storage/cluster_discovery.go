// Package storage provides data persistence implementations
package storage

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/container-resource-predictor/recommendation-api/internal/api/rest"
)

// DiscoverClusters discovers Kubernetes clusters from kubeconfig and kind
func DiscoverClusters(ctx context.Context, store *InMemoryClusterStore) error {
	// Discover kind clusters
	if err := discoverKindClusters(ctx, store); err != nil {
		slog.Warn("Failed to discover kind clusters", "error", err)
	}

	// Discover clusters from kubeconfig
	if err := discoverKubeconfigClusters(ctx, store); err != nil {
		slog.Warn("Failed to discover kubeconfig clusters", "error", err)
	}

	return nil
}

// discoverKindClusters discovers kind clusters
func discoverKindClusters(ctx context.Context, store *InMemoryClusterStore) error {
	cmd := exec.CommandContext(ctx, "kind", "get", "clusters")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	clusters := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, name := range clusters {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		clusterID := "kind-" + name
		
		// Check if cluster is healthy by getting nodes
		status := "healthy"
		nodeCount := 0
		podCount := 0

		nodeCmd := exec.CommandContext(ctx, "kubectl", "--context", clusterID, "get", "nodes", "--no-headers")
		if nodeOutput, err := nodeCmd.Output(); err == nil {
			lines := strings.Split(strings.TrimSpace(string(nodeOutput)), "\n")
			nodeCount = len(lines)
			for _, line := range lines {
				if strings.Contains(line, "NotReady") {
					status = "degraded"
				}
			}
		} else {
			status = "disconnected"
		}

		// Get pod count
		podCmd := exec.CommandContext(ctx, "kubectl", "--context", clusterID, "get", "pods", "-A", "--no-headers")
		if podOutput, err := podCmd.Output(); err == nil {
			lines := strings.Split(strings.TrimSpace(string(podOutput)), "\n")
			podCount = len(lines)
		}

		cluster := &rest.Cluster{
			ID:                   clusterID,
			Name:                 name,
			Status:               status,
			ContainersMonitored:  podCount,
			PredictionsGenerated: 0,
			AnomaliesDetected:    0,
			ModelVersion:         "v1.0.0",
			LastSeen:             time.Now(),
		}

		if err := store.RegisterCluster(ctx, cluster); err != nil {
			slog.Error("Failed to register kind cluster", "cluster", name, "error", err)
			continue
		}

		slog.Info("Discovered kind cluster",
			"cluster", name,
			"status", status,
			"nodes", nodeCount,
			"pods", podCount,
		)
	}

	return nil
}

// discoverKubeconfigClusters discovers clusters from kubeconfig
func discoverKubeconfigClusters(ctx context.Context, store *InMemoryClusterStore) error {
	// Get kubeconfig path
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		kubeconfigPath = filepath.Join(home, ".kube", "config")
	}

	// Check if kubeconfig exists
	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		return nil
	}

	// Get contexts from kubeconfig
	cmd := exec.CommandContext(ctx, "kubectl", "config", "get-contexts", "-o", "name")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	contexts := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, contextName := range contexts {
		contextName = strings.TrimSpace(contextName)
		if contextName == "" {
			continue
		}

		// Skip kind clusters (already discovered)
		if strings.HasPrefix(contextName, "kind-") {
			continue
		}

		// Check cluster health
		status := "healthy"
		nodeCount := 0

		nodeCmd := exec.CommandContext(ctx, "kubectl", "--context", contextName, "get", "nodes", "--no-headers")
		if nodeOutput, err := nodeCmd.Output(); err == nil {
			lines := strings.Split(strings.TrimSpace(string(nodeOutput)), "\n")
			nodeCount = len(lines)
			for _, line := range lines {
				if strings.Contains(line, "NotReady") {
					status = "degraded"
				}
			}
		} else {
			status = "disconnected"
		}

		cluster := &rest.Cluster{
			ID:                   contextName,
			Name:                 contextName,
			Status:               status,
			ContainersMonitored:  0,
			PredictionsGenerated: 0,
			AnomaliesDetected:    0,
			ModelVersion:         "v1.0.0",
			LastSeen:             time.Now(),
		}

		if err := store.RegisterCluster(ctx, cluster); err != nil {
			slog.Error("Failed to register kubeconfig cluster", "cluster", contextName, "error", err)
			continue
		}

		slog.Info("Discovered kubeconfig cluster",
			"cluster", contextName,
			"status", status,
			"nodes", nodeCount,
		)
	}

	return nil
}

// StartClusterDiscovery starts periodic cluster discovery
func StartClusterDiscovery(ctx context.Context, store *InMemoryClusterStore, interval time.Duration) {
	// Initial discovery
	if err := DiscoverClusters(ctx, store); err != nil {
		slog.Error("Initial cluster discovery failed", "error", err)
	}

	// Periodic discovery
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				if err := DiscoverClusters(ctx, store); err != nil {
					slog.Error("Cluster discovery failed", "error", err)
				}
			}
		}
	}()
}
