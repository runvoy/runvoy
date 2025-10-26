package main

import (
	"crypto/rand"
	"fmt"
)

// generateExecutionID creates a UUID v4 for execution tracking
// Uses crypto/rand from standard library - no external dependencies
func generateExecutionID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		// crypto/rand.Read should never fail on supported platforms
		panic(fmt.Sprintf("failed to generate UUID: %v", err))
	}

	// Set version (4) and variant bits per RFC 4122
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant is 10

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
