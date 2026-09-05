//go:build linux

package platform

import (
	"errors"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/hkdb/aerion/internal/logging"
)

func TestConnectSleepWakeBusUsesSystemBus(t *testing.T) {
	want := &dbus.Conn{}
	got, usedHost, err := connectSleepWakeBusWith(
		func() (*dbus.Conn, error) { return want, nil },
		func(string, ...dbus.ConnOption) (*dbus.Conn, error) {
			t.Fatal("host bus should not be used")
			return nil, nil
		},
		false,
		false,
	)
	if err != nil || got != want || usedHost {
		t.Fatalf("connectSleepWakeBusWith() = (%p, %t, %v), want system bus", got, usedHost, err)
	}
}

func TestConnectSleepWakeBusContainerWithoutHostBusIsGraceful(t *testing.T) {
	systemErr := errors.New("system bus unavailable")
	_, _, err := connectSleepWakeBusWith(
		func() (*dbus.Conn, error) { return nil, systemErr },
		func(string, ...dbus.ConnOption) (*dbus.Conn, error) {
			t.Fatal("host bus should not be dialed")
			return nil, nil
		},
		true,
		false,
	)
	if !errors.Is(err, ErrSleepWakeMonitoringUnavailable) {
		t.Fatalf("error = %v, want ErrSleepWakeMonitoringUnavailable", err)
	}
}

func TestConnectSleepWakeBusPreservesNormalLinuxFailure(t *testing.T) {
	systemErr := errors.New("permission denied")
	_, _, err := connectSleepWakeBusWith(
		func() (*dbus.Conn, error) { return nil, systemErr },
		func(string, ...dbus.ConnOption) (*dbus.Conn, error) {
			t.Fatal("host bus should not be used")
			return nil, nil
		},
		false,
		false,
	)
	if !errors.Is(err, systemErr) {
		t.Fatalf("error = %v, want preserved system bus error", err)
	}
}

func TestLinuxSleepWakeMonitorWakeSignalEmitsOnce(t *testing.T) {
	monitor := NewSleepWakeMonitor().(*LinuxSleepWakeMonitor)
	log := logging.WithComponent("sleep-wake-test")
	monitor.handleSignal(&dbus.Signal{
		Name: "org.freedesktop.login1.Manager.PrepareForSleep",
		Body: []any{false},
	}, log)

	event := <-monitor.Events()
	if event.IsSleeping || event.Timestamp.IsZero() {
		t.Fatalf("wake event = %#v, want one timestamped wake event", event)
	}
	select {
	case extra := <-monitor.Events():
		t.Fatalf("unexpected second wake event: %#v", extra)
	default:
	}
}
