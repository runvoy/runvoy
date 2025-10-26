package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// generateExecutionID creates a short ID for execution tracking
// Format: {timestamp_hex}{random_hex} (12 characters)
// Uses crypto/rand from standard library - no external dependencies
// TODO: Make entropy level configurable via init command (future enhancement)
func generateExecutionID() string {
	// Get timestamp in hex (8 characters for ~100 years)
	timestamp := fmt.Sprintf("%08x", time.Now().Unix())

	// Generate 2 random bytes (4 hex characters)
	// Provides ~65k combinations per second (adequate for most use cases)
	randomBytes := make([]byte, 2)
	_, err := rand.Read(randomBytes)
	if err != nil {
		// crypto/rand.Read should never fail on supported platforms
		panic(fmt.Sprintf("failed to generate execution ID: %v", err))
	}

	randomHex := hex.EncodeToString(randomBytes)

	// Combine timestamp + random for a total of 12 characters
	return timestamp + randomHex
}
