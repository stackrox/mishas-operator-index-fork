# Project Overview

Operator-index repository contains Advanced Cluster Security (ACS) operator File-based catalog (FBC). FBC is a Operator Lifecycle Manager (OLM) format for operator bundles.

## Key Commands
- `make clean && make valid-catalogs` - Generate catalog-csv-metadata/rhacs-operator/catalog.json and catalog-bundle-object/rhacs-operator/catalog.json files
- `./scripts/get-built-images.sh <short_commit> <branch>` - Get list of build images on Konflux for each OCP version 
- `./scripts/generate-releases.sh staging|prod` - Generate release file in the release-history folder.
- `./scripts/monitor-release.sh <short_commit>` - Monitors release status on Konflux for the provided commit.

## Important Notes
- Never read or edit catalog-csv-metadata/rhacs-operator/catalog.json and catalog-bundle-object/rhacs-operator/catalog.json files.
