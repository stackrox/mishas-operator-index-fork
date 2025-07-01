#!/usr/bin/env bash

set -euo pipefail

INPUT_DIR="channels"
OUTPUT_FILE="catalog-template-new.yaml"
TMP_DIR="$(mktemp -d)"

# Helper: Extract version number from channel name, e.g. rhacs-4.7 → 4.7
extract_version() {
    echo "$1" | sed -E 's/.*-([0-9]+\.[0-9]+)$/\1/'
}

# Gather all channel files and sort by version (descending)
CHANNEL_FILES=($(ls "$INPUT_DIR"/*.yaml | sort -t. -k2,2nr -k3,3nr))
CHANNELS=()
for file in "${CHANNEL_FILES[@]}"; do
    channel=$(awk '/^name:/ {print $2}' "$file")
    CHANNELS+=("$channel")
done

# Highest version
HIGHEST_VERSION=$(extract_version "${CHANNELS[0]}")

# Compose the header
cat > "$OUTPUT_FILE" <<EOF
---
schema: olm.template.basic
entries:
EOF

# Compose all channels
ALL_IMAGES=()
CHANNEL_ENTRIES=()
for file in "${CHANNEL_FILES[@]}"; do
    channel=$(awk '/^name:/ {print $2}' "$file")
    # Extract entries
    echo "" >> "$OUTPUT_FILE"
    echo "- schema: olm.channel" >> "$OUTPUT_FILE"
    echo "  name: $channel" >> "$OUTPUT_FILE"
    echo "  package: rhacs-operator" >> "$OUTPUT_FILE"
    echo "  entries:" >> "$OUTPUT_FILE"
    awk '
        /^entries:/ { in_entries=1; next }
        /^images:/ { in_entries=0 }
        in_entries && /^[[:space:]]*-/ { print "  " $0 }
        in_entries && /^[[:space:]]*[a-z]/ { print "    " $0 }
    ' "$file" >> "$OUTPUT_FILE"
    # Gather images
    awk '
        /^images:/ { in_images=1; next }
        in_images && /image:/ { print $2 }
        in_images && !/^-/ { in_images=0 }
    ' "$file" >> "$TMP_DIR/images.txt"
    mapfile -t new_images < "$TMP_DIR/images.txt"
    ALL_IMAGES+=("${new_images[@]}")
    # Save entries for stable/latest composition
    awk '
        /^entries:/ { in_entries=1; next }
        /^images:/ { in_entries=0 }
        in_entries { print }
    ' "$file" > "$TMP_DIR/${channel}.entries"
done

# Compose stable (all 4.* channels)
echo "" >> "$OUTPUT_FILE"
echo "- schema: olm.channel" >> "$OUTPUT_FILE"
echo "  name: stable" >> "$OUTPUT_FILE"
echo "  package: rhacs-operator" >> "$OUTPUT_FILE"
echo "  entries:" >> "$OUTPUT_FILE"
for file in "${CHANNEL_FILES[@]}"; do
    channel=$(awk '/^name:/ {print $2}' "$file")
    if [[ $channel == rhacs-4.* ]]; then
        sed 's/^/  /' "$TMP_DIR/${channel}.entries" >> "$OUTPUT_FILE"
    fi
done

# Compose latest (all 3.* channels)
echo "" >> "$OUTPUT_FILE"
echo "- schema: olm.channel" >> "$OUTPUT_FILE"
echo "  name: latest" >> "$OUTPUT_FILE"
echo "  package: rhacs-operator" >> "$OUTPUT_FILE"
echo "  entries:" >> "$OUTPUT_FILE"
for file in "${CHANNEL_FILES[@]}"; do
    channel=$(awk '/^name:/ {print $2}' "$file")
    if [[ $channel == rhacs-3.* ]]; then
        sed 's/^/  /' "$TMP_DIR/${channel}.entries" >> "$OUTPUT_FILE"
    fi
done

# Compose deprecations (2 versions below highest)
echo "" >> "$OUTPUT_FILE"
echo "- schema: olm.deprecations" >> "$OUTPUT_FILE"
echo "  package: rhacs-operator" >> "$OUTPUT_FILE"
echo "  entries:" >> "$OUTPUT_FILE"

# Deprecate all but the two highest versions
for channel in "${CHANNELS[@]}"; do
    version=$(extract_version "$channel")
    # Only deprecate if version is more than 2 below highest
    if [[ "$version" == "$HIGHEST_VERSION" ]]; then continue; fi
    HIGH_MAJOR=$(echo "$HIGHEST_VERSION" | cut -d. -f1)
    HIGH_MINOR=$(echo "$HIGHEST_VERSION" | cut -d. -f2)
    THIS_MAJOR=$(echo "$version" | cut -d. -f1)
    THIS_MINOR=$(echo "$version" | cut -d. -f2)
    if (( THIS_MAJOR < HIGH_MAJOR )) || (( THIS_MAJOR == HIGH_MAJOR && THIS_MINOR <= HIGH_MINOR - 2 )); then
      echo "  - reference:" >> "$OUTPUT_FILE"
      echo "      schema: olm.channel" >> "$OUTPUT_FILE"
      echo "      name: $channel" >> "$OUTPUT_FILE"
      echo "    message: |" >> "$OUTPUT_FILE"
      echo "      This version is no longer supported. Please switch to the 'stable' channel or a channel for a version that is still supported." >> "$OUTPUT_FILE"
    fi
done

# Compose bundle image blocks (unique only)
echo "" >> "$OUTPUT_FILE"
for img in $(printf "%s\n" "${ALL_IMAGES[@]}" | sort -u); do
    echo "- image: $img" >> "$OUTPUT_FILE"
    echo "  schema: olm.bundle" >> "$OUTPUT_FILE"
done

rm -rf "$TMP_DIR"

echo "Generated $OUTPUT_FILE with channels, stable/latest, deprecations and bundle images."