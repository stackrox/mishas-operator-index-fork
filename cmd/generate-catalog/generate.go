package main

import (
	// needed for digest algorithm validation
	_ "crypto/sha256"

	"encoding/base64"
	"fmt"
	"log"
	"os"
	"slices"

	semver "github.com/Masterminds/semver/v3"
	"github.com/distribution/reference"
	"github.com/goccy/go-yaml"
)

const (
	inputFile                       = "bundles.yaml"
	outputFile                      = "catalog-template.yaml"
	channelDeprecationMessage       = "This version is no longer supported. Please switch to the `stable` channel or a channel for a version that is still supported.\n"
	bundleDeprecationMessage        = "This operator bundle version is no longer supported. Please switch to non deprecated bundle version for support.\n"
	deprecationMessageLatestChannel = "The `latest` channel is no longer supported.  Please switch to the `stable` channel.\n"
)

type Input struct {
	OldestSupportedVersion *semver.Version   `yaml:"oldest_supported_version"`
	BrokenVersions         []*semver.Version `yaml:"broken_versions"`
	Images                 []BundleImage     `yaml:"images"`
}

type BundleImage struct {
	Image   string          `yaml:"image"`
	Version *semver.Version `yaml:"version"`
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
	input, err := readInputFile()
	if err != nil {
		log.Fatalf("Failed to read bundle list file: %v", err)
	}

	versions := make([]*semver.Version, 0)
	for _, img := range input.Images {
		versions = append(versions, img.Version)
	}

	versionToImageMap, err := buildMapVersionToImage(input.Images)
	if err != nil {
		log.Fatalf("Failed to parse versions: %v", err)
	}

	pkg, err := generatePackageWithIcon()
	if err != nil {
		log.Fatalf("Failed to generate package object with icon: %v", err)
	}
	channels := generateChannels(versions, input.BrokenVersions)
	deprecations := generateDeprecations(versions, input.OldestSupportedVersion)
	bundles := generateBundles(versions, versionToImageMap)

	catalog := newCatalogTemplate()
	catalog.addPackage(pkg)
	catalog.addChannels(channels)
	catalog.addDeprecations(deprecations)
	catalog.addBundles(bundles)

	writeCatalogTemplateToFile(catalog)

	fmt.Printf("%s generated successfully.\n", outputFile)
}

// readInputFile reads the operator bundle image list, broken versions list and the latest supported version from the input YAML file.
func readInputFile() (Input, error) {
	inputBytes, err := os.ReadFile(inputFile)
	if err != nil {
		return Input{}, fmt.Errorf("Failed to read %s: %v", inputFile, err)
	}
	var input Input
	if err := yaml.Unmarshal(inputBytes, &input); err != nil {
		return Input{}, fmt.Errorf("Failed to parse YAML: %v", err)
	}

	for i := 0; i < len(input.Images)-1; i++ {
		version := input.Images[i].Version
		nextVersion := input.Images[i+1].Version
		if version.GreaterThan(nextVersion) {
			return Input{}, fmt.Errorf("Operator bundle images are not sorted in ascending order: %s > %s", version.Original(), nextVersion.Original())
		}
	}

	return input, nil
}

// buildMapVersionToImage returns a mapping from version to BundleImage.
func buildMapVersionToImage(images []BundleImage) (map[*semver.Version]BundleImage, error) {
	versionToImageMap := make(map[*semver.Version]BundleImage)

	for _, img := range images {
		err := validateImageReference(img.Image)
		if err != nil {
			return nil, fmt.Errorf("invalid image reference %q: %w", img.Image, err)
		}
		versionToImageMap[img.Version] = img
	}

	return versionToImageMap, nil
}

// generatePackageWithIcon creates a new "olm.package" object with an operator icon.
func generatePackageWithIcon() (Package, error) {
	iconFile := "icon.png"

	data, err := os.ReadFile(iconFile)
	if err != nil {
		return Package{}, fmt.Errorf("Failed to read icon.png: %v", err)
	}
	iconBase64 := base64.StdEncoding.EncodeToString(data)

	packageWithIcon := Package{
		Schema:         "olm.package",
		Name:           "rhacs-operator",
		DefaultChannel: "stable",
		Icon: Icon{
			Base64data: iconBase64,
			MediaType:  "image/png",
		},
	}

	return packageWithIcon, nil
}

// generateChannels creates a list of channels based on the provided bundle versions.
func generateChannels(versions []*semver.Version, brokenVersions []*semver.Version) []Channel {
	channels := make([]Channel, 0)
	majorEntries := make([]ChannelEntry, 0)
	// Very first version in the catalog replaces 3.61.0 and skipRanges starts from 3.61.0
	previousEntryVersion := semver.MustParse("3.61.0")
	previousChannelVersion := semver.MustParse("3.61.0")
	var channel *Channel

	for n, v := range versions {
		// Refresh major channel entries list when new major version is reached
		if v.Major() != previousEntryVersion.Major() {
			majorEntries = make([]ChannelEntry, 0)
		}

		// Create a new channel entry for each new minor version (patch = 0)
		if v.Patch() == 0 {
			previousChannelVersion = previousEntryVersion
			if v.Original() != "3.63.0" {
				if channel != nil {
					channels = append(channels, *channel)
				}
				channel = newChannel(v, slices.Clone(majorEntries))
			}
		}

		catalogChannelEntry := newChannelEntry(v, previousEntryVersion, previousChannelVersion, brokenVersions)
		if v.Original() != "3.63.0" {
			channel.Entries = append(channel.Entries, catalogChannelEntry)
		}
		majorEntries = append(majorEntries, catalogChannelEntry)

		// Add "latest" channel when "3.74.9" is reached
		if v.Original() == "3.74.9" {
			latestChannel := generateLatestChannel(channel.Entries)
			channels = append(channels, latestChannel)
		}

		// Add "stable" channel when the last version is reached
		if n == len(versions)-1 {
			channels = append(channels, *channel)
			stableChannel := generateStableChannel(channel.Entries)
			channels = append(channels, stableChannel)
		}

		previousEntryVersion = v
	}

	return channels
}

// generateDeprecations creates an object with a list of deprecations based on the provided versions.
func generateDeprecations(versions []*semver.Version, oldestSupportedVersion *semver.Version) Deprecations {
	var deprecations []DeprecationEntry
	var channelVersions []*semver.Version
	for _, v := range versions {
		// assume that each 0 Patch version indicates a new channel except for 3.63.0. There is no 3.63 channel
		if v.Patch() == 0 && v.Original() != "3.63.0" {
			channelVersions = append(channelVersions, v)
		}
	}

	// Deprecate all channels that are older than the oldest supported version
	for _, channelVersion := range channelVersions {
		if channelVersion.LessThan(oldestSupportedVersion) {
			deprecations = append(deprecations, newChannelDeprecationEntry(channelVersion))
		}
	}

	// Deprecate all bundles that are older than the oldest supported version
	for _, v := range versions {
		if v.LessThan(oldestSupportedVersion) {
			deprecations = append(deprecations, newBundleDeprecationEntry(v))
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
func (c *CatalogTemplate) addPackage(pkg Package) {
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
		Name:    generateChannelName(version),
		Package: "rhacs-operator",
		Entries: entries,
	}
}

// Create a special "olm.channel" object with name "latest".
// It is a deprecated channel which was used before 4.x.x version.
func newLatestChannel(entries []ChannelEntry) Channel {
	return Channel{
		Schema:  "olm.channel",
		Name:    "latest",
		Package: "rhacs-operator",
		Entries: entries,
	}
}

// Create a special "olm.channel" object with name "stable".
// It is a default channel for all versions after 4.x.x.
func newStableChannel(entries []ChannelEntry) Channel {
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
func newChannelEntry(version, previousEntryVersion, previousChannelVersion *semver.Version, brokenVersions []*semver.Version) ChannelEntry {
	entry := ChannelEntry{
		Name: generateBundleName(version),
	}
	entry.addReplaces(version, previousEntryVersion)
	entry.addSkipRange(version, previousChannelVersion)
	entry.addSkips(version, brokenVersions)
	return entry
}

func (entry *ChannelEntry) addReplaces(version, previousEntryVersion *semver.Version) {
	// skip setting "replaces" key for specific versions
	versionsWithoutReplaces := []string{"4.0.0", "3.62.0"}

	replacesVersion := previousEntryVersion.Original()
	if !slices.Contains(versionsWithoutReplaces, version.Original()) {
		entry.Replaces = "rhacs-operator.v" + replacesVersion
	}
}

func (entry *ChannelEntry) addSkipRange(version, previousChannelVersion *semver.Version) {
	skipRangeGreaterThanEqual := fmt.Sprintf("%d.%d.0", previousChannelVersion.Major(), previousChannelVersion.Minor())
	skipRangeLessThan := version.Original()
	entry.SkipRange = fmt.Sprintf(">= %s < %s", skipRangeGreaterThanEqual, skipRangeLessThan)
}

func (entry *ChannelEntry) addSkips(version *semver.Version, brokenVersions []*semver.Version) {
	for _, brokenVersion := range brokenVersions {
		// for any broken X.Y.Z version add "skips" for all versions > X.Y.Z and < X.Y+2.0
		skipsUntilVersion := semver.MustParse(fmt.Sprintf("%d.%d.0", brokenVersion.Major(), brokenVersion.Minor()+2))
		if version.GreaterThan(brokenVersion) && version.LessThan(skipsUntilVersion) {
			entry.Skips = append(entry.Skips, fmt.Sprintf("rhacs-operator.v%s", brokenVersion.Original()))
		}
	}
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

// Create a new channel DeprecationEntry reference object which should be added to Deprecation reference list.
// it will be represented in YAML like this:
//   - reference:
//     schema: olm.channel
//     name: rhacs-<version>
//     message: |
//     <message>
func newChannelDeprecationEntry(version *semver.Version) DeprecationEntry {
	return DeprecationEntry{
		Reference: DeprecationReference{
			Schema: "olm.channel",
			Name:   generateChannelName(version),
		},
		Message: channelDeprecationMessage,
	}
}

// Create a new channel DeprecationEntry reference object which should be added to Deprecation reference list.
// it will be represented in YAML like this:
//   - reference:
//     schema: olm.bundle
//     name: rhacs-<version>
//     message: |
//     <message>
func newBundleDeprecationEntry(version *semver.Version) DeprecationEntry {
	return DeprecationEntry{
		Reference: DeprecationReference{
			Schema: "olm.bundle",
			Name:   generateBundleName(version),
		},
		Message: bundleDeprecationMessage,
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

func generateChannelName(version *semver.Version) string {
	return fmt.Sprintf("rhacs-%d.%d", version.Major(), version.Minor())
}

func generateBundleName(version *semver.Version) string {
	return fmt.Sprintf("rhacs-operator.v%d.%d.%d", version.Major(), version.Minor(), version.Patch())
}

// validateImageReference validates that the provided image string is a valid container image reference with a digest
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
