package network

import (
	"log"
	"testing"
	"time"
)

func TestSocketStats(t *testing.T) {
	var prof NetProfile
	prof.SocketStats()

	if len(prof.Sockets) == 0 {
		t.Error("Expected at least one socket stat, got none")
	}

	for _, s := range prof.Sockets {
		log.Printf("SocketStat: Protocol=%s, TotalSockets=%d, States=%v",
			s.Protocol, s.TotalSockets, s.StateCounts)
	}
}

func TestNetworkStats(t *testing.T) {
	var prof NetProfile

	// Create a dummy previous stat
	prev := make(map[string]NetworkInterfaceStat)
	// Call once to get current stats
	prof.NetworkStats(prev, 1.0)
	if len(prof.NetworkInterface) == 0 {
		t.Error("Expected at least one network interface stat, got none")
	}

	// Save current as previous
	for _, stat := range prof.NetworkInterface {
		prev[stat.Name] = stat
	}

	// Wait a second to create measurable difference
	time.Sleep(1 * time.Second)

	// Call again with previous data and deltaTime = 1 second
	prof.NetworkStats(prev, 1.0)

	for _, stat := range prof.NetworkInterface {
		log.Printf("NetworkInterfaceStat: %+v", stat)
		if stat.BpsSent < 0 || stat.BpsRecv < 0 {
			t.Errorf("Negative Bps detected for %s", stat.Name)
		}
	}
}
