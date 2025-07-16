#!/bin/bash

set -euo pipefail

if [[ "$#" -gt 1 ]]; then
    echo "USAGE: ./monitor-release.sh [COMMIT]"
    echo ""
    echo "COMMIT - an optional 40 character-long SHA of the commit to pull releases CRs associeted with this commit. Default: the latest commit in the current branch"
    echo ""
    echo "You must have your KUBECONFIG point to the Konflux cluster, see https://spaces.redhat.com/pages/viewpage.action?pageId=407312060#HowtoeverythingKonfluxforRHACS-GettingocCLItoworkwithKonflux."
    exit 1
fi

COMMIT="${1:-$(git rev-parse HEAD)}"
    
echo -e "Release status for \033[0;32m$COMMIT\033[0m commit (Press ctrl+C to quit):"
kubectl get releases --watch -l pac.test.appstudio.openshift.io/sha="${COMMIT}"
