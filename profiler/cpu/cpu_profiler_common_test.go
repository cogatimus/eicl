// Package cpu provides comprehensive testing for the CPU profiler functionality.
// This test suite covers integration testing, concurrent access patterns,
// edge cases, and validation of both static and dynamic CPU metrics collection.
package cpu

import (
	"sync"
	"testing"
	"time"
)

// tolerance is the allowed error margin for timing intervals in tests.
const tolerance = 0.2 / 100.0

// TestCPUProfilerIntegration performs comprehensive integration testing of the CPU profiler.
// It validates the complete lifecycle: initialization, profiling start/stop, metrics collection,
// and data structure integrity.
func TestCPUProfilerIntegration(t *testing.T) {
	interval := 100 * time.Millisecond // Shorter interval for faster tests
	metricsStream := NewCPUMetricStream()

	// Test starting profiling
	t.Run("StartProfiling", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(1)

		go metricsStream.StartProfiling(interval, &wg)
		time.Sleep(500 * time.Millisecond) // Let it collect some metrics

		// Stop profiling with proper error handling
		if err := metricsStream.StopProfiling(nil); err != nil {
			t.Errorf("StopProfiling failed: %v", err)
		}
		wg.Wait() // Wait for the goroutine to finish
	})

	// Test stopping profiling
	t.Run("StopProfiling", func(t *testing.T) {
		metricsStream.mutex.RLock()
		metricsCount := len(metricsStream.CPUDynamicMetrics)
		metricsStream.mutex.RUnlock()

		if metricsCount == 0 {
			t.Error("Expected dynamic metrics to be collected")
		}
		t.Logf("Collected %d metrics", metricsCount)
	})

	// Test collected metrics structure
	t.Run("ValidateCollectedMetrics", func(t *testing.T) {
		metricsStream.mutex.RLock()
		defer metricsStream.mutex.RUnlock()
		isInitialized := false
		var previous_timestamp time.Time
		for i, metrics := range metricsStream.CPUDynamicMetrics {
			if metrics.Timestamp.IsZero() {
				t.Errorf("Metric %d has zero timestamp", i)
			}
			if !isInitialized {
				isInitialized = true
				previous_timestamp = metrics.Timestamp
			} else {
				// Validate the interval between metrics

				diff := metrics.Timestamp.Sub(previous_timestamp).Abs()
				allowedError := time.Duration(float64(interval) * tolerance)

				if diff > interval+allowedError || diff < interval-allowedError {
					t.Errorf("Expected interval ~%v (+/-%v), got: %v", interval, allowedError, diff)
				}
				previous_timestamp = metrics.Timestamp
			}
			if metrics.CPUUtilization == nil {
				t.Errorf("Metric %d missing CPU utilization", i)
			}
			if metrics.CPUFrequency == nil {
				t.Errorf("Metric %d missing CPU frequency", i)
			}
			if len(metrics.CPUUtilization) == 0 {
				t.Errorf("Metric %d has empty CPU utilization slice", i)
			}
			if len(metrics.CPUFrequency) == 0 {
				t.Errorf("Metric %d has empty CPU frequency slice", i)
			}
		}
	})

	// Test static metrics YAML output
	t.Run("StaticMetricsYAML", func(t *testing.T) {
		cpuYAML := metricsStream.CPUStaticMetrics.cpu.YAMLString()
		if cpuYAML == "" {
			t.Error("Expected non-empty CPU YAML string")
		}

		topologyYAML := metricsStream.CPUStaticMetrics.topology.YAMLString()
		if topologyYAML == "" {
			t.Error("Expected non-empty topology YAML string")
		}
	})
}

// TestCPUProfilerWithWaitGroup verifies proper synchronization using sync.WaitGroup.
// This test ensures that the profiler correctly signals completion and handles
// goroutine lifecycle management.
func TestCPUProfilerWithWaitGroup(t *testing.T) {
	interval := 100 * time.Millisecond
	metricsStream := NewCPUMetricStream()

	t.Run("ProfilingWithWaitGroup", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(1)

		go metricsStream.StartProfiling(interval, &wg)

		time.Sleep(300 * time.Millisecond)
		if err := metricsStream.StopProfiling(nil); err != nil {
			t.Errorf("StopProfiling failed: %v", err)
		}

		// Wait for the goroutine to finish
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Profiling completed successfully
		case <-time.After(2 * time.Second):
			t.Error("Profiling did not complete within timeout")
		}

		metricsStream.mutex.RLock()
		metricsCount := len(metricsStream.CPUDynamicMetrics)
		metricsStream.mutex.RUnlock()

		if metricsCount == 0 {
			t.Error("Expected to collect dynamic metrics")
		}
		t.Logf("Collected %d metrics with WaitGroup", metricsCount)
	})
}

// TestCPUMetricStreamCreation validates the proper initialization of CPUMetricStream.
// It verifies that all required fields are properly initialized and static metrics
// are collected during construction.
func TestCPUMetricStreamCreation(t *testing.T) {
	t.Run("NewCPUMetricStream", func(t *testing.T) {
		stream := NewCPUMetricStream()
		if stream == nil {
			t.Error("Expected non-nil CPU metric stream")
		}

		// Verify static metrics are populated
		if stream.CPUStaticMetrics.cpu.TotalCores == 0 {
			t.Error("Expected CPU static metrics to be populated with core count")
		}

		// Verify channels are initialized
		if stream.stopChan == nil {
			t.Error("Expected stop channel to be initialized")
		}

		// Verify dynamic metrics slice is initialized
		if stream.CPUDynamicMetrics == nil {
			t.Error("Expected dynamic metrics slice to be initialized")
		}
	})
}

// TestConcurrentAccess validates thread-safety of the CPU profiler.
// This test ensures that concurrent reads of metrics while profiling is active
// do not cause race conditions or data corruption.
func TestConcurrentAccess(t *testing.T) {
	t.Run("ConcurrentMetricsAccess", func(t *testing.T) {
		interval := 50 * time.Millisecond
		metricsStream := NewCPUMetricStream()

		var wg sync.WaitGroup
		wg.Add(1)

		// Start profiling
		go metricsStream.StartProfiling(interval, &wg)

		// Concurrently read metrics while profiling
		done := make(chan struct{})
		go func() {
			defer close(done)
			for i := 0; i < 10; i++ {
				metricsStream.mutex.RLock()
				_ = len(metricsStream.CPUDynamicMetrics)
				metricsStream.mutex.RUnlock()
				time.Sleep(30 * time.Millisecond)
			}
		}()

		time.Sleep(400 * time.Millisecond)
		if err := metricsStream.StopProfiling(nil); err != nil {
			t.Errorf("StopProfiling failed: %v", err)
		}
		wg.Wait()

		<-done // Wait for concurrent reader to finish

		metricsStream.mutex.RLock()
		metricsCount := len(metricsStream.CPUDynamicMetrics)
		metricsStream.mutex.RUnlock()

		if metricsCount == 0 {
			t.Error("Expected to collect metrics during concurrent access")
		}
		t.Logf("Collected %d metrics with concurrent access", metricsCount)
	})
}

// TestCPUProfilerEdgeCases tests edge cases and error conditions.
// This includes scenarios like stopping without starting, multiple stops,
// and other boundary conditions that should be handled gracefully.
func TestCPUProfilerEdgeCases(t *testing.T) {
	t.Run("StopWithoutStart", func(t *testing.T) {
		metricsStream := NewCPUMetricStream()
		// This should not panic
		if err := metricsStream.StopProfiling(nil); err != nil {
			t.Logf("Expected error when stopping without starting: %v", err)
		}
	})

	t.Run("MultipleStops", func(t *testing.T) {
		metricsStream := NewCPUMetricStream()
		var wg sync.WaitGroup
		wg.Add(1)

		go metricsStream.StartProfiling(100*time.Millisecond, &wg)
		time.Sleep(200 * time.Millisecond)

		// First stop
		if err := metricsStream.StopProfiling(nil); err != nil {
			t.Errorf("First StopProfiling failed: %v", err)
		}

		// Second stop should not panic
		if err := metricsStream.StopProfiling(nil); err != nil {
			t.Logf("Expected error on second stop: %v", err)
		}

		wg.Wait()
	})
}
