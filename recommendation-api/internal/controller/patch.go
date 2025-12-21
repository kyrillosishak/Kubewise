package controller

import (
	"bytes"
	"fmt"
	"text/template"
)

// PatchGenerator generates Kubernetes YAML patches from recommendations
type PatchGenerator struct{}

// NewPatchGenerator creates a new PatchGenerator
func NewPatchGenerator() *PatchGenerator {
	return &PatchGenerator{}
}

// PatchData contains data for generating a patch
type PatchData struct {
	Namespace     string
	Name          string
	ContainerName string
	CPURequest    string
	CPULimit      string
	MemoryRequest string
	MemoryLimit   string
}

const strategicMergePatchTemplate = `apiVersion: apps/v1
kind: {{.Kind}}
metadata:
  name: {{.Name}}
  namespace: {{.Namespace}}
spec:
  template:
    spec:
      containers:
      - name: {{.ContainerName}}
        resources:
          requests:
            {{- if .CPURequest}}
            cpu: "{{.CPURequest}}"
            {{- end}}
            {{- if .MemoryRequest}}
            memory: "{{.MemoryRequest}}"
            {{- end}}
          limits:
            {{- if .CPULimit}}
            cpu: "{{.CPULimit}}"
            {{- end}}
            {{- if .MemoryLimit}}
            memory: "{{.MemoryLimit}}"
            {{- end}}
`

const jsonPatchTemplate = `[
  {"op": "replace", "path": "/spec/template/spec/containers/{{.ContainerIndex}}/resources/requests/cpu", "value": "{{.CPURequest}}"},
  {"op": "replace", "path": "/spec/template/spec/containers/{{.ContainerIndex}}/resources/requests/memory", "value": "{{.MemoryRequest}}"},
  {"op": "replace", "path": "/spec/template/spec/containers/{{.ContainerIndex}}/resources/limits/cpu", "value": "{{.CPULimit}}"},
  {"op": "replace", "path": "/spec/template/spec/containers/{{.ContainerIndex}}/resources/limits/memory", "value": "{{.MemoryLimit}}"}
]`

// StrategicMergePatchData contains data for strategic merge patch
type StrategicMergePatchData struct {
	Kind          string
	Name          string
	Namespace     string
	ContainerName string
	CPURequest    string
	CPULimit      string
	MemoryRequest string
	MemoryLimit   string
}

// JSONPatchData contains data for JSON patch
type JSONPatchData struct {
	ContainerIndex int
	CPURequest     string
	CPULimit       string
	MemoryRequest  string
	MemoryLimit    string
}

// GenerateStrategicMergePatch generates a strategic merge patch YAML
func (g *PatchGenerator) GenerateStrategicMergePatch(rec *ResourceRecommendation) (string, error) {
	tmpl, err := template.New("patch").Parse(strategicMergePatchTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	containerName := rec.Spec.TargetRef.ContainerName
	if containerName == "" {
		containerName = rec.Spec.TargetRef.Name // Default to workload name
	}

	data := StrategicMergePatchData{
		Kind:          rec.Spec.TargetRef.Kind,
		Name:          rec.Spec.TargetRef.Name,
		Namespace:     rec.Namespace,
		ContainerName: containerName,
		CPURequest:    rec.Spec.Recommendation.CPURequest,
		CPULimit:      rec.Spec.Recommendation.CPULimit,
		MemoryRequest: rec.Spec.Recommendation.MemoryRequest,
		MemoryLimit:   rec.Spec.Recommendation.MemoryLimit,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// GenerateJSONPatch generates a JSON patch for the recommendation
func (g *PatchGenerator) GenerateJSONPatch(rec *ResourceRecommendation, containerIndex int) (string, error) {
	tmpl, err := template.New("jsonpatch").Parse(jsonPatchTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	data := JSONPatchData{
		ContainerIndex: containerIndex,
		CPURequest:     rec.Spec.Recommendation.CPURequest,
		CPULimit:       rec.Spec.Recommendation.CPULimit,
		MemoryRequest:  rec.Spec.Recommendation.MemoryRequest,
		MemoryLimit:    rec.Spec.Recommendation.MemoryLimit,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// GenerateKubectlCommand generates a kubectl command to apply the recommendation
func (g *PatchGenerator) GenerateKubectlCommand(rec *ResourceRecommendation) string {
	return fmt.Sprintf(
		"kubectl patch %s %s -n %s --type=strategic -p '%s'",
		rec.Spec.TargetRef.Kind,
		rec.Spec.TargetRef.Name,
		rec.Namespace,
		g.generateInlinePatch(rec),
	)
}

func (g *PatchGenerator) generateInlinePatch(rec *ResourceRecommendation) string {
	containerName := rec.Spec.TargetRef.ContainerName
	if containerName == "" {
		containerName = rec.Spec.TargetRef.Name
	}

	return fmt.Sprintf(
		`{"spec":{"template":{"spec":{"containers":[{"name":"%s","resources":{"requests":{"cpu":"%s","memory":"%s"},"limits":{"cpu":"%s","memory":"%s"}}}]}}}}`,
		containerName,
		rec.Spec.Recommendation.CPURequest,
		rec.Spec.Recommendation.MemoryRequest,
		rec.Spec.Recommendation.CPULimit,
		rec.Spec.Recommendation.MemoryLimit,
	)
}
