Add a new Konflux release and to the release-history folder and test it on stage environment for version: $ARGUMENTS.

Read version from $ARGUMENTS and do the following:

1. Check that version in a foramt X.Y.Z is passed via $ARGUMENTS.
2. Run `oc project` command and validate that the output has `rh-acs-tenant` and `stone-prd-rh01.pg1f.p1.openshiftapps.com`. Otherwise stop and tell user to login to Konflux cluster using `oc login --web https://api.stone-prd-rh01.pg1f.p1.openshiftapps.com:6443/` and then try again.
3. Run `git checkout master && git pull`
4. Run `./scripts/generate-releases.sh prod` command. It will generate a new production release for the version.
5. Create a branch with pattern "add-<version>-release".
6. Add newly generated production release and open a draft PR with title "Add <version> release". Remember the PR number.
7. Switch back to master branch by `git checkout master`.
8. Run `./scripts/generate-releases.sh staging` and remember the current commit. It will be used later.
9. Deploy the newly generated stage release to the cluster `oc create -f release-history/<name-of-the-new-stage-release-file>`
10. Run the `kubectl -n rh-acs-tenant get releases.appstudio.redhat.com -l pac.test.appstudio.openshift.io/sha="<stage_release_commit>"` command on background every 30 seconds until all releases have "Succeeded" in "RELEASE STATUS". If any release has "Failed" status stop the background task and tell user to follow the `Restarting Konflux Release` steps from README.md.
11. If All releases "Succeeded" from the previous step then add a comment to the "Add <version> release" PR from the previous step that staging release succeeded.

Also remember to use the GitHub CLI (`gh`) for all GitHub-related tasks.