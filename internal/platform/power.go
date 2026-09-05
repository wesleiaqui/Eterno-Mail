// Package platform provides platform-specific functionality
package platform

import (
	"context"
	"errors"
	"time"
)

// ErrSleepWakeMonitoringUnavailable indicates that the current environment
// intentionally does not expose a system sleep/wake event source.
var ErrSleepWakeMonitoringUnavailable = errors.New("sleep/wake monitoring unavailable")

// PowerState represents the current power state
type PowerState string

const (
	PowerStateAC         PowerState = "ac"
	PowerStateBattery    PowerState = "battery"
	PowerStateLowBattery PowerState = "low-battery"
	PowerStateUnknown    PowerState = "unknown"
)

// PowerInfo contains information about the current power state
type PowerInfo struct {
	State             PowerState `json:"state"`
	BatteryPercentage int        `json:"batteryPercentage"` // -1 if unknown
	IsCharging        bool       `json:"isCharging"`
}

// PowerMonitor monitors power state changes
type PowerMonitor interface {
	// GetPowerInfo returns the current power state
	GetPowerInfo() (*PowerInfo, error)

	// Subscribe registers a callback for power state changes
	Subscribe(callback func(PowerInfo)) error

	// Unsubscribe removes the callback
	Unsubscribe() error

	// Close cleans up resources
	Close() error
}

// SleepWakeEvent represents a system sleep or wake event
type SleepWakeEvent struct {
	IsSleeping bool      // true = going to sleep, false = waking up
	Timestamp  time.Time // When the event occurred
}

// SleepWakeMonitor monitors system sleep/wake events
type SleepWakeMonitor interface {
	// Start begins monitoring for sleep/wake events
	Start(ctx context.Context) error

	// Events returns a channel that receives sleep/wake events
	Events() <-chan SleepWakeEvent

	// Stop stops the monitor and cleans up resources
	Stop() error
}

// NetworkEvent represents a network connectivity change
type NetworkEvent struct {
	Connected bool      // true = network available, false = offline
	Timestamp time.Time // When the event occurred
}

// NetworkMonitor monitors network connectivity changes.
// Implementations should be event-driven (zero polling) using OS APIs.
type NetworkMonitor interface {
	// Start begins monitoring for network connectivity changes
	Start(ctx context.Context) error

	// Events returns a channel that receives network connectivity change events
	Events() <-chan NetworkEvent

	// IsConnected returns the current connectivity state
	IsConnected() bool

	// WaitForConnection blocks until network is available or context is cancelled.
	// Returns true if connected, false if context was cancelled.
	WaitForConnection(ctx context.Context) bool

	// Invalidate resets the cached connectivity state to disconnected.
	// Call this when connections are known to be dead (e.g. system sleep)
	// so that WaitForConnection will wait for a fresh signal.
	Invalidate()

	// Stop stops the monitor and cleans up resources
	Stop() error
}
