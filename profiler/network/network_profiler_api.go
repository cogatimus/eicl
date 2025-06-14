package main

import (
	"fmt"
	"log"
	"time"

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
	NetworkInterface []NetworkInterfaceStat
	Sockets          []SocketStat
}


// SocketStats populates the NetProfile's Sockets field by retrieving
// all current network connections on the system and aggregating them
// by protocol and connection state.
//
// It uses the gopsutil library to gather low-level socket data.
// If an error occurs while retrieving connections, it panics.
func (Netprof *NetProfile) SocketStats() {

	connections, err := net.Connections("all")
	if err != nil {
		panic(err)
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
	}

}
// TODO need to impelemt NetworkInterfaceStat method to get the Network Interface data








// wrote the main function with GPT for testing because I was lazy
func main() {
	var netprof NetProfile

	// Call your method
	netprof.SocketStats()

	// Check if any stats were found
	if len(netprof.Sockets) == 0 {
		log.Println("No socket stats found.")
		return
	}

	// Print ALL socket stats
	for i, socket := range netprof.Sockets {
		fmt.Printf("SocketStat #%d: %+v\n", i+1, socket)
	}
}
