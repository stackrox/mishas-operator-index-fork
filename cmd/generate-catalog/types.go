package main

import (
	"fmt"
	"slices"

	semver "github.com/Masterminds/semver/v3"
)

// A list of versions which must not have "replaces" key in they channel entries.
var versionsWithoutReplaces = []string{first3MajorVersion, first4MajorVersion}

// Describes format of the input file for catalog template generation.
// It contains:
// - OldestSupportedVersion - the oldest supported version of the operator. All versions < OldestSupportedVersion are marked as deprecated.
// - BrokenVersions - a list of versions which are broken and should be skipped in the catalog.
// - Images - a list of bundle images with their versions.
type Input struct {
	OldestSupportedVersion string             `yaml:"oldest_supported_version"`
	BrokenVersions         []string           `yaml:"broken_versions"`
	Images                 []InputBundleImage `yaml:"images"`
}

type InputBundleImage struct {
	Image   string `yaml:"image"`
	Version string `yaml:"version"`
}

// Describes domain logic configuration for the catalog template generation.
type Configuration struct {
	OldestSupportedVersion *semver.Version
	BrokenVersions         map[*semver.Version]bool
	Images                 []BundleImage
	Versions               []*semver.Version
}

type BundleImage struct {
	Image   string
	Version *semver.Version
}

// Describes catalog template structure which is used to generate the catalog YAML file.
// See OLM catalog template documentation for more details: https://olm.operatorframework.io/docs/reference/catalog-templates/
type CatalogTemplate struct {
	Schema  string         `yaml:"schema"`
	Entries []CatalogEntry `yaml:"entries"`
}

type CatalogEntry interface {
	isCatalogEntry()
}

func (Package) isCatalogEntry()      {}
func (Channel) isCatalogEntry()      {}
func (Deprecations) isCatalogEntry() {}
func (BundleEntry) isCatalogEntry()  {}

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
	Schema         string          `yaml:"schema"`
	Name           string          `yaml:"name"`
	Package        string          `yaml:"package"`
	Entries        []ChannelEntry  `yaml:"entries"`
	YStreamVersion *semver.Version `yaml:"-"`
}

type ChannelEntry struct {
	Name      string          `yaml:"name"`
	Replaces  string          `yaml:"replaces,omitempty"`
	SkipRange string          `yaml:"skipRange"`
	Skips     []string        `yaml:"skips,omitempty"`
	Version   *semver.Version `yaml:"-"`
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

// Create base catalog template block.
// It has to contain objects with schema equal to: "olm.package", "olm.channel", "olm.deprecations" or "olm.bundle".
func newCatalogTemplate() CatalogTemplate {
	return CatalogTemplate{
		Schema: "olm.template.basic",
	}
}

// addPackage adds a "olm.package" object to the base catalog.
func (c *CatalogTemplate) addPackage(pkg Package) {
	c.Entries = append(c.Entries, CatalogEntry(pkg))
}

// addChannels adds a list of "olm.channel" objects to the base catalog.
func (c *CatalogTemplate) addChannels(channels []Channel) {
	for _, channel := range channels {
		c.Entries = append(c.Entries, CatalogEntry(channel))
	}
}

// addDeprecations adds a "olm.deprecations" object to the base catalog.
func (c *CatalogTemplate) addDeprecations(deprecations Deprecations) {
	c.Entries = append(c.Entries, CatalogEntry(deprecations))
}

// addBundles adds a list of "olm.bundle" objects to the base catalog.
func (c *CatalogTemplate) addBundles(bundles []BundleEntry) {
	for _, bundle := range bundles {
		c.Entries = append(c.Entries, CatalogEntry(bundle))
	}
}

// Create a new "olm.channel" object which should be added to the catalog base.
// it will be represented in YAML like this:
// |  - schema: olm.channel
// |    name: rhacs-3.64
// |    package: rhacs-operator
// |    entries:
// |      - <ChannelEntry>
func newChannel(version *semver.Version) Channel {
	return Channel{
		Schema:         "olm.channel",
		Name:           fmt.Sprintf("rhacs-%d.%d", version.Major(), version.Minor()),
		Package:        "rhacs-operator",
		YStreamVersion: makeYStreamVersion(version),
	}
}

// Create a special "olm.channel" object with name "latest".
// It is a deprecated channel which was used before 4.x.x version.
func newLatestChannel() Channel {
	return Channel{
		Schema:  "olm.channel",
		Name:    latestChannelName,
		Package: "rhacs-operator",
	}
}

// Create a special "olm.channel" object with name "stable".
// It is a default channel for all versions after 4.x.x.
func newStableChannel() Channel {
	return Channel{
		Schema:  "olm.channel",
		Name:    stableChannelName,
		Package: "rhacs-operator",
	}
}

// Create a new Chanel entry object which should be added to Channel entries list.
// it will be represented in YAML like this:
// |  - name: rhacs-operator.v<version>
// |    replaces: rhacs-operator.v<previousEntryVersion>
// |    skipRange: '>= <previousYStreamVersion> < <version>'
// |    skips:
// |      - rhacs-operator.v<skippedVersions>
func newChannelEntry(version, previousEntryVersion, previousYStreamVersion *semver.Version, skippedVersions map[*semver.Version]bool) ChannelEntry {
	entry := ChannelEntry{
		Name:    generateBundleName(version),
		Version: version,
	}
	entry.addReplaces(version, previousEntryVersion)
	entry.addSkipRange(version, previousYStreamVersion)
	entry.addSkips(version, skippedVersions)
	return entry
}

func (e *ChannelEntry) addReplaces(version, previousEntryVersion *semver.Version) {
	if !slices.Contains(versionsWithoutReplaces, version.Original()) {
		e.Replaces = generateBundleName(previousEntryVersion)
	}
}

func (e *ChannelEntry) addSkipRange(version, previousYStreamVersion *semver.Version) {
	skipRangeFrom := previousYStreamVersion.String()
	skipRangeTo := version.Original()
	e.SkipRange = fmt.Sprintf(">= %s < %s", skipRangeFrom, skipRangeTo)
}

func (e *ChannelEntry) addSkips(version *semver.Version, skippedVersions map[*semver.Version]bool) {
	for skippedVersion := range skippedVersions {
		// for any broken X.Y.Z version add "skips" for all versions > X.Y.Z and < X.Y+brokenVersionSkippingOffset.0
		skipsUntilVersion := semver.New(skippedVersion.Major(), skippedVersion.Minor()+brokenVersionSkippingOffset, 0, "", "")
		if version.GreaterThan(skippedVersion) && version.LessThan(skipsUntilVersion) {
			e.Skips = append(e.Skips, generateBundleName(skippedVersion))
		}
	}
}

// Create a new "olm.deprecations" object which should be added to the catalog base.
// It will be represented in YAML like this:
// |  - schema: olm.deprecations
// |    package: rhacs-operator
// |    entries:
// |      - <DeprecationEntry>
func newDeprecations(entries []DeprecationEntry) Deprecations {
	return Deprecations{
		Schema:  "olm.deprecations",
		Package: "rhacs-operator",
		Entries: entries,
	}
}

// Create a new channel DeprecationEntry reference object which should be added to Deprecation reference list.
// it will be represented in YAML like this:
// |  - reference:
// |    schema: olm.channel
// |    name: rhacs-<version>
// |    message: |
// |      <message>
func newChannelDeprecationEntry(name string, message string) DeprecationEntry {
	return DeprecationEntry{
		Reference: DeprecationReference{
			Schema: "olm.channel",
			Name:   name,
		},
		Message: message,
	}
}

// Create a new channel DeprecationEntry reference object which should be added to Deprecation reference list.
// it will be represented in YAML like this:
// |  - reference:
// |    schema: olm.bundle
// |    name: rhacs-<version>
// |    message: |
// |      <message>
func newBundleDeprecationEntry(version *semver.Version, message string) DeprecationEntry {
	return DeprecationEntry{
		Reference: DeprecationReference{
			Schema: "olm.bundle",
			Name:   generateBundleName(version),
		},
		Message: message,
	}
}

// Create a new "olm.bundle" object which should be added to the catalog base.
// it will be represented in YAML like this:
// |  - image: <bundle_image_reference>
// |    schema: olm.bundle
func newBundleEntry(image string) BundleEntry {
	return BundleEntry{
		Schema: "olm.bundle",
		Image:  image,
	}
}

// getAllVersions extracts all operator versions from the input images.
func getAllVersions(images []BundleImage) []*semver.Version {
	versions := make([]*semver.Version, 0, len(images))
	for _, img := range images {
		versions = append(versions, img.Version)
	}
	return versions
}

func generateBundleName(version *semver.Version) string {
	return fmt.Sprintf("rhacs-operator.v%s", version.Original())
}

func makeYStreamVersion(v *semver.Version) *semver.Version {
	return semver.New(v.Major(), v.Minor(), 0, "", "")
}
