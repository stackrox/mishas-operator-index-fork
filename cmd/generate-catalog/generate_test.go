package main

import (
	"testing"

	semver "github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
)

func TestNewChannelEntry(t *testing.T) {
	tests := []struct {
		name                   string
		version                string
		previousEntryVersion   string
		previousChannelVersion string
		expected               ChannelEntry
	}{
		{
			name:                   "Patch version with replaces and skips",
			version:                "4.1.1",
			previousEntryVersion:   "4.1.0",
			previousChannelVersion: "4.0.0",
			expected: ChannelEntry{
				Name:      "rhacs-operator.v4.1.1",
				Replaces:  "rhacs-operator.v4.1.0",
				SkipRange: ">= 4.0.0 < 4.1.1",
				Skips:     []string{"rhacs-operator.v4.1.0"},
			},
		},
		{
			name:                   "Version without replaces",
			version:                "4.0.0",
			previousEntryVersion:   "3.62.0",
			previousChannelVersion: "3.61.0",
			expected: ChannelEntry{
				Name:      "rhacs-operator.v4.0.0",
				SkipRange: ">= 3.61.0 < 4.0.0",
			},
		},
		{
			name:                   "First version in the channel with skips",
			version:                "4.2.0",
			previousEntryVersion:   "4.1.1",
			previousChannelVersion: "4.1.0",
			expected: ChannelEntry{
				Name:      "rhacs-operator.v4.2.0",
				Replaces:  "rhacs-operator.v4.1.0",
				SkipRange: ">= 4.1.0 < 4.2.0",
				Skips:     []string{"rhacs-operator.v4.1.0"},
			},
		},
		{
			name:                   "Version greater than 4.1.0 but it's an exepton and should have skips empty",
			version:                "4.7.4",
			previousEntryVersion:   "4.7.3",
			previousChannelVersion: "4.7.0",
			expected: ChannelEntry{
				Name:      "rhacs-operator.v4.7.4",
				Replaces:  "rhacs-operator.v4.7.3",
				SkipRange: ">= 4.7.0 < 4.7.4",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version := semver.MustParse(tt.version)
			previousEntryVersion := semver.MustParse(tt.previousEntryVersion)
			previousChannelVersion := semver.MustParse(tt.previousChannelVersion)

			result := newChannelEntry(version, previousEntryVersion, previousChannelVersion)

			assert.Equal(t, tt.expected.Name, result.Name)
			assert.Equal(t, tt.expected.Replaces, result.Replaces)
			assert.Equal(t, tt.expected.SkipRange, result.SkipRange)
			assert.Equal(t, tt.expected.Skips, result.Skips)
		})
	}
}
func TestShouldBePersistentToMajorVersion(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected bool
	}{
		{
			name:     "Version in listToKeepForMajorVersion",
			version:  "4.1.1",
			expected: true,
		},
		{
			name:     "Version not in listToKeepForMajorVersion but is a major version",
			version:  "4.2.0",
			expected: true,
		},
		{
			name:     "Version not in listToKeepForMajorVersion and not a major version",
			version:  "4.2.1",
			expected: false,
		},
		{
			name:     "Version in listToKeepForMajorVersion with patch",
			version:  "4.1.2",
			expected: true,
		},
		{
			name:     "Version not in listToKeepForMajorVersion and not a major version with patch",
			version:  "4.3.1",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version := semver.MustParse(tt.version)
			result := shouldBePersistentToMajorVersion(version)
			assert.Equal(t, tt.expected, result)
		})
	}
}
