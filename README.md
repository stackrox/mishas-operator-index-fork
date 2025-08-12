# ACS Operator Index

This repository is for building the ACS (downstream) operator indexes on Konflux.


## Development

### Restarting Konflux job

Comment in the PR `/test <job_name>` (e.g. `/test operator-index-ocp-v4-16-on-push`).
See more in [our docs](https://spaces.redhat.com/display/StackRox/How+to+everything+Konflux+for+RHACS#HowtoeverythingKonfluxforRHACS-Howtore-runpipelines).

### Adding new ACS operator version

1. Open `bundles.yaml` file.
2. Add a new operator bundle image with version:
   ```yaml
      - image: brew.registry.redhat.io/rh-osbs/rhacs-operator-bundle@sha256:c82e8330c257e56eb43cb5fa7b0c842a7f7f0414e32e26a792ef36817cb5ca02
        version: X.Y.Z
   ```
   * Note that the image must be referenced by digest, not by tag.
   * Keep entries sorted by version.
   * You may add bundle images from `quay.io`, `brew.registry.redhat.io` and so on (provided they exist and are pullable) during development and/or when preparing to release.  
      Ultimately, all released bundle images must come from
      `registry.redhat.io/advanced-cluster-security/rhacs-operator-bundle` repo because this is where customers expect
      to find them. There's a CI check which should make it impossible to push to `master` if there's any bundle from
      a different repo.
   * If you're adding a bundle as part of downstream release, you will find bundle image's digest in
      `[CVP] (SUCCESS) cvp-redhatadvancedclustersecurity: rhacs-operator-bundle-container-4.Y.Z-x` email. Open the
      link in `Brew Build Info` and find the digest of the
      `registry-proxy.engineering.redhat.com/rh-osbs/rhacs-operator-bundle` image. Take that image and replace
      `registry-proxy.engineering.redhat.com` with `brew.registry.redhat.io`.
3. Update `oldest_supported_version` value if necessary. Older supported version should be equal to the lowest version from the [Red Hat Advanced Cluster Security for Kubernetes Support Policy](https://access.redhat.com/support/policy/updates/rhacs) (scroll to `Maintenance Support` version).
4. Run `make catalog-template.yaml`. This step should update `catalog-template.yaml` with the new version.
5. Update catalogs (follow [updating catalogs steps](#updating-catalogs))
6. open a PR with `Add 4.Y.Z version` title



### Updating catalogs

Run
```
make clean && make valid-catalogs
```
Note: this will take a while.

If a new bundle was added then you should see that `catalog-bundle-object/rhacs-operator/catalog.json` and `catalog-csv-metadata/rhacs-operator/catalog.json` files are changed.

#### Historical note

The following documentation was used for setting up catalogs update ([this](https://gitlab.cee.redhat.com/konflux/docs/users/-/blob/main/topics/getting-started/building-olm-products.md) and [this](https://github.com/konflux-ci/olm-operator-konflux-sample/blob/main/docs/konflux-onboarding.md)).

### Getting built images for specific commit

Run `./scripts/get-built-images.sh [COMMIT]` to fetch built operator catalog images for the provided `COMMIT` for each supported OCP version.
*Note:* The script uses current branch commit if no `COMMIT` argument provided.

### Catalog formats

This directory contains two versions of the catalog, in subdirectories `catalog-bundle-object` and `catalog-csv-metadata`.
The former is expected by OpenShift versions up to and including 4.16, and the latter - by 4.17 and later.

See [konflux docs](https://github.com/konflux-ci/build-definitions/blob/c93ea73dbc30b8be15615e4d230040c70a0cf826/task/fbc-validation/0.1/TROUBLESHOOTING.md?plain=1#L7-L8).

## Release File-based operator catalog

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
