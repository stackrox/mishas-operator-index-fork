package main

import (
	"testing"

	semver "github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
)

func TestNewChannelEntryWithBrokenVersions(t *testing.T) {
	tests := []struct {
		name                   string
		version                string
		previousEntryVersion   string
		previousChannelVersion string
		brokenVersions         []string
		expected               ChannelEntry
	}{
		{
			name:                   "Version with broken versions in skips",
			version:                "4.1.2",
			previousEntryVersion:   "4.1.1",
			previousChannelVersion: "4.1.0",
			brokenVersions:         []string{"4.1.0", "4.1.1"},
			expected: ChannelEntry{
				Name:      "rhacs-operator.v4.1.2",
				Replaces:  "rhacs-operator.v4.1.1",
				SkipRange: ">= 4.1.0 < 4.1.2",
				Skips:     []string{"rhacs-operator.v4.1.0", "rhacs-operator.v4.1.1"},
			},
		},
		{
			name:                   "Version without broken versions in skips",
			version:                "4.2.0",
			previousEntryVersion:   "4.1.2",
			previousChannelVersion: "4.1.0",
			brokenVersions:         []string{},
			expected: ChannelEntry{
				Name:      "rhacs-operator.v4.2.0",
				Replaces:  "rhacs-operator.v4.1.2",
				SkipRange: ">= 4.1.0 < 4.2.0",
			},
		},
		{
			name:                   "Version with no replaces and broken versions",
			version:                "4.0.0",
			previousEntryVersion:   "3.62.0",
			previousChannelVersion: "3.61.0",
			brokenVersions:         []string{"3.61.0"},
			expected: ChannelEntry{
				Name:      "rhacs-operator.v4.0.0",
				SkipRange: ">= 3.61.0 < 4.0.0",
				Skips:     []string{"rhacs-operator.v3.61.0"},
			},
		},
		{
			name:                   "Version with multiple broken versions",
			version:                "4.3.0",
			previousEntryVersion:   "4.2.0",
			previousChannelVersion: "4.1.0",
			brokenVersions:         []string{"4.1.0", "4.2.0"},
			expected: ChannelEntry{
				Name:      "rhacs-operator.v4.3.0",
				Replaces:  "rhacs-operator.v4.2.0",
				SkipRange: ">= 4.1.0 < 4.3.0",
				Skips:     []string{"rhacs-operator.v4.1.0", "rhacs-operator.v4.2.0"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version := semver.MustParse(tt.version)
			previousEntryVersion := semver.MustParse(tt.previousEntryVersion)
			previousChannelVersion := semver.MustParse(tt.previousChannelVersion)

			var brokenVersions []*semver.Version
			for _, bv := range tt.brokenVersions {
				brokenVersions = append(brokenVersions, semver.MustParse(bv))
			}

			result := newChannelEntry(version, previousEntryVersion, previousChannelVersion, brokenVersions)

			assert.Equal(t, tt.expected.Name, result.Name)
			assert.Equal(t, tt.expected.Replaces, result.Replaces)
			assert.Equal(t, tt.expected.SkipRange, result.SkipRange)
			assert.ElementsMatch(t, tt.expected.Skips, result.Skips)
		})
	}
}
