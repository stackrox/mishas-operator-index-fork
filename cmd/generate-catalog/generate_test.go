package main

import (
	"maps"
	"slices"
	"testing"

	semver "github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
)

var (
	v3610           = semver.MustParse("3.61.0")
	v3620           = semver.MustParse("3.62.0")
	v3621           = semver.MustParse("3.62.1")
	v4000           = semver.MustParse("4.0.0")
	v4001           = semver.MustParse("4.0.1")
	v4002           = semver.MustParse("4.0.2")
	v4100           = semver.MustParse("4.1.0")
	v4101           = semver.MustParse("4.1.1")
	v4200           = semver.MustParse("4.2.0")
	skippedVersions = map[*semver.Version]bool{
		v4001: true,
	}

	entry3620 = newChannelEntry(v3620, v3610, v3610, nil)
	entry3621 = newChannelEntry(v3621, v3620, v3610, nil)
	entry4000 = newChannelEntry(v4000, v3621, v3620, nil)
	entry4001 = newChannelEntry(v4001, v4000, v3620, skippedVersions)
	entry4002 = newChannelEntry(v4002, v4001, v3620, skippedVersions)
	entry4100 = newChannelEntry(v4100, v4002, v4000, skippedVersions)
	entry4101 = newChannelEntry(v4101, v4100, v4000, skippedVersions)
	entry4200 = newChannelEntry(v4200, v4101, v4100, skippedVersions)

	channel36     = newChannel(v3620)
	latestChannel = newLatestChannel()
	channel40     = newChannel(v4000)
	channel41     = newChannel(v4100)
	channel42     = newChannel(v4200)
	stableChannel = newStableChannel()
)

func TestGenerateChannels(t *testing.T) {
	channel36.Entries = append(channel36.Entries, entry3620, entry3621)
	latestChannel.Entries = append(latestChannel.Entries, entry3620, entry3621)
	channel40.Entries = append(channel40.Entries, entry4000, entry4001, entry4002)
	channel41.Entries = append(channel41.Entries, entry4000, entry4001, entry4002, entry4100, entry4101)
	channel42.Entries = append(channel42.Entries, entry4000, entry4001, entry4002, entry4100, entry4101, entry4200)
	stableChannel.Entries = append(stableChannel.Entries, entry4000, entry4001, entry4002, entry4100, entry4101, entry4200)

	tests := []struct {
		name             string
		versions         []*semver.Version
		expectedChannels []Channel
	}{
		{
			name: "Single major version with patch versions",
			versions: []*semver.Version{
				v3620,
				v3621,
			},
			expectedChannels: []Channel{
				newChannel(v3620),
				newStableChannel(),
			},
		},
		{
			name: "Multiple major versions with patch versions",
			versions: []*semver.Version{
				v3620,
				v3621,
				v4000,
				v4001,
			},
			expectedChannels: []Channel{
				newChannel(v3620),
				newLatestChannel(),
				newChannel(v4000),
				newStableChannel(),
			},
		},
		{
			name:     "Only stable channel with no versions",
			versions: []*semver.Version{},
			expectedChannels: []Channel{
				newStableChannel(),
			},
		},
		{
			name: "First 4.x version triggers latest channel",
			versions: []*semver.Version{
				v4000,
			},
			expectedChannels: []Channel{
				newLatestChannel(),
				newChannel(v4000),
				newStableChannel(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualChannels := generateChannels(tt.versions)
			assert.Equal(t, tt.expectedChannels, actualChannels)
		})
	}
}
func TestReadInputFile(t *testing.T) {
	tests := []struct {
		name           string
		filePath       string
		expectedError  string
		expectedConfig Configuration
	}{
		{
			name:          "Valid input file",
			filePath:      "testdata/valid_input.yaml",
			expectedError: "",
			expectedConfig: Configuration{
				OldestSupportedVersion: semver.MustParse("4.0.0"),
				BrokenVersions: map[*semver.Version]bool{
					semver.MustParse("4.1.0"): true,
				},
				Images: []BundleImage{
					{
						Image:   "example.com/image@sha256:6cdcf20771f9c46640b466f804190d00eaf2e59caee6d420436e78b283d177bf",
						Version: semver.MustParse("3.62.0"),
					},
					{
						Image:   "example.com/image@sha256:7fd7595e6a61352088f9a3a345be03a6c0b9caa0bbc5ddd8c61ba1d38b2c3b8e",
						Version: semver.MustParse("4.0.0"),
					},
					{
						Image:   "example.com/image@sha256:272e3d6e2f7f207b3d3866d8be00715e6a6086d50b110c45662d99d217d48dbc",
						Version: semver.MustParse("4.1.0"),
					},
					{
						Image:   "example.com/image@sha256:68633e6b12768689f352e1318dc0acc388522d8b6295bf6ca662834cf1367b85",
						Version: semver.MustParse("4.2.0"),
					},
				},
			},
		},
		{
			name:          "Invalid YAML format",
			filePath:      "testdata/invalid_yaml.yaml",
			expectedError: "failed to unmarshal YAML",
		},
		{
			name:          "Invalid oldest_supported_version",
			filePath:      "testdata/invalid_oldest_supported_version.yaml",
			expectedError: "invalid oldest_supported_version",
		},
		{
			name:          "Invalid broken_versions",
			filePath:      "testdata/invalid_broken_versions.yaml",
			expectedError: "invalid item in broken_versions",
		},
		{
			name:          "Invalid image version",
			filePath:      "testdata/invalid_image_version.yaml",
			expectedError: "invalid version",
		},
		{
			name:          "Image reference without digest",
			filePath:      "testdata/image_without_digest.yaml",
			expectedError: "image reference does not include a digest",
		},
		{
			name:          "Image reference is not a strict semantic version",
			filePath:      "testdata/not_strict_image_version.yaml",
			expectedError: "invalid semantic version",
		},
		{
			name:          "oldest_supported_version is not a strict semantic version",
			filePath:      "testdata/not_strict_oldest_supported_version.yaml",
			expectedError: "invalid semantic version",
		},
		{
			name:          "broken_versions is not a strict semantic version",
			filePath:      "testdata/not_strict_broken_versions.yaml",
			expectedError: "invalid semantic version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the function under test
			config, err := readInputFile(tt.filePath)

			// Check for expected errors
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedConfig.Images, config.Images)
				assert.True(t, tt.expectedConfig.OldestSupportedVersion.Equal(config.OldestSupportedVersion))
				expectedBrokenVersions := slices.Collect(maps.Keys(tt.expectedConfig.BrokenVersions))
				actualBrokenVersions := slices.Collect(maps.Keys(config.BrokenVersions))
				assert.ElementsMatch(t, expectedBrokenVersions, actualBrokenVersions)
			}
		})
	}
}
