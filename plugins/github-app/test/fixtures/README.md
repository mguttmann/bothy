# Fixtures of the GitHub app tests

The webhook payloads of the tests come from the GitHub webhook documentation (brief M2-49 Design
note 17, RK-83). Nothing of the source package is copied into the repository.

## `@octokit/webhooks-examples`

- Package: `@octokit/webhooks-examples` 7.6.1 (license MIT, pinned exactly in `package.json`), a
  dev dependency read by `test/helpers/fixtures.ts`.
- Origin, in the package's own words (its `README.md`): "The update script is scraping [GitHub's
  Webhooks Event Types & Payloads](https://docs.github.com/en/free-pro-team@latest/developers/webhooks-and-events/webhook-events-and-payloads)
  documentation page and extracts the meta information using cheerio."
- Documentation page: https://docs.github.com/en/webhooks/webhook-events-and-payloads (the
  current address of the page the package names). The package was retrieved from the npm registry
  on 2026-10-25; the page itself was not opened from the build environment, whose only network
  access is package downloads.
- The tests choose an example by event and action and change only the repository name, numbers
  and IDs (`retarget`); the `html_url` values follow those changes.
- The package's tarball ships no `LICENSE` file; its license is the `license` member of its
  `package.json` (`MIT`), which the license gate reads.

## Minimal files of `github/`

The package holds no example for three variants the app handles. These files carry only the
fields of Design note 6, with the values of the documented examples of the same event:

| File | Event and action | Documentation |
|---|---|---|
| `github/issues.closed.json` | `issues` `closed` | https://docs.github.com/en/webhooks/webhook-events-and-payloads#issues |
| `github/pull_request.closed.merged.json` | `pull_request` `closed` with `merged` true | https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request |
| `github/issue_comment.created.pull_request.json` | `issue_comment` `created` on a pull request (`issue.pull_request` set) | https://docs.github.com/en/webhooks/webhook-events-and-payloads#issue_comment |

The pages were not opened from the build environment (see above); the values are those of the
package's examples of the same event (2026-10-25). When GitHub changes a format or a signature
(RK-83), update the package version and these files together and name the new date here.
