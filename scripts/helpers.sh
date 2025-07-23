# Expand commit to a full 40-character SHA. Returns the full commit SHA if successful, or an error message if not.
expand_commit() {
    if ! git rev-parse --verify --end-of-options "$1^{commit}"; then
        echo "Cannot expand commit $1 to a full 40-character long SHA." >&2
        return 1
    fi
}

# Check if BRANCH is a valid known branch git branch for the remote repository, otherwise it's unlikely that any Snapshots will be found for it.
validate_branch() {
    if ! git ls-remote --exit-code --heads origin "$1" > /dev/null; then
        echo "ERROR: $1 branch does not exist on remote. Please check the branch name." >&2
        return 1
    fi
}
