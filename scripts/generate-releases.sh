#!/bin/bash

set -euo pipefail

if [[ "$#" -lt 1 || "$#" -gt 3 ]]; then
    echo "USAGE: ./generate-releases.sh <ENVIRONMENT> [COMMIT] [BRANCH]" >&2
    echo "" >&2
    echo "ENVIRONMENT - allowed values: staging|prod" >&2
    echo "COMMIT - a 40 character-long SHA of the commit to pull Snapshots only with this commit label for the Release. Default: currently checked out commit" >&2
    echo "BRANCH - an optional parameter to specify git branch name for filtering snapshots by having branch name in annotations. Default: currently checked out branch" >&2
    echo "" >&2
    echo "You must have your KUBECONFIG point to the Konflux cluster, see https://spaces.redhat.com/pages/viewpage.action?pageId=407312060#HowtoeverythingKonfluxforRHACS-GettingocCLItoworkwithKonflux." >&2
    exit 1
fi

ENVIRONMENT="$1"
COMMIT="${2:-$(git rev-parse HEAD)}"
BRANCH="${3:-$(git rev-parse --abbrev-ref HEAD)}"
ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." &> /dev/null && pwd)"

release_name="${ENVIRONMENT}-$(git rev-parse --short HEAD)-$(date +'%Y-%m-%d-%H-%M')"
# Fetch the list of snapshots for COMMIT and BRANCH. 
# Make sure that only one the most recent snapshot per application is returned.
snapshot_list="$(kubectl -n rh-acs-tenant get snapshot.appstudio.redhat.com -l pac.test.appstudio.openshift.io/sha="${COMMIT}" -o json | jq -r '
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
    case ${ENVIRONMENT} in
    staging)
        ENV_SHORT=stg
        ;;
    prod)
        ENV_SHORT=prd
        ;;
    *)
        echo "ERROR: ENVIRONMENT input must either be 'staging' or 'prod'." >&2
        exit 1
      ;;
    esac
    if [[ "${ENVIRONMENT}" == "prod" && "${BRANCH}" != "master" ]]; then
        echo "ERROR: prod release has to be done on master branch" >&2
        exit 1
    fi
    if [[ "$snapshots_count" -ne "$pipelines_count" ]]; then
        echo "ERROR: The number of snapshots ($snapshots_count) for $COMMIT in branch $BRANCH does not match the number of supported OCP versions ($pipelines_count)." >&2
        exit 1
    fi
}

generate_release_resources() {
    release_name="$(date +"%Y-%m-%d")-$(git rev-parse --short "$COMMIT")-${ENVIRONMENT}"
    release_name_short="$(date +"%Y%m%d")-$(git rev-parse --short "$COMMIT")-${ENV_SHORT}" # save space for retry suffix
    whitelist_file="$ROOT_DIR/release-history/.whitelist.yaml"
    out_file="$ROOT_DIR/release-history/${release_name}.yaml"
    echo "Writing resources to ${out_file} ..." >&2

    while IFS= read -r line
    do
        snapshot="$(echo "$line" | cut -d "|" -f 1)"
        snapshot_copy="${snapshot%-*}-${release_name}" # replace random suffix with release name
        echo "---"
        kubectl -n rh-acs-tenant get snapshot.appstudio.redhat.com "${snapshot}" -o yaml | \
        yq -P 'load("'"${whitelist_file}"'") as $whitelisted
         | del(.metadata.annotations |keys[]|select(. as $needle | $whitelisted.annotations | has($needle) | not))
         | del(.metadata.labels |keys[]|select(. as $needle | $whitelisted.labels | has($needle) | not))
         | {"apiVersion": .apiVersion,
            "kind": .kind,
            "metadata": {
              "annotations": .metadata.annotations + {"acs.redhat.com/original-snapshot-name": "'"${snapshot}"'"},
              "labels": .metadata.labels,
              "name": "'"${snapshot_copy}"'",
              "namespace": .metadata.namespace
            },
            "spec": .spec
           }'

        application="$(echo "$line" | cut -d "|" -f 2)"
        release_plan="${application/acs-operator-index/acs-operator-index-${ENVIRONMENT}}"
        sed -E 's/^[[:blank:]]{8}//' <<<"
        ---
        apiVersion: appstudio.redhat.com/v1alpha1
        kind: Release
        metadata:
          name: ${application}-${release_name_short}
          namespace: rh-acs-tenant
        spec:
          releasePlan: ${release_plan}
          snapshot: ${snapshot_copy}"

    done <<< "$snapshot_list" > "${out_file}"

    echo "Staging the file for commit..."
    git add --verbose "${out_file}"
}

validate_input
generate_release_resources
