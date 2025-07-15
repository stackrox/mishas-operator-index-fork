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
	outputFile                      = "catalog-template.yaml"
	deprecationMessage              = "This version is no longer supported. Please switch to the `stable` channel or a channel for a version that is still supported.\n"
	deprecationMessageLatestChannel = "The `latest` channel is no longer supported.  Please switch to the `stable` channel.\n"
)

type BundleImage struct {
	Image string `yaml:"image"`
	Tag   string `yaml:"tag"`
}

type BundleList struct {
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

type Deprecations struct {
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

func main() {
	inputBundleList, err := readBundleListFile()
	if err != nil {
		log.Fatalf("Failed to read bundle list file: %v", err)
	}

	versions, versionToImageMap, err := parseAndSortVersions(inputBundleList.Images)
	if err != nil {
		log.Fatalf("Failed to parse versions: %v", err)
	}

	pkg, err := generatePackageWithIcon()
	if err != nil {
		log.Fatalf("Failed to generate package object with icon: %v", err)
	}
	channels := generateChannels(versions)
	deprecations := generateDeprecations(versions)
	bundles := generateBundles(versions, versionToImageMap)

	catalog := newCatalogTemplate()
	catalog.addPackage(pkg)
	catalog.addChannels(channels)
	catalog.addDeprecations(deprecations)
	catalog.addBundles(bundles)

	writeCatalogTemplateToFile(catalog)

	fmt.Printf("%s generated successfully.\n", outputFile)
}

// readBundleListFile reads the bundle list from the input YAML file.
func readBundleListFile() (BundleList, error) {
	inputBytes, err := os.ReadFile(inputFile)
	if err != nil {
		return BundleList{}, fmt.Errorf("Failed to read %s: %v", inputFile, err)
	}
	var bundleList BundleList
	if err := yaml.Unmarshal(inputBytes, &bundleList); err != nil {
		return BundleList{}, fmt.Errorf("Failed to parse YAML: %v", err)
	}

	return bundleList, nil
}

// parseAndSortVersions parses the bundle images and versions from the list of BundleImage
// and returns a sorted list of versions along with a map from version to BundleImage mapping.
func parseAndSortVersions(images []BundleImage) ([]*semver.Version, map[*semver.Version]BundleImage, error) {
	var versions []*semver.Version
	versionToImageMap := make(map[*semver.Version]BundleImage)

	for _, img := range images {
		// ignore image validation for now. Looks like docker library has a bug in it
		err := validateImageReference(img.Image)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid image reference %s: %w", img.Image, err)
		}

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

// generatePackageWithIcon creates a new "olm.package" object with an operator icon.
func generatePackageWithIcon() (Package, error) {
	iconFile := "icon.png"

	data, err := os.ReadFile(iconFile)
	if err != nil {
		return Package{}, fmt.Errorf("Failed to read icon.png: %v", err)
	}
	iconBase64 := base64.StdEncoding.EncodeToString(data)

	packageWithicon := Package{
		Schema:         "olm.package",
		Name:           "rhacs-operator",
		DefaultChannel: "stable",
		Icon: Icon{
			Base64data: iconBase64,
			MediaType:  "image/png",
		},
	}

	return packageWithicon, nil
}

// generateChannels creates a list of channels based on the provided bundle versions.
func generateChannels(versions []*semver.Version) []Channel {
	channels := make([]Channel, 0)
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
					channels = append(channels, *channel)
				}
				channel = newChannel(v, slices.Clone(majorPersistentEntries))
			}
		}

		catalogChannelEntry := newChannelEntry(v, previousEntryVersion, previousChannelVersion)
		if v.Original() != "3.63.0" {
			channel.Entries = append(channel.Entries, catalogChannelEntry)
		}

		if shouldBePersistentToMajorVersion(v) {
			majorPersistentEntries = append(majorPersistentEntries, catalogChannelEntry)
			lastPersistentMajorVersion = v
		}

		// Add "latest" channel when "3.74.9" is reached
		if v.Original() == "3.74.9" {
			latestChannel := generateLatestChannel(channel.Entries)
			channels = append(channels, latestChannel)
		}

		if n == len(versions)-1 {
			channels = append(channels, *channel)

			// Add "stable" channel when the last version is reached
			stableChannel := generateStableChannel(channel.Entries)
			channels = append(channels, stableChannel)
		}

		previousEntryVersion = v
	}

	return channels
}

// generateDeprecations creates an object with a list of deprecations based on the provided versions.
func generateDeprecations(versions []*semver.Version) Deprecations {
	var deprecations []DeprecationEntry
	var channelVersions []*semver.Version
	for _, v := range versions {
		// assume that each 0 Patch version indicates a new channel except for 3.63.0. There is no 3.63 channel
		if v.Patch() == 0 && v.Original() != "3.63.0" {
			channelVersions = append(channelVersions, v)
		}
	}
	// TODO: mark as  deprecated based on the ipput bundle list parameter
	// If there are more than 3 channels, we need to deprecate the older ones
	if len(channelVersions) > 3 {
		for _, v := range channelVersions[:len(channelVersions)-3] {
			deprecations = append(deprecations, *newDeprecationEntry(v))
		}
	}

	return newDeprecation(deprecations)
}

// generateBundles creates a list of bundle entries based on the provided versions and their corresponding images.
func generateBundles(versions []*semver.Version, versionToImageMap map[*semver.Version]BundleImage) []BundleEntry {
	var bundleEntries []BundleEntry
	for _, v := range versions {
		bundleEntries = append(bundleEntries, newBundleEntry(versionToImageMap[v].Image))
	}
	return bundleEntries
}

// Create base catalog template block.
// It has to contain objects with schema equal to: "olm.package", "olm.channel", "olm.deprecations" or "olm.bundle".
func newCatalogTemplate() CatalogTemplate {
	return CatalogTemplate{
		Schema: "olm.template.basic",
	}
}

// addPackage adds a "olm.package" object to the base catalog.
func (catalog *CatalogTemplate) addPackage(pkg Package) {
	catalog.Entries = append(catalog.Entries, CatalogEntry(pkg))
}

// addChannels adds a list of "olm.channel" objects to the base catalog.
func (catalog *CatalogTemplate) addChannels(channels []Channel) {
	for _, channel := range channels {
		catalog.Entries = append(catalog.Entries, CatalogEntry(channel))
	}
}

// addDeprecations adds a "olm.deprecations" object to the base catalog.
func (catalog *CatalogTemplate) addDeprecations(deprecations Deprecations) {
	catalog.Entries = append(catalog.Entries, CatalogEntry(deprecations))
}

// addBundles adds a list of "olm.bundle" objects to the base catalog.
func (catalog *CatalogTemplate) addBundles(bundles []BundleEntry) {
	for _, bundle := range bundles {
		catalog.Entries = append(catalog.Entries, CatalogEntry(bundle))
	}
}

// writeCatalogTemplateToFile writes the resulting catalog template to the output YAML file.
func writeCatalogTemplateToFile(catalog CatalogTemplate) error {
	out, err := yaml.Marshal(&catalog)
	if err != nil {
		return fmt.Errorf("Failed to marshal catalog: %v", err)
	}
	if err := os.WriteFile(outputFile, out, 0644); err != nil {
		return fmt.Errorf("Failed to write output: %v", err)
	}

	return nil
}

// Create a new "olm.channel" object which should be added to the catalog base.
// it will be represented in YAML like this:
//   - schema: olm.channel
//     name: rhacs-3.64
//     package: rhacs-operator
//     entries:
//   - <ChannelEntry>
func newChannel(version *semver.Version, entries []ChannelEntry) *Channel {
	return &Channel{
		Schema:  "olm.channel",
		Name:    fmt.Sprintf("rhacs-%d.%d", version.Major(), version.Minor()),
		Package: "rhacs-operator",
		Entries: entries,
	}
}

// Create a special "olm.channel" object with name "latest".
// It is a deprecated channel which was used before 4.x.x version.
func generateLatestChannel(entries []ChannelEntry) Channel {
	return Channel{
		Schema:  "olm.channel",
		Name:    "latest",
		Package: "rhacs-operator",
		Entries: entries,
	}
}

// Create a special "olm.channel" object with name "stable".
// It is a default channel for all versions after 4.x.x.
func generateStableChannel(entries []ChannelEntry) Channel {
	return Channel{
		Schema:  "olm.channel",
		Name:    "stable",
		Package: "rhacs-operator",
		Entries: entries,
	}
}

// Create a new Chanel entry object which should be added to Channel entries list.
// it will be represented in YAML like this:
//   - name: rhacs-operator.v<version>
//     replaces: rhacs-operator.v<previousEntryVersion>
//     skipRange: '>= <previousChannelVersion> < <version>'
//     skips:
//   - rhacs-operator.v4.1.0
func newChannelEntry(version *semver.Version, previousEntryVersion *semver.Version, previousChannelVersion *semver.Version) ChannelEntry {
	// skip setting "replaces" key for specific versions
	versionsWithoutReplaces := []string{"4.0.0", "3.62.0"}

	// after 4.1.0 version should be added "skips"
	skipVersion := semver.MustParse("4.1.0")
	versionsWithoutSkips := []string{"4.7.4", "4.6.7"}
	// all version after 4.7.4 should not have skips
	withoutSkipVersion := semver.MustParse("4.7.4")

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
	if version.GreaterThan(skipVersion) && version.LessThan(withoutSkipVersion) && !slices.Contains(versionsWithoutSkips, version.Original()) {
		entry.Skips = []string{
			fmt.Sprintf("rhacs-operator.v%s", skipVersion.Original()),
		}
	}
	return entry
}

// Create a new "olm.deprecations" object which should be added to the catalog base.
// it will be represented in YAML like this:
//   - schema: olm.deprecations
//     package: rhacs-operator
//     entries:
//   - <DeprecationEntry>
func newDeprecation(entries []DeprecationEntry) Deprecations {
	// Add a deprecation entry for the "latest" channel
	latestDeprecationEntry := &DeprecationEntry{
		Reference: DeprecationReference{
			Schema: "olm.channel",
			Name:   "latest",
		},
		Message: deprecationMessageLatestChannel,
	}
	entries = slices.Insert(entries, 0, *latestDeprecationEntry)

	return Deprecations{
		Schema:  "olm.deprecations",
		Package: "rhacs-operator",
		Entries: entries,
	}
}

// Create a newdDeprecation reference object which should be added to Deprecation reference list.
// it will be represented in YAML like this:
//   - reference:
//     schema: olm.channel
//     name: rhacs-<version>
//     message: |
//     <message>
func newDeprecationEntry(version *semver.Version) *DeprecationEntry {
	return &DeprecationEntry{
		Reference: DeprecationReference{
			Schema: "olm.channel",
			Name:   fmt.Sprintf("rhacs-%d.%d", version.Major(), version.Minor()),
		},
		Message: deprecationMessage,
	}
}

// Create a new "olm.bundle" object which should be added to the catalog base.
// it will be represented in YAML like this:
//   - image: <bundle_image_reference>
//     schema: olm.bundle
func newBundleEntry(image string) BundleEntry {
	return BundleEntry{
		Schema: "olm.bundle",
		Image:  image,
	}
}

// shouldBePersistentToMajorVersion checks if the version should be persistent to the next major version.
func shouldBePersistentToMajorVersion(version *semver.Version) bool {
	listToKeepForMajorVersion := []string{"4.1.1", "4.1.2", "4.1.3"}
	// Define version after which all versions should be persistent
	thresholdVersion := semver.MustParse("4.7.0")

	// If the version is in the list of versions that should be kept for the next channels
	if slices.Contains(listToKeepForMajorVersion, version.Original()) || version.GreaterThanEqual(thresholdVersion) {
		return true
	}
	return version.Patch() == 0
}

// validateImageReference validates that the provided image string is a valid cintainer image reference with a digest
func validateImageReference(image string) error {
	// Validate the image reference using the distribution/reference package
	ref, err := reference.Parse(image)
	if err != nil {
		return fmt.Errorf("Cannot parse string as docker image %s: %w", image, err)
	}
	if _, ok := ref.(reference.Canonical); !ok {
		return fmt.Errorf("image reference does not include a digest")
	}
	return nil
}
