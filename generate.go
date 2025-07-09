package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"slices"
	"sort"

	semver "github.com/Masterminds/semver/v3"
	"github.com/distribution/reference"
	"github.com/goccy/go-yaml"
)

const (
	inputFile                       = "bundles.yaml"
	outputFile                      = "catalog-template-new.yaml"
	deprecationMessage              = "This version is no longer supported. Please switch to the `stable` channel or a channel for a version that is still supported.\n"
	deprecationMessageLatestChannel = "The `latest` channel is no longer supported.  Please switch to the `stable` channel.\n"
)

type BundleImage struct {
	Image string `yaml:"image"`
	Tag   string `yaml:"tag"`
}

type BundleLiist struct {
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

func readBundleListFile() BundleLiist {
	// Read the file
	inputBytes, err := os.ReadFile(inputFile)
	if err != nil {
		log.Fatalf("Failed to read %s: %v", inputFile, err)
	}
	var bundleList BundleLiist
	if err := yaml.Unmarshal(inputBytes, &bundleList); err != nil {
		log.Fatalf("Failed to parse YAML: %v", err)
	}

	return bundleList
}

func writeCatalogTemplateToFile(catalog CatalogTemplate) error {
	out, err := yaml.Marshal(&catalog)
	if err != nil {
		log.Fatalf("Failed to marshal catalog: %v", err)
	}
	if err := os.WriteFile(outputFile, out, 0644); err != nil {
		log.Fatalf("Failed to write output: %v", err)
	}

	return nil
}

func newCatalogTemplate() CatalogTemplate {
	return CatalogTemplate{
		Schema:  "olm.template.basic",
		Entries: []CatalogEntry{},
	}
}

func newChannelEntry(version *semver.Version, previousEntryVersion *semver.Version, previousChannelVersion *semver.Version) ChannelEntry {
	// skip setting "replaces" key for specific versions
	versionsWithoutReplaces := []string{"4.0.0", "3.62.0"}

	// 4.1.0 version should be added to "skips"
	skipVersion := semver.MustParse("4.1.0")
	versionsWithoutSkips := []string{"4.7.4", "4.6.7"}

	entry := ChannelEntry{
		Name: "rhacs-operator.v" + version.Original(),
	}
	replacesVersion := previousEntryVersion.Original()
	if version.Patch() == 0 {
		replacesVersion = previousChannelVersion.Original()
	}
	if !slices.Contains(versionsWithoutReplaces, version.Original()) {
		entry.Replaces = "rhacs-operator.v" + replacesVersion
	}
	entry.SkipRange = fmt.Sprintf(">= %d.%d.0 < %s", previousChannelVersion.Major(), previousChannelVersion.Minor(), version.Original())
	if skipVersion.LessThan(version) && !slices.Contains(versionsWithoutSkips, version.Original()) {
		entry.Skips = []string{
			fmt.Sprintf("rhacs-operator.v%s", skipVersion.Original()),
		}
	}
	return entry
}

func newChannel(version *semver.Version, entries []ChannelEntry) *Channel {
	return &Channel{
		Schema:  "olm.channel",
		Name:    fmt.Sprintf("rhacs-%d.%d", version.Major(), version.Minor()),
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

func newDeprecationEntry(version *semver.Version) *DeprecationEntry {
	return &DeprecationEntry{
		Reference: DeprecationReference{
			Schema: "olm.channel",
			Name:   fmt.Sprintf("rhacs-%d.%d", version.Major(), version.Minor()),
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

func generateLatestChannel(entries []ChannelEntry) *Channel {
	return &Channel{
		Schema:  "olm.channel",
		Name:    "latest",
		Package: "rhacs-operator",
		Entries: entries,
	}
}

func generateStableChannel(entries []ChannelEntry) *Channel {
	return &Channel{
		Schema:  "olm.channel",
		Name:    "stable",
		Package: "rhacs-operator",
		Entries: entries,
	}
}

func ShouldBePersistentToMajorVersion(version *semver.Version) bool {
	listToKeepForMajorVersion := []string{"4.1.1", "4.1.2", "4.1.3"}

	// If the version is in the list of versions that should be kept for the next channels
	if slices.Contains(listToKeepForMajorVersion, version.Original()) {
		return true
	}
	return version.Patch() == 0
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

func parseAndSortVersions(images []BundleImage) ([]*semver.Version, map[*semver.Version]BundleImage, error) {
	var versions []*semver.Version
	versionToImageMap := make(map[*semver.Version]BundleImage)

	for _, img := range images {
		// ignore image validation for now. Looks like docker library has a bug in it
		_ = validateImageReference(img.Image)

		v, err := semver.StrictNewVersion(img.Tag)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid tag %s: %w", img.Tag, err)
		}

		versionToImageMap[v] = img
		versions = append(versions, v)
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i].LessThan(versions[j])
	})

	return versions, versionToImageMap, nil
}

func addPackageWithIcon(catalogEntries []CatalogEntry) []CatalogEntry {
	iconFile := "icon.png"

	data, err := os.ReadFile(iconFile)
	if err != nil {
		log.Fatalf("Failed to read icon.png: %v", err)
	}
	iconBase64 := base64.StdEncoding.EncodeToString(data)

	packageWithicon := &Package{
		Schema:         "olm.package",
		Name:           "rhacs-operator",
		DefaultChannel: "stable",
		Icon: Icon{
			Base64data: iconBase64,
			MediaType:  "image/png",
		},
	}

	return append(catalogEntries, packageWithicon)
}

func addChannels(catalogEntries []CatalogEntry, versions []*semver.Version) []CatalogEntry {
	channels := make([]CatalogEntry, 0)
	majorPersistentEntries := make([]ChannelEntry, 0)
	// Very first version in the catalog repalces 3.61.0 and skipRanges starts from 3.61.0
	previousEntryVersion := semver.MustParse("3.61.0")
	previousChannelVersion := semver.MustParse("3.61.0")
	lastPersistentMajorVersion := semver.MustParse("3.61.0")
	var channel *Channel

	for n, v := range versions {
		if v.Original() == "4.0.0" {
			majorPersistentEntries = make([]ChannelEntry, 0)
		}

		// Create a new channel entry for each minor version
		if v.Patch() == 0 {
			previousChannelVersion = lastPersistentMajorVersion
			if v.Original() != "3.63.0" {
				if channel != nil {
					channels = append(channels, channel)
				}
				channel = newChannel(v, slices.Clone(majorPersistentEntries))
			}
		}

		catalogChannelEntry := newChannelEntry(v, previousEntryVersion, previousChannelVersion)
		if v.Original() != "3.63.0" {
			channel.Entries = append(channel.Entries, catalogChannelEntry)
		}

		if ShouldBePersistentToMajorVersion(v) {
			majorPersistentEntries = append(majorPersistentEntries, catalogChannelEntry)
			lastPersistentMajorVersion = v
		}

		// Add "latest" channel when "3.74.9" is reached
		if v.Original() == "3.74.9" {
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

	for _, c := range channels {
		catalogEntries = append(catalogEntries, c)
	}

	return catalogEntries
}

func addDeprecations(catalogEntries []CatalogEntry, versions []*semver.Version) []CatalogEntry {
	var deprecations []DeprecationEntry
	var channelVersions []*semver.Version
	for _, v := range versions {
		// assume that each 0 Patch version indicates a new channel except for 3.63.0. There is no 3.63 channel
		if v.Patch() == 0 && v.Original() != "3.63.0" {
			channelVersions = append(channelVersions, v)
		}
	}
	// If there are more than 2 channels, we need to deprecate the older ones
	if len(channelVersions) > 2 {
		for _, v := range channelVersions[:len(channelVersions)-2] {
			deprecations = append(deprecations, *newDeprecationEntry(v))
		}
	}

	return append(catalogEntries, newDeprecation(deprecations))
}

func addBundles(catalogEntries []CatalogEntry, versions []*semver.Version, versionToImageMap map[*semver.Version]BundleImage) []CatalogEntry {
	var bundleEntries []*BundleEntry
	for _, v := range versions {
		bundleEntries = append(bundleEntries, newBundleEntry(versionToImageMap[v].Image))
	}
	for _, bundle := range bundleEntries {
		catalogEntries = append(catalogEntries, bundle)
	}
	return catalogEntries
}

func main() {
	inputBundleList := readBundleListFile()

	versions, versionToImageMap, err := parseAndSortVersions(inputBundleList.Images)
	if err != nil {
		log.Fatalf("Failed to parse versions: %v", err)
	}

	catalog := newCatalogTemplate()
	catalog.Entries = addPackageWithIcon(catalog.Entries)
	catalog.Entries = addChannels(catalog.Entries, versions)
	catalog.Entries = addDeprecations(catalog.Entries, versions)
	catalog.Entries = addBundles(catalog.Entries, versions, versionToImageMap)

	writeCatalogTemplateToFile(catalog)

	fmt.Printf(" %s generated successfully.\n", outputFile)
}
