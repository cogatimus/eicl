// Package cpu provides comprehensive testing for the CPU profiler functionality.
// This test suite covers integration testing, concurrent access patterns,
// edge cases, utility functions, data structures, and validation of all common cpu profiler stuff
// static and dynamic CPU metrics collection.
package cpu

import (
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// tolerance is the allowed error margin for timing intervals in tests.
const tolerance = 2 / 100.0

// TestCPUProfilerIntegration performs comprehensive integration testing of the CPU profiler.
// It validates the complete lifecycle: initialization, profiling start/stop, metrics collection,
// and data structure integrity.
func TestCPUProfilerIntegration(t *testing.T) {
	interval := 100 * time.Millisecond // Shorter interval for faster tests
	metricsStream, err := NewCPUMetricStream()
	if err != nil {
		t.Fatalf("Failed to create CPU metrics stream: %v", err)
	}

	// Test starting profiling
	t.Run("StartProfiling", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			if err := metricsStream.StartProfiling(interval, &wg); err != nil {
				t.Errorf("StartProfiling failed: %v", err)
			}
		}()
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
		var previousTimestamp time.Time
		for i, metrics := range metricsStream.CPUDynamicMetrics {
			if metrics.Timestamp.IsZero() {
				t.Errorf("Metric %d has zero timestamp", i)
			}
			if !isInitialized {
				isInitialized = true
				previousTimestamp = metrics.Timestamp
			} else {
				// Validate the interval between metrics
				diff := metrics.Timestamp.Sub(previousTimestamp).Abs()
				allowedError := time.Duration(float64(interval) * tolerance)

				if diff > interval+allowedError || diff < interval-allowedError {
					t.Errorf("Expected interval ~%v (+/-%v), got: %v", interval, allowedError, diff)
				}
				previousTimestamp = metrics.Timestamp
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
		cpuYAML := metricsStream.CPUStaticMetrics.CPU.YAMLString()
		if cpuYAML == "" {
			t.Error("Expected non-empty CPU YAML string")
		}

		topologyYAML := metricsStream.CPUStaticMetrics.Topology.YAMLString()
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
	metricsStream, err := NewCPUMetricStream()
	if err != nil {
		t.Fatalf("Failed to create CPU metrics stream: %v", err)
	}

	t.Run("ProfilingWithWaitGroup", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			if err := metricsStream.StartProfiling(interval, &wg); err != nil {
				t.Errorf("StartProfiling failed: %v", err)
			}
		}()

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
		if metricsCount != 3 {
			t.Errorf("Expected 3 metrics, got %d", metricsCount)
		}
		t.Logf("Collected %d metrics with WaitGroup", metricsCount)
	})
}

// TestCPUMetricStreamCreation validates the proper initialization of CPUMetricStream.
// It verifies that all required fields are properly initialized and static metrics
// are collected during construction.
func TestCPUMetricStreamCreation(t *testing.T) {
	t.Run("NewCPUMetricStream", func(t *testing.T) {
		stream, err := NewCPUMetricStream()
		if err != nil {
			t.Fatalf("Failed to create CPU metrics stream: %v", err)
		}
		if stream == nil {
			t.Fatalf("Expected non-nil CPU metric stream")
		}

		// Verify logger is initialized
		if stream.logger == nil {
			t.Error("Expected logger to be initialized")
		}

		// Verify static metrics are populated
		if stream.CPUStaticMetrics.CPU.TotalCores == 0 {
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
		metricsStream, err := NewCPUMetricStream()
		if err != nil {
			t.Fatalf("Failed to create CPU metrics stream: %v", err)
		}

		var wg sync.WaitGroup
		wg.Add(1)

		// Start profiling
		go func() {
			if err := metricsStream.StartProfiling(interval, &wg); err != nil {
				t.Errorf("StartProfiling failed: %v", err)
			}
		}()

		// Concurrently read metrics while profiling
		done := make(chan struct{})
		go func() {
			defer close(done)
			for range 10 {
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
		metricsStream, err := NewCPUMetricStream()
		if err != nil {
			t.Fatalf("Failed to create CPU metrics stream: %v", err)
		}
		// This should not panic
		if err := metricsStream.StopProfiling(nil); err != nil {
			t.Logf("Expected error when stopping without starting: %v", err)
		}
	})

	t.Run("MultipleStops", func(t *testing.T) {
		metricsStream, err := NewCPUMetricStream()
		if err != nil {
			t.Fatalf("Failed to create CPU metrics stream: %v", err)
		}
		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			if err := metricsStream.StartProfiling(100*time.Millisecond, &wg); err != nil {
				t.Errorf("StartProfiling failed: %v", err)
			}
		}()
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

	t.Run("InvalidInterval", func(t *testing.T) {
		metricsStream, err := NewCPUMetricStream()
		if err != nil {
			t.Fatalf("Failed to create CPU metrics stream: %v", err)
		}

		// Test zero interval
		err = metricsStream.StartProfiling(0, nil)
		if err == nil {
			t.Error("Expected error for zero interval")
		}

		// Test negative interval
		err = metricsStream.StartProfiling(-time.Second, nil)
		if err == nil {
			t.Error("Expected error for negative interval")
		}
	})

	t.Run("DoubleStart", func(t *testing.T) {
		metricsStream, err := NewCPUMetricStream()
		if err != nil {
			t.Fatalf("Failed to create CPU metrics stream: %v", err)
		}

		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			if err := metricsStream.StartProfiling(100*time.Millisecond, &wg); err != nil {
				t.Errorf("First StartProfiling failed: %v", err)
			}
		}()

		time.Sleep(50 * time.Millisecond)

		// Try to start again while running
		err = metricsStream.StartProfiling(100*time.Millisecond, nil)
		if err == nil {
			t.Error("Expected error when starting profiling twice")
		}

		// Clean up
		if err := metricsStream.StopProfiling(nil); err != nil {
			t.Errorf("StopProfiling failed: %v", err)
		}
		wg.Wait()
	})
}

// TestCPUStaticMetricsCreation tests the creation and validation of static CPU metrics.
func TestCPUStaticMetricsCreation(t *testing.T) {
	t.Run("NewCPUStaticMetrics", func(t *testing.T) {
		staticMetrics, err := NewCPUStaticMetrics()
		if err != nil {
			t.Fatalf("Failed to create CPU static metrics: %v", err)
		}
		if staticMetrics == nil {
			t.Fatalf("Expected non-nil CPU static metrics")
		}

		// Verify CPU info is populated
		if staticMetrics.CPU.TotalCores == 0 {
			t.Error("Expected non-zero total cores")
		}
		if staticMetrics.CPU.TotalHardwareThreads == 0 {
			t.Error("Expected non-zero total hardware threads")
		}

		// Verify topology info is populated
		if len(staticMetrics.Topology.Nodes) == 0 {
			t.Error("Expected non-empty topology nodes")
		}

		t.Logf("CPU: %d cores, %d threads", staticMetrics.CPU.TotalCores, staticMetrics.CPU.TotalHardwareThreads)
		t.Logf("Topology: %d nodes", len(staticMetrics.Topology.Nodes))
	})
}

// TestCPUStaticMetricsStringMethods tests all string representation methods.
func TestCPUStaticMetricsStringMethods(t *testing.T) {
	staticMetrics, err := NewCPUStaticMetrics()
	if err != nil {
		t.Fatalf("Failed to create CPU static metrics: %v", err)
	}

	t.Run("String", func(t *testing.T) {
		str := staticMetrics.String()
		if str == "" {
			t.Error("Expected non-empty string representation")
		}
		if !strings.Contains(str, "Extended Static CPU Info") {
			t.Error("Expected string to contain extended CPU info")
		}
	})

	t.Run("ShortString", func(t *testing.T) {
		shortStr := staticMetrics.ShortString()
		if shortStr == "" {
			t.Error("Expected non-empty short string representation")
		}
		if !strings.Contains(shortStr, "Family ID") {
			t.Error("Expected short string to contain Family ID")
		}
	})

	t.Run("JSONString", func(t *testing.T) {
		jsonStr := staticMetrics.JSONString()
		if jsonStr == "" {
			t.Error("Expected non-empty JSON string")
		}
		if strings.HasPrefix(jsonStr, "ERROR:") {
			t.Errorf("JSON serialization failed: %s", jsonStr)
		}
		if !strings.Contains(jsonStr, "cpu") || !strings.Contains(jsonStr, "topology") {
			t.Error("Expected JSON to contain cpu and topology fields")
		}
	})

	t.Run("YAMLString", func(t *testing.T) {
		yamlStr := staticMetrics.YAMLString()
		if yamlStr == "" {
			t.Error("Expected non-empty YAML string")
		}
		if strings.HasPrefix(yamlStr, "ERROR:") {
			t.Errorf("YAML serialization failed: %s", yamlStr)
		}
		if !strings.Contains(yamlStr, "cpu:") || !strings.Contains(yamlStr, "topology:") {
			t.Error("Expected YAML to contain cpu and topology fields")
		}
	})
}

// TestCPUDynamicMetricsCreation tests the creation of dynamic CPU metrics.
func TestCPUDynamicMetricsCreation(t *testing.T) {
	staticMetrics, err := NewCPUStaticMetrics()
	if err != nil {
		t.Fatalf("Failed to create CPU static metrics: %v", err)
	}

	t.Run("NewCPUMetricsAtInstant", func(t *testing.T) {
		dynamicMetrics, err := NewCPUMetricsAtInstant(*staticMetrics)
		if err != nil {
			// Note: This might have errors due to system limitations, but shouldn't be nil
			t.Logf("Warning: NewCPUMetricsAtInstant returned error (may be expected): %v", err)
		}
		if dynamicMetrics == nil {
			t.Fatalf("Expected non-nil dynamic metrics")
		}

		// Verify timestamp is set
		if dynamicMetrics.Timestamp.IsZero() {
			t.Error("Expected non-zero timestamp")
		}

		// Verify slices are properly sized
		expectedCores := int(staticMetrics.CPU.TotalCores)
		expectedThreads := int(staticMetrics.CPU.TotalHardwareThreads)

		if len(dynamicMetrics.CPUFrequency) != expectedCores {
			t.Errorf("Expected CPU frequency slice size %d, got %d", expectedCores, len(dynamicMetrics.CPUFrequency))
		}
		if len(dynamicMetrics.CPUTemperature) != expectedCores {
			t.Errorf("Expected CPU temperature slice size %d, got %d", expectedCores, len(dynamicMetrics.CPUTemperature))
		}
		if len(dynamicMetrics.CPUPower) != expectedCores {
			t.Errorf("Expected CPU power slice size %d, got %d", expectedCores, len(dynamicMetrics.CPUPower))
		}
		if len(dynamicMetrics.CPUUtilization) != expectedThreads {
			t.Errorf("Expected CPU utilization slice size %d, got %d", expectedThreads, len(dynamicMetrics.CPUUtilization))
		}

		t.Logf("Dynamic metrics created successfully with timestamp: %v", dynamicMetrics.Timestamp)
	})
}

// TestCPUDynamicMetricsStringMethods tests all string representation methods for dynamic metrics.
func TestCPUDynamicMetricsStringMethods(t *testing.T) {
	staticMetrics, err := NewCPUStaticMetrics()
	if err != nil {
		t.Fatalf("Failed to create CPU static metrics: %v", err)
	}

	dynamicMetrics, err := NewCPUMetricsAtInstant(*staticMetrics)
	if err != nil {
		t.Logf("Warning: NewCPUMetricsAtInstant returned error: %v", err)
	}
	if dynamicMetrics == nil {
		t.Fatalf("Expected non-nil dynamic metrics")
	}

	// Add some mock cache data to avoid nil pointer dereference in ShortString
	dynamicMetrics.CacheUsage = []float64{0.1, 0.2, 0.3}

	t.Run("String", func(t *testing.T) {
		str := dynamicMetrics.String()
		if str == "" {
			t.Error("Expected non-empty string representation")
		}
	})

	t.Run("ShortString", func(t *testing.T) {
		shortStr := dynamicMetrics.ShortString()
		if shortStr == "" {
			t.Error("Expected non-empty short string representation")
		}
		if !strings.Contains(shortStr, "CPU Utilization") {
			t.Error("Expected short string to contain CPU Utilization")
		}
		if !strings.Contains(shortStr, "Temperature Source") {
			t.Error("Expected short string to contain Temperature Source")
		}
	})

	t.Run("JSONString", func(t *testing.T) {
		jsonStr := dynamicMetrics.JSONString()
		if jsonStr == "" {
			t.Error("Expected non-empty JSON string")
		}
		if strings.HasPrefix(jsonStr, "ERROR:") {
			t.Errorf("JSON serialization failed: %s", jsonStr)
		}
		if !strings.Contains(jsonStr, "cpu_utilization") || !strings.Contains(jsonStr, "timestamp") {
			t.Error("Expected JSON to contain cpu_utilization and timestamp fields")
		}
	})

	t.Run("YAMLString", func(t *testing.T) {
		yamlStr := dynamicMetrics.YAMLString()
		if yamlStr == "" {
			t.Error("Expected non-empty YAML string")
		}
		if strings.HasPrefix(yamlStr, "ERROR:") {
			t.Errorf("YAML serialization failed: %s", yamlStr)
		}
		if !strings.Contains(yamlStr, "cpu_utilization:") || !strings.Contains(yamlStr, "timestamp:") {
			t.Error("Expected YAML to contain cpu_utilization and timestamp fields")
		}
	})
}

// TestCPUMetricsStreamStringMethods tests all string representation methods for the metrics stream.
func TestCPUMetricsStreamStringMethods(t *testing.T) {
	metricsStream, err := NewCPUMetricStream()
	if err != nil {
		t.Fatalf("Failed to create CPU metrics stream: %v", err)
	}

	t.Run("String", func(t *testing.T) {
		str := metricsStream.String()
		if str == "" {
			t.Error("Expected non-empty string representation")
		}
	})

	t.Run("ShortString", func(t *testing.T) {
		shortStr := metricsStream.ShortString()
		if shortStr == "" {
			t.Error("Expected non-empty short string representation")
		}
		if !strings.Contains(shortStr, "CPU Metrics Stream") {
			t.Error("Expected short string to contain 'CPU Metrics Stream'")
		}
		if !strings.Contains(shortStr, "Dynamic Metrics Count") {
			t.Error("Expected short string to contain 'Dynamic Metrics Count'")
		}
	})

	t.Run("JSONString", func(t *testing.T) {
		jsonStr := metricsStream.JSONString()
		if jsonStr == "" {
			t.Error("Expected non-empty JSON string")
		}
		if strings.HasPrefix(jsonStr, "ERROR:") {
			t.Errorf("JSON serialization failed: %s", jsonStr)
		}
		if !strings.Contains(jsonStr, "CPUStaticMetrics") || !strings.Contains(jsonStr, "CPUDynamicMetrics") {
			t.Error("Expected JSON to contain static and dynamic metrics fields")
		}
	})

	t.Run("YAMLString", func(t *testing.T) {
		yamlStr := metricsStream.YAMLString()
		if yamlStr == "" {
			t.Error("Expected non-empty YAML string")
		}
		if strings.HasPrefix(yamlStr, "ERROR:") {
			t.Errorf("YAML serialization failed: %s", yamlStr)
		}
		if !strings.Contains(yamlStr, "cpustaticmetrics:") || !strings.Contains(yamlStr, "cpudynamicmetrics:") {
			t.Error("Expected YAML to contain static and dynamic metrics fields")
		}
	})
}

// TestUtilityFunctions tests individual utility functions.
func TestUtilityFunctions(t *testing.T) {
	t.Run("getCPUFrequencies", func(t *testing.T) {
		freqs, err := getCPUFrequencies()
		if err != nil {
			t.Logf("Warning: getCPUFrequencies failed (may be expected on some systems): %v", err)
			return
		}
		if len(freqs) == 0 {
			t.Error("Expected at least one CPU frequency reading")
		}
		for i, freq := range freqs {
			if freq <= 0 {
				t.Errorf("Expected positive frequency for CPU %d, got %f", i, freq)
			}
		}
		t.Logf("CPU frequencies: %v", freqs)
	})

	t.Run("collectCPUUtilization", func(t *testing.T) {
		utilization, err := collectCPUUtilization()
		if err != nil {
			t.Logf("Warning: collectCPUUtilization failed (may be expected on some systems): %v", err)
			return
		}
		if len(utilization) == 0 {
			t.Error("Expected at least one CPU utilization reading")
		}
		for i, util := range utilization {
			if util < 0 || util > 100 {
				t.Errorf("Expected utilization between 0-100 for CPU %d, got %f", i, util)
			}
		}
		t.Logf("CPU utilization: %v", utilization)
	})

	t.Run("collectCPUTemperatures", func(t *testing.T) {
		// Test with different core counts
		for _, numCores := range []int{1, 2, 4, 8} {
			temps, source, labels, err := collectCPUTemperatures(numCores)
			if err != nil {
				t.Logf("Warning: collectCPUTemperatures failed for %d cores (may be expected): %v", numCores, err)
				continue
			}
			if len(temps) != numCores {
				t.Errorf("Expected %d temperature readings, got %d", numCores, len(temps))
			}
			if source == "" {
				t.Error("Expected non-empty temperature source")
			}
			if len(labels) != numCores {
				t.Errorf("Expected %d temperature labels, got %d", numCores, len(labels))
			}
			t.Logf("Temperatures for %d cores: %v (source: %s)", numCores, temps, source)
			break // If one succeeds, don't test others
		}
	})
}

// TestErrorHandling tests various error conditions and edge cases.
func TestErrorHandling(t *testing.T) {
	t.Run("VeryShortInterval", func(t *testing.T) {
		metricsStream, err := NewCPUMetricStream()
		if err != nil {
			t.Fatalf("Failed to create CPU metrics stream: %v", err)
		}

		// Test very short interval (should warn but not error)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			if err := metricsStream.StartProfiling(5*time.Millisecond, &wg); err != nil {
				t.Errorf("StartProfiling with short interval failed: %v", err)
			}
		}()

		time.Sleep(50 * time.Millisecond)
		if err := metricsStream.StopProfiling(nil); err != nil {
			t.Errorf("StopProfiling failed: %v", err)
		}
		wg.Wait()
	})

	t.Run("StopProfilingWithWaitGroup", func(t *testing.T) {
		metricsStream, err := NewCPUMetricStream()
		if err != nil {
			t.Fatalf("Failed to create CPU metrics stream: %v", err)
		}

		var wg sync.WaitGroup
		wg.Add(1)

		// Test StopProfiling with WaitGroup
		err = metricsStream.StopProfiling(&wg)
		if err != nil {
			t.Logf("StopProfiling with WaitGroup returned error (expected): %v", err)
		}

		// Wait for WaitGroup (should complete immediately)
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(1 * time.Second):
			t.Error("WaitGroup did not complete within timeout")
		}
	})
}

// TestDataIntegrity tests that collected data maintains integrity across operations.
func TestDataIntegrity(t *testing.T) {
	t.Run("MetricsConsistency", func(t *testing.T) {
		staticMetrics, err := NewCPUStaticMetrics()
		if err != nil {
			t.Fatalf("Failed to create CPU static metrics: %v", err)
		}

		// Create multiple dynamic metrics and ensure they're consistent
		for i := range 5 {
			dynamicMetrics, err := NewCPUMetricsAtInstant(*staticMetrics)
			if err != nil {
				t.Logf("Warning: NewCPUMetricsAtInstant iteration %d returned error: %v", i, err)
			}
			if dynamicMetrics == nil {
				t.Fatalf("Expected non-nil dynamic metrics at iteration %d", i)
			}

			// Verify slice sizes are consistent
			expectedCores := int(staticMetrics.CPU.TotalCores)
			expectedThreads := int(staticMetrics.CPU.TotalHardwareThreads)

			if len(dynamicMetrics.CPUFrequency) != expectedCores {
				t.Errorf("Iteration %d: inconsistent CPU frequency slice size", i)
			}
			if len(dynamicMetrics.CPUTemperature) != expectedCores {
				t.Errorf("Iteration %d: inconsistent CPU temperature slice size", i)
			}
			if len(dynamicMetrics.CPUUtilization) != expectedThreads {
				t.Errorf("Iteration %d: inconsistent CPU utilization slice size", i)
			}

			time.Sleep(10 * time.Millisecond) // Small delay between collections
		}
	})

	t.Run("TimestampProgression", func(t *testing.T) {
		staticMetrics, err := NewCPUStaticMetrics()
		if err != nil {
			t.Fatalf("Failed to create CPU static metrics: %v", err)
		}

		var timestamps []time.Time
		for i := range 3 {
			dynamicMetrics, err := NewCPUMetricsAtInstant(*staticMetrics)
			if err != nil {
				t.Logf("Warning: NewCPUMetricsAtInstant iteration %d returned error: %v", i, err)
			}
			if dynamicMetrics == nil {
				t.Fatalf("Expected non-nil dynamic metrics at iteration %d", i)
			}
			timestamps = append(timestamps, dynamicMetrics.Timestamp)
			time.Sleep(50 * time.Millisecond)
		}

		// Verify timestamps are in ascending order
		for i := 1; i < len(timestamps); i++ {
			if timestamps[i].Before(timestamps[i-1]) {
				t.Errorf("Timestamps not in ascending order: %v should be after %v",
					timestamps[i], timestamps[i-1])
			}
		}
	})
}

// TestTemperatureCollection tests temperature collection edge cases.
func TestTemperatureCollection(t *testing.T) {
	t.Run("TemperatureCollectionEdgeCases", func(t *testing.T) {
		// Test with zero cores (should return error)
		temps, source, labels, err := collectCPUTemperatures(0)
		if err == nil {
			t.Error("Expected error when collecting temperatures for 0 cores")
		}
		if temps != nil {
			t.Error("Expected nil temperatures for 0 cores")
		}
		if source != "none" {
			t.Error("Expected 'none' source for 0 cores")
		}
		if labels != nil {
			t.Error("Expected nil labels for 0 cores")
		}

		// Test with negative cores (should return error)
		_, _, _, err = collectCPUTemperatures(-1)
		if err == nil {
			t.Error("Expected error when collecting temperatures for negative cores")
		}
	})
}

// TestMemoryManagement tests memory allocation and cleanup.
func TestMemoryManagement(t *testing.T) {
	t.Run("MemoryLeakPrevention", func(t *testing.T) {
		// Create and destroy multiple metric streams
		for i := range 10 {
			stream, err := NewCPUMetricStream()
			if err != nil {
				t.Fatalf("Failed to create CPU metrics stream %d: %v", i, err)
			}

			// Collect some metrics
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				if err := stream.StartProfiling(10*time.Millisecond, &wg); err != nil {
					t.Errorf("StartProfiling failed for stream %d: %v", i, err)
				}
			}()

			time.Sleep(50 * time.Millisecond)
			if err := stream.StopProfiling(nil); err != nil {
				t.Errorf("StopProfiling failed for stream %d: %v", i, err)
			}
			wg.Wait()

			// Verify some metrics were collected
			stream.mutex.RLock()
			metricsCount := len(stream.CPUDynamicMetrics)
			stream.mutex.RUnlock()

			if metricsCount == 0 {
				t.Errorf("Stream %d collected no metrics", i)
			}
		}
	})
}

// TestFieldValidation tests that all struct fields are properly validated.
func TestFieldValidation(t *testing.T) {
	t.Run("StaticMetricsFieldValidation", func(t *testing.T) {
		staticMetrics, err := NewCPUStaticMetrics()
		if err != nil {
			t.Fatalf("Failed to create CPU static metrics: %v", err)
		}

		// Test that all important fields are set
		if staticMetrics.CPU.TotalCores == 0 {
			t.Error("TotalCores should be greater than 0")
		}
		if staticMetrics.CPU.TotalHardwareThreads == 0 {
			t.Error("TotalHardwareThreads should be greater than 0")
		}
		if staticMetrics.CPU.TotalHardwareThreads < staticMetrics.CPU.TotalCores {
			t.Error("TotalHardwareThreads should be >= TotalCores")
		}

		// Family, Model, Stepping, and Microcode may be empty on some systems
		t.Logf("CPU Family: %s", staticMetrics.Family)
		t.Logf("CPU Model: %s", staticMetrics.Model)
		t.Logf("CPU Stepping: %s", staticMetrics.Stepping)
		t.Logf("CPU Microcode: %s", staticMetrics.Microcode)
	})

	t.Run("DynamicMetricsFieldValidation", func(t *testing.T) {
		staticMetrics, err := NewCPUStaticMetrics()
		if err != nil {
			t.Fatalf("Failed to create CPU static metrics: %v", err)
		}

		dynamicMetrics, err := NewCPUMetricsAtInstant(*staticMetrics)
		if err != nil {
			t.Logf("Warning: NewCPUMetricsAtInstant returned error: %v", err)
		}
		if dynamicMetrics == nil {
			t.Fatalf("Expected non-nil dynamic metrics")
		}

		// Verify all fields are initialized
		if dynamicMetrics.Timestamp.IsZero() {
			t.Error("Timestamp should be set")
		}
		if dynamicMetrics.CPUUtilization == nil {
			t.Error("CPUUtilization should be initialized")
		}
		if dynamicMetrics.CPUFrequency == nil {
			t.Error("CPUFrequency should be initialized")
		}
		if dynamicMetrics.CPUTemperature == nil {
			t.Error("CPUTemperature should be initialized")
		}
		if dynamicMetrics.CPUPower == nil {
			t.Error("CPUPower should be initialized")
		}

		// TempSource should be set to something meaningful
		if dynamicMetrics.TempSource == "" {
			t.Error("TempSource should be set")
		}
		validTempSources := []string{"per-core", "shared", "none"}

		if !slices.Contains(validTempSources, dynamicMetrics.TempSource) {
			t.Errorf("TempSource should be one of %v, got %s", validTempSources, dynamicMetrics.TempSource)
		}

		t.Logf("Temperature source: %s", dynamicMetrics.TempSource)
		t.Logf("Total CPU power: %d", dynamicMetrics.TotalCPUPower)
	})
}

// TestConcurrentStreamCreation tests creating multiple streams concurrently.
func TestConcurrentStreamCreation(t *testing.T) {
	t.Run("ConcurrentStreamCreation", func(t *testing.T) {
		const numStreams = 5
		streams := make([]*CPUMetricsStream, numStreams)
		var wg sync.WaitGroup
		errors := make([]error, numStreams)

		// Create streams concurrently
		for i := range numStreams {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				stream, err := NewCPUMetricStream()
				streams[index] = stream
				errors[index] = err
			}(i)
		}
		wg.Wait()

		// Verify all streams were created successfully
		for i, err := range errors {
			if err != nil {
				t.Errorf("Failed to create stream %d: %v", i, err)
			}
			if streams[i] == nil {
				t.Errorf("Stream %d is nil", i)
			}
		}

		// Verify all streams have the same static metrics
		if streams[0] != nil {
			expectedCores := streams[0].CPUStaticMetrics.CPU.TotalCores
			for i := 1; i < numStreams; i++ {
				if streams[i] != nil && streams[i].CPUStaticMetrics.CPU.TotalCores != expectedCores {
					t.Errorf("Stream %d has different core count: expected %d, got %d",
						i, expectedCores, streams[i].CPUStaticMetrics.CPU.TotalCores)
				}
			}
		}
	})
}

// TestProfilingStateManagement tests the internal state management of profiling.
func TestProfilingStateManagement(t *testing.T) {
	t.Run("ProfilingStateTracking", func(t *testing.T) {
		stream, err := NewCPUMetricStream()
		if err != nil {
			t.Fatalf("Failed to create CPU metrics stream: %v", err)
		}

		// Initially should not be running
		stream.mutex.RLock()
		isRunning := stream.isRunning
		stream.mutex.RUnlock()
		if isRunning {
			t.Error("Stream should not be running initially")
		}

		// Start profiling
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			if err := stream.StartProfiling(100*time.Millisecond, &wg); err != nil {
				t.Errorf("StartProfiling failed: %v", err)
			}
		}()

		// Give it time to start
		time.Sleep(50 * time.Millisecond)

		// Should be running now
		stream.mutex.RLock()
		isRunning = stream.isRunning
		stream.mutex.RUnlock()
		if !isRunning {
			t.Error("Stream should be running after StartProfiling")
		}

		// Stop profiling
		if err := stream.StopProfiling(nil); err != nil {
			t.Errorf("StopProfiling failed: %v", err)
		}
		wg.Wait()

		// Give it time to stop
		time.Sleep(50 * time.Millisecond)

		// Should not be running anymore
		stream.mutex.RLock()
		isRunning = stream.isRunning
		stream.mutex.RUnlock()
		if isRunning {
			t.Error("Stream should not be running after StopProfiling")
		}
	})
}

// TestJSONYAMLSerializationEdgeCases tests edge cases in serialization.
func TestJSONYAMLSerializationEdgeCases(t *testing.T) {
	t.Run("SerializationWithNilFields", func(t *testing.T) {
		// Create a dynamic metrics with nil fields
		dynamicMetrics := &CPUDynamicsMetrics{
			CPUUtilization: nil,
			CPUFrequency:   nil,
			CPUTemperature: nil,
			CPUPower:       nil,
			CacheUsage:     nil,
			TotalCPUPower:  -1,
			Timestamp:      time.Now(),
			TempSource:     "test",
			TempLabels:     nil,
		}

		// Should not panic or return error strings
		jsonStr := dynamicMetrics.JSONString()
		if strings.HasPrefix(jsonStr, "ERROR:") {
			t.Errorf("JSON serialization failed with nil fields: %s", jsonStr)
		}

		yamlStr := dynamicMetrics.YAMLString()
		if strings.HasPrefix(yamlStr, "ERROR:") {
			t.Errorf("YAML serialization failed with nil fields: %s", yamlStr)
		}
	})

	t.Run("SerializationWithEmptyStream", func(t *testing.T) {
		// Create an empty metrics stream
		stream := &CPUMetricsStream{
			CPUDynamicMetrics: []CPUDynamicsMetrics{},
		}

		// Should not panic or return error strings
		jsonStr := stream.JSONString()
		if strings.HasPrefix(jsonStr, "ERROR:") {
			t.Errorf("JSON serialization failed with empty stream: %s", jsonStr)
		}

		yamlStr := stream.YAMLString()
		if strings.HasPrefix(yamlStr, "ERROR:") {
			t.Errorf("YAML serialization failed with empty stream: %s", yamlStr)
		}
	})
}
