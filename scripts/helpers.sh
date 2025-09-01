# Expand commit to a full 40-character SHA. Returns the full commit SHA if successful, or an error message if not.
expand_commit() {
    git fetch --all --quiet
    
    if ! git rev-parse --verify --end-of-options "$1^{commit}"; then
        echo "Cannot expand commit $1 to a full 40-character long SHA." >&2
        return 1
    fi
}

# Check if BRANCH is a valid known branch git branch for the remote repository, otherwise it's unlikely that any Snapshots will be found for it.
# Also check if the provided commit belongs to the branch.
validate_branch() {
    git fetch --all

    if ! git ls-remote --exit-code --heads origin "$1" > /dev/null; then
        echo "ERROR: $1 branch does not exist on remote. Please check the branch name." >&2
        return 1
    fi

    if ! git merge-base --is-ancestor "$2" "$1"; then
        echo "ERROR: commit $2 does not belong to $1 branch. Please check the branch name." >&2
        return 1
    fi
}

# Fetch the list of snapshots for provided commit and branch values. 
# Make sure that only one the most recent snapshot per application is returned.
get_snapshots() {
    local -r commit="$1"
    local -r branch="$2"
    kubectl get -n rh-acs-tenant snapshot -l pac.test.appstudio.openshift.io/sha="${commit}" -o json | jq '
        .items
        | map(select((.metadata.annotations["pac.test.appstudio.openshift.io/source-branch"]=="'"${branch}"'") or (.metadata.annotations["pac.test.appstudio.openshift.io/source-branch"]=="refs/heads/'${branch}'")))
        | sort_by(.spec.application)
        | group_by(.spec.application)
        | map(sort_by(.metadata.creationTimestamp) | last)'
}
