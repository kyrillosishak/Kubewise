// Package scheduler provides business hours pattern scheduling.
package scheduler

import (
	"time"
)

// BusinessHoursConfig defines business hours pattern configuration.
type BusinessHoursConfig struct {
	// PeakStartHour is the start of peak hours (0-23)
	PeakStartHour int `json:"peakStartHour"`
	// PeakEndHour is the end of peak hours (0-23)
	PeakEndHour int `json:"peakEndHour"`
	// WeekendDays defines which days are weekends (0=Sunday, 6=Saturday)
	WeekendDays []time.Weekday `json:"weekendDays"`
	// PeakConfig is the component configuration during peak hours
	PeakConfig map[string]ComponentConfig `json:"peakConfig"`
	// OffPeakConfig is the component configuration during off-peak hours
	OffPeakConfig map[string]ComponentConfig `json:"offPeakConfig"`
}

// DefaultBusinessHoursConfig returns a default business hours configuration.
func DefaultBusinessHoursConfig() BusinessHoursConfig {
	return BusinessHoursConfig{
		PeakStartHour: 9,  // 9 AM
		PeakEndHour:   17, // 5 PM
		WeekendDays:   []time.Weekday{time.Saturday, time.Sunday},
		PeakConfig: map[string]ComponentConfig{
			"load-generator": {
				Name: "load-generator",
				Mode: "constant",
				Config: map[string]interface{}{
					"rps": 100,
				},
			},
			"cpu-burster": {
				Name: "cpu-burster",
				Mode: "steady",
				Config: map[string]interface{}{
					"targetPercent": 60,
				},
			},
			"memory-hog": {
				Name: "memory-hog",
				Mode: "steady",
				Config: map[string]interface{}{
					"targetMB": 384,
				},
			},
		},
		OffPeakConfig: map[string]ComponentConfig{
			"load-generator": {
				Name: "load-generator",
				Mode: "constant",
				Config: map[string]interface{}{
					"rps": 20,
				},
			},
			"cpu-burster": {
				Name: "cpu-burster",
				Mode: "steady",
				Config: map[string]interface{}{
					"targetPercent": 15,
				},
			},
			"memory-hog": {
				Name: "memory-hog",
				Mode: "steady",
				Config: map[string]interface{}{
					"targetMB": 128,
				},
			},
		},
	}
}

// BusinessHoursScheduler manages business hours patterns.
type BusinessHoursScheduler struct {
	config     BusinessHoursConfig
	timeConfig *TimeConfig
	isPeak     bool
}

// NewBusinessHoursScheduler creates a new business hours scheduler.
func NewBusinessHoursScheduler(config BusinessHoursConfig, timeConfig *TimeConfig) *BusinessHoursScheduler {
	return &BusinessHoursScheduler{
		config:     config,
		timeConfig: timeConfig,
	}
}

// IsPeakHours returns true if the simulated time is during peak hours.
func (b *BusinessHoursScheduler) IsPeakHours() bool {
	simTime := b.timeConfig.SimulatedNow()
	return b.isPeakTime(simTime)
}

// isPeakTime checks if a given time is during peak hours.
func (b *BusinessHoursScheduler) isPeakTime(t time.Time) bool {
	// Check if it's a weekend
	for _, weekend := range b.config.WeekendDays {
		if t.Weekday() == weekend {
			return false
		}
	}

	// Check if within peak hours
	hour := t.Hour()
	return hour >= b.config.PeakStartHour && hour < b.config.PeakEndHour
}

// GetCurrentConfig returns the appropriate configuration based on simulated time.
func (b *BusinessHoursScheduler) GetCurrentConfig() map[string]ComponentConfig {
	if b.IsPeakHours() {
		return b.config.PeakConfig
	}
	return b.config.OffPeakConfig
}

// CheckTransition checks if there's a transition between peak and off-peak.
// Returns (transitioned, isPeak).
func (b *BusinessHoursScheduler) CheckTransition() (bool, bool) {
	currentIsPeak := b.IsPeakHours()
	if currentIsPeak != b.isPeak {
		b.isPeak = currentIsPeak
		return true, currentIsPeak
	}
	return false, currentIsPeak
}

// GetPeakConfig returns the peak hours configuration.
func (b *BusinessHoursScheduler) GetPeakConfig() map[string]ComponentConfig {
	return b.config.PeakConfig
}

// GetOffPeakConfig returns the off-peak hours configuration.
func (b *BusinessHoursScheduler) GetOffPeakConfig() map[string]ComponentConfig {
	return b.config.OffPeakConfig
}

// GetTimeUntilTransition returns the duration until the next peak/off-peak transition.
func (b *BusinessHoursScheduler) GetTimeUntilTransition() time.Duration {
	simTime := b.timeConfig.SimulatedNow()
	isPeak := b.isPeakTime(simTime)

	if isPeak {
		// Calculate time until end of peak hours
		endOfPeak := time.Date(simTime.Year(), simTime.Month(), simTime.Day(),
			b.config.PeakEndHour, 0, 0, 0, simTime.Location())
		if simTime.After(endOfPeak) {
			// Already past peak end, next transition is tomorrow's peak start
			nextPeakStart := time.Date(simTime.Year(), simTime.Month(), simTime.Day()+1,
				b.config.PeakStartHour, 0, 0, 0, simTime.Location())
			return nextPeakStart.Sub(simTime)
		}
		return endOfPeak.Sub(simTime)
	}

	// Calculate time until start of peak hours
	hour := simTime.Hour()
	if hour < b.config.PeakStartHour {
		// Before peak hours today
		startOfPeak := time.Date(simTime.Year(), simTime.Month(), simTime.Day(),
			b.config.PeakStartHour, 0, 0, 0, simTime.Location())
		return startOfPeak.Sub(simTime)
	}

	// After peak hours, next peak is tomorrow
	nextPeakStart := time.Date(simTime.Year(), simTime.Month(), simTime.Day()+1,
		b.config.PeakStartHour, 0, 0, 0, simTime.Location())
	return nextPeakStart.Sub(simTime)
}

// GetStatus returns the current business hours status.
func (b *BusinessHoursScheduler) GetStatus() map[string]interface{} {
	simTime := b.timeConfig.SimulatedNow()
	isPeak := b.IsPeakHours()

	return map[string]interface{}{
		"simulatedTime":       simTime.Format(time.RFC3339),
		"simulatedHour":       simTime.Hour(),
		"simulatedWeekday":    simTime.Weekday().String(),
		"isPeakHours":         isPeak,
		"peakStartHour":       b.config.PeakStartHour,
		"peakEndHour":         b.config.PeakEndHour,
		"timeUntilTransition": b.GetTimeUntilTransition().String(),
	}
}
