// Package events handles AWS event processing for runvoy.
// It processes CloudWatch events and calculates execution costs.
package events

import (
	"fmt"
	"strconv"
)

// Fargate pricing constants for ARM64 (us-east-1, as of 2024)
const (
	// Price per vCPU hour
	fargateCPUPricePerHour = 0.04048

	// Price per GB memory hour
	fargateMemoryPricePerGB = 0.004445
)

// CalculateFargateCost calculates the cost of running a Fargate task
// based on CPU, memory, and duration.
//
// Parameters:
//   - cpu: CPU units (e.g., "256" = 0.25 vCPU)
//   - memory: Memory in MB (e.g., "512" = 0.5 GB)
//   - durationSeconds: Duration in seconds
//
// Returns:
//   - cost in USD
func CalculateFargateCost(cpu, memory string, durationSeconds int) (float64, error) {
	// Parse CPU units to vCPU (1024 units = 1 vCPU)
	cpuUnits, err := strconv.ParseFloat(cpu, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse CPU units: %w", err)
	}
	vCPU := cpuUnits / 1024.0

	// Parse memory MB to GB
	memoryMB, err := strconv.ParseFloat(memory, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse memory: %w", err)
	}
	memoryGB := memoryMB / 1024.0

	// Calculate hours
	hours := float64(durationSeconds) / 3600.0

	// Calculate cost components
	cpuCost := vCPU * fargateCPUPricePerHour * hours
	memoryCost := memoryGB * fargateMemoryPricePerGB * hours

	return cpuCost + memoryCost, nil
}
