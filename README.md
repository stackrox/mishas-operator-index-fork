# ACS Operator Index

This repository is for building the ACS (downstream) operator indexes on Konflux.


## Restarting Konflux job
Comment in the PR `/test <job_name>` (e.g. `/test operator-index-ocp-v4-16-on-push`)

## Adding Z-stream
Make similar changes as in #111 and run `make`

## Adding Y-stream
Make similar changes as in #112 and run `make`

## Initialization

Followed [this](https://gitlab.cee.redhat.com/konflux/docs/users/-/blob/main/topics/getting-started/building-olm-products.md)
and [this](https://github.com/konflux-ci/olm-operator-konflux-sample/blob/main/docs/konflux-onboarding.md) doc:
```
make import-legacy && make valid-catalogs
```

## Catalog formats

This directory contains two versions of the catalog, in subdirectories `catalog-bundle-object` and `catalog-csv-metadata`.
The former is expected by OpenShift versions up to and including 4.16, and the latter - by 4.17 and later.

See [konflux docs](https://github.com/konflux-ci/build-definitions/blob/c93ea73dbc30b8be15615e4d230040c70a0cf826/task/fbc-validation/0.1/TROUBLESHOOTING.md?plain=1#L7-L8).

## Scripts

See [./scripts](./scripts/README.md).
