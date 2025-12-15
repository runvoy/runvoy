package constants

import (
	"testing"
)

func TestProvidersContainsAWSAndGCP(t *testing.T) {
	if len(Providers) == 0 {
		t.Fatalf("Providers slice should not be empty")
	}

	var hasAWS, hasGCP bool
	for _, p := range Providers {
		switch p {
		case AWS:
			hasAWS = true
		case GCP:
			hasGCP = true
		}
	}

	if !hasAWS {
		t.Errorf("Providers slice should contain AWS")
	}
	if !hasGCP {
		t.Errorf("Providers slice should contain GCP")
	}
}

func TestProvidersString(t *testing.T) {
	got := ProvidersString()

	// The default Providers slice contains AWS and GCP, lowercased and comma-separated.
	const expected = "aws, gcp"
	if got != expected {
		t.Errorf("ProvidersString() = %q, want %q", got, expected)
	}
}
