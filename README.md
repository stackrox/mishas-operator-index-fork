# ACS Operator Index

This repository is for building and releasing the ACS operator indexes on Konflux.  
It's for updating Operator Catalogs, i.e., so OpenShift clusters can see new versions of ACS operator in their
OperatorHub.

## Development

### Restarting Konflux pipeline

If some pipeline failed, you can restart it by commenting in the PR `/test <pipeline-name>` (e.g. `/test operator-index-ocp-v4-16-on-push`).
See more in [our docs](https://spaces.redhat.com/display/StackRox/How+to+everything+Konflux+for+RHACS#HowtoeverythingKonfluxforRHACS-Howtore-runpipelines).

### <a name="add-bundle"></a> Adding new ACS operator version

Do the following changes in the `catalog-template.yaml` file.

1. Add bundle image.
   1. Find entries with `schema: olm.bundle` towards the end of the file.
   2. Add a new entry for your bundle image.  
      It should look something like this:
      ```yaml
      - schema: olm.bundle
        # 4.7.9
        image: brew.registry.redhat.io/rh-osbs/rhacs-operator-bundle@sha256:c82e8330c257e56eb43cb5fa7b0c842a7f7f0414e32e26a792ef36817cb5ca02
      ```
      * Note that the image must be referenced by digest, not by tag.
      * Keep entries sorted according to version.
      * Add a comment stating the version, see how it's done for other items there.
      * You may add bundle images from `quay.io`, `brew.registry.redhat.io` and so on (provided they exist and are
        pullable) during development and/or when preparing to release.  
        Ultimately, all released bundle images must come from
        `registry.redhat.io/advanced-cluster-security/rhacs-operator-bundle` repo because this is where customers expect
        to find them. There's a CI check which prevents pushing to `master` if there's any bundle from
        a different repo.
      * If you're adding a bundle as part of downstream release, you will find bundle image's digest in the email with
        a subject `[CVP] (SUCCESS) cvp-redhatadvancedclustersecurity: rhacs-operator-bundle-container-4.Y.Z-x`. Open the
        link in `Brew Build Info` and find the digest of the
        `registry-proxy.engineering.redhat.com/rh-osbs/rhacs-operator-bundle` image. Take that image and replace
        `registry-proxy.engineering.redhat.com` with `brew.registry.redhat.io`.
2. Add entry to the `stable` channel.
   1. Find the `stable` channel block. It starts with:
      ```yaml
      - schema: olm.channel
        name: stable
      ```
   2. Add a new item into its `entries` list.
      * Entries must be sorted by version (e.g., you must insert `4.8.2` after `4.8.1` but before `4.9.0`).
      * Ensure there are consistent blank lines between entries of different Y-streams.
      * Entry format is
        ```yaml
        - &bundle-4-Y-Z
          name: rhacs-operator.v4.Y.Z
          replaces: rhacs-operator.v4.PREVIOUS_Y.PREVIOUS_Z
          skipRange: '>= 4.(Y-1).0 < 4.Y.Z'
        ```
        Replace
        * `Y` with minor version (e.g., `8` in `4.8.2`),
        * `Z` with patch version (e.g., `2` in `4.8.2`),
        * `PREVIOUS_Y` and `PREVIOUS_Z` with minor and patch versions of the previous item (e.g., when you add `4.8.2`
          after `4.8.1`, that'd be `8` and `1`; when you add `4.9.0` after `4.8.3`, that'd be `8` and `3`),
        * `(Y-1)` with the value of `Y` minus 1 (e.g., when you add `4.8.2`, that'd be `7`).
   3. If the item you added is not the last one in the `entries` list, i.e., not the highest version, adjust the next
      item in the `entries` list.  
      Set its `replaces:` to be `rhacs-operator.v4.Y.Z`.  
      For example:
      ```yaml
      - &bundle-4-7-4  # <-------- this was already there
        name: rhacs-operator.v4.7.4
        replaces: rhacs-operator.v4.7.3
        skipRange: '>= 4.6.0 < 4.7.4'
      - &bundle-4-7-5  # <-------- this one I'm adding
        name: rhacs-operator.v4.7.5
        replaces: rhacs-operator.v4.7.4
        skipRange: '>= 4.6.0 < 4.7.5'

      - &bundle-4-8-0  # <-------- this was already there
        name: rhacs-operator.v4.8.0
        replaces: rhacs-operator.v4.7.4   # <-------- must change here to rhacs-operator.v4.7.5
        skipRange: '>= 4.7.0 < 4.8.0'
      ```
3. Add entry to `rhacs-4.?` channels.
   * For every `schema: olm.channel` with `name` like `rhacs-4.?` where `?` is >= `Y`,
   * add `- *bundle-4-Y-Z` to the `entries:` list (replacing `Y` and `Z` with minor and patch versions).
   * Maintain the entries sorted and with consistent linebreaks.
4. Add `rhacs-4.Y` channel. Skip this step if the channel already exists (i.e., when `Z` > `0`).
   * Keep the channels sorted.
   * In the `entries:`, reference all items from `4.0.0` up to (and including) `4.Y.Z`.

   It should look something like this (replacing `Y`, `Z` as appropriate):
   ```yaml
   - schema: olm.channel
     name: rhacs-4.Y
     package: rhacs-operator
     entries:
     - *bundle-4-0-0
     - *bundle-4-0-1
     # ... and so on ...

     - *bundle-4-Y-Z
   ```

Once done with `catalog-template.yaml`:

1. Update catalogs (follow [updating catalogs](#updating-catalogs) steps below).
2. Open a PR with `Add 4.Y.Z version` title.

### Updating catalogs

Run
```
make clean && make valid-catalogs
```
Note: this will take a while.

If a new bundle was added then you should see that `catalog-bundle-object/rhacs-operator/catalog.json` and `catalog-csv-metadata/rhacs-operator/catalog.json` files are changed.

#### Historical note

The following documentation was used for setting up catalogs update ([this](https://gitlab.cee.redhat.com/konflux/docs/users/-/blob/main/topics/getting-started/building-olm-products.md) and [this](https://github.com/konflux-ci/olm-operator-konflux-sample/blob/main/docs/konflux-onboarding.md)).

### <a name="get-built-image-index"></a> Getting built images for specific commit

Run `./scripts/get-built-images.sh [COMMIT]` to fetch built operator catalog images for the provided `COMMIT` for each supported OCP version.
*Note:* The script uses current branch commit if no `COMMIT` argument provided.

### Catalog formats

This directory contains two versions of the catalog, in subdirectories `catalog-bundle-object` and `catalog-csv-metadata`.
The former is expected by OpenShift versions up to and including 4.16, and the latter - by 4.17 and later.

See [konflux docs](https://github.com/konflux-ci/build-definitions/blob/c93ea73dbc30b8be15615e4d230040c70a0cf826/task/fbc-validation/0.1/TROUBLESHOOTING.md?plain=1#L7-L8).

## <a name="release"></a> Release File-based operator catalog

### Release process

1. Make sure you [logged in](https://spaces.redhat.com/pages/viewpage.action?pageId=407312060#HowtoeverythingKonfluxforRHACS-GettingocCLItoworkwithKonflux) to the Konflux cluster.
2. Make sure you checked out the latest master branch: `git checkout master && git pull`
3. Generate Release and Snapshot CRs by running `./scripts/generate-releases.sh <staging|prod>`. Use `staging` for test release and `prod` for production one.
4. (Skip for `staging` release.) Create a PR which adds the file created by the script, get the PR reviewed and merged.
5. (Skip for `staging` release.) Go to the [#acs-operator-index-release](https://redhat.enterprise.slack.com/archives/C096WU0GZUG) channel, and:
   1. make sure the previous operator index release is complete (has a green check mark emoticon)
   2. if not, coordinate with the person conducting that release
   3. once that release is complete, start a new thread for your release
6. Apply generated CRs to the cluster: `oc create -f release-history/<YYYY-MM-DD>-<SHA>-<staging|prod>.yaml`
7. Monitor release [using monitor release script](#monitoring-release). Each supported OCP version has its own `Release`. Successfully finished `Release` has `Succeeded` status.
8. Follow [the restarting release step below](#restarting-konflux-release) if any of the `Release`s fails for any OCP version.
9. (Skip for `staging` release.) Once done, go back to the Slack thread you started earlier, add a message that your release is done and add a green check mark emoticon on the initial message of the thread.
10. Once releases for all OCP versions successfully finish, then the operator catalog release is done. If you perform it as part of a bigger release procedure, you should go back to that procedure and continue with further steps.


### Monitoring Release

Run `./scripts/monitor-release.sh [COMMIT]` to see the current status for the releases associated with the provided `COMMIT`.
*Note:* The script uses current branch commit if no `COMMIT` argument provided.

### Restarting Konflux Release

If a particular Release fails (i.e. the CRs status changes to `Failed`), you should restart it until it succeeds. Failing to do so will leave corresponding OpenShift Operator catalog without updates.

1. Go to [the list of Konflux applications](https://konflux-ui.apps.stone-prd-rh01.pg1f.p1.openshiftapps.com/ns/rh-acs-tenant/applications).
<details>
<summary>Click to see Release rerun navigation gif</summary>

![rerun_release](https://github.com/user-attachments/assets/a24f3bbc-e81f-42e2-8db7-05e3cbcdff7f)
</details>

2. Open `acs-operator-index-ocp-v4-XX` Konflux application for OCP version you want to restart (`XX` means minor part of the OCP version).
3. Select `Releases` tab.
4. Find release by name you want to restart.
5. Click on the action menu (3 dots) on the right.
6. Press "Re-run release" option. This creates a new Release CR.
7. Monitor the new release.
8. Repeat restarting release if the release keeps failing. If you find yourself re-running a given `Release` five times or more, open a high severity request in [#konflux-users](https://redhat.enterprise.slack.com/archives/C04PZ7H0VA8) Slack channel describing the problem and providing names/links to Release CRs.
