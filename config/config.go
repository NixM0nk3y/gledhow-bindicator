package config

import (
	_ "embed"
	"net/netip"
	"strings"
	"time"
)

var (
	//go:embed broker.text
	brokerAddr string

	//go:embed clientid.text
	clientID string

	//go:embed telemetry_collector.text
	telemetryCollector string

	//go:embed wake_interval.text
	wakeIntervalStr string

	//go:embed schedule_refresh_interval.text
	scheduleRefreshIntervalStr string
)

// BrokerAddr returns the MQTT broker address from broker.text file.
// Format: "host:port" e.g., "192.168.1.100:1883"
func BrokerAddr() (netip.AddrPort, error) {
	addr := strings.TrimSpace(brokerAddr)
	return netip.ParseAddrPort(addr)
}

// ClientID returns the MQTT client ID from clientid.text file.
func ClientID() string {
	return strings.TrimSpace(clientID)
}

// TelemetryCollectorAddr returns the telemetry collector address from telemetry_collector.text file.
// Format: "host:port" e.g., "192.168.1.100:4318"
func TelemetryCollectorAddr() (netip.AddrPort, error) {
	addr := strings.TrimSpace(telemetryCollector)
	return netip.ParseAddrPort(addr)
}

// WakeInterval returns how often the device wakes to process LED states.
// This is the frequency at which LEDs are evaluated against the schedule.
// Default: 15m (from wake_interval.text)
func WakeInterval() (time.Duration, error) {
	return time.ParseDuration(strings.TrimSpace(wakeIntervalStr))
}

// ScheduleRefreshInterval returns how often the device fetches a new schedule from MQTT.
// This is separate from the wake interval to allow more responsive LED updates.
// Default: 3h (from schedule_refresh_interval.text)
func ScheduleRefreshInterval() (time.Duration, error) {
	return time.ParseDuration(strings.TrimSpace(scheduleRefreshIntervalStr))
}
