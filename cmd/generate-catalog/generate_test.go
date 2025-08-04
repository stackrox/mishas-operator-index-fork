package main

import (
	"testing"

	semver "github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
)

func TestGenerateChannels(t *testing.T) {
	// Define test cases
	tests := []struct {
		name             string
		versions         []*semver.Version
		brokenVersions   []*semver.Version
		expectedChannels []Channel
	}{
		{
			name: "Single major version with no broken versions",
			versions: []*semver.Version{
				semver.MustParse("4.0.0"),
				semver.MustParse("4.0.1"),
				semver.MustParse("4.1.0"),
				semver.MustParse("4.1.1"),
			},
			brokenVersions:   nil,
			expectedChannels: []Channel{
				// Expected channels for the given versions
			},
		},
		{
			name: "Multiple major versions with broken versions",
			versions: []*semver.Version{
				semver.MustParse("3.61.0"),
				semver.MustParse("4.0.0"),
				semver.MustParse("4.0.1"),
				semver.MustParse("4.1.0"),
				semver.MustParse("5.0.0"),
			},
			brokenVersions: []*semver.Version{
				semver.MustParse("4.0.1"),
			},
			expectedChannels: []Channel{
				// Expected channels for the given versions
			},
		},
		{
			name:             "Empty versions list",
			versions:         []*semver.Version{},
			brokenVersions:   nil,
			expectedChannels: []Channel{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the function
			actualChannels := generateChannels(tt.versions, tt.brokenVersions)

			// Assert the result
			assert.Equal(t, tt.expectedChannels, actualChannels)
		})
	}
}
