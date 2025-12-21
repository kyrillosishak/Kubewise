// Package cost provides cost estimation and pricing functionality
package cost

import (
	"testing"
)

func TestNewPricingConfig(t *testing.T) {
	tests := []struct {
		name     string
		provider CloudProvider
		wantCPU  float64
		wantMem  float64
	}{
		{
			name:     "AWS pricing",
			provider: ProviderAWS,
			wantCPU:  0.0425,
			wantMem:  0.00533,
		},
		{
			name:     "GCP pricing",
			provider: ProviderGCP,
			wantCPU:  0.0335,
			wantMem:  0.00449,
		},
		{
			name:     "Azure pricing",
			provider: ProviderAzure,
			wantCPU:  0.0420,
			wantMem:  0.00525,
		},
		{
			name:     "On-premise pricing",
			provider: ProviderOnPrem,
			wantCPU:  0.030,
			wantMem:  0.004,
		},
		{
			name:     "Custom provider",
			provider: ProviderCustom,
			wantCPU:  0,
			wantMem:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewPricingConfig(tt.provider)
			if config.CPUPricePerCoreHour != tt.wantCPU {
				t.Errorf("CPUPricePerCoreHour = %v, want %v", config.CPUPricePerCoreHour, tt.wantCPU)
			}
			if config.MemoryPricePerGBHour != tt.wantMem {
				t.Errorf("MemoryPricePerGBHour = %v, want %v", config.MemoryPricePerGBHour, tt.wantMem)
			}
		})
	}
}

func TestCalculateCost(t *testing.T) {
	pricing := NewPricingConfig(ProviderAWS)
	calc := NewCalculator(pricing)

	tests := []struct {
		name     string
		usage    ResourceUsage
		wantCPU  float64
		wantMem  float64
		wantTotal float64
	}{
		{
			name: "1 core, 1GB memory",
			usage: ResourceUsage{
				Namespace:            "default",
				Deployment:           "test-app",
				CPURequestMillicores: 1000, // 1 core
				MemoryRequestBytes:   1073741824, // 1 GB
				ReplicaCount:         1,
			},
			wantCPU:   31.03, // 0.0425 * 730
			wantMem:   3.89,  // 0.00533 * 730
			wantTotal: 34.92,
		},
		{
			name: "500m CPU, 512Mi memory",
			usage: ResourceUsage{
				Namespace:            "default",
				Deployment:           "small-app",
				CPURequestMillicores: 500,
				MemoryRequestBytes:   536870912, // 512 MB
				ReplicaCount:         1,
			},
			wantCPU:   15.51,
			wantMem:   1.95,
			wantTotal: 17.46,
		},
		{
			name: "2 cores, 4GB memory, 3 replicas",
			usage: ResourceUsage{
				Namespace:            "production",
				Deployment:           "large-app",
				CPURequestMillicores: 2000,
				MemoryRequestBytes:   4294967296, // 4 GB
				ReplicaCount:         3,
			},
			wantCPU:   186.15, // 2 * 3 * 0.0425 * 730
			wantMem:   46.69,  // 4 * 3 * 0.00533 * 730
			wantTotal: 232.84,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := calc.CalculateCost(tt.usage)

			if cost.CPUCostMonthly != tt.wantCPU {
				t.Errorf("CPUCostMonthly = %v, want %v", cost.CPUCostMonthly, tt.wantCPU)
			}
			if cost.MemoryCostMonthly != tt.wantMem {
				t.Errorf("MemoryCostMonthly = %v, want %v", cost.MemoryCostMonthly, tt.wantMem)
			}
			if cost.TotalCostMonthly != tt.wantTotal {
				t.Errorf("TotalCostMonthly = %v, want %v", cost.TotalCostMonthly, tt.wantTotal)
			}
			if cost.Currency != "USD" {
				t.Errorf("Currency = %v, want USD", cost.Currency)
			}
		})
	}
}


func TestCalculateSavings(t *testing.T) {
	pricing := NewPricingConfig(ProviderAWS)
	calc := NewCalculator(pricing)

	tests := []struct {
		name           string
		current        ResourceUsage
		recommended    ResourceUsage
		wantSavings    float64
		wantPercent    float64
	}{
		{
			name: "30% CPU reduction",
			current: ResourceUsage{
				Namespace:            "default",
				CPURequestMillicores: 1000,
				MemoryRequestBytes:   1073741824,
				ReplicaCount:         1,
			},
			recommended: ResourceUsage{
				Namespace:            "default",
				CPURequestMillicores: 700,
				MemoryRequestBytes:   1073741824,
				ReplicaCount:         1,
			},
			wantSavings: 9.31,  // 30% of CPU cost
			wantPercent: 26.66,
		},
		{
			name: "50% memory reduction",
			current: ResourceUsage{
				Namespace:            "default",
				CPURequestMillicores: 1000,
				MemoryRequestBytes:   2147483648, // 2 GB
				ReplicaCount:         1,
			},
			recommended: ResourceUsage{
				Namespace:            "default",
				CPURequestMillicores: 1000,
				MemoryRequestBytes:   1073741824, // 1 GB
				ReplicaCount:         1,
			},
			wantSavings: 3.89,
			wantPercent: 10.02,
		},
		{
			name: "No change",
			current: ResourceUsage{
				Namespace:            "default",
				CPURequestMillicores: 1000,
				MemoryRequestBytes:   1073741824,
				ReplicaCount:         1,
			},
			recommended: ResourceUsage{
				Namespace:            "default",
				CPURequestMillicores: 1000,
				MemoryRequestBytes:   1073741824,
				ReplicaCount:         1,
			},
			wantSavings: 0,
			wantPercent: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delta := calc.CalculateSavings(tt.current, tt.recommended)

			if delta.MonthlySavings != tt.wantSavings {
				t.Errorf("MonthlySavings = %v, want %v", delta.MonthlySavings, tt.wantSavings)
			}
			if delta.SavingsPercent != tt.wantPercent {
				t.Errorf("SavingsPercent = %v, want %v", delta.SavingsPercent, tt.wantPercent)
			}
			if delta.AnnualSavings != roundToTwoDecimals(tt.wantSavings*12) {
				t.Errorf("AnnualSavings = %v, want %v", delta.AnnualSavings, tt.wantSavings*12)
			}
		})
	}
}

func TestCustomRates(t *testing.T) {
	pricing := NewPricingConfig(ProviderAWS)
	
	// Set custom rates for a namespace
	pricing.SetCustomRates("premium", ResourceRates{
		CPUPricePerCoreHour:  0.10,
		MemoryPricePerGBHour: 0.02,
	})

	calc := NewCalculator(pricing)

	// Test default namespace uses default rates
	defaultUsage := ResourceUsage{
		Namespace:            "default",
		CPURequestMillicores: 1000,
		MemoryRequestBytes:   1073741824,
		ReplicaCount:         1,
	}
	defaultCost := calc.CalculateCost(defaultUsage)

	// Test premium namespace uses custom rates
	premiumUsage := ResourceUsage{
		Namespace:            "premium",
		CPURequestMillicores: 1000,
		MemoryRequestBytes:   1073741824,
		ReplicaCount:         1,
	}
	premiumCost := calc.CalculateCost(premiumUsage)

	// Premium should be more expensive
	if premiumCost.TotalCostMonthly <= defaultCost.TotalCostMonthly {
		t.Errorf("Premium cost (%v) should be greater than default cost (%v)", 
			premiumCost.TotalCostMonthly, defaultCost.TotalCostMonthly)
	}

	// Verify premium rates
	expectedPremiumCPU := 73.0  // 0.10 * 730
	expectedPremiumMem := 14.6  // 0.02 * 730
	if premiumCost.CPUCostMonthly != expectedPremiumCPU {
		t.Errorf("Premium CPUCostMonthly = %v, want %v", premiumCost.CPUCostMonthly, expectedPremiumCPU)
	}
	if premiumCost.MemoryCostMonthly != expectedPremiumMem {
		t.Errorf("Premium MemoryCostMonthly = %v, want %v", premiumCost.MemoryCostMonthly, expectedPremiumMem)
	}
}

func TestPricingConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *PricingConfig
		wantErr bool
	}{
		{
			name: "Valid config",
			config: &PricingConfig{
				Provider:             ProviderAWS,
				Currency:             "USD",
				CPUPricePerCoreHour:  0.0425,
				MemoryPricePerGBHour: 0.00533,
				CustomRates:          make(map[string]ResourceRates),
			},
			wantErr: false,
		},
		{
			name: "Negative CPU price",
			config: &PricingConfig{
				Provider:             ProviderAWS,
				Currency:             "USD",
				CPUPricePerCoreHour:  -0.01,
				MemoryPricePerGBHour: 0.00533,
				CustomRates:          make(map[string]ResourceRates),
			},
			wantErr: true,
		},
		{
			name: "Negative memory price",
			config: &PricingConfig{
				Provider:             ProviderAWS,
				Currency:             "USD",
				CPUPricePerCoreHour:  0.0425,
				MemoryPricePerGBHour: -0.01,
				CustomRates:          make(map[string]ResourceRates),
			},
			wantErr: true,
		},
		{
			name: "Empty currency",
			config: &PricingConfig{
				Provider:             ProviderAWS,
				Currency:             "",
				CPUPricePerCoreHour:  0.0425,
				MemoryPricePerGBHour: 0.00533,
				CustomRates:          make(map[string]ResourceRates),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCalculateNamespaceCost(t *testing.T) {
	pricing := NewPricingConfig(ProviderAWS)
	calc := NewCalculator(pricing)

	usages := []ResourceUsage{
		{
			Namespace:            "production",
			Deployment:           "app1",
			CPURequestMillicores: 1000,
			MemoryRequestBytes:   1073741824,
			ReplicaCount:         2,
		},
		{
			Namespace:            "production",
			Deployment:           "app2",
			CPURequestMillicores: 500,
			MemoryRequestBytes:   536870912,
			ReplicaCount:         3,
		},
	}

	cost := calc.CalculateNamespaceCost(usages)

	if cost.Namespace != "production" {
		t.Errorf("Namespace = %v, want production", cost.Namespace)
	}
	if cost.TotalCostMonthly <= 0 {
		t.Error("TotalCostMonthly should be positive")
	}
	if cost.CPUCores <= 0 {
		t.Error("CPUCores should be positive")
	}
	if cost.MemoryGB <= 0 {
		t.Error("MemoryGB should be positive")
	}
}

func TestRoundToTwoDecimals(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{1.234, 1.23},
		{1.235, 1.24},
		{1.999, 2.0},
		{0.001, 0.0},
		{0.005, 0.01},
		{100.0, 100.0},
	}

	for _, tt := range tests {
		result := roundToTwoDecimals(tt.input)
		if result != tt.expected {
			t.Errorf("roundToTwoDecimals(%v) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}
