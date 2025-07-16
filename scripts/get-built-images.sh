#!/bin/bash

set -euo pipefail

if [[ "$#" -gt 1 ]]; then
    echo "USAGE: ./get-built-images.sh [COMMIT]"
    echo ""
    echo "COMMIT - an optional 40 character-long SHA of the commit to pull built images only with this commit sha. Default: the latest commit in the current branch"
    echo ""
    echo "You must have your KUBECONFIG point to the Konflux cluster, see https://spaces.redhat.com/pages/viewpage.action?pageId=407312060#HowtoeverythingKonfluxforRHACS-GettingocCLItoworkwithKonflux."
    exit 1
fi

COMMIT="${1:-$(git rev-parse HEAD)}"

echo -e "Operator catalog images for commit \033[0;32m$COMMIT\033[0m:"
kubectl get pipelinerun -l pipelinesascode.tekton.dev/sha="${COMMIT}" -o json | jq -r '
    if .items | length == 0 then
        "No PipelineRun CRs found for the current commit. It might be pruned already from the cluster. Use Konflux UI instead: https://konflux-ui.apps.stone-prd-rh01.pg1f.p1.openshiftapps.com/ns/rh-acs-tenant/applications"
    else
        .items[] | select(.status.conditions[]? | select(.type == "Succeeded" and .status == "True")) .status.results[]? | select(.name == "IMAGE_URL") | .value
    end'
