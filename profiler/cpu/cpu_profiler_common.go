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
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-yaml/yaml"
)

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

func (cpumetricsstream *CPUMetricsStream) String() string {
	return cpumetricsstream.ShortString()
}

func (cpumetricsstream *CPUMetricsStream) ShortString() string {
	return fmt.Sprintf("CPU Metrics Stream:\nStatic Metrics:\n%s\nDynamic Metrics Count: %d",
		cpumetricsstream.CPUStaticMetrics.ShortString(),
		len(cpumetricsstream.CPUDynamicMetrics),
	)
}

func (cpumetricsstream *CPUMetricsStream) JSONString() string {
	b, err := json.MarshalIndent(cpumetricsstream, "", "  ")
	if err != nil {
		return "ERROR: Failed to marshal CPUMetricsStream to JSON"
	}
	return string(b)
}

func (cpumetricsstream *CPUMetricsStream) YAMLString() string {
	b, err := yaml.Marshal(cpumetricsstream)
	if err != nil {
		return "ERROR: Failed to marshal CPUMetricsStream to YAML"
	}
	return string(b)
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
func (cpumetricsstream *CPUMetricsStream) StartProfiling(interval time.Duration, wg *sync.WaitGroup) error {
	if wg != nil {
		defer wg.Done()
	}

	// Validate interval
	if interval <= 0 {
		err := fmt.Errorf("invalid profiling interval: %v", interval)
		cpumetricsstream.logger.Printf("ERROR: %v", err)
		return err
	}

	if interval < 10*time.Millisecond {
		cpumetricsstream.logger.Printf("WARNING: Very short profiling interval (%v) may cause high CPU usage", interval)
	}

	cpumetricsstream.mutex.Lock()
	if cpumetricsstream.isRunning {
		cpumetricsstream.mutex.Unlock()
		err := fmt.Errorf("profiling is already running")
		cpumetricsstream.logger.Printf("ERROR: %v", err)
		return err
	}
	cpumetricsstream.isRunning = true
	cpumetricsstream.mutex.Unlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	metricsCollected := 0

	for {
		select {
		case <-ticker.C:

			// TODO: Replace with actual metric collection implementation
			cpumetricsatinstant, errs := NewCPUMetricsAtInstant(cpumetricsstream.CPUStaticMetrics)
			if len(errs) != 0 {
				cpumetricsstream.logger.Printf("ERROR: Failed to collect CPU metrics: %v", errs)
				continue // Skip this collection cycle but continue profiling
			}

			cpumetricsstream.mutex.Lock()
			cpumetricsstream.CPUDynamicMetrics = append(cpumetricsstream.CPUDynamicMetrics, *cpumetricsatinstant)
			metricsCollected++
			cpumetricsstream.mutex.Unlock()

		case <-cpumetricsstream.stopChan:
			cpumetricsstream.mutex.Lock()
			cpumetricsstream.isRunning = false
			cpumetricsstream.mutex.Unlock()

			cpumetricsstream.logger.Printf("INFO: CPU profiling stopped. Total metrics collected: %d", metricsCollected)
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
func (cpumetricsstream *CPUMetricsStream) StopProfiling(wg *sync.WaitGroup) error {
	cpumetricsstream.logger.Println("INFO: Stopping CPU profiling")

	var stopErr error
	cpumetricsstream.stopOnce.Do(func() {
		if cpumetricsstream.stopChan != nil {
			cpumetricsstream.logger.Println("DEBUG: Sending stop signal")
			close(cpumetricsstream.stopChan)
		} else {
			stopErr = fmt.Errorf("stop channel is nil")
			cpumetricsstream.logger.Printf("ERROR: %v", stopErr)
		}
	})

	if wg != nil {
		wg.Done()
		cpumetricsstream.logger.Println("DEBUG: WaitGroup signaled")
	}

	if stopErr != nil {
		return stopErr
	}

	cpumetricsstream.logger.Println("INFO: CPU profiling stop completed")
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
		cpuStaticMetrics.CPU.TotalCores, cpuStaticMetrics.CPU.TotalHardwareThreads)
	return stream, nil
}
