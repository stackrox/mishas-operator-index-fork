package main

import (
	"fmt"
	"log"
	"os"
	"slices"
	"sort"

	"gopkg.in/yaml.v3"
)

type Image struct {
	Image string `yaml:"image"`
	Tag   string `yaml:"tag"`
}

type Input struct {
	Images []Image `yaml:"images"`
}

type CatalogChannelEntry struct {
	Name      string   `yaml:"name"`
	Replaces  string   `yaml:"replaces,omitempty"`
	SkipRange string   `yaml:"skipRange"`
	Skips     []string `yaml:"skips,omitempty"`
}

type CatalogTemplate struct {
	Channels     map[string][]CatalogChannelEntry `yaml:"olm.channels"`
	Deprecations []string                         `yaml:"olm.deprecations"`
	Images       []Image                          `yaml:"images"`
}

type CatalogBase struct {
	Schema  string         `yaml:"schema"`
	Entries []CatalogEntry `yaml:"entries"`
}

type CatalogEntry interface {
}

type ChannelEntry struct {
	Schema  string                `yaml:"schema"`
	Name    string                `yaml:"name,omitempty"`
	Package string                `yaml:"package,omitempty"`
	Entries []CatalogChannelEntry `yaml:"entries,omitempty"`
}

type BundleEntry struct {
	Schema string `yaml:"schema"`
	Image  string `yaml:"image"`
}

type Version struct {
	Major int
	Minor int
	Patch int
	Tag   string
}

func parseVersion(tag string) (Version, error) {
	var v Version
	n, err := fmt.Sscanf(tag, "%d.%d.%d", &v.Major, &v.Minor, &v.Patch)
	if err != nil || n != 3 {
		return v, fmt.Errorf("invalid version tag: %s", tag)
	}
	v.Tag = tag
	return v, nil
}

func forceParseVersion(tag string) Version {
	v, err := parseVersion(tag)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse version tag: %s, error: %v", tag, err))
	}
	return v
}

func versionLess(a, b Version) bool {
	if a.Major != b.Major {
		return a.Major < b.Major
	}
	if a.Minor != b.Minor {
		return a.Minor < b.Minor
	}
	return a.Patch < b.Patch
}

func buildCatalogChannelEntry(version Version, previousEntryVersion Version, previousChannelVersion string) CatalogChannelEntry {
	// skip setting "replaces" key for specific versions
	versionsWithoutReplaces := []string{"4.0.0", "3.62.0"}

	entry := CatalogChannelEntry{
		Name: "rhacs-operator.v" + version.Tag,
	}
	if !slices.Contains(versionsWithoutReplaces, version.Tag) {
		entry.Replaces = "rhacs-operator.v" + previousEntryVersion.Tag
	}
	entry.SkipRange = fmt.Sprintf(">= %s < %s", previousChannelVersion, version.Tag)
	return entry
}

func newChannelEntry(version Version, entries []CatalogChannelEntry) *ChannelEntry {
	return &ChannelEntry{
		Schema:  "olm.channel",
		Name:    fmt.Sprintf("rhacs-%d.%d", version.Major, version.Minor),
		Package: "rhacs-operator",
		Entries: entries,
	}
}

func newBundleEntry(image string) *BundleEntry {
	return &BundleEntry{
		Schema: "olm.bundle",
		Image:  image,
	}
}

func generateLatestChannel(entries []CatalogChannelEntry) *ChannelEntry {
	return &ChannelEntry{
		Schema:  "olm.channel",
		Name:    "latest",
		Package: "rhacs-operator",
		Entries: entries,
	}
}

func generateStableChannel(entries []CatalogChannelEntry) *ChannelEntry {
	return &ChannelEntry{
		Schema:  "olm.channel",
		Name:    "stable",
		Package: "rhacs-operator",
		Entries: entries,
	}
}

func ShouldBePreviousChannelEntry(version Version) bool {
	listToKeepForNextChannels := []string{"4.1.1", "4.1.2", "4.1.3"}

	// If the version is in the list of versions that should be kept for the next channels
	if slices.Contains(listToKeepForNextChannels, version.Tag) {
		return true
	}
	return version.Patch == 0
}

func main() {
	inputFile := "bundles.yaml"
	outputFile := "catalog-template-new.yaml"

	inputBytes, err := os.ReadFile(inputFile)
	if err != nil {
		log.Fatalf("Failed to read %s: %v", inputFile, err)
	}
	var input Input
	if err := yaml.Unmarshal(inputBytes, &input); err != nil {
		log.Fatalf("Failed to parse YAML: %v", err)
	}

	// Parse and sort versions
	var versions []Version
	tagToImage := make(map[string]Image)
	for _, img := range input.Images {
		v, err := parseVersion(img.Tag)
		if err != nil {
			log.Fatalf("Invalid tag: %s", img.Tag)
		}
		versions = append(versions, v)
		tagToImage[img.Tag] = img
	}
	sort.Slice(versions, func(i, j int) bool {
		return versionLess(versions[i], versions[j])
	})

	var deprecations []string
	var minorVersions []Version
	for _, v := range versions {
		if v.Patch == 0 {
			minorVersions = append(minorVersions, v)
		}
	}
	if len(minorVersions) > 2 {
		for _, v := range versions[:len(versions)-2] {
			deprecations = append(deprecations, "rhacs-operator.v"+v.Tag)
		}
	}

	var baseEntries []CatalogEntry

	// Very first version in the catalog repalces 3.61.0 and skipRanges starts from 3.61.0
	previousEntryVersion := forceParseVersion("3.61.0")
	previousChannelVersionTag := "3.61.0"
	previousEntries := make([]CatalogChannelEntry, 0)

	var channel *ChannelEntry = nil
	for n, v := range versions {
		if channel == nil {
			channel = newChannelEntry(v, previousEntries)
		}
		catalogChannelEntry := buildCatalogChannelEntry(v, previousEntryVersion, previousChannelVersionTag)

		// Create a new channel entry for each minor version
		if n != 0 && v.Minor != previousEntryVersion.Minor {
			// Do not add rhacs-3.63 channel to the catalog
			if previousEntryVersion.Tag != "3.63.0" {
				baseEntries = append(baseEntries, channel)
			}
			channel = newChannelEntry(v, previousEntries)
			previousChannelVersionTag = previousEntryVersion.Tag
		}
		channel.Entries = append(channel.Entries, catalogChannelEntry)

		if v.Tag == "4.0.0" {
			previousEntries = make([]CatalogChannelEntry, 0)
		}

		if ShouldBePreviousChannelEntry(v) {
			previousEntries = append(previousEntries, catalogChannelEntry)
		}

		// Add "latest" channel when "3.74.9" is reached
		if v.Tag == "3.74.9" {
			latestChannel := generateLatestChannel(channel.Entries)
			baseEntries = append(baseEntries, latestChannel)
		}

		if n == len(versions)-1 {
			baseEntries = append(baseEntries, channel)

			// Add "stable" channel when the last version is reached
			stableChannel := generateStableChannel(channel.Entries)
			baseEntries = append(baseEntries, stableChannel)
		}

		previousEntryVersion = v
	}

	catalog := CatalogBase{
		Schema:  "olm.template.basic",
		Entries: baseEntries,
	}

	out, err := yaml.Marshal(catalog)
	if err != nil {
		log.Fatalf("Failed to marshal output: %v", err)
	}
	if err := os.WriteFile(outputFile, out, 0644); err != nil {
		log.Fatalf("Failed to write output: %v", err)
	}

	fmt.Println("catalog-template.yaml generated successfully.")
}
