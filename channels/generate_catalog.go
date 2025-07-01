package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type ChannelYaml struct {
	Name    string       `yaml:"name"`
	Entries []string     `yaml:"entries"`
	Images  []ImageEntry `yaml:"images"`
}

type EntryYaml struct {
	Name  string `yaml:"name"`
	Image string `yaml:"image"`
}

type ImageEntry struct {
	Image string `yaml:"image"`
}

type Entry struct {
	Name      string   `yaml:"name"`
	Replaces  string   `yaml:"replaces"`
	SkipRange string   `yaml:"skipRange"`
	Skips     []string `yaml:"skips,omitempty"`
}

type Channel struct {
	Name    string   `yaml:"name"`
	Entries []Entry  `yaml:"entries"`
	Images  []string `yaml:"images"`
}

// Parse bundle string like "bundle-4-7-0" into its version components
func parseBundleString(bundle string) (major, minor, patch int, err error) {
	parts := strings.Split(bundle, "-")
	if len(parts) < 4 {
		return 0, 0, 0, fmt.Errorf("invalid bundle string: %s", bundle)
	}
	major, err = strconv.Atoi(parts[1])
	if err != nil {
		return
	}
	minor, err = strconv.Atoi(parts[2])
	if err != nil {
		return
	}
	patch, err = strconv.Atoi(parts[3])
	return
}

// Compare two version strings "x.y.z"
func isVersionGreater(a, b string) bool {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		ai, _ := strconv.Atoi(aParts[i])
		bi, _ := strconv.Atoi(bParts[i])
		if ai > bi {
			return true
		} else if ai < bi {
			return false
		}
	}
	return false
}

// Generate entry from bundle string
func generateEntry(bundle string) (*Entry, error) {
	major, minor, patch, err := parseBundleString(bundle)
	if err != nil {
		return nil, err
	}
	versionStr := fmt.Sprintf("%d.%d.%d", major, minor, patch)
	prevPatch := patch - 1
	prevMinor := minor
	prevMajor := major
	if prevPatch < 0 {
		prevMinor = minor - 1
		if prevMinor < 0 {
			prevMajor = major - 1
			prevMinor = 0 // can be improved for other major rollovers
		}
		prevPatch = 0 // first patch always 0
	}
	prevVersionStr := fmt.Sprintf("%d.%d.%d", prevMajor, prevMinor, prevPatch)

	entry := &Entry{
		Name:      fmt.Sprintf("rhacs-operator.v%s", versionStr),
		Replaces:  fmt.Sprintf("rhacs-operator.v%s", prevVersionStr),
		SkipRange: fmt.Sprintf("'>= %s <%s'", prevVersionStr, versionStr),
	}

	// Add skips for any Version > "4.1.0"
	if major == 4 && (minor > 1 || (minor == 1 && patch > 0)) {
		entry.Skips = append(entry.Skips, "rhacs-operator.v4.1.0")
	}

	return entry, nil
}

func main() {
	inputDir := "channels"
	outputFile := "catalog-template-new.yaml"

	files, err := os.ReadDir(inputDir)
	if err != nil {
		panic(err)
	}

	var channels []Channel

	for _, fi := range files {
		if fi.IsDir() || !strings.HasSuffix(fi.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(inputDir, fi.Name()))
		if err != nil {
			panic(err)
		}
		var cy ChannelYaml
		if err := yaml.Unmarshal(data, &cy); err != nil {
			panic(fmt.Errorf("in %s: %w", fi.Name(), err))
		}
		var images []string
		for _, img := range cy.Images {
			images = append(images, img.Image)
		}
		entries := []Entry{}
		for _, bundle := range cy.Entries {
			entry, err := generateEntry(bundle)
			if err != nil {
				panic(fmt.Errorf("in %s: %w", fi.Name(), err))
			}
			entries = append(entries, *entry)
		}
		channels = append(channels, Channel{
			Name:    cy.Name,
			Entries: entries,
			Images:  images,
		})
	}

	var stableEntries []Entry
	var latestEntries []Entry
	var allImages []string
	for _, ch := range channels {
		allImages = append(allImages, ch.Images...)
		if strings.HasPrefix(ch.Name, "rhacs-4.") {
			stableEntries = append(stableEntries, ch.Entries...)
		}
		if strings.HasPrefix(ch.Name, "rhacs-3.") {
			latestEntries = append(latestEntries, ch.Entries...)
		}
	}

	imageSet := map[string]struct{}{}
	var uniqueImages []string
	for _, img := range allImages {
		if _, exists := imageSet[img]; !exists {
			imageSet[img] = struct{}{}
			uniqueImages = append(uniqueImages, img)
		}
	}

	var out strings.Builder
	out.WriteString("---\nschema: olm.template.basic\nentries:\n")

	// Write channels
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	for _, ch := range channels {
		out.WriteString("\n- schema: olm.channel\n")
		out.WriteString(fmt.Sprintf("  name: %s\n  package: rhacs-operator\n  entries:\n", ch.Name))
		for _, entry := range ch.Entries {
			out.WriteString("  ")
			b, _ := yaml.Marshal(entry)
			out.Write(b)
		}
	}

	// Write stable channel
	out.WriteString("\n- schema: olm.channel\n  name: stable\n  package: rhacs-operator\n  entries:\n")
	for _, entry := range stableEntries {
		out.WriteString("  ")
		b, _ := yaml.Marshal(entry)
		out.Write(b)
	}

	// Write latest channel
	out.WriteString("\n- schema: olm.channel\n  name: latest\n  package: rhacs-operator\n  entries:\n")
	for _, entry := range latestEntries {
		out.WriteString("  ")
		b, _ := yaml.Marshal(entry)
		out.Write(b)
	}

	if len(channels) > 2 {
		out.WriteString("\n- schema: olm.deprecations\n  package: rhacs-operator\n  entries:\n")
		for i := 2; i < len(channels); i++ {
			ch := channels[i]
			out.WriteString(fmt.Sprintf("  - reference:\n      schema: olm.channel\n      name: %s\n    message: |\n      This version is no longer supported. Please switch to the 'stable' channel or a channel for a version that is still supported.\n", ch.Name))
		}
	}

	for _, img := range uniqueImages {
		out.WriteString(fmt.Sprintf("\n- image: %s\n  schema: olm.bundle\n", img))
	}

	if err := os.WriteFile(outputFile, []byte(out.String()), 0644); err != nil {
		panic(err)
	}
	fmt.Println("Generated", outputFile)
}
