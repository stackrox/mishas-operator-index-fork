package main

import (
	"testing"

	semver "github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
)

var (
	v3610 = semver.MustParse("3.61.0")
	v3620 = semver.MustParse("3.62.0")
	v3621 = semver.MustParse("3.62.1")
	v4000 = semver.MustParse("4.0.0")
	v4001 = semver.MustParse("4.0.1")
	v4002 = semver.MustParse("4.0.2")
	v4100 = semver.MustParse("4.1.0")
	v4101 = semver.MustParse("4.1.1")
	v4200 = semver.MustParse("4.2.0")

	entry3620 = newChannelEntry(v3620, v3610, v3610, nil)
	entry3621 = newChannelEntry(v3621, v3620, v3610, nil)
	entry4000 = newChannelEntry(v4000, v3621, v3620, nil)
	entry4001 = newChannelEntry(v4001, v4000, v3620, []*semver.Version{v4001})
	entry4002 = newChannelEntry(v4002, v4001, v3620, []*semver.Version{v4001})
	entry4100 = newChannelEntry(v4100, v4002, v4000, []*semver.Version{v4001})
	entry4101 = newChannelEntry(v4101, v4100, v4000, []*semver.Version{v4001})
	entry4200 = newChannelEntry(v4200, v4101, v4100, []*semver.Version{v4001})

	channel36     = newChannel(v3620, []ChannelEntry{entry3620, entry3621})
	latestChannel = newLatestChannel([]ChannelEntry{entry3620, entry3621})
	channel40     = newChannel(v4000, []ChannelEntry{entry4000, entry4001, entry4002})
	channel41     = newChannel(v4100, []ChannelEntry{entry4000, entry4001, entry4002, entry4100, entry4101})
	channel42     = newChannel(v4200, []ChannelEntry{entry4000, entry4001, entry4002, entry4100, entry4101, entry4200})
	stableChannel = newStableChannel([]ChannelEntry{entry4000, entry4001, entry4002, entry4100, entry4101, entry4200})
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
			name: "Multiple major versions with broken version",
			versions: []*semver.Version{
				v3620,
				v3621,
				v4000,
				v4001,
				v4002,
				v4100,
				v4101,
				v4200,
			},
			brokenVersions: []*semver.Version{
				v4001,
			},
			expectedChannels: []Channel{
				*channel36,
				latestChannel,
				*channel40,
				*channel41,
				*channel42,
				stableChannel,
			},
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
