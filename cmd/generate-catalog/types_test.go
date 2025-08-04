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
		brokenVersions         []string
		expectedName           string
		expectedReplaces       string
		expectedSkipRange      string
		expectedSkips          []string
	}{
		{
			name:                   "Valid channel entry with no broken versions",
			version:                "4.1.0",
			previousEntryVersion:   "4.0.0",
			previousChannelVersion: "4.0.0",
			brokenVersions:         []string{},
			expectedName:           "rhacs-operator.v4.1.0",
			expectedReplaces:       "rhacs-operator.v4.0.0",
			expectedSkipRange:      ">= 4.0.0 < 4.1.0",
			expectedSkips:          []string{},
		},
		{
			name:                   "Valid channel entry with broken versions",
			version:                "4.2.0",
			previousEntryVersion:   "4.1.0",
			previousChannelVersion: "4.1.0",
			brokenVersions:         []string{"4.1.1"},
			expectedName:           "rhacs-operator.v4.2.0",
			expectedReplaces:       "rhacs-operator.v4.1.0",
			expectedSkipRange:      ">= 4.1.0 < 4.2.0",
			expectedSkips:          []string{"rhacs-operator.v4.1.1"},
		},
		{
			name:                   "Version without replaces",
			version:                "4.0.0",
			previousEntryVersion:   "3.62.0",
			previousChannelVersion: "3.62.0",
			brokenVersions:         []string{},
			expectedName:           "rhacs-operator.v4.0.0",
			expectedReplaces:       "",
			expectedSkipRange:      ">= 3.62.0 < 4.0.0",
			expectedSkips:          []string{},
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

			entry := newChannelEntry(version, previousEntryVersion, previousChannelVersion, brokenVersions)

			assert.Equal(t, tt.expectedName, entry.Name)
			assert.Equal(t, tt.expectedReplaces, entry.Replaces)
			assert.Equal(t, tt.expectedSkipRange, entry.SkipRange)
			assert.Equal(t, tt.expectedSkips, entry.Skips)
		})
	}
}
