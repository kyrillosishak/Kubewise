// Package scheduler provides predefined test scenarios.
package scheduler

import "time"

// PredefinedScenarios contains all available test scenarios.
var PredefinedScenarios = map[ScenarioName]Scenario{
	ScenarioBaseline: {
		Name:        ScenarioBaseline,
		Description: "Baseline test with steady resource usage for prediction accuracy validation",
		Duration:    2 * time.Hour,
		Components: []ComponentConfig{
			{
				Name: "steady-worker",
				Mode: "steady",
				Config: map[string]interface{}{
					"cpuWorkMs":     10,
					"memoryAllocKB": 64,
					"baseMemoryMB":  128,
				},
			},
			{
				Name: "memory-hog",
				Mode: "steady",
				Config: map[string]interface{}{
					"targetMB": 256,
				},
			},
			{
				Name: "cpu-burster",
				Mode: "steady",
				Config: map[string]interface{}{
					"targetPercent": 20,
				},
			},
			{
				Name: "load-generator",
				Mode: "constant",
				Config: map[string]interface{}{
					"rps":       50,
					"targetURL": "http://steady-worker-svc:8084/work",
				},
			},
		},
		Phases: []Phase{},
	},
	ScenarioStress: {
		Name:        ScenarioStress,
		Description: "Stress test with high load and CPU spikes",
		Duration:    1 * time.Hour,
		Components: []ComponentConfig{
			{
				Name: "load-generator",
				Mode: "burst",
				Config: map[string]interface{}{
					"rps":           100,
					"burstRPS":      500,
					"burstInterval": "5m",
					"burstDuration": "30s",
					"targetURL":     "http://steady-worker-svc:8084/work",
				},
			},
			{
				Name: "cpu-burster",
				Mode: "spike",
				Config: map[string]interface{}{
					"targetPercent": 30,
					"spikePercent":  90,
					"spikeInterval": "5m",
					"spikeDuration": "30s",
				},
			},
		},
		Phases: []Phase{},
	},
	ScenarioAnomaly: {
		Name:        ScenarioAnomaly,
		Description: "Anomaly detection test with memory leaks and CPU spikes",
		Duration:    2 * time.Hour,
		Components: []ComponentConfig{
			{
				Name: "memory-hog",
				Mode: "steady",
				Config: map[string]interface{}{
					"targetMB": 128,
				},
			},
			{
				Name: "cpu-burster",
				Mode: "steady",
				Config: map[string]interface{}{
					"targetPercent": 20,
				},
			},
		},
		Phases: []Phase{
			{
				StartsAt:  0,
				Component: "memory-hog",
				Action:    "configure",
				Config: ComponentConfig{
					Name: "memory-hog",
					Mode: "leak",
					Config: map[string]interface{}{
						"leakRateMBMin": 10,
					},
				},
			},
			{
				StartsAt:  30 * time.Minute,
				Component: "cpu-burster",
				Action:    "configure",
				Config: ComponentConfig{
					Name: "cpu-burster",
					Mode: "spike",
					Config: map[string]interface{}{
						"spikePercent":  95,
						"spikeInterval": "2m",
						"spikeDuration": "20s",
					},
				},
			},
			{
				StartsAt:  60 * time.Minute,
				Component: "memory-hog",
				Action:    "configure",
				Config: ComponentConfig{
					Name: "memory-hog",
					Mode: "spike",
					Config: map[string]interface{}{
						"spikeSizeMB":   200,
						"spikeInterval": "3m",
						"spikeDuration": "30s",
					},
				},
			},
		},
	},
	ScenarioFullValidation: {
		Name:        ScenarioFullValidation,
		Description: "Full validation test combining all patterns with time acceleration",
		Duration:    4 * time.Hour,
		Components: []ComponentConfig{
			{
				Name: "steady-worker",
				Mode: "steady",
				Config: map[string]interface{}{
					"cpuWorkMs":     10,
					"memoryAllocKB": 64,
					"baseMemoryMB":  128,
				},
			},
			{
				Name: "memory-hog",
				Mode: "steady",
				Config: map[string]interface{}{
					"targetMB": 128,
				},
			},
			{
				Name: "cpu-burster",
				Mode: "steady",
				Config: map[string]interface{}{
					"targetPercent": 20,
				},
			},
			{
				Name: "load-generator",
				Mode: "constant",
				Config: map[string]interface{}{
					"rps":       30,
					"targetURL": "http://steady-worker-svc:8084/work",
				},
			},
		},
		Phases: []Phase{
			// Phase 1: Baseline (0-30min)
			{
				StartsAt:  0,
				Component: "load-generator",
				Action:    "start",
				Config:    ComponentConfig{},
			},
			// Phase 2: Ramp up load (30min)
			{
				StartsAt:  30 * time.Minute,
				Component: "load-generator",
				Action:    "configure",
				Config: ComponentConfig{
					Name: "load-generator",
					Mode: "ramp-up",
					Config: map[string]interface{}{
						"rampStartRPS": 30,
						"rampEndRPS":   150,
						"rampDuration": "15m",
					},
				},
			},
			// Phase 3: Memory leak (45min)
			{
				StartsAt:  45 * time.Minute,
				Component: "memory-hog",
				Action:    "configure",
				Config: ComponentConfig{
					Name: "memory-hog",
					Mode: "leak",
					Config: map[string]interface{}{
						"leakRateMBMin": 15,
					},
				},
			},
			// Phase 4: CPU spikes (1h)
			{
				StartsAt:  60 * time.Minute,
				Component: "cpu-burster",
				Action:    "configure",
				Config: ComponentConfig{
					Name: "cpu-burster",
					Mode: "spike",
					Config: map[string]interface{}{
						"spikePercent":  90,
						"spikeInterval": "3m",
						"spikeDuration": "20s",
					},
				},
			},
			// Phase 5: High load burst (1h30min)
			{
				StartsAt:  90 * time.Minute,
				Component: "load-generator",
				Action:    "configure",
				Config: ComponentConfig{
					Name: "load-generator",
					Mode: "burst",
					Config: map[string]interface{}{
						"rps":           100,
						"burstRPS":      500,
						"burstInterval": "5m",
						"burstDuration": "30s",
					},
				},
			},
			// Phase 6: Memory spikes (2h)
			{
				StartsAt:  120 * time.Minute,
				Component: "memory-hog",
				Action:    "configure",
				Config: ComponentConfig{
					Name: "memory-hog",
					Mode: "spike",
					Config: map[string]interface{}{
						"spikeSizeMB":   256,
						"spikeInterval": "4m",
						"spikeDuration": "45s",
					},
				},
			},
			// Phase 7: Wave CPU pattern (2h30min)
			{
				StartsAt:  150 * time.Minute,
				Component: "cpu-burster",
				Action:    "configure",
				Config: ComponentConfig{
					Name: "cpu-burster",
					Mode: "wave",
					Config: map[string]interface{}{
						"waveMin":    10,
						"waveMax":    80,
						"wavePeriod": "10m",
					},
				},
			},
			// Phase 8: Return to baseline (3h)
			{
				StartsAt:  180 * time.Minute,
				Component: "memory-hog",
				Action:    "configure",
				Config: ComponentConfig{
					Name: "memory-hog",
					Mode: "steady",
					Config: map[string]interface{}{
						"targetMB": 128,
					},
				},
			},
			{
				StartsAt:  180 * time.Minute,
				Component: "cpu-burster",
				Action:    "configure",
				Config: ComponentConfig{
					Name: "cpu-burster",
					Mode: "steady",
					Config: map[string]interface{}{
						"targetPercent": 20,
					},
				},
			},
			{
				StartsAt:  180 * time.Minute,
				Component: "load-generator",
				Action:    "configure",
				Config: ComponentConfig{
					Name: "load-generator",
					Mode: "constant",
					Config: map[string]interface{}{
						"rps": 30,
					},
				},
			},
		},
	},
}

// GetScenario returns a scenario by name.
func GetScenario(name ScenarioName) (Scenario, bool) {
	s, ok := PredefinedScenarios[name]
	return s, ok
}

// ListScenarios returns all available scenario names.
func ListScenarios() []ScenarioName {
	names := make([]ScenarioName, 0, len(PredefinedScenarios))
	for name := range PredefinedScenarios {
		names = append(names, name)
	}
	return names
}
