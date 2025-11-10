#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"
ROOT_DIR="$(realpath "$SCRIPT_DIR/..")"
source "$SCRIPT_DIR/helpers.sh"

YQ_VERSION="v4.44.3"
YQ="${ROOT_DIR}/.bin/yq-${YQ_VERSION}"

# Make sure appropriate YQ is available. There are incompatibilities between versions so we use a version which is
# known to produce good results.
ensure_yq() {
    if [[ -x "${YQ}" ]]; then
        return 0
    fi
    local kernel_name
    local os_name
    local arch
    kernel_name="$(uname -s)"
    os_name="$(uname | tr '[:upper:]' '[:lower:]')"
    arch="$(go env GOARCH)"
    local -r url="https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/yq_${os_name}_${arch}"

    rm -f "${YQ}" # In case it's a stale partial download.
    mkdir -p "$(dirname "${YQ}")"
    echo "Fetching ${url}..." >&2
    curl --silent --fail --location --retry 3 --output "${YQ}" "${url}"
    if [[ "${kernel_name}" == "Darwin" ]]; then
        xattr -c "${YQ}"
    fi
    chmod +x "${YQ}"
    echo "Done." >&2
}

# Check command-line arguments and display usage help, if needed.
usage() {
    if [[ "$#" -lt 1 || "$#" -gt 3 || "${1:-}" == "--help" ]]; then
        echo "USAGE: ./$(basename "${BASH_SOURCE[0]}") <ENVIRONMENT> [COMMIT] [BRANCH]" >&2
        echo "" >&2
        echo "ENVIRONMENT - allowed values: stage|prod" >&2
        echo "COMMIT - a SHA of the commit to pull Snapshots only with this commit label for the Release." >&2
        echo "If provided commit SHA is less than 40 characters then it will be expanded to a full 40-characters long SHA. Default: currently checked out commit" >&2
        echo "BRANCH - an optional parameter to specify git branch name for filtering snapshots by having branch name in annotations. Default: currently checked out branch" >&2
        echo "" >&2
        echo "You must have your KUBECONFIG point to the Konflux cluster, see https://spaces.redhat.com/pages/viewpage.action?pageId=407312060#HowtoeverythingKonfluxforRHACS-GettingocCLItoworkwithKonflux." >&2
        return 1
    fi
}

# Validate environment input and ensure it is either 'stage' or 'prod' (for master branch only).
validate_environment() {
    local -r environment="$1"
    local -r branch="$2"
    if [[ "${environment}" != "stage" && "${environment}" != "prod" ]]; then
        echo "ERROR: ENVIRONMENT input must either be 'stage' or 'prod'." >&2
        return 1
    fi
    if [[ "${environment}" == "prod" && "${branch}" != "master" ]]; then
        echo "ERROR: prod release has to be done on master branch" >&2
        return 1
    fi
}

# Fetch the list of snapshot names and associented application name for provided commit and branch values.
get_snapshots_data() {
    local -r commit="$1"
    local -r branch="$2"
    snapshot_list="$(get_snapshots "$commit" "$branch")"
    echo "$snapshot_list" | jq -r '.[] | "\(.metadata.name)|\(.spec.application)"'
}

# Validate all expected Snapshots were found.
validate_snapshots() {
    local -r commit="$1"
    local -r snapshots_data="$2"

    local pipelines_count
    local snapshots_count
    pipelines_count="$(find ".tekton" -maxdepth 1 -type f -name "operator-index-ocp-*-build.yaml" | wc -l)"
    snapshots_count="$(echo "$snapshots_data" | sed '/^$/d' | wc -l)"

    echo -e "Found the following snapshots for \033[0;32m$commit\033[0m commit:" >&2
    echo "$snapshots_data" >&2

    if [[ "$snapshots_count" -eq 0 ]]; then
        echo "ERROR: Could not find any Snapshots for the commit '${commit}'." >&2
        return 1
    fi
    if [[ "$snapshots_count" -ne "$pipelines_count" ]]; then
        echo "ERROR: The number of snapshots $snapshots_count for $commit does not match the number of supported OCP versions $pipelines_count." >&2
        return 1
    fi
}

# Validate the length of the release name.
# To allow for Konflux' re-run suffix (12 chars), the full release name must be at most 51 characters long.
# Otherwise, the re-run Release CR will fail to be created as its name would exceed the Kubernetes limit of 63 characters.
validate_release_name_length() {
    local -r release_name_with_application="$1"

    if [[ ${#release_name_with_application} -gt 51 ]]; then
        echo "ERROR: Generated release name '${release_name_with_application}' is ${#release_name_with_application} characters long, which exceeds the 51 character limit." >&2
        echo "You must update the script to generate a shorter release name." >&2
        exit 1
    fi
}

# Generate the Release resources for each snapshot found.
generate_release_resources() {
    local -r environment="$1"
    local -r commit="$2"
    local -r snapshots_data="$3"

    local release_name_suffix
    local snapshot
    local application
    local release_plan

    release_name_suffix="$(date +"%Y%m%d")-${environment}-$(git rev-parse "$commit")"
    whitelist_file="$ROOT_DIR/release-history/.whitelist.yaml"
    out_file="$ROOT_DIR/release-history/${release_name_suffix}.yaml"

    echo "Writing resources to ${out_file} ..." >&2
    while IFS= read -r line
    do
        snapshot="$(echo "$line" | cut -d "|" -f 1)"
        snapshot_copy_name="${snapshot%-*}-${release_name_suffix}" # replace random suffix with release name
        echo "---"
        kubectl -n rh-acs-tenant get snapshot.appstudio.redhat.com "${snapshot}" -o yaml | \
        "${YQ}" -P 'load("'"${whitelist_file}"'") as $whitelisted
         | del(.metadata.annotations |keys[]|select(. as $needle | $whitelisted.annotations | has($needle) | not))
         | del(.metadata.labels |keys[]|select(. as $needle | $whitelisted.labels | has($needle) | not))
         | {"apiVersion": .apiVersion,
            "kind": .kind,
            "metadata": {
              "annotations": .metadata.annotations + {"acs.redhat.com/original-snapshot-name": "'"${snapshot}"'"},
              "labels": .metadata.labels,
              "name": "'"${snapshot_copy_name}"'",
              "namespace": .metadata.namespace
            },
            "spec": .spec
           }'

        application="$(echo "$line" | cut -d "|" -f 2)"

        # Crop the release name to 51 characters to avoid the Kubernetes limit of 63 characters for re-runs.
        release_name_with_application="$(echo "${application}-${release_name_suffix}" | cut -c -51)"
        release_plan="${application/acs-operator-index/acs-operator-index-${environment}}"

        sed -E 's/^[[:blank:]]{8}//' <<<"
        ---
        apiVersion: appstudio.redhat.com/v1alpha1
        kind: Release
        metadata:
          name: ${release_name_with_application}
          namespace: rh-acs-tenant
        spec:
          releasePlan: ${release_plan}
          snapshot: ${snapshot_copy_name}"

    done <<< "$snapshots_data" > "${out_file}"

    echo "Staging the file for commit..."
    git add --verbose "${out_file}"
}

usage "$@"

environment="$1"

commit="${2:-$(git rev-parse HEAD)}"
commit="$(expand_commit "$commit")"

branch="${3:-$(git rev-parse --abbrev-ref HEAD)}"
validate_branch "$branch" "$commit"

validate_environment "$environment" "$branch"

snapshots_data=$(get_snapshots_data "$commit" "$branch")
validate_snapshots "$commit" "$snapshots_data"

ensure_yq
generate_release_resources "$environment" "$commit" "$snapshots_data"
