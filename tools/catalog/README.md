# tools/catalog

The action catalogue [`docs/spec/actions.yaml`](../../docs/spec/actions.yaml) is the single
source of actions, rate-limit classes, event types with their receive action, and WebSocket ops
([ADR 0049](../../docs/adr/0049-machine-readable-action-catalogue-as-the-single-source.md)).
`action_catalog.py` loads and checks it and generates from it the tables of sections 3, 4, 6
and 7 of [chapter 05](../../docs/spec/05-actions.md) and the lists `authz_actions`,
`rate_limit_classes` and `receive_actions` in
[`docs/api/redocly.yaml`](../../docs/api/redocly.yaml).

## Commands

| Command | Effect |
|---|---|
| `make catalog` | rewrites the generated blocks (`action_catalog.py generate`); writes nothing while `make catalog-check` reports any finding other than stale block content, and otherwise writes chapter 05 and `redocly.yaml` together or neither of them; if the write itself fails, the output ends with `cannot write the generated blocks` instead of `catalogue or blocks invalid` |
| `make catalog-check` | Gate: all rules below (`action_catalog.py check`); on a finding the script ends with exit 1 and writes one line per finding to stderr, `make` then ends with exit 2 |
| `uv run --project tools --locked pytest tools/catalog/tests` | Tests, including the end-to-end test with the pinned Redocly CLI (`make bootstrap`) |

Flow of a change: edit `actions.yaml`, run `make catalog`, generate the server again
(`cd services/server && go generate ./api/...`; the route table and the action catalogue in
`api/routes` come from `actions.yaml`, see [services/server/api](../../services/server/api/README.md))
and commit the diffs of 05, `redocly.yaml` and `services/server/api/routes` along with it. A
manual edit in a generated block fails the gate, a stale server state `make codegen-check`; for
that, CI selects the component `server` on a change to `actions.yaml` (`tools/ci/components.py`).

A new section of actions needs:

1. in `actions.yaml`, its key in `sections` (at the position of the table in 05) and
   its actions;
2. in 05, its level-3 heading in the section "Actions" with the empty block
   `actions.<key>` below it (only the two markers; `make catalog` fills it);
3. the same title in `ACTION_SECTION_TITLES` in `action_catalog.py`;
4. the key in `SECTIONS` in `tests/test_action_catalog.py`; there
   `test_repository_catalog_loads` and the cell-by-cell comparison pin the sections.

A new hand-written section of level 1 or 2 with a table needs its title in
`HAND_WRITTEN_SECTIONS` in `action_catalog.py`; otherwise every table row outside the
generated blocks is a finding.

## Schema

Top level, exactly these keys in this order: `version` (1),
`rate_limit_classes`, `sections`, `actions`, `events`, `ws_ops`, `channel_ops`. Unknown
keys are a finding everywhere. The fields of each entry and their meaning are in the
docstrings of the data classes `RateLimitClass`, `Action`, `Event`, `WsOp` and `ChannelOp` in
`action_catalog.py`; in short:

| List | Required fields | Other fields |
|---|---|---|
| `rate_limit_classes` | `id` (`rl.<name>`), `key`, `purpose` | |
| `actions` | `id`, `section`, `description`, `resource`, `right`, `owner` (`true`, `false`, `as_target`), `step_up` (`true`, `false`, `conditional`, `fresh`), `audit` (`always`, `denials`, `never`, `conditional`), `rights_change`, `rl_class` (class or `none`), `effect` (`read`, `write`) | `audit_text` (exactly with `conditional`), `delivery_check`, `app_scope` |
| `events` | `type`, `kind` (`durable`, `ephemeral`), `resource`, `resource_version` | `receive_action` (action or `self`), `audience` (`self`), `receivers` (`principal` as default, `agent`, `agent_or_app`), `receive_text`, `note` |
| `ws_ops` | `op`, `direction` | `action`, `also_actions`, `action_text`, `row_group`, `per_event_receive_action` |
| `channel_ops` | `channel`, `ops`, `direction` | `channel_text` (exactly in the first row of a channel), `action`, `action_text` |

Texts that go into a table cell of 05 are written verbatim in the file (Markdown, one
line, no spaces at the edges), and each stays exactly one cell on the site. The check
reads them the way the Python-Markdown extension `tables` splits a table row: `\\`
is an escaped backslash, `\|` an escaped `|`, a backslash before backticks escapes
one of them; every backtick run that the cell opens is closed inside the cell by a later run
of the same length, and every `|` stands between such a pair or after an odd number
of backslashes (Python-Markdown in the version from `tools/uv.lock`, extension
[tables](https://python-markdown.github.io/extensions/tables/), retrieved 2026-10-12; the
tests named below compare this reading with that renderer). Outside its code spans (read like the lines of 05 outside the
blocks) a cell holds no HTML (no `<` before a letter, `!`, `?` or `/`) and no
brace pair `{...}`: raw HTML can swallow the table or end the cell early,
and an attribute list takes text out of the cell or hides it. Otherwise a
finding line names the entry, field and column. One test compares the
rule with the renderer, another renders every generated table and compares it cell by
cell with the catalogue. A `*_text` field replaces the cell when it says more than the action;
the machine value is kept alongside it.

## Rules

Every finding has the form `<file>: <rule>: <detail>`.

| Rule | Checks |
|---|---|
| `schema` | required fields, types and allowed values, no unknown or duplicate keys, `version` equal to 1; cells as in the paragraph above; `right` names `system` only as exactly `system` |
| `unique` | class IDs, action IDs, event types and op names are unique |
| `section` | every action in a listed section, every section with an action, actions in the order of the sections |
| `rate-limit-class` | class known; `none` exactly for system actions and internal delivery checks; every class used |
| `rate-limit-config` | per class exactly one row `ratelimit.<class>.per_minute` / `.burst` in the configuration values of 05, no unknown class; the section holds exactly one table with the header row `Name \| Default \| Meaning` and a delimiter row with the same number of columns |
| `event` | receive action present, known, not a system action; `self` only with `audience: self`; `receivers` only with `audience: self`; `receive_text` names the action; the cell starts with `self` exactly when `audience: self`; durable types before ephemeral types |
| `op` | action present, known, not a system action (also `also_actions`); groups contiguous and equal in direction, action, `action_text`, `also_actions` and `per_event_receive_action`; `action_text` names the actions; `also_actions` and `per_event_receive_action` only with `action_text`; `per_event_receive_action` only for `dispatch` |
| `channel-op` | like `op`; rows of a channel contiguous; op name unique per channel and direction |
| `app-scope` | `app_scope` equal to the table "Scopes for apps" (05, 2.6); the table names only known actions, each at most once; the section holds exactly one table with the header row `Actions \| Scope` and a delimiter row with the same number of columns |
| `effect` | `effect` equal to the closed list "Effect" (05, 2.7); every list item of the section starts with `- ` in the first column, with no indented or nested item either, and holds no heading (items after a blank line belong to the same list, as on the site). For these three rules no other heading has a title with the same letters as the section read (digits do not count, because a number can be written in different ways, and at the same time `1` counts as `l` and `0` as `o`; `I` and `l` count as equal), and the section ends, as in Markdown, at every line that starts with at most as many `#` |
| `generated-block` | markers complete; block names in 05 and `redocly.yaml` exactly the expected ones, in 05 in catalogue order (in `redocly.yaml` the order of the rules is free, it does not change the lint); in 05 every table in its level-2 section (`BLOCK_SECTIONS`; a block kind without an entry is a finding), every action table under the heading of its catalogue section (`ACTION_SECTION_TITLES`), and between every generated table and the last level-2 or level-3 heading above it there is no heading of level 4 or deeper; every block equal to the generated output. Outside the blocks there is no heading without a visible title (at least one character from `[0-9A-Za-z]`), and every title consists only of these characters, spaces, backticks and `"(),-.:/_`; so no other script, no entity and no invisible character can make one heading look like another. The check reads headings with an approximation of the block rules of Python-Markdown: tabs expanded to four columns, a blank line consists only of spaces; ATX only with `#` in the first column, also with an empty title; Setext only with an underline in the first column below a line that starts a block (at the start, after a blank line, a heading, a horizontal rule, a marker, a link reference or indented code). 05 contains no character that the check and Markdown read differently: no line separator that only `str.splitlines` uses (U+000B, U+000C, U+001C to U+001E, U+0085, U+2028, U+2029) and no STX or ETX (U+0002, U+0003), which Markdown removes. Chapter 05 starts with its `# ` heading (no page metadata, no front matter). Outside the blocks, 05 holds no HTML (no `<` before a letter, `!`, `?` or `/` outside a code span; the check finds code spans and escapes per line like the docs site, that is like `find_code_spans` from pymdownx.inlinehilite in the version from `tools/uv.lock`), no heading without a space after `#`, no brace pair `{...}` outside a code span (so no attribute list), no admonition (`!!!`), no collapsible block (`???`) and no footnote at the start of a line (the render test catches indented forms), and in no section a line with `\|` or `<table`, not even in a code block, except in the hand-written sections "Terms of the tables", "Agent invocation `agent.invoke`", "Decisions of this chapter" and "Configuration values of this chapter"; each of these titles appears exactly once. `redocly.yaml` stays within the allow-list: at the top only `extends` (exactly `[recommended]`), `apis` and `rules`, per API only `root` (relative, unique after resolution against `docs/api`) and `rules`, no key with `$` (Redocly resolves `$ref`), no duplicate keys. The lint rules of the catalogue keep their generated form (only `subject` with type and property, `assertions` with exactly `enum` equal to the generated list, `message`), and no rule set under `apis` names them. `docs/api/.redocly.lint-ignore.yaml` names no catalogue rule |

`load_catalog` checks the rules that need only the YAML file, `check` adds the comparison
with chapter 05 and `redocly.yaml`. The findings come in stages, and a stage runs only if
the stages before it found nothing: reading and parsing the file (a one-line finding, also for a
value that Python cannot build, such as an invalid date, or for nesting that is too
deep), then duplicate keys and recursive aliases, then the schema, then the
relational rules; only a valid catalogue is compared with 05 and `redocly.yaml`, and their
rules then all report together. After a fix, findings of the next stage can therefore
appear. The check reads aliases once per node, including deeply nested ones.

## Interface for later tickets

M1-03, M1-06 and M1-25 import the module. Under pytest, `tools/catalog` is on the
`pythonpath` of `tools/pyproject.toml`; outside pytest the caller adds
`tools/catalog` to `sys.path` or `PYTHONPATH` itself, because the module is not an installed
package. The public names are:
`load_catalog`, `catalog_from_data`, `Catalog` with `actions_by_id`, `events_by_type`,
`ws_ops_by_name`, `classes_by_id`, `CatalogError`, `CatalogWriteError` (subclass for
a write error of `generate`), `generated_blocks`,
`replace_generated_block`, `yaml_generated_blocks`, `replace_yaml_block`,
`render_chapter_blocks`, `chapter_table_cells`, `redocly_lists`, `receive_cell`, `scope_assignments`,
`read_effect_ids`, `relational_findings`, `chapter_findings`, `check`, `generate`.

## What the check does not cover

- The static checks of the rule `generated-block` give fast feedback; the class is held by
  tests against the real artifacts in `tests/test_action_catalog.py`:
  - An end-to-end test sets `x-authz-action`, `x-rate-limit-class` or `x-receive-action`
    at every place in the contracts to an excluded value, runs the real `make api-lint` on a
    copy of `docs/api` and `Makefile` and requires exactly as many errors as places it
    changed.
  - A test pins the recipe of `make api-lint` verbatim, and a direct Redocly run
    without make requires that the unchanged contracts pass the real configuration.
  - The recipes of `make catalog` and `make catalog-check` are pinned verbatim, and a
    run of the real `make catalog-check` on a copy must fail on a stale
    block.
  - A test renders chapter 05 with the Markdown configuration from `mkdocs.yml` and requires:
    no page metadata and no front matter (asked of the mkdocs reader);
    the same sequence of headings as the static check; only elements and attributes of an
    allow-list (what Markdown generates for text, lists, links and tables, on links only the
    class `headerlink`; no `div`, `span`, `details`, `style`, no attribute `hidden`, `style`
    or `data-*`); no raw HTML except the markers, not even through `md_in_html`, asked of the
    renderer itself; outside the hand-written sections exactly the generated tables,
    each in the section and under the heading of its block with no heading of level 4 or
    deeper in between, even if a heading stands in
    a blockquote, a list, a definition list or a code block; a
    heading without a visible title, with a character outside the allow-list or in
    a list item or blockquote is a finding. The site shows table 2.6 with its header row,
    list 2.7 and the configuration table exactly as the check reads them, and no
    other heading, of any level, shows the same letters as their titles.
    No check sees look-alikes made of several Latin letters (such as `rn` for `m`) or digits
    other than `1` and `0`.
    A code block or an admonition in 05 would have to extend the allow-list in review.
  - Statically, only a sequence of lines that start with `|` counts as a table; only the
    render test sees tables in another form (without a leading `|`, in a blockquote).
    Statically, only a line with `#` ends a section being read, not a Setext heading. The
    static view of headings is an approximation that recognizes each block rule only by the
    form of a single line; among other things it reads no containers, not the interplay of
    several block rules in one block (such as a horizontal rule and a link reference, or a line
    consisting only of non-breaking spaces after indented code), no block rule across lines
    (a link reference with a remainder or with its target on the next line), no ATX special
    cases (a title that ends in `\`) and no markup in the title (entities and escapes are
    static findings, because `&`, `;` and `\` are not in the allow-list). A
    test compares it with the rendered sequence for the cases from the reviews; the render
    test holds the class.
  - A test compares the static view of code spans and escapes line by line with the
    renderer (all lines of up to `_MAX_BUILDING_BLOCKS` building blocks with `<i>`, a constant
    of the tests), and a test compares the
    escape characters with those of the renderer. Spans across several lines and code spans in
    table rows outside the blocks (there `tables` first pairs the backtick runs of the whole
    line, splits at the remaining `|` and then reads each cell on its own) are seen only by
    the render test.
  - A test renders every generated table and compares it cell by cell with the cells
    of the catalogue (`chapter_table_cells`), and a test compares the cell rule of the rule
    `schema` with the renderer for all cells of up to `_MAX_BUILDING_BLOCKS` building blocks.
- Not covered: Makefile content outside the pinned recipes that only takes effect together
  with other files (included Makefiles, conditions on other files, the
  Git state or the environment), weakenings that only take effect in CI, for rendering
  everything in `mkdocs.yml` except `markdown_extensions` (hooks, plugins, `nav`,
  `theme.custom_dir`, `extra_css`, `extra_javascript`), renderings other than the docs site
  (for example GitHub) and deliberate changes to `tools/catalog` or its tests. The review and
  the red-team gate check these ([red-team-gate.md](../../docs/quality/red-team-gate.md), item
  "Gates").
- No gate checks the value of `resource_version`: the rule `schema` requires the field on
  every event type as a boolean; whether it is right (column `version` of the resource in 03,
  the exceptions from 04 "Delivery guarantee" item 3, ruling M1-02/6) stays with review. Later
  the frame conformance test from M1-67 checks every sent event against it.
- M1-03 brings the direct comparison of the contracts with the catalogue (gate G2). It also
  catches an operation whose `x-authz-action` is an object; the Redocly enumeration skips it
  (reproduced on 2026-10-12: `x-authz-action: {id: instance.discovery.read}` in a copy of
  `docs/api/openapi.yaml` passes the lint with `docs/api/redocly.yaml` and the `@redocly/cli`
  version pinned in `tools/node/package.json`).
- Redocly does not apply decorators during lint: its documentation describes them as changes
  "during bundling" ([decorators](https://redocly.com/docs/cli/decorators), retrieved
  2026-10-12), and a run on 2026-10-11 with `remove-x-internal` and the `@redocly/cli` version
  pinned in `tools/node/package.json` confirmed it; the allow-list still does not admit them.
- The rule `schema` does not detect a cell `right` that writes `system` with look-alike
  characters of another alphabet; a real system action is still caught by
  `rate-limit-class` as long as its class stays `none` and `delivery_check` is missing. The
  rest stays with review.
- Comparison of `openapi.yaml`, `asyncapi.yaml` and `webhooks.md` with the catalogue (M1-03).
- Assignment of actions to modules; it is in the module register (ADR 0050, M1-06).
- Go constants and the closed catalogue in the server (M1-25).
- Whether the verbatim texts (description, right) are correct in content; that stays
  with review.
- `tests/table_import.py` reads only the frozen version of 05 before the rework
  (`tests/fixtures/05-actions-before.md.txt`) and serves only to prove that the tables
  stayed identical row for row.
