#!/bin/bash

set -euo pipefail

if [[ "$#" -lt 1 || "$#" -gt 3 ]]; then
    echo "USAGE: ./generate-releases.sh <ENVIRONMENT> [COMMIT] [BRANCH]" >&2
    echo "" >&2
    echo "ENVIRONMENT - allowed values: staging|prod" >&2
    echo "COMMIT - a 40 character-long SHA of the commit to pull Snapshots only with this commit label for the Release. Default: currently checked out commit" >&2
    echo "BRANCH - an optional parameter to specify git branch name for filtering snapshots by having branch name in annotations. Default: currently checked out branch" >&2
    echo "" >&2
    echo "You must have your KUBECONFIG point to the Konflux cluster, see https://spaces.redhat.com/pages/viewpage.action?pageId=407312060#HowtoeverythingKonflux/RHTAPforRHACS-GettingocCLItoworkwithKonflux." >&2
    exit 1
fi

ENVIRONMENT="$1"
COMMIT="${2:-$(git rev-parse HEAD)}"
BRANCH="${3:-$(git rev-parse --abbrev-ref HEAD)}"

release_name="${ENVIRONMENT}-$(git rev-parse --short HEAD)-$(date +'%Y-%m-%d-%H-%M')"
# Fetch the list of snapshots for COMMIT and BRANCH. 
# Make sure that only one the most recent snapshot per application is returned.
snapshot_list="$(kubectl get snapshot -l pac.test.appstudio.openshift.io/sha="${COMMIT}" -o json | jq -r '
  .items
  | map(select((.metadata.annotations["pac.test.appstudio.openshift.io/source-branch"]=="'"${BRANCH}"'") or (.metadata.annotations["pac.test.appstudio.openshift.io/source-branch"]=="refs/heads/'${BRANCH}'")))
  | sort_by(.spec.application)
  | group_by(.spec.application)
  | map(sort_by(.metadata.creationTimestamp) | last)
  | .[]
  | "\(.metadata.name)|\(.spec.application)"
')"

validate_input() {
    pipelines_count="$(find ".tekton" -maxdepth 1 -type f -name "operator-index-ocp-*-build.yaml" | wc -l )"
    snapshots_count="$(echo "$snapshot_list" | wc -l )"
    echo -e "found the following snapshots for \033[0;32m$COMMIT\033[0m commit in \033[0;32m$BRANCH\033[0m branch:" >&2
    echo "$snapshot_list" >&2

    if [[ "$snapshots_count" -eq 0 ]]; then
        echo "ERROR: Could not find any Snapshots for the commit '${COMMIT}'." >&2
        exit 1
    fi
    if [[ "${ENVIRONMENT}" != "staging" && "${ENVIRONMENT}" != "prod" ]]; then
        echo "ERROR: ENVIRONMENT input must either be 'staging' or 'prod'." >&2
        exit 1
    fi
    if [[ "${ENVIRONMENT}" == "prod" && "${BRANCH}" != "master" ]]; then
        echo "ERROR: prod release has to be done on master branch" >&2
        exit 1
    fi
    if [[ "$snapshots_count" -ne "$pipelines_count" ]]; then
        echo "ERROR: The number of snapshots for $COMMIT in branch $BRANCH does not match the number of supported OCP versions $pipelines_count." >&2
        exit 1
    fi
}

generate_release_resources() {
    while IFS= read -r line
    do
        snapshot="$(echo "$line" | cut -d "|" -f 1)"
        application="$(echo "$line" | cut -d "|" -f 2)"
        release_plan="${application/acs-operator-index/acs-operator-index-${ENVIRONMENT}}"
        echo "---
apiVersion: appstudio.redhat.com/v1alpha1
kind: Release
metadata:
  name: ${application}-${release_name}
  namespace: rh-acs-tenant
spec:
  releasePlan: ${release_plan}
  snapshot: ${snapshot}"
    done <<< "$snapshot_list"
}

validate_input
generate_release_resources
