package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetReleaseRegions(t *testing.T) {
	tests := []struct {
		name            string
		setupRegions    string
		expectedRegions []string
	}{
		{
			name:            "empty regions",
			setupRegions:    "",
			expectedRegions: []string{},
		},
		{
			name:            "single region",
			setupRegions:    "us-east-1",
			expectedRegions: []string{"us-east-1"},
		},
		{
			name:            "multiple regions",
			setupRegions:    "us-east-1,us-west-2,eu-west-1",
			expectedRegions: []string{"us-east-1", "us-west-2", "eu-west-1"},
		},
		{
			name:            "regions with spaces",
			setupRegions:    "us-east-1, us-west-2 , eu-west-1",
			expectedRegions: []string{"us-east-1", "us-west-2", "eu-west-1"},
		},
		{
			name:            "regions with extra commas",
			setupRegions:    "us-east-1,,us-west-2,",
			expectedRegions: []string{"us-east-1", "us-west-2"},
		},
		{
			name:            "regions with only spaces",
			setupRegions:    "  ,  ,  ",
			expectedRegions: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original value
			originalRegions := rawReleaseRegions
			defer func() {
				rawReleaseRegions = originalRegions
			}()

			// Set test value
			rawReleaseRegions = tt.setupRegions

			result := GetReleaseRegions()
			assert.Equal(t, tt.expectedRegions, result)
		})
	}
}

func TestValidateRegion(t *testing.T) {
	tests := []struct {
		name          string
		setupRegions  string
		region        string
		expectedError string
	}{
		{
			name:          "valid region in list",
			setupRegions:  "us-east-1,us-west-2,eu-west-1",
			region:        "us-east-1",
			expectedError: "",
		},
		{
			name:          "valid region with spaces",
			setupRegions:  "us-east-1,us-west-2",
			region:        "  us-east-1  ",
			expectedError: "",
		},
		{
			name:          "invalid region not in list",
			setupRegions:  "us-east-1,us-west-2",
			region:        "eu-west-1",
			expectedError: "region \"eu-west-1\" is not supported. Supported regions: us-east-1, us-west-2",
		},
		{
			name:          "empty region",
			setupRegions:  "us-east-1,us-west-2",
			region:        "",
			expectedError: "region cannot be empty",
		},
		{
			name:          "no regions configured - should skip validation",
			setupRegions:  "",
			region:        "any-region",
			expectedError: "",
		},
		{
			name:          "case sensitive mismatch",
			setupRegions:  "us-east-1",
			region:        "US-EAST-1",
			expectedError: "region \"US-EAST-1\" is not supported. Supported regions: us-east-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original value
			originalRegions := rawReleaseRegions
			defer func() {
				rawReleaseRegions = originalRegions
			}()

			// Set test value
			rawReleaseRegions = tt.setupRegions

			err := ValidateRegion(tt.region)
			if tt.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err.Error())
			}
		})
	}
}
