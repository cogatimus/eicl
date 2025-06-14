package cpu

import (
	"sync"
	"time"

	"github.com/jaypipes/ghw"
)

// Collect static data once, and cache it to avoid repeated calls to ghw.CPU() and ghw.Topology().
// Give me in comments, more details about the control flow of my profiler
// The CPUStaticInfoCache struct is used to cache the static CPU metrics.
// It contains a map where the key is a string identifier (like CPU model name) and the value is the CPUStaticMetrics struct.
// The CPUStaticMetrics struct contains static information about the CPU and its topology.
// The cache is protected by a mutex to ensure thread safety when accessing or modifying the data.
// The NewCPUMetricsAtInstant function initializes a CPUMetricsAtInstant struct with static CPU metrics.
// It retrieves the CPU and topology information using the ghw library and initializes the dynamic metrics slices.

type CPUStaticMetrics struct {
	cpu      ghw.CPUInfo
	topology ghw.TopologyInfo
}

// How do i Cache the CPU static metrics?
// The CPUStaticMetrics struct contains static information about the CPU and its topology.
func NewCPUStaticMetrics() (*CPUStaticMetrics, error) {
	cpu, err := ghw.CPU()
	if err != nil {
		return nil, err
	}
	topology, err := ghw.Topology()
	if err != nil {
		return nil, err
	}
	return &CPUStaticMetrics{
		cpu:      *cpu,
		topology: *topology,
	}, nil
}

type CPUDynamicsMetrics struct {
	CPUUtilization []float64
	CPUFrequency   []float64
	CPUTemperature []float64
	CPUPower       []float64
	CacheUsage     [3]float64
	TotalCPUPower  int32
	Timestamp      time.Time
}

func NewCPUMetricsAtInstant(staticmetrics CPUStaticMetrics) *CPUDynamicsMetrics {
	// get the dynamic metrics
	timestamp := time.Now()
	return &CPUDynamicsMetrics{
		CPUUtilization: make([]float64, staticmetrics.cpu.TotalHardwareThreads),
		CPUFrequency:   make([]float64, staticmetrics.cpu.TotalCores),
		CPUTemperature: make([]float64, staticmetrics.cpu.TotalCores),
		CPUPower:       make([]float64, staticmetrics.cpu.TotalCores),
		CacheUsage:     [3]float64{0.0, 0.0, 0.0},
		TotalCPUPower:  int32(0),
		Timestamp:      timestamp,
	}
}

type CPUMetricsStream struct {
	CPUStaticMetrics  CPUStaticMetrics
	CPUDynamicMetrics []CPUDynamicsMetrics
	stopChan          chan struct{}
	mutex             sync.RWMutex
	stopOnce          sync.Once
}

// Now the profiling function which is a goroutine that polls the CPU metrics at regular intervals and stores them in the CPUMetricsAtInstant struct.
func (metricsStream *CPUMetricsStream) StartProfiling(interval time.Duration, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cpumetricsatinstant := NewCPUMetricsAtInstant(metricsStream.CPUStaticMetrics)

			metricsStream.mutex.Lock()
			metricsStream.CPUDynamicMetrics = append(metricsStream.CPUDynamicMetrics, *cpumetricsatinstant)
			metricsStream.mutex.Unlock()

		case <-metricsStream.stopChan:
			return
		}
	}
}

func (s *CPUMetricsStream) StopProfiling(wg *sync.WaitGroup) error {
	s.stopOnce.Do(func() {
		if s.stopChan != nil {
			close(s.stopChan)
		}
	})
	if wg != nil {
		wg.Done()
	}
	return nil
}

func NewCPUMetricStream() *CPUMetricsStream {
	cpuStaticMetrics, err := NewCPUStaticMetrics()
	if err != nil {
		panic(err)
	}
	metricsStream := &CPUMetricsStream{
		CPUStaticMetrics:  *cpuStaticMetrics,
		CPUDynamicMetrics: make([]CPUDynamicsMetrics, 0),
		stopChan:          make(chan struct{}),
	}

	return metricsStream
}
