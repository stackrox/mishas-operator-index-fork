package main

import (
	"fmt"
	"os"
	"slices"

	semver "github.com/Masterminds/semver/v3"
	"github.com/goccy/go-yaml"
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
func (catalog *CatalogTemplate) writeCatalogTemplateToFile() error {
	headComment := yaml.HeadComment(
		"---------------------------------------------------------------------------",
		firstLineHeadComment,
		secondLineHeadComment,
		"---------------------------------------------------------------------------",
	)
	comments := yaml.CommentMap{
		"$": []*yaml.Comment{headComment}, // "$" means top-level comment
	}

	out, err := yaml.MarshalWithOptions(&catalog, yaml.WithComment(comments))
	if err != nil {
		return fmt.Errorf("failed to marshal catalog: %v", err)
	}
	if err := os.WriteFile(outputFile, out, 0644); err != nil {
		return fmt.Errorf("failed to write output: %v", err)
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
