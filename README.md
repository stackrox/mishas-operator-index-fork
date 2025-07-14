# ACS Operator Index

This repository is for building the ACS (downstream) operator indexes on Konflux.


## Restarting Konflux job

Comment in the PR `/test <job_name>` (e.g. `/test operator-index-ocp-v4-16-on-push`).
See more in [our docs](https://spaces.redhat.com/display/StackRox/How+to+everything+Konflux+for+RHACS#HowtoeverythingKonfluxforRHACS-Howtore-runpipelines).

## Updating catalogs

Followed [this](https://gitlab.cee.redhat.com/konflux/docs/users/-/blob/main/topics/getting-started/building-olm-products.md)
and [this](https://github.com/konflux-ci/olm-operator-konflux-sample/blob/main/docs/konflux-onboarding.md) doc:
```
make clean && make valid-catalogs
```

## Release operator catalog

1. Make sure you logged in to the Konflux cluster.
2. Make sure you checked out the latest master branch: `git checkout master && git pull`
3. Generate Release CR by running `./scripts/generate-releases.sh <staging|prod> > operator-index-release.yaml`. Use `staging` for test release and `prod` for production one.
4. Apply generated Release CR to the cluster: `oc create -f operator-index-release.yaml`
5. Monitor release [using monitor release script](#monitoring-release). Release can have `Succeeded` or `Failed` statuses

## Getting built images for specific commit

Run `./scripts/get-built-images.sh [COMMIT]` to fetch built operator catalog images for the provided `COMMIT` for each supported OCP version.
*Note:* The script uses current branch commit if no `COMMIT` argument provided.

## Monitoring Release

Run `./scripts/monitor-release.sh [COMMIT]` to see the current status for the releases associated with the provided `COMMIT`.
*Note:* The script uses current branch commit if no `COMMIT` argument provided.

## Catalog formats

This directory contains two versions of the catalog, in subdirectories `catalog-bundle-object` and `catalog-csv-metadata`.
The former is expected by OpenShift versions up to and including 4.16, and the latter - by 4.17 and later.

See [konflux docs](https://github.com/konflux-ci/build-definitions/blob/c93ea73dbc30b8be15615e4d230040c70a0cf826/task/fbc-validation/0.1/TROUBLESHOOTING.md?plain=1#L7-L8).
