# Fixtures of the Jira app tests

The webhook payloads of the tests follow the Jira webhook documentation (brief M2-50 Design
note 18, RK-83). No package publishes Jira's example payloads, so nothing is copied from a
package; the files in `jira/` are written by hand.

## Source

- Documentation page: https://developer.atlassian.com/cloud/jira/platform/webhooks/, the page
  cited in `docs/spec/11-extensions.md` section 10.2, retrieved there on 2026-10-10.
- The fixtures were written and reduced by hand, without network access from the build
  environment, from the example payloads of that page: they keep the members the app reads
  (Design note 6) and the members around them that give each object its shape, and drop the
  rest.
- The values are those of this repository's world, not of the page: site
  `https://jira.example.com`, projects `SUP` (`Support`) and `WEB`, Jira display names Sam and
  Robin, account IDs such as `example-account-sam` and `example-account-robin`, issue `SUP-12`
  with ID `10012`.
- Rich text (`issue.fields.description`, `comment.body`) is a plain string in the files; the
  tests that need an ADF document set it through the `text` override (see below).

## Members per file

Member paths in dot notation; `[]` marks the elements of an array.

### `jira/jira_issue_created.json`

`webhookEvent` `jira:issue_created`, issue `SUP-12` created by Sam.

- `timestamp`, `webhookEvent`, `issue_event_type_name`
- `user.self`, `user.accountId`, `user.displayName`, `user.active`, `user.timeZone`,
  `user.accountType`
- `issue.id`, `issue.self`, `issue.key`
- `issue.fields.summary`, `issue.fields.description`, `issue.fields.created`,
  `issue.fields.updated`
- `issue.fields.issuetype.self`, `issue.fields.issuetype.id`, `issue.fields.issuetype.name`
- `issue.fields.project.self`, `issue.fields.project.id`, `issue.fields.project.key`,
  `issue.fields.project.name`
- `issue.fields.status.self`, `issue.fields.status.id`, `issue.fields.status.name`

### `jira/jira_issue_updated.json`

`webhookEvent` `jira:issue_updated`, issue `SUP-12` changed by Sam, with changelog items for
`status`, `summary` and `description`.

- `timestamp`, `webhookEvent`, `issue_event_type_name`
- `user.self`, `user.accountId`, `user.displayName`, `user.active`, `user.timeZone`,
  `user.accountType`
- `issue.id`, `issue.self`, `issue.key`
- `issue.fields.summary`, `issue.fields.description`, `issue.fields.created`,
  `issue.fields.updated`
- `issue.fields.issuetype.self`, `issue.fields.issuetype.id`, `issue.fields.issuetype.name`
- `issue.fields.project.self`, `issue.fields.project.id`, `issue.fields.project.key`,
  `issue.fields.project.name`
- `issue.fields.status.self`, `issue.fields.status.id`, `issue.fields.status.name`
- `changelog.id`
- `changelog.items[].field`, `changelog.items[].fieldtype`, `changelog.items[].fieldId`,
  `changelog.items[].from`, `changelog.items[].fromString`, `changelog.items[].to`,
  `changelog.items[].toString`

### `jira/comment_created.json`

`webhookEvent` `comment_created`, a comment by Robin on issue `SUP-12`.

- `timestamp`, `webhookEvent`
- `comment.self`, `comment.id`, `comment.body`, `comment.created`, `comment.updated`,
  `comment.jsdPublic`
- `comment.author.self`, `comment.author.accountId`, `comment.author.displayName`,
  `comment.author.active`, `comment.author.timeZone`, `comment.author.accountType`
- `comment.updateAuthor.self`, `comment.updateAuthor.accountId`,
  `comment.updateAuthor.displayName`, `comment.updateAuthor.active`,
  `comment.updateAuthor.timeZone`, `comment.updateAuthor.accountType`
- `issue.id`, `issue.self`, `issue.key`
- `issue.fields.summary`
- `issue.fields.issuetype.self`, `issue.fields.issuetype.id`, `issue.fields.issuetype.name`
- `issue.fields.project.self`, `issue.fields.project.id`, `issue.fields.project.key`,
  `issue.fields.project.name`

## How the tests use them

`tests/fakes/jira_api.py` loads a file by its event name (`load_fixture`) and `render_fixture`
returns a copy with overrides: `issue_id`, `key` (which also sets the project key unless
`project` is given), `project`, `summary`, `text` (the description, or the comment body of
`comment_created`, as a string or an ADF document), `comment_id`, `actor` (the display name of
`user` or of `comment.author`), `timestamp` and `changelog` (a list of items, or `None` to drop
the member). `FakeJira.deliver` renders a fixture this way, signs the body with HMAC-SHA256 in
hex like Jira and posts it to the app with `X-Hub-Signature`. The files themselves are never
changed by a test.

## What is not proven

The fixtures were not compared with the live documentation page: the build environment has no
network, and the files were written without network access. That the member names, the
nesting and the value types match what Jira sends today is therefore not proven by these tests
(see the brief's report, "What is not proven"). When Atlassian changes a payload format or the
signature method (RK-83), update these files and the retrieval date in
`docs/spec/11-extensions.md` section 10.2 together.
