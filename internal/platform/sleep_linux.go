//go:build linux

package platform

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/hkdb/aerion/internal/logging"
	"github.com/rs/zerolog"
)

const hostSystemBusSocket = "/run/host/run/dbus/system_bus_socket"

// LinuxSleepWakeMonitor monitors sleep/wake events on Linux using D-Bus
type LinuxSleepWakeMonitor struct {
	conn     *dbus.Conn
	events   chan SleepWakeEvent
	stopChan chan struct{}
	running  bool
}

// NewSleepWakeMonitor creates a new sleep/wake monitor for Linux
func NewSleepWakeMonitor() SleepWakeMonitor {
	return &LinuxSleepWakeMonitor{
		events:   make(chan SleepWakeEvent, 10),
		stopChan: make(chan struct{}),
	}
}

// Start begins monitoring for sleep/wake events via D-Bus
func (m *LinuxSleepWakeMonitor) Start(ctx context.Context) error {
	log := logging.WithComponent("sleep-wake")

	if m.running {
		return nil
	}

	conn, usedHostBus, err := connectSleepWakeBus()
	if err != nil {
		return err
	}
	m.conn = conn
	if usedHostBus {
		log.Debug().Msg("Using exposed host system D-Bus for sleep/wake monitoring")
	}

	// Subscribe to PrepareForSleep signal from systemd-logind
	// Signal: org.freedesktop.login1.Manager.PrepareForSleep(boolean going_to_sleep)
	// - true = system is about to sleep
	// - false = system just woke up
	matchRule := "type='signal',interface='org.freedesktop.login1.Manager',member='PrepareForSleep'"
	call := conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, matchRule)
	if call.Err != nil {
		conn.Close()
		m.conn = nil
		return call.Err
	}

	m.running = true

	// Start listening for signals in a goroutine
	go m.listenForSignals(ctx)

	log.Info().Msg("Sleep/wake monitor started (D-Bus)")
	return nil
}

// listenForSignals listens for D-Bus signals
func (m *LinuxSleepWakeMonitor) listenForSignals(ctx context.Context) {
	log := logging.WithComponent("sleep-wake")

	// Create signal channel
	signals := make(chan *dbus.Signal, 10)
	m.conn.Signal(signals)

	for {
		select {
		case <-ctx.Done():
			log.Debug().Msg("Context cancelled, stopping sleep/wake listener")
			return

		case <-m.stopChan:
			log.Debug().Msg("Stop requested, stopping sleep/wake listener")
			return

		case signal := <-signals:
			m.handleSignal(signal, log)
		}
	}
}

func (m *LinuxSleepWakeMonitor) handleSignal(signal *dbus.Signal, log zerolog.Logger) {
	if signal == nil || signal.Name != "org.freedesktop.login1.Manager.PrepareForSleep" || len(signal.Body) == 0 {
		return
	}
	isSleeping, ok := signal.Body[0].(bool)
	if !ok {
		return
	}
	event := SleepWakeEvent{IsSleeping: isSleeping, Timestamp: time.Now()}
	if isSleeping {
		log.Info().Msg("System is going to sleep")
	} else {
		log.Info().Msg("System woke from sleep")
	}
	select {
	case m.events <- event:
	default:
		log.Warn().Msg("Sleep/wake event channel full, dropping event")
	}
}

func connectSleepWakeBus() (*dbus.Conn, bool, error) {
	return connectSleepWakeBusWith(
		dbus.SystemBus,
		dbus.Connect,
		isContainerEnvironment(),
		hostSystemBusAvailable(),
	)
}

func connectSleepWakeBusWith(
	systemBus func() (*dbus.Conn, error),
	connect func(string, ...dbus.ConnOption) (*dbus.Conn, error),
	inContainer, hostBusAvailable bool,
) (*dbus.Conn, bool, error) {
	conn, err := systemBus()
	if err == nil {
		return conn, false, nil
	}
	if !inContainer {
		return nil, false, err
	}
	if !hostBusAvailable {
		return nil, false, fmt.Errorf("%w: container has no exposed system D-Bus", ErrSleepWakeMonitoringUnavailable)
	}
	conn, hostErr := connect("unix:path=" + hostSystemBusSocket)
	if hostErr != nil {
		return nil, false, fmt.Errorf("connect exposed host system D-Bus: %w", hostErr)
	}
	return conn, true, nil
}

func hostSystemBusAvailable() bool {
	_, err := os.Stat(hostSystemBusSocket)
	return err == nil
}

func isContainerEnvironment() bool {
	if os.Getenv("container") != "" {
		return true
	}
	for _, marker := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := os.Stat(marker); err == nil {
			return true
		}
	}
	return false
}

// Events returns the channel for receiving sleep/wake events
func (m *LinuxSleepWakeMonitor) Events() <-chan SleepWakeEvent {
	return m.events
}

// Stop stops the monitor and cleans up resources
func (m *LinuxSleepWakeMonitor) Stop() error {
	log := logging.WithComponent("sleep-wake")

	if !m.running {
		return nil
	}

	m.running = false

	// Signal stop
	close(m.stopChan)

	// Close D-Bus connection
	if m.conn != nil {
		m.conn.Close()
		m.conn = nil
	}

	log.Info().Msg("Sleep/wake monitor stopped")
	return nil
}
