package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"slices"
	"sort"

	"github.com/distribution/reference"
	"github.com/goccy/go-yaml"
)

const (
	deprecationMessage              = "This version is no longer supported. Please switch to the `stable` channel or a channel for a version that is still supported.\n"
	deprecationMessageLatestChannel = "The `latest` channel is no longer supported.  Please switch to the `stable` channel.\n"
)

type BundleImage struct {
	Image string `yaml:"image"`
	Tag   string `yaml:"tag"`
}

type Input struct {
	Images []BundleImage `yaml:"images"`
}

type CatalogTemplate struct {
	Schema  string         `yaml:"schema"`
	Entries []CatalogEntry `yaml:"entries"`
}

type CatalogEntry any

type Package struct {
	Schema         string `yaml:"schema"`
	Name           string `yaml:"name"`
	DefaultChannel string `yaml:"defaultChannel"`
	Icon           Icon   `yaml:"icon"`
}

type Icon struct {
	Base64data string `yaml:"base64data"`
	MediaType  string `yaml:"mediatype"`
}

type Channel struct {
	Schema  string         `yaml:"schema"`
	Name    string         `yaml:"name,omitempty"`
	Package string         `yaml:"package,omitempty"`
	Entries []ChannelEntry `yaml:"entries,omitempty"`
}
type ChannelEntry struct {
	Name      string   `yaml:"name"`
	Replaces  string   `yaml:"replaces,omitempty"`
	SkipRange string   `yaml:"skipRange"`
	Skips     []string `yaml:"skips,omitempty"`
}

type Deprecation struct {
	Schema  string             `yaml:"schema"`
	Package string             `yaml:"package"`
	Entries []DeprecationEntry `yaml:"entries,omitempty"`
}

type DeprecationEntry struct {
	Reference DeprecationReference `yaml:"reference"`
	Message   string               `yaml:"message"`
}

type DeprecationReference struct {
	Schema string `yaml:"schema"`
	Name   string `yaml:"name"`
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

func createPackageWithIcon() Package {
	iconFile := "icon.png"

	// Read the file
	data, err := os.ReadFile(iconFile)
	if err != nil {
		log.Fatalf("Failed to read icon.png: %v", err)
	}
	// Encode to base64
	iconBase64 := base64.StdEncoding.EncodeToString(data)

	return Package{
		Schema:         "olm.package",
		Name:           "rhacs-operator",
		DefaultChannel: "stable",
		Icon: Icon{
			Base64data: iconBase64,
			MediaType:  "image/png",
		},
	}
}

func newChannelEntry(version Version, previousEntryVersion Version, previousChannelVersion Version) ChannelEntry {
	// skip setting "replaces" key for specific versions
	versionsWithoutReplaces := []string{"4.0.0", "3.62.0"}

	// 4.1.0 version should be added to "skips"
	skipVersion := forceParseVersion("4.1.0")
	versionsWithoutSkips := []string{"4.7.4", "4.6.7"}

	entry := ChannelEntry{
		Name: "rhacs-operator.v" + version.Tag,
	}
	replacesVersion := previousEntryVersion.Tag
	if version.Patch == 0 {
		replacesVersion = previousChannelVersion.Tag
	}
	if !slices.Contains(versionsWithoutReplaces, version.Tag) {
		entry.Replaces = "rhacs-operator.v" + replacesVersion
	}
	entry.SkipRange = fmt.Sprintf(">= %d.%d.0 < %s", previousChannelVersion.Major, previousChannelVersion.Minor, version.Tag)
	if versionLess(skipVersion, version) && !slices.Contains(versionsWithoutSkips, version.Tag) {
		entry.Skips = []string{
			fmt.Sprintf("rhacs-operator.v%s", skipVersion.Tag),
		}
	}
	return entry
}

func newChannel(version Version, entries []ChannelEntry) *Channel {
	return &Channel{
		Schema:  "olm.channel",
		Name:    fmt.Sprintf("rhacs-%d.%d", version.Major, version.Minor),
		Package: "rhacs-operator",
		Entries: entries,
	}
}

func newDeprecation(entries []DeprecationEntry) *Deprecation {
	// Add a deprecation entry for the "latest" channel
	latestDeprecationEntry := &DeprecationEntry{
		Reference: DeprecationReference{
			Schema: "olm.channel",
			Name:   "latest",
		},
		Message: deprecationMessageLatestChannel,
	}
	entries = slices.Insert(entries, 0, *latestDeprecationEntry)

	return &Deprecation{
		Schema:  "olm.deprecations",
		Package: "rhacs-operator",
		Entries: entries,
	}
}

func newDeprecationEntry(version Version) *DeprecationEntry {
	return &DeprecationEntry{
		Reference: DeprecationReference{
			Schema: "olm.channel",
			Name:   fmt.Sprintf("rhacs-%d.%d", version.Major, version.Minor),
		},
		Message: deprecationMessage,
	}
}

func newBundleEntry(image string) *BundleEntry {
	return &BundleEntry{
		Schema: "olm.bundle",
		Image:  image,
	}
}

func generateLatestChannel(entries []ChannelEntry) Channel {
	return Channel{
		Schema:  "olm.channel",
		Name:    "latest",
		Package: "rhacs-operator",
		Entries: entries,
	}
}

func generateStableChannel(entries []ChannelEntry) Channel {
	return Channel{
		Schema:  "olm.channel",
		Name:    "stable",
		Package: "rhacs-operator",
		Entries: entries,
	}
}

func ShouldBePersistentToMajorVersion(version Version) bool {
	listToKeepForMajorVersion := []string{"4.1.1", "4.1.2", "4.1.3"}

	// If the version is in the list of versions that should be kept for the next channels
	if slices.Contains(listToKeepForMajorVersion, version.Tag) {
		return true
	}
	return version.Patch == 0
}

func validateImageReference(image string) error {
	// Validate the image reference using the distribution/reference package
	ref, err := reference.ParseAnyReference(image)
	if err != nil {
		return fmt.Errorf("Cannot parse string as docker image %s: %w", image, err)
	}
	if _, ok := ref.(reference.Canonical); !ok {
		return fmt.Errorf("image reference does not include a digest")
	}
	return nil
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
	versionToImageMap := make(map[Version]BundleImage)
	tagToImage := make(map[string]BundleImage)
	for _, img := range input.Images {
		// ignore image validation for now. Looks like docker library has a bug in it
		_ = validateImageReference(img.Image)

		v, err := parseVersion(img.Tag)
		if err != nil {
			log.Fatalf("Invalid tag: %s", img.Tag)
		}
		versionToImageMap[v] = img
		versions = append(versions, v)
		tagToImage[img.Tag] = img
	}
	sort.Slice(versions, func(i, j int) bool {
		return versionLess(versions[i], versions[j])
	})

	var baseEntries []CatalogEntry

	iconPackage := createPackageWithIcon()

	channelVersions := make([]Version, 0)
	baseEntries = append(baseEntries, iconPackage)

	// Very first version in the catalog repalces 3.61.0 and skipRanges starts from 3.61.0
	previousEntryVersion := forceParseVersion("3.61.0")
	previousChannelVersion := forceParseVersion("3.61.0")
	lastPersistentMajorVersion := forceParseVersion("3.61.0")

	majorPersistentEntries := make([]ChannelEntry, 0)
	channels := make([]CatalogEntry, 0)

	var channel *Channel
	for n, v := range versions {
		if v.Tag == "4.0.0" {
			majorPersistentEntries = make([]ChannelEntry, 0)
		}

		// Create a new channel entry for each minor version
		if v.Patch == 0 {
			previousChannelVersion = lastPersistentMajorVersion
			if v.Tag != "3.63.0" {
				if channel != nil {
					channels = append(channels, channel)
				}
				channel = newChannel(v, slices.Clone(majorPersistentEntries))
				channelVersions = append(channelVersions, v)
			}
		}

		catalogChannelEntry := newChannelEntry(v, previousEntryVersion, previousChannelVersion)
		if v.Tag != "3.63.0" {
			channel.Entries = append(channel.Entries, catalogChannelEntry)
		}

		if ShouldBePersistentToMajorVersion(v) {
			majorPersistentEntries = append(majorPersistentEntries, catalogChannelEntry)
			lastPersistentMajorVersion = v
		}

		// Add "latest" channel when "3.74.9" is reached
		if v.Tag == "3.74.9" {
			latestChannel := generateLatestChannel(channel.Entries)
			channels = append(channels, latestChannel)
		}

		if n == len(versions)-1 {
			channels = append(channels, channel)

			// Add "stable" channel when the last version is reached
			stableChannel := generateStableChannel(channel.Entries)
			channels = append(channels, stableChannel)
		}

		previousEntryVersion = v
	}

	baseEntries = append(baseEntries, channels...)

	var deprecations []DeprecationEntry
	if len(channelVersions) > 2 {
		for _, v := range channelVersions[:len(channelVersions)-2] {
			deprecations = append(deprecations, *newDeprecationEntry(v))
		}
	}
	baseEntries = append(baseEntries, newDeprecation(deprecations))

	for _, v := range versions {
		baseEntries = append(baseEntries, newBundleEntry(versionToImageMap[v].Image))
	}

	catalog := CatalogTemplate{
		Schema:  "olm.template.basic",
		Entries: baseEntries,
	}

	// Set indention for catalog YAML representation
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	err = encoder.Encode(catalog)
	if err != nil {
		log.Fatalf("Failed to encode catalog: %v", err)
	}
	encoder.Close()
	out := buf.Bytes()

	// Write catalog to the file
	if err := os.WriteFile(outputFile, out, 0644); err != nil {
		log.Fatalf("Failed to write output: %v", err)
	}

	fmt.Println("catalog-template.yaml generated successfully.")
}
