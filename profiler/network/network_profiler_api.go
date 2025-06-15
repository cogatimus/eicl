package network

import (
	"time"
	"log"
	"github.com/shirou/gopsutil/v4/net"
)

// NetworkInterfaceStat represents statistics for a network interface,
// including bytes sent/received, packets sent/received, errors, drops,
// and derived metrics such as bits per second and packets per second.
type NetworkInterfaceStat struct {
	Name        string
	BytesSent   uint64
	BytesRecv   uint64
	PacketsSent uint64
	PacketsRecv uint64
	ErrIn       uint64
	ErrOut      uint64
	DropIn      uint64
	DropOut     uint64

	// Derived per second
	BpsSent      float64
	BpsRecv      float64
	PpsSent      float64
	PpsRecv      float64
	ErrorRateIn  float64
	ErrorRateOut float64
}

// SocketStat represents aggregated statistics for socket connections
// for a specific protocol (e.g., TCP or UDP). It includes a map of
// connection states and their counts, as well as the total number of sockets.
type SocketStat struct {
	Protocol     string         // TCP/UDP
	StateCounts  map[string]int // ESTABLISHED, TIME_WAIT, etc.
	TotalSockets int
}

// NetProfile holds a snapshot of network profiling data, including a
// timestamp, network interface statistics, and socket connection statistics.
type NetProfile struct {
	Timestamp        time.Time
	Sockets          []SocketStat
	NetworkInterface []NetworkInterfaceStat
	
}

// SocketStats populates the NetProfile's Sockets field by retrieving
// all current network connections on the system and aggregating them
// by protocol and connection state.
//
// It uses the gopsutil library to gather low-level socket data.
// If an error occurs while retrieving connections, it panics.
func (Netprof *NetProfile) SocketStats() {
	log.Println("Starting SocketStats collection")
	connections, err := net.Connections("all")

	if err != nil {
		log.Fatalf("Failed to get network connections: %v", err)
	}

	protoMap := make(map[string]map[string]int)

	for _, conn := range connections {
		proto := conn.Type
		status := conn.Status

		var protoStr string

		switch proto {
		case 1:
			protoStr = "tcp"
		case 2:
			protoStr = "udp"
		default:
			protoStr = "unknown"
		}

		if _, ok := protoMap[protoStr]; !ok {
			protoMap[protoStr] = make(map[string]int)
		}

		protoMap[protoStr][status]++

	}

	for proto, stateCounts := range protoMap {
		total := 0
		for _, count := range stateCounts {
			total += count
		}
		Netprof.Sockets = append(Netprof.Sockets, SocketStat{
			Protocol:     proto,
			StateCounts:  stateCounts,
			TotalSockets: total,
		})
		log.Printf("Collected SocketStat for protocol %s: %+v\n", proto, stateCounts)
	}
	log.Println("Completed SocketStats collection")
}

// NetworkStats collects and updates network interface statistics.
//
// It retrieves I/O counters for all available network interfaces and populates
// the NetProfile's NetworkInterface slice with the current stats.
//
// If previous statistics are provided in `prev`, it calculates the per-second
// rates for bytes sent/received, packets sent/received, and error rates
// based on the given `deltaTime` (time elapsed in seconds).

func (Netprof *NetProfile) NetworkStats(prev map[string]NetworkInterfaceStat, deltaTime float64) {
	log.Println("Starting NetworkStats collection")
	// Get IO counters for all interfaces
	IOstats, err := net.IOCounters(true)
	if err != nil {
		log.Fatalf("Failed to get IO counters: %v", err)
	}

	// Reset current list
	Netprof.NetworkInterface = []NetworkInterfaceStat{}

	for _, io := range IOstats {
		name := io.Name

		stat := NetworkInterfaceStat{
			Name:        name,
			BytesSent:   io.BytesSent,
			BytesRecv:   io.BytesRecv,
			PacketsSent: io.PacketsSent,
			PacketsRecv: io.PacketsRecv,
			ErrIn:       io.Errin,
			ErrOut:      io.Errout,
			DropIn:      io.Dropin,
			DropOut:     io.Dropout,
		}

		// If previous data exists, compute per-second rates
		if p, ok := prev[name]; ok {
			stat.BpsSent = float64(io.BytesSent-p.BytesSent) / deltaTime
			stat.BpsRecv = float64(io.BytesRecv-p.BytesRecv) / deltaTime
			stat.PpsSent = float64(io.PacketsSent-p.PacketsSent) / deltaTime
			stat.PpsRecv = float64(io.PacketsRecv-p.PacketsRecv) / deltaTime
			stat.ErrorRateIn = float64(io.Errin-p.ErrIn) / deltaTime
			stat.ErrorRateOut = float64(io.Errout-p.ErrOut) / deltaTime
		}

		Netprof.NetworkInterface = append(Netprof.NetworkInterface, stat)
		log.Printf("Collected NetworkInterfaceStat for %s: %+v\n", name, stat)
	}
	log.Println("Completed NetworkStats collection")
}

