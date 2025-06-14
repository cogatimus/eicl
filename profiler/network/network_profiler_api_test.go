package main

import (
	"testing"
	"time"
	"github.com/shirou/gopsutil/v4/net"
)

// Mock function for net.Connections
func mockConnections(t*testing.T,protocols []string, statuses []string) []net.ConnectionStat {
	var connections []net.ConnectionStat
	for _, proto := range protocols {
		for _, status := range statuses {
			connections = append(connections, net.ConnectionStat{
				Type:   protoToType(t,proto),
				Status: status,
			})
		}
	}
	return connections
}

// Helper function to convert protocol string to type
func protoToType(t *testing.T,proto string) uint32 {
	t.Helper()
	switch proto {
	case "tcp":
		return 1
	case "udp":
		return 2
	default:
		return 0
	}
}

func TestSocketStats(t *testing.T) {
	// Mock data
	protocols := []string{"tcp", "udp"}
	// statuses := []string{"ESTABLISHED", "TIME_WAIT", "CLOSE_WAIT"}

	// Replace net.Connections with mock function
	// netConnections := mockConnections(t,protocols, statuses)

	// Create NetProfile instance
	netprof := NetProfile{
		Timestamp: time.Now(),

	}

	// Call SocketStats method
	netprof.SocketStats()

	// Validate results
	if len(netprof.Sockets) != len(protocols) {
		t.Errorf("Expected %d protocols, got %d", len(protocols), len(netprof.Sockets))
	}

	for _, socket := range netprof.Sockets {
		if socket.TotalSockets == 0 {
			t.Errorf("Expected non-zero TotalSockets for protocol %s", socket.Protocol)
		}
	}
}
