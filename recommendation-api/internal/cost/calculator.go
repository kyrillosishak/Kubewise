// Package cost provides cost estimation and pricing functionality
package cost

import (
	"context"
	"fmt"
	"time"
)

const (
	// HoursPerMonth is the average hours in a month for cost calculations
	HoursPerMonth = 730.0

	// BytesPerGB is the number of bytes in a gigabyte
	BytesPerGB = 1024 * 1024 * 1024

	// MillicoresPerCore is the number of millicores in a CPU core
	MillicoresPerCore = 1000
)

// ResourceUsage represents resource usage for cost calculation
type ResourceUsage struct {
	// Namespace is the Kubernetes namespace
	Namespace string

	// Deployment is the deployment name
	Deployment string

	// CPURequestMillicores is the CPU request in millicores
	CPURequestMillicores uint32

	// CPULimitMillicores is the CPU limit in millicores
	CPULimitMillicores uint32

	// MemoryRequestBytes is the memory request in bytes
	MemoryRequestBytes uint64

	// MemoryLimitBytes is the memory limit in bytes
	MemoryLimitBytes uint64

	// ReplicaCount is the number of replicas
	ReplicaCount int32
}

// CostBreakdown provides detailed cost breakdown
type CostBreakdown struct {
	// Namespace is the Kubernetes namespace
	Namespace string `json:"namespace"`

	// Deployment is the deployment name
	Deployment string `json:"deployment,omitempty"`

	// CPUCostMonthly is the monthly CPU cost
	CPUCostMonthly float64 `json:"cpu_cost_monthly"`

	// MemoryCostMonthly is the monthly memory cost
	MemoryCostMonthly float64 `json:"memory_cost_monthly"`

	// TotalCostMonthly is the total monthly cost
	TotalCostMonthly float64 `json:"total_cost_monthly"`

	// Currency is the currency code
	Currency string `json:"currency"`

	// CPUCores is the total CPU cores
	CPUCores float64 `json:"cpu_cores"`

	// MemoryGB is the total memory in GB
	MemoryGB float64 `json:"memory_gb"`

	// ReplicaCount is the number of replicas
	ReplicaCount int32 `json:"replica_count"`
}

// SavingsDelta represents the savings between current and recommended resources
type SavingsDelta struct {
	// CurrentCost is the current monthly cost
	CurrentCost CostBreakdown `json:"current_cost"`

	// RecommendedCost is the recommended monthly cost
	RecommendedCost CostBreakdown `json:"recommended_cost"`

	// MonthlySavings is the monthly savings amount
	MonthlySavings float64 `json:"monthly_savings"`

	// SavingsPercent is the savings as a percentage
	SavingsPercent float64 `json:"savings_percent"`

	// AnnualSavings is the projected annual savings
	AnnualSavings float64 `json:"annual_savings"`
}

// Calculator handles cost calculations
type Calculator struct {
	pricing *PricingConfig
}

// NewCalculator creates a new cost calculator
func NewCalculator(pricing *PricingConfig) *Calculator {
	return &Calculator{pricing: pricing}
}

// CalculateCost calculates the monthly cost for given resource usage
func (c *Calculator) CalculateCost(usage ResourceUsage) CostBreakdown {
	cpuPrice := c.pricing.GetCPUPrice(usage.Namespace)
	memPrice := c.pricing.GetMemoryPrice(usage.Namespace)

	// Convert millicores to cores
	cpuCores := float64(usage.CPURequestMillicores) / MillicoresPerCore

	// Convert bytes to GB
	memoryGB := float64(usage.MemoryRequestBytes) / BytesPerGB

	// Apply replica count
	replicas := int32(1)
	if usage.ReplicaCount > 0 {
		replicas = usage.ReplicaCount
	}

	totalCPUCores := cpuCores * float64(replicas)
	totalMemoryGB := memoryGB * float64(replicas)

	// Calculate monthly costs
	cpuCostMonthly := totalCPUCores * cpuPrice * HoursPerMonth
	memoryCostMonthly := totalMemoryGB * memPrice * HoursPerMonth

	return CostBreakdown{
		Namespace:         usage.Namespace,
		Deployment:        usage.Deployment,
		CPUCostMonthly:    roundToTwoDecimals(cpuCostMonthly),
		MemoryCostMonthly: roundToTwoDecimals(memoryCostMonthly),
		TotalCostMonthly:  roundToTwoDecimals(cpuCostMonthly + memoryCostMonthly),
		Currency:          c.pricing.Currency,
		CPUCores:          totalCPUCores,
		MemoryGB:          totalMemoryGB,
		ReplicaCount:      replicas,
	}
}

// CalculateSavings calculates the savings between current and recommended resources
func (c *Calculator) CalculateSavings(current, recommended ResourceUsage) SavingsDelta {
	currentCost := c.CalculateCost(current)
	recommendedCost := c.CalculateCost(recommended)

	monthlySavings := currentCost.TotalCostMonthly - recommendedCost.TotalCostMonthly

	var savingsPercent float64
	if currentCost.TotalCostMonthly > 0 {
		savingsPercent = (monthlySavings / currentCost.TotalCostMonthly) * 100
	}

	return SavingsDelta{
		CurrentCost:     currentCost,
		RecommendedCost: recommendedCost,
		MonthlySavings:  roundToTwoDecimals(monthlySavings),
		SavingsPercent:  roundToTwoDecimals(savingsPercent),
		AnnualSavings:   roundToTwoDecimals(monthlySavings * 12),
	}
}

// CalculateHourlyCost calculates the hourly cost for given resource usage
func (c *Calculator) CalculateHourlyCost(usage ResourceUsage) float64 {
	cpuPrice := c.pricing.GetCPUPrice(usage.Namespace)
	memPrice := c.pricing.GetMemoryPrice(usage.Namespace)

	cpuCores := float64(usage.CPURequestMillicores) / MillicoresPerCore
	memoryGB := float64(usage.MemoryRequestBytes) / BytesPerGB

	replicas := int32(1)
	if usage.ReplicaCount > 0 {
		replicas = usage.ReplicaCount
	}

	cpuCost := cpuCores * float64(replicas) * cpuPrice
	memoryCost := memoryGB * float64(replicas) * memPrice

	return roundToTwoDecimals(cpuCost + memoryCost)
}

// CalculateDailyCost calculates the daily cost for given resource usage
func (c *Calculator) CalculateDailyCost(usage ResourceUsage) float64 {
	return roundToTwoDecimals(c.CalculateHourlyCost(usage) * 24)
}

// ProjectCostForPeriod projects cost for a given time period
func (c *Calculator) ProjectCostForPeriod(usage ResourceUsage, duration time.Duration) float64 {
	hourlyRate := c.CalculateHourlyCost(usage)
	hours := duration.Hours()
	return roundToTwoDecimals(hourlyRate * hours)
}

// CalculateNamespaceCost calculates total cost for multiple deployments in a namespace
func (c *Calculator) CalculateNamespaceCost(usages []ResourceUsage) CostBreakdown {
	if len(usages) == 0 {
		return CostBreakdown{Currency: c.pricing.Currency}
	}

	var totalCPUCost, totalMemoryCost float64
	var totalCPUCores, totalMemoryGB float64
	namespace := usages[0].Namespace

	for _, usage := range usages {
		cost := c.CalculateCost(usage)
		totalCPUCost += cost.CPUCostMonthly
		totalMemoryCost += cost.MemoryCostMonthly
		totalCPUCores += cost.CPUCores
		totalMemoryGB += cost.MemoryGB
	}

	return CostBreakdown{
		Namespace:         namespace,
		CPUCostMonthly:    roundToTwoDecimals(totalCPUCost),
		MemoryCostMonthly: roundToTwoDecimals(totalMemoryCost),
		TotalCostMonthly:  roundToTwoDecimals(totalCPUCost + totalMemoryCost),
		Currency:          c.pricing.Currency,
		CPUCores:          totalCPUCores,
		MemoryGB:          totalMemoryGB,
		ReplicaCount:      int32(len(usages)),
	}
}

// CalculateClusterCost calculates total cost for all namespaces
func (c *Calculator) CalculateClusterCost(usagesByNamespace map[string][]ResourceUsage) map[string]CostBreakdown {
	result := make(map[string]CostBreakdown)

	for namespace, usages := range usagesByNamespace {
		result[namespace] = c.CalculateNamespaceCost(usages)
	}

	return result
}

// GetPricingConfig returns the current pricing configuration
func (c *Calculator) GetPricingConfig() *PricingConfig {
	return c.pricing.Clone()
}

// UpdatePricing updates the pricing configuration
func (c *Calculator) UpdatePricing(pricing *PricingConfig) error {
	if err := pricing.Validate(); err != nil {
		return fmt.Errorf("invalid pricing configuration: %w", err)
	}
	c.pricing = pricing
	return nil
}

// roundToTwoDecimals rounds a float to two decimal places
func roundToTwoDecimals(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}

// CostEstimator provides high-level cost estimation interface
type CostEstimator interface {
	CalculateCost(usage ResourceUsage) CostBreakdown
	CalculateSavings(current, recommended ResourceUsage) SavingsDelta
	CalculateNamespaceCost(usages []ResourceUsage) CostBreakdown
}

// Ensure Calculator implements CostEstimator
var _ CostEstimator = (*Calculator)(nil)

// EstimateFromRecommendation estimates savings from a recommendation
func (c *Calculator) EstimateFromRecommendation(
	ctx context.Context,
	namespace, deployment string,
	currentCPUMillicores, currentMemoryBytes uint32,
	recommendedCPUMillicores uint32, recommendedMemoryBytes uint64,
	replicaCount int32,
) SavingsDelta {
	current := ResourceUsage{
		Namespace:            namespace,
		Deployment:           deployment,
		CPURequestMillicores: currentCPUMillicores,
		MemoryRequestBytes:   uint64(currentMemoryBytes),
		ReplicaCount:         replicaCount,
	}

	recommended := ResourceUsage{
		Namespace:            namespace,
		Deployment:           deployment,
		CPURequestMillicores: recommendedCPUMillicores,
		MemoryRequestBytes:   recommendedMemoryBytes,
		ReplicaCount:         replicaCount,
	}

	return c.CalculateSavings(current, recommended)
}
