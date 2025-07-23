#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd)"
source "$SCRIPT_DIR/helpers.sh"

if [[ "$#" -gt 1 || "${1:-}" == "--help" ]]; then
    echo "USAGE: ./$(basename "${BASH_SOURCE[0]}") [COMMIT]"
    echo ""
    echo "COMMIT - an optional 40 character-long SHA of the commit to pull releases CRs associeted with this commit. Default: the latest commit in the current branch"
    echo ""
    echo "You must have your KUBECONFIG point to the Konflux cluster, see https://spaces.redhat.com/pages/viewpage.action?pageId=407312060#HowtoeverythingKonfluxforRHACS-GettingocCLItoworkwithKonflux."
    exit 1
fi

COMMIT="${1:-$(git rev-parse HEAD)}"
COMMIT="$(expand_commit "$COMMIT")"

# Set the interval for checking releases in seconds
interval=10
while true; do
    clear # Clear the terminal screen
    echo -e "Release status for \033[0;32m$COMMIT\033[0m commit (Press ctrl+C to quit):"
    date  # Show current timestamp
    kubectl -n rh-acs-tenant get releases.appstudio.redhat.com -l pac.test.appstudio.openshift.io/sha="${COMMIT}"
    echo -e "\nChecking again in \033[0;33m$interval\033[0m seconds..."
    sleep "$interval"
done
