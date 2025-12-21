// Package cost provides cost estimation and pricing functionality
package cost

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// CloudProvider represents a supported cloud provider
type CloudProvider string

const (
	ProviderAWS    CloudProvider = "aws"
	ProviderGCP    CloudProvider = "gcp"
	ProviderAzure  CloudProvider = "azure"
	ProviderOnPrem CloudProvider = "on_premise"
	ProviderCustom CloudProvider = "custom"
)

// PricingConfig holds the pricing configuration for cost calculations
type PricingConfig struct {
	mu sync.RWMutex

	// Provider is the cloud provider type
	Provider CloudProvider `json:"provider"`

	// Region is the cloud region (e.g., us-east-1, us-central1)
	Region string `json:"region"`

	// Currency for cost calculations (default: USD)
	Currency string `json:"currency"`

	// CPUPricePerCoreHour is the price per CPU core per hour
	CPUPricePerCoreHour float64 `json:"cpu_price_per_core_hour"`

	// MemoryPricePerGBHour is the price per GB of memory per hour
	MemoryPricePerGBHour float64 `json:"memory_price_per_gb_hour"`

	// InstanceTypes holds pricing for specific instance types (optional)
	InstanceTypes map[string]InstancePricing `json:"instance_types,omitempty"`

	// CustomRates allows overriding rates per namespace or team
	CustomRates map[string]ResourceRates `json:"custom_rates,omitempty"`
}

// InstancePricing holds pricing for a specific instance type
type InstancePricing struct {
	Name             string  `json:"name"`
	CPUCores         float64 `json:"cpu_cores"`
	MemoryGB         float64 `json:"memory_gb"`
	PricePerHour     float64 `json:"price_per_hour"`
	SpotPricePerHour float64 `json:"spot_price_per_hour,omitempty"`
}

// ResourceRates holds custom resource rates
type ResourceRates struct {
	CPUPricePerCoreHour  float64 `json:"cpu_price_per_core_hour"`
	MemoryPricePerGBHour float64 `json:"memory_price_per_gb_hour"`
}

// DefaultPricingConfigs provides default pricing for major cloud providers
// Note: These are template configs without mutex - NewPricingConfig creates proper instances
var DefaultPricingConfigs = map[CloudProvider]struct {
	Provider             CloudProvider
	Region               string
	Currency             string
	CPUPricePerCoreHour  float64
	MemoryPricePerGBHour float64
}{
	ProviderAWS: {
		Provider:             ProviderAWS,
		Region:               "us-east-1",
		Currency:             "USD",
		CPUPricePerCoreHour:  0.0425,  // Based on m5.large pricing
		MemoryPricePerGBHour: 0.00533, // Based on m5.large pricing
	},
	ProviderGCP: {
		Provider:             ProviderGCP,
		Region:               "us-central1",
		Currency:             "USD",
		CPUPricePerCoreHour:  0.0335,  // Based on n2-standard pricing
		MemoryPricePerGBHour: 0.00449, // Based on n2-standard pricing
	},
	ProviderAzure: {
		Provider:             ProviderAzure,
		Region:               "eastus",
		Currency:             "USD",
		CPUPricePerCoreHour:  0.0420,  // Based on D2s v3 pricing
		MemoryPricePerGBHour: 0.00525, // Based on D2s v3 pricing
	},
	ProviderOnPrem: {
		Provider:             ProviderOnPrem,
		Region:               "default",
		Currency:             "USD",
		CPUPricePerCoreHour:  0.030, // Conservative on-premise estimate
		MemoryPricePerGBHour: 0.004, // Conservative on-premise estimate
	},
}

// NewPricingConfig creates a new pricing configuration with defaults
func NewPricingConfig(provider CloudProvider) *PricingConfig {
	if defaultConfig, ok := DefaultPricingConfigs[provider]; ok {
		return &PricingConfig{
			Provider:             defaultConfig.Provider,
			Region:               defaultConfig.Region,
			Currency:             defaultConfig.Currency,
			CPUPricePerCoreHour:  defaultConfig.CPUPricePerCoreHour,
			MemoryPricePerGBHour: defaultConfig.MemoryPricePerGBHour,
			InstanceTypes:        make(map[string]InstancePricing),
			CustomRates:          make(map[string]ResourceRates),
		}
	}

	// Return custom provider with zero rates (must be configured)
	return &PricingConfig{
		Provider:      ProviderCustom,
		Currency:      "USD",
		InstanceTypes: make(map[string]InstancePricing),
		CustomRates:   make(map[string]ResourceRates),
	}
}

// LoadFromFile loads pricing configuration from a JSON file
func LoadFromFile(path string) (*PricingConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read pricing config file: %w", err)
	}

	var config PricingConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse pricing config: %w", err)
	}

	if config.InstanceTypes == nil {
		config.InstanceTypes = make(map[string]InstancePricing)
	}
	if config.CustomRates == nil {
		config.CustomRates = make(map[string]ResourceRates)
	}
	if config.Currency == "" {
		config.Currency = "USD"
	}

	return &config, nil
}

// SaveToFile saves pricing configuration to a JSON file
func (p *PricingConfig) SaveToFile(path string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal pricing config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write pricing config file: %w", err)
	}

	return nil
}

// GetCPUPrice returns the CPU price per core per hour for a namespace
func (p *PricingConfig) GetCPUPrice(namespace string) float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if rates, ok := p.CustomRates[namespace]; ok && rates.CPUPricePerCoreHour > 0 {
		return rates.CPUPricePerCoreHour
	}
	return p.CPUPricePerCoreHour
}

// GetMemoryPrice returns the memory price per GB per hour for a namespace
func (p *PricingConfig) GetMemoryPrice(namespace string) float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if rates, ok := p.CustomRates[namespace]; ok && rates.MemoryPricePerGBHour > 0 {
		return rates.MemoryPricePerGBHour
	}
	return p.MemoryPricePerGBHour
}

// SetCustomRates sets custom rates for a namespace
func (p *PricingConfig) SetCustomRates(namespace string, rates ResourceRates) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.CustomRates[namespace] = rates
}

// RemoveCustomRates removes custom rates for a namespace
func (p *PricingConfig) RemoveCustomRates(namespace string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.CustomRates, namespace)
}

// SetInstancePricing sets pricing for a specific instance type
func (p *PricingConfig) SetInstancePricing(instanceType string, pricing InstancePricing) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.InstanceTypes[instanceType] = pricing
}

// GetInstancePricing returns pricing for a specific instance type
func (p *PricingConfig) GetInstancePricing(instanceType string) (InstancePricing, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	pricing, ok := p.InstanceTypes[instanceType]
	return pricing, ok
}

// Validate validates the pricing configuration
func (p *PricingConfig) Validate() error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.CPUPricePerCoreHour < 0 {
		return fmt.Errorf("CPU price per core hour cannot be negative")
	}
	if p.MemoryPricePerGBHour < 0 {
		return fmt.Errorf("memory price per GB hour cannot be negative")
	}
	if p.Currency == "" {
		return fmt.Errorf("currency must be specified")
	}

	for name, rates := range p.CustomRates {
		if rates.CPUPricePerCoreHour < 0 {
			return fmt.Errorf("custom CPU rate for %s cannot be negative", name)
		}
		if rates.MemoryPricePerGBHour < 0 {
			return fmt.Errorf("custom memory rate for %s cannot be negative", name)
		}
	}

	return nil
}

// Clone creates a deep copy of the pricing configuration
func (p *PricingConfig) Clone() *PricingConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()

	clone := &PricingConfig{
		Provider:             p.Provider,
		Region:               p.Region,
		Currency:             p.Currency,
		CPUPricePerCoreHour:  p.CPUPricePerCoreHour,
		MemoryPricePerGBHour: p.MemoryPricePerGBHour,
		InstanceTypes:        make(map[string]InstancePricing),
		CustomRates:          make(map[string]ResourceRates),
	}

	for k, v := range p.InstanceTypes {
		clone.InstanceTypes[k] = v
	}
	for k, v := range p.CustomRates {
		clone.CustomRates[k] = v
	}

	return clone
}
