---
name: k6 Hotfix release
about: Bugs or security issues have been identified and they requires a patch release.
title: 'k6 hotfix release v<major>.<minor>.<patch>'
labels: ["release"]
---

**Release Date**:

25th May 2024 <<  TODO: WRITE HERE THE UPDATED RELEASE DATE

## Release Activities

Note, the assumption is a minor branch release already exists from the latest minor release. This branch is named in the format `release-v<major>.<minor>.x`.

#### Documentation

- [ ] If required, open and merge pull-request from `main` applying the changes to the affected version.

#### In k6 repository, if the defect affects the latest release and the `master` branch

- [ ] Open, get approvals and merge on `master` the patch for the defect.
- [ ] Open, get approvals and merge on `master` the release notes. Ensure that all merged pull-requests are referenced.
- [ ] Switch on the release branch and cherry-pick the following changes:
      1. the patch for the defect
      2. the release notes
- [ ] Open, get approvals and merge on the already existing release branch a separate pull-request for bumping [the k6 Go project's version](https://github.com/grafana/k6/blob/master/lib/consts/consts.go#L11-L12). Note, `master` should be already on a next major/minor version.
- [ ] Pull locally the previously merged changes.
- [ ] Create and push a new tag of the form `vX.Y.Z` using git. For instance, if the current release is v0.52, the command could look like `git tag v0.52.1 -m "v0.52.1"` if this is the first hotfix.
- [ ] If the defect affects DefinitelyTyped's definitions, open a pull request in the `DefinitelyTyped/DefinitelyTyped` repository by creating a branch on the `grafana/k6-DefinitelyTyped` fork, to update the k6 type definitions.

#### In k6 repository, if the defect affects the latest release but not the `master` branch

The steps are the same as the previous, with the unique exception that the patch pull request has to be directly merged against the release branch instead of being cherry-picked.

#### In k6 repository, if the defect affects the latest release and the `master` branch but they are not anymore the same compatible code.

The union of the previous steps have to be executed, where the changes have to be applied independently to both branches without cherry-picking. Each version get a dedicated patch.

#### Announcements

- [ ] Publish a link to the new GitHub release in the #k6-changelog channel.
- [ ] Notify the larger team in the #k6 channel, letting them know that the release is published.

## Wrapping Release

- [ ] Ensure the `DefinitelyTyped/DefinitelyTyped` PR(s) are merged.
- [ ] Update the k6 repository's `.github/ISSUE_TEMPLATE/hotfix-release.md` in the event steps from this checklist were incorrect or missing.
