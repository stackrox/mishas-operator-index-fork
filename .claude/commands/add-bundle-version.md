Add a new version of ACS bundle to the `catalog-template.yaml` file: $ARGUMENTS.

Do the following changes in the `catalog-template.yaml` file using version and bundle image provided via $ARGUMENTS:

1. Add bundle image. Add a new `olm.bundle` entity using the provided operator image. Insert a comment with provided version. Keep `olm.bundle` entities sorted by version.
2. Find `stable` channel. Add a new item into its `entries` list but keep entries sorted by version. Update next version's `replaces` with the new version if next version exists.
3. Add entry to the next version's channel(s) if next version(s) exists.
4. Add channel with entries for the new version if it doesn't exist yet. It should contain all previous versions of the major version.
5. Run `make clean && make valid-catalogs` command.
6. Open a PR with with "Add <version> version" title.

There are `olm.channel` (channel) items in the file. Channels are named with pattern "rhacs-<version_major>.<version_minor>".
Also remember to use the GitHub CLI (`gh`) for all GitHub-related tasks.