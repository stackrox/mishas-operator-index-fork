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

func buildCatalogChannelEntry(version Version, previousVersion Version, previousMinor string) CatalogChannelEntry {
	// skip setting "replaces" key for specific versions
	versionsWithoutReplaces := []string{"4.0.0", "3.62.0"}

	entry := CatalogChannelEntry{
		Name: "rhacs-operator.v" + version.Tag,
	}
	if !slices.Contains(versionsWithoutReplaces, version.Tag) {
		entry.Replaces = "rhacs-operator.v" + previousVersion.Tag
	}
	entry.SkipRange = fmt.Sprintf("'>= %s < %s'", previousMinor, version.Tag)
	return entry
}

func newChannelEntry(version Version) *ChannelEntry {
	return &ChannelEntry{
		Schema:  "olm.channel",
		Name:    fmt.Sprintf("rhacs-%d.%d", version.Major, version.Minor),
		Package: "rhacs-operator",
		Entries: []CatalogChannelEntry{},
	}
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

	// Build channel maps
	channels := make(map[string][]CatalogChannelEntry)
	minorVersions := make(map[string][]string)
	majorVersions := make(map[int][]string)

	for _, v := range versions {
		minorKey := fmt.Sprintf("%d.%d", v.Major, v.Minor)
		minorVersions[minorKey] = append(minorVersions[minorKey], v.Tag)
		majorVersions[v.Major] = append(majorVersions[v.Major], v.Tag)
	}

	// Build previous version mapping
	previousVersionMap := make(map[string]string)
	for i, v := range versions {
		if i > 0 {
			previousVersionMap[v.Tag] = versions[i-1].Tag
		}
	}

	for minor, tags := range minorVersions {
		var entries []CatalogChannelEntry
		for i, tag := range tags {
			entry := CatalogChannelEntry{
				Name: "rhacs-operator.v" + tag,
			}
			// replaces
			if i > 0 {
				entry.Replaces = "rhacs-operator.v" + tags[i-1]
			}
			// skips for >=4.1.1
			v, _ := parseVersion(tag)
			if (v.Major > 4) ||
				(v.Major == 4 && v.Minor > 1) ||
				(v.Major == 4 && v.Minor == 1 && v.Patch >= 1) {
				entry.Skips = []string{"rhacs-operator.v4.1.0"}
			}
			prevMinor := v.Minor - 1
			if prevMinor >= 0 {
				entry.SkipRange = fmt.Sprintf(">=%d.%d.0 <=%d.%d.%d", v.Major, prevMinor, v.Major, v.Minor, v.Patch)
			}
			entries = append(entries, entry)
		}
		channels[minor] = entries
	}

	var latestMajors []int
	for major := range majorVersions {
		latestMajors = append(latestMajors, major)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(latestMajors)))
	if len(latestMajors) > 3 {
		latestMajors = latestMajors[:3]
	}
	var latestTags []string
	for _, major := range latestMajors {
		latestTags = append(latestTags, majorVersions[major]...)
	}
	sort.Slice(latestTags, func(i, j int) bool {
		va, _ := parseVersion(latestTags[i])
		vb, _ := parseVersion(latestTags[j])
		return versionLess(va, vb)
	})
	var latestEntries []CatalogChannelEntry
	for i, tag := range latestTags {
		entry := CatalogChannelEntry{
			Name: "rhacs-operator.v" + tag,
		}
		if i > 0 {
			entry.Replaces = "rhacs-operator.v" + latestTags[i-1]
		}
		latestEntries = append(latestEntries, entry)
	}
	channels["latest"] = latestEntries

	var stableMajors []int
	for major := range majorVersions {
		stableMajors = append(stableMajors, major)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(stableMajors)))
	if len(stableMajors) > 4 {
		stableMajors = stableMajors[:4]
	}
	var stableTags []string
	for _, major := range stableMajors {
		stableTags = append(stableTags, majorVersions[major]...)
	}
	sort.Slice(stableTags, func(i, j int) bool {
		va, _ := parseVersion(stableTags[i])
		vb, _ := parseVersion(stableTags[j])
		return versionLess(va, vb)
	})
	var stableEntries []CatalogChannelEntry
	for i, tag := range stableTags {
		entry := CatalogChannelEntry{
			Name: "rhacs-operator.v" + tag,
		}
		if i > 0 {
			entry.Replaces = "rhacs-operator.v" + stableTags[i-1]
		}
		stableEntries = append(stableEntries, entry)
	}
	channels["stable"] = stableEntries

	var deprecations []string
	if len(versions) > 2 {
		for _, v := range versions[:len(versions)-2] {
			deprecations = append(deprecations, "rhacs-operator.v"+v.Tag)
		}
	}

	catalog := CatalogTemplate{
		Channels:     channels,
		Deprecations: deprecations,
		Images:       input.Images,
	}

	var baseEntries []CatalogEntry

	// Very first version in the catalog repalces 3.61.0 and skipRanges starts from 3.61.0
	previousVersion := forceParseVersion("3.61.0")
	previousMinor := "3.61.0"

	// Add dummy version to properly traverse all version
	versions = append(versions, forceParseVersion("9.9.9"))

	var channel *ChannelEntry = nil
	for _, v := range versions {

		catalogChannelEntry := buildCatalogChannelEntry(v, previousVersion, previousMinor)

		// Create a new channel entry for each minor version
		if v.Minor != previousVersion.Minor {
			if channel != nil {
				baseEntries = append(baseEntries, channel)
			}
			channel = newChannelEntry(v)
			channel.Entries = append(channel.Entries, catalogChannelEntry)
		}

		log.Printf("Processing version: %d.%d.%d: %s", v.Major, v.Minor, v.Patch, v.Tag)
	}

	catalogbase := CatalogBase{
		Schema:  "olm.template.basic",
		Entries: baseEntries,
	}

	fmt.Printf("Catalog Base Schema: %s\n", catalogbase.Schema)

	out, err := yaml.Marshal(catalog)
	if err != nil {
		log.Fatalf("Failed to marshal output: %v", err)
	}
	if err := os.WriteFile(outputFile, out, 0644); err != nil {
		log.Fatalf("Failed to write output: %v", err)
	}

	fmt.Println("catalog-template.yaml generated successfully.")
}
