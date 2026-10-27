<!--
Template of the GitHub release pages (brief M7-28 item 15).

How it is used: the job `publish` of .github/workflows/release.yml renders this file with
tools/ci/release_notes.py and passes the result to `gh release create --notes-file`. The title
of the page is the tag (`<component>/v<version>`, `v<version>` for the product), set by
`gh release create --title`. Nobody fills this template by hand; the content comes from the
section `## [<version>] - <date>` of the component's CHANGELOG.md, which the release pull
request reviews.

Placeholders have the form {{name}}; tools/ci/release_notes.py rejects any other name:

- summary: the `Summary:` paragraph of the section without its prefix; it comes first.
  tools/ci/changelog_summary.py (in `make check`) fails while it is missing or still the
  placeholder that `make release-prep` writes.
- changes: text before the first `###` heading and the Keep a Changelog categories Added,
  Changed, Deprecated, Removed, Fixed, Security.
- upgrade_notes: the headings Upgrade notes, Migrations, Compatibility, Known issues
  (docs/spec/18-release.md section 4).
- technical: every other `###` heading of the section.
- tag, version, repository: the release tag, its version and `owner/name` of the repository.

A part without content reads "None listed in the changelog.". This comment is not rendered.
-->
{{summary}}

## What changed for users

{{changes}}

## Upgrade notes

{{upgrade_notes}}

## Technical entries

{{technical}}

## Verification

Every file of this release is listed in `SHA256SUMS`. Where the release chain of the component
signs its files, a Sigstore bundle `<file>.sigstore.json` stands next to each of them and next
to `SHA256SUMS`; a release with container images names them by digest in `images.txt`.
Download the assets and check them:

```
gh release download {{tag}} --repo {{repository}}
sha256sum --check --strict SHA256SUMS
```

The `cosign verify` and `cosign verify-blob` commands of the current signing phase are in
[Check like an instance operator](https://github.com/{{repository}}/blob/{{tag}}/docs/operations/release.md#5-check-like-an-instance-operator).
