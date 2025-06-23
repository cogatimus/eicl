// Package cpu provides CPU profiling capabilities for collecting both static and dynamic CPU metrics.
// This package offers real-time monitoring of CPU utilization, frequency, temperature, power consumption,
// and cache usage across all CPU cores and hardware threads.
//
// The profiler operates in two phases:
// 1. Static metrics collection: Gathered once during initialization (CPU topology, core counts, etc.)
// 2. Dynamic metrics collection: Continuously sampled at specified intervals during profiling
//
// Thread-safety is ensured through mutex protection for all shared data structures.
package cpu

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jaypipes/ghw"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/sensors"
)

// CPUStaticMetrics holds immutable CPU hardware information collected once during initialization.
// This includes CPU topology, core counts, socket information, and hardware capabilities.
type CPUStaticMetrics struct {
	cpu      ghw.CPUInfo      // CPU hardware information and capabilities
	topology ghw.TopologyInfo // CPU socket and core topology layout
}

// NewCPUStaticMetrics initializes and collects static CPU metrics using the ghw library.
// This function is called once during CPUMetricsStream creation to cache hardware information.
//
// Returns:
//   - *CPUStaticMetrics: Populated static metrics structure
//   - error: Any error encountered during hardware information gathering
func NewCPUStaticMetrics() (*CPUStaticMetrics, error) {
	cpu, err := ghw.CPU()
	if err != nil {
		log.Printf("ERROR: Failed to collect CPU information: %v", err)
		return nil, fmt.Errorf("failed to collect CPU information: %w", err)
	}

	if cpu == nil {
		log.Println("ERROR: CPU information is nil")
		return nil, fmt.Errorf("CPU information is nil")
	}

	topology, err := ghw.Topology()
	if err != nil {
		log.Printf("ERROR: Failed to collect topology information: %v", err)
		return nil, fmt.Errorf("failed to collect topology information: %w", err)
	}

	if topology == nil {
		log.Println("ERROR: Topology information is nil")
		return nil, fmt.Errorf("topology information is nil")
	}

	log.Printf("INFO: Topology detected - Nodes: %d", len(topology.Nodes))
	log.Println("INFO: Static CPU metrics collection completed successfully")

	return &CPUStaticMetrics{
		cpu:      *cpu,
		topology: *topology,
	}, nil
}

// String provides a human-readable representation of the CPUStaticMetrics.
func (c *CPUStaticMetrics) String() string {
	return c.cpu.String() + "\n" + c.topology.String()
}

// CPUDynamicsMetrics represents a snapshot of CPU performance metrics at a specific point in time.
// All slice lengths are determined by the static CPU topology information.
type CPUDynamicsMetrics struct {
	CPUUtilization []float64  // Per-thread utilization percentage (0-100)
	CPUFrequency   []float64  // Per-core frequency in MHz
	CPUTemperature []float64  // Per-core temperature in Celsius
	CPUPower       []float64  // Per-core power consumption in watts
	CacheUsage     [3]float64 // L1, L2, L3 cache usage percentages
	TotalCPUPower  int32      // Total CPU package power consumption in watts
	Timestamp      time.Time  // Exact time when metrics were collected
	TempSource     string     // "per-core", "fallback", or "none"
	TempLabels     []string   // optional: what each temperature reading represents
}

func getCPUFrequencies() ([]float64, error) {
	freqStats, err := cpu.Info()
	if err != nil {
		return nil, err
	}

	freqs := make([]float64, len(freqStats))
	for i, stat := range freqStats {
		freqs[i] = stat.Mhz
	}
	return freqs, nil
}

func collectCPUTemperatures(numCores int, all []sensors.TemperatureStat) ([]float64, string, []string, error) {
	type entry struct {
		temp  float64
		label string
	}

	perCore := make([]entry, numCores)
	coreFound := 0
	var sharedCandidates []entry

	for _, sensor := range all {
		key := strings.ToLower(sensor.SensorKey)

		// Skip known non-CPU sensors
		switch {
		case strings.Contains(key, "it87"), strings.Contains(key, "nvme"),
			strings.Contains(key, "acpitz"), strings.Contains(key, "pch"),
			strings.Contains(key, "gpu"), strings.Contains(key, "ec"),
			strings.Contains(key, "tz"): // thermal zone
			continue
		}

		// Now strictly consider only likely CPU sensors
		entry := entry{temp: sensor.Temperature, label: sensor.SensorKey}

		switch {
		case strings.HasPrefix(key, "core"):
			// Intel: "core0", "core1", ...
			if i, err := strconv.Atoi(strings.TrimPrefix(key, "core")); err == nil && i < numCores {
				perCore[i] = entry
				coreFound++
				continue
			}

		case strings.Contains(key, "tccd"):
			// AMD: "k10temp_tccd0", etc.
			if i, err := strconv.Atoi(regexp.MustCompile(`\d+`).FindString(key)); err == nil && i < numCores {
				perCore[i] = entry
				coreFound++
				continue
			}

		case strings.Contains(key, "tctl"), strings.Contains(key, "package"),
			strings.Contains(key, "cpu"), strings.Contains(key, "tdie"):
			// Fallback CPU package sensor
			sharedCandidates = append(sharedCandidates, entry)
		}
	}

	// Use per-core if found sufficiently
	if coreFound >= numCores/2 {
		outTemps := make([]float64, numCores)
		outLabels := make([]string, numCores)
		for i := range perCore {
			outTemps[i] = perCore[i].temp
			outLabels[i] = perCore[i].label
		}
		return outTemps, "per-core", outLabels, nil
	}

	// Fallback: distribute shared CPU sensors
	if len(sharedCandidates) > 0 {
		outTemps := make([]float64, numCores)
		outLabels := make([]string, numCores)
		for i := range numCores {
			entry := sharedCandidates[i%len(sharedCandidates)]
			outTemps[i] = entry.temp
			outLabels[i] = fmt.Sprintf("%s (shared)", entry.label)
		}
		return outTemps, "shared", outLabels, nil
	}

	// No valid CPU temps
	return nil, "none", nil, fmt.Errorf("no usable CPU temperature sensors found (strict mode)")
}

type tempEntry struct {
	value float64
	index int
	label string
}

// NewCPUMetricsAtInstant creates a new CPUDynamicsMetrics instance with properly sized slices
// based on the provided static metrics. The slices are pre-allocated but not populated with data.
//
// Parameters:
//   - staticmetrics: Static CPU information used to determine slice sizes
//
// Returns:
//   - *CPUDynamicsMetrics: Initialized metrics structure with timestamp
func NewCPUMetricsAtInstant(staticmetrics CPUStaticMetrics) (*CPUDynamicsMetrics, []error) {
	timestamp := time.Now()
	numCores := staticmetrics.cpu.TotalCores
	numThreads := staticmetrics.cpu.TotalHardwareThreads

	metrics := &CPUDynamicsMetrics{
		CPUUtilization: make([]float64, numThreads),
		CPUFrequency:   make([]float64, numCores),
		CPUTemperature: make([]float64, numCores),
		CPUPower:       make([]float64, numCores),
		CacheUsage:     [3]float64{-1, -1, -1},
		TotalCPUPower:  0,
		Timestamp:      timestamp,
		TempSource:     "none",
	}

	var errslice []error

	// Utilization
	if cpuUtil, err := cpu.Percent(0, true); err == nil {
		metrics.CPUUtilization = cpuUtil
	} else {
		errslice = append(errslice, fmt.Errorf("cpu.Percent failed: %w", err))
	}

	// Frequency (placeholder — replace with per-core if needed)
	cpuFreqs, cpuFreqError := getCPUFrequencies()
	copy(metrics.CPUFrequency, cpuFreqs)

	allTemps, sensorserror := sensors.SensorsTemperatures()
	// Temperature
	cpuTemps, source, labels, temparsingerr := collectCPUTemperatures(int(staticmetrics.cpu.TotalCores), allTemps)

	if cpuFreqError != nil {
		errslice = append(errslice, cpuFreqError)
	}
	if temparsingerr != nil {
		errslice = append(errslice, sensorserror)
	} else {
		metrics.CPUTemperature = cpuTemps
		metrics.TempSource = source
		metrics.TempLabels = labels
	}
	metricsstring := metrics.String()
	fmt.Println(metricsstring)
	return metrics, errslice
}

func (cpumetrics *CPUDynamicsMetrics) String() string {
	return "CPU Utilization: " + fmt.Sprintf("%v", cpumetrics.CPUUtilization) +
		"\nCPU Frequency: " + fmt.Sprintf("%v", cpumetrics.CPUFrequency) +
		"\nCPU Temperature: " + fmt.Sprintf("%v", cpumetrics.CPUTemperature) +
		"\nCPU Power: " + fmt.Sprintf("%v", cpumetrics.CPUPower) +
		"\nCache Usage: L1: " + fmt.Sprintf("%.2f", cpumetrics.CacheUsage[0]) +
		", L2: " + fmt.Sprintf("%.2f", cpumetrics.CacheUsage[1]) +
		", L3: " + fmt.Sprintf("%.2f", cpumetrics.CacheUsage[2]) +
		"\nTotal CPU Power: " + fmt.Sprintf("%d", cpumetrics.TotalCPUPower) + "\nTemperature Sources: " +
		cpumetrics.TempSource + "\nTimestamp: " + cpumetrics.Timestamp.String()
}

// CPUMetricsStream manages the lifecycle and data collection of CPU profiling.
// It maintains both static CPU information and a time-series of dynamic metrics.
//
// Control Flow:
// 1. NewCPUMetricStream() -> Initializes static metrics and data structures
// 2. StartProfiling() -> Begins periodic metric collection in a goroutine
// 3. StopProfiling() -> Signals collection to stop and cleans up resources
//
// Thread Safety:
// - All access to CPUDynamicMetrics is protected by mutex
// - stopChan is used for clean goroutine termination
// - stopOnce ensures StopProfiling can be called multiple times safely
type CPUMetricsStream struct {
	CPUStaticMetrics  CPUStaticMetrics     // Immutable hardware information
	CPUDynamicMetrics []CPUDynamicsMetrics // Time-series of collected metrics
	stopChan          chan struct{}        // Signal channel for stopping profiling
	mutex             sync.RWMutex         // Protects CPUDynamicMetrics access
	stopOnce          sync.Once            // Ensures stopChan is closed only once
	logger            *log.Logger          // Logger for debugging and error reporting
	isRunning         bool                 // Tracks profiling state
}

func (metricsstream *CPUMetricsStream) String() string {
	return "CPU Metrics Stream:\n" +
		"Static Metrics: " + metricsstream.CPUStaticMetrics.String() +
		"\nDynamic Metrics Count: " + fmt.Sprintf("%d", len(metricsstream.CPUDynamicMetrics))
}

// StartProfiling begins continuous CPU metrics collection at the specified interval.
// This method runs in a separate goroutine and collects metrics until StopProfiling is called.
//
// Parameters:
//   - interval: Time between metric collection samples
//   - wg: Optional WaitGroup for synchronization (Done() called when profiling stops)
//
// Implementation Notes:
// TODO: Add actual CPU utilization collection via /proc/stat or similar
// TODO: Add CPU frequency collection via /sys/devices/system/cpu/
// TODO: Add temperature collection via thermal zones
// TODO: Add power consumption monitoring
// TODO: Add cache usage statistics collection
func (metricsstream *CPUMetricsStream) StartProfiling(interval time.Duration, wg *sync.WaitGroup) error {
	if wg != nil {
		defer wg.Done()
	}

	// Validate interval
	if interval <= 0 {
		err := fmt.Errorf("invalid profiling interval: %v", interval)
		metricsstream.logger.Printf("ERROR: %v", err)
		return err
	}

	if interval < 10*time.Millisecond {
		metricsstream.logger.Printf("WARNING: Very short profiling interval (%v) may cause high CPU usage", interval)
	}

	metricsstream.mutex.Lock()
	if metricsstream.isRunning {
		metricsstream.mutex.Unlock()
		err := fmt.Errorf("profiling is already running")
		metricsstream.logger.Printf("ERROR: %v", err)
		return err
	}
	metricsstream.isRunning = true
	metricsstream.mutex.Unlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	metricsCollected := 0

	for {
		select {
		case <-ticker.C:

			// TODO: Replace with actual metric collection implementation
			cpumetricsatinstant, err := NewCPUMetricsAtInstant(metricsstream.CPUStaticMetrics)
			if err != nil {
				metricsstream.logger.Printf("ERROR: Failed to collect CPU metrics: %v", err)
				continue // Skip this collection cycle but continue profiling
			}

			metricsstream.mutex.Lock()
			metricsstream.CPUDynamicMetrics = append(metricsstream.CPUDynamicMetrics, *cpumetricsatinstant)
			metricsCollected++
			metricsstream.mutex.Unlock()

		case <-metricsstream.stopChan:
			metricsstream.mutex.Lock()
			metricsstream.isRunning = false
			metricsstream.mutex.Unlock()

			metricsstream.logger.Printf("INFO: CPU profiling stopped. Total metrics collected: %d", metricsCollected)
			return nil
		}
	}
}

// StopProfiling signals the profiling goroutine to stop and performs cleanup.
// This method is safe to call multiple times and will not panic.
//
// Parameters:
//   - wg: Optional WaitGroup to signal completion (typically nil, as StartProfiling handles this)
//
// Returns:
//   - error: Any error encountered during profiling stop
func (metricsstream *CPUMetricsStream) StopProfiling(wg *sync.WaitGroup) error {
	metricsstream.logger.Println("INFO: Stopping CPU profiling")

	var stopErr error
	metricsstream.stopOnce.Do(func() {
		if metricsstream.stopChan != nil {
			metricsstream.logger.Println("DEBUG: Sending stop signal")
			close(metricsstream.stopChan)
		} else {
			stopErr = fmt.Errorf("stop channel is nil")
			metricsstream.logger.Printf("ERROR: %v", stopErr)
		}
	})

	if wg != nil {
		wg.Done()
		metricsstream.logger.Println("DEBUG: WaitGroup signaled")
	}

	if stopErr != nil {
		return stopErr
	}

	metricsstream.logger.Println("INFO: CPU profiling stop completed")
	return nil
}

// NewCPUMetricStream creates and initializes a new CPUMetricsStream instance.
// This constructor collects static CPU metrics and prepares data structures for profiling.
//
// Returns:
//   - *CPUMetricsStream: Fully initialized metrics stream ready for profiling
//
// Panics:
//   - If static CPU metrics collection fails (indicates system compatibility issues)
//
// TODO: Replace panic with proper error handling and return (stream, error)
func NewCPUMetricStream() (*CPUMetricsStream, error) {
	cpuStaticMetrics, err := NewCPUStaticMetrics()
	if err != nil {
		log.Printf("ERROR: Failed to initialize static CPU metrics: %v", err)
		return nil, fmt.Errorf("failed to initialize static CPU metrics: %w", err)
	}

	if cpuStaticMetrics == nil {
		err := fmt.Errorf("static CPU metrics is nil")
		log.Printf("ERROR: %v", err)
		return nil, err
	}

	// Use the standard logger for this example; you can customize as needed
	stdLogger := log.Default()

	stream := &CPUMetricsStream{
		CPUStaticMetrics:  *cpuStaticMetrics,
		CPUDynamicMetrics: make([]CPUDynamicsMetrics, 0),
		stopChan:          make(chan struct{}),
		stopOnce:          sync.Once{},
		logger:            stdLogger,
		isRunning:         false,
	}

	stdLogger.Printf("INFO: CPU metrics stream initialized successfully - Cores: %d, Threads: %d",
		cpuStaticMetrics.cpu.TotalCores, cpuStaticMetrics.cpu.TotalHardwareThreads)

	return stream, nil
}
