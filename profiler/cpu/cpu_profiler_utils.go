package cpu

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jaypipes/ghw"
	gopsutilcpu "github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/sensors"
	"gopkg.in/yaml.v3"
)

// CPUStaticMetrics holds immutable CPU hardware information collected once during initialization.
// This includes CPU topology, core counts, socket information, and hardware capabilities.
type CPUStaticMetrics struct {
	CPU      ghw.CPUInfo      `json:"cpu" yaml:"cpu"`
	Topology ghw.TopologyInfo `json:"topology" yaml:"topology"`

	Family    string `json:"family" yaml:"family"`
	Model     string `json:"model" yaml:"model"`
	Stepping  string `json:"stepping" yaml:"stepping"`
	Microcode string `json:"microcode" yaml:"microcode"`
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

	// Fetch extra metadata from gopsutil
	infoStats, err := gopsutilcpu.Info()
	if err != nil || len(infoStats) == 0 {
		log.Printf("WARN: Failed to collect gopsutil CPU info: %v", err)
		return &CPUStaticMetrics{
			CPU:      *cpu,
			Topology: *topology,
		}, nil
	}

	// Use only the first entry (logical CPU 0)
	stat := infoStats[0]

	cpustaticmetrics := CPUStaticMetrics{
		CPU:       *cpu,
		Topology:  *topology,
		Family:    stat.Family,
		Model:     stat.Model,
		Stepping:  fmt.Sprint(stat.Stepping),
		Microcode: stat.Microcode,
	}
	return &cpustaticmetrics, nil
}

// String provides a human-readable representation of the CPUStaticMetrics.
func (cpustaticmetrics *CPUStaticMetrics) String() string {
	return cpustaticmetrics.ShortString()
}

func (cpustaticmetrics *CPUStaticMetrics) ShortString() string {
	base := cpustaticmetrics.CPU.String() + "\n" + cpustaticmetrics.Topology.String()
	extras := fmt.Sprintf(`
Extended Static CPU Info:
  Family ID   : %s
  Model ID    : %s
  Stepping    : %s
  Microcode   : %s`, cpustaticmetrics.Family, cpustaticmetrics.Model, cpustaticmetrics.Stepping, cpustaticmetrics.Microcode)
	return base + extras
}

func (cpustaticmetrics *CPUStaticMetrics) JSONString() string {
	b, err := json.MarshalIndent(cpustaticmetrics, "", "  ")
	if err != nil {
		return "ERROR: Failed to marshal CPUStaticMetrics to JSON"
	}
	return string(b)
}

func (cpustaticmetrics *CPUStaticMetrics) YAMLString() string {
	b, err := yaml.Marshal(cpustaticmetrics)
	if err != nil {
		return "ERROR: Failed to marshal CPUStaticMetrics to YAML"
	}
	return string(b)
}

// CPUDynamicsMetrics represents a snapshot of CPU performance metrics at a specific point in time.
// All slice lengths are determined by the static CPU topology information.
type CPUDynamicsMetrics struct {
	CPUUtilization []float64 `json:"cpu_utilization" yaml:"cpu_utilization"`
	CPUFrequency   []float64 `json:"cpu_frequency" yaml:"cpu_frequency"`
	CPUTemperature []float64 `json:"cpu_temperature" yaml:"cpu_temperature"`
	CPUPower       []float64 `json:"cpu_power" yaml:"cpu_power"`
	CacheUsage     []float64 `json:"cache_usage" yaml:"cache_usage"`
	TotalCPUPower  int32     `json:"total_cpu_power" yaml:"total_cpu_power"`
	Timestamp      time.Time `json:"timestamp" yaml:"timestamp"`
	TempSource     string    `json:"temp_source" yaml:"temp_source"`
	TempLabels     []string  `json:"temp_labels" yaml:"temp_labels"`
}

// Collects CPU Frequencies once at instant
func getCPUFrequencies() ([]float64, error) {
	freqStats, err := gopsutilcpu.Info()
	if err != nil {
		return nil, err
	}

	freqs := make([]float64, len(freqStats))
	for i, stat := range freqStats {
		freqs[i] = stat.Mhz
	}
	return freqs, nil
}

// Temperature Entry
// Used as intermediate store of value while parsing sensors output
type temperatureEntry struct {
	temp  float64
	label string
}

// Collects CPU Temperatures once at instant
// numCores determines the expected slice size
func collectCPUTemperatures(numCores int) ([]float64, string, []string, error) {
	all, sensorserror := sensors.SensorsTemperatures()
	if sensorserror != nil {
		return nil, "", nil, sensorserror
	}
	perCore := make([]temperatureEntry, numCores)
	coreFound := 0
	var sharedCandidates []temperatureEntry

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
		entry := temperatureEntry{temp: sensor.Temperature, label: sensor.SensorKey}

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

// Collect CPU Utilization per core at an instant
func collectCPUUtilization() ([]float64, error) {
	cpuUtilization, errReturn := gopsutilcpu.Percent(0, true)
	if errReturn != nil {
		errReturn = fmt.Errorf("cpu.Percent failed: %w", errReturn)
		cpuUtilization = nil
	}
	return cpuUtilization, errReturn
}

// NewCPUMetricsAtInstant creates a new CPUDynamicsMetrics instance with properly sized slices
// based on the provided static metrics. The slices are pre-allocated but not populated with data.
//
// Parameters:
//   - staticmetrics: Static CPU information used to determine slice sizes
//
// Returns:
//   - *CPUDynamicsMetrics: Initialized metrics structure with timestamp
func NewCPUMetricsAtInstant(staticmetrics CPUStaticMetrics) (*CPUDynamicsMetrics, error) {
	timestamp := time.Now()
	numCores := staticmetrics.CPU.TotalCores
	numThreads := staticmetrics.CPU.TotalHardwareThreads

	metrics := &CPUDynamicsMetrics{
		CPUUtilization: make([]float64, numThreads),
		CPUFrequency:   make([]float64, numCores),
		CPUTemperature: make([]float64, numCores),
		CPUPower:       make([]float64, numCores),
		CacheUsage:     nil,
		TotalCPUPower:  -1,
		Timestamp:      timestamp,
		TempSource:     "none",
	}

	var errAggregator error

	cpuUtilization, cpuUtilizationErr := collectCPUUtilization()
	if cpuUtilizationErr != nil {
		errAggregator = errors.Join(errAggregator, fmt.Errorf("collectCPUUtilization: %w", cpuUtilizationErr))
	}
	metrics.CPUUtilization = cpuUtilization

	cpuFreqs, cpuFreqError := getCPUFrequencies()
	if cpuFreqError != nil {
		errAggregator = errors.Join(errAggregator, fmt.Errorf("getCPUFrequencies: %w", cpuFreqError))
	}
	copy(metrics.CPUFrequency, cpuFreqs)

	cpuTemps, source, labels, temperrs := collectCPUTemperatures(int(staticmetrics.CPU.TotalCores))

	if temperrs != nil {
		errAggregator = errors.Join(errAggregator, fmt.Errorf("collectCPUTemperatures: %w", temperrs))
	} else {
		metrics.CPUTemperature = cpuTemps
		metrics.TempSource = source
		metrics.TempLabels = labels
	}

	return metrics, errAggregator
}

func (cpudynamicmetrics *CPUDynamicsMetrics) String() string {
	return cpudynamicmetrics.ShortString()
}

func (cpudynamicmetrics *CPUDynamicsMetrics) ShortString() string {
	return fmt.Sprintf(`CPU Utilization: %v
CPU Frequency: %v
CPU Temperature: %v
CPU Power: %v
Cache Usage: L1: %.2f, L2: %.2f, L3: %.2f
Total CPU Power: %d
Temperature Source: %s
Timestamp: %s`,
		cpudynamicmetrics.CPUUtilization,
		cpudynamicmetrics.CPUFrequency,
		cpudynamicmetrics.CPUTemperature,
		cpudynamicmetrics.CPUPower,
		cpudynamicmetrics.CacheUsage[0], cpudynamicmetrics.CacheUsage[1], cpudynamicmetrics.CacheUsage[2],
		cpudynamicmetrics.TotalCPUPower,
		cpudynamicmetrics.TempSource,
		cpudynamicmetrics.Timestamp.Format(time.RFC3339),
	)
}

func (cpudynamicmetrics *CPUDynamicsMetrics) JSONString() string {
	b, err := json.MarshalIndent(cpudynamicmetrics, "", "  ")
	if err != nil {
		return "ERROR: Failed to marshal CPUDynamicsMetrics to JSON"
	}
	return string(b)
}

func (cpudynamicmetrics *CPUDynamicsMetrics) YAMLString() string {
	b, err := yaml.Marshal(cpudynamicmetrics)
	if err != nil {
		return "ERROR: Failed to marshal CPUDynamicsMetrics to YAML"
	}
	return string(b)
}
