# Agent templates

Status: built (A1-06). Generic starting points for a built-in agent
([26-builtin-agents.md](../../docs/spec/26-builtin-agents.md) 7). The server embeds a copy of this
folder and lists the templates to owners with `listAgentTemplates`; the apps turn a template into
a definition draft (`origin.kind` `template`) that the owner reviews before anything is stored.

| Slug | Topic | Tool class wish | MCP servers |
|---|---|---|---|
| `writing-helper` | drafts, shortens and polishes texts | `chat` | none |
| `code-reviewer` | reviews the code in its agent folder | `scoped` | none |
| `research-summarizer` | searches and summarizes sources | `scoped` | one `http` server with a `secret_ref` |
| `meeting-notes` | turns rough notes into a summary with decisions and action items | `chat` | none |

## Layout

One folder per template, named by its slug (`^[a-z0-9][a-z0-9-]{0,63}$`), with exactly these
files and nothing else:

```text
templates/agents/<slug>/
  template.yaml            schema agent_template.v1
  CLAUDE.md                the instructions of the agent
  skills/<name>/SKILL.md   one per name in skills[]; frontmatter with name and description only
```

`template.yaml` holds `name`, `description` (one line each), `version`, `tool_class_wish`
(`chat`, `scoped` or `host`) and `skills` (the skill folder names, in order), and optionally
`model_hint` (becomes `settings.model`), `required_secrets` (the names of the secrets the MCP
servers use) and `mcp_servers` (the shape of `agent_definition.v1`). A `secret_ref` in a template
is always `env:<NAME>` with `NAME` listed in `required_secrets`; the host operator provides the
value to the hosting runner, never the template.

The template becomes a definition draft one to one: `CLAUDE.md` is `instructions`, each skill is
`name`, `description` from its frontmatter and the text after the frontmatter as `body`, and
`origin` is `template` with the slug and the version. The draft must pass every rule of 26 4.3
for its tool class wish, so a `stdio` MCP server needs `host`.

## Rules

Templates are written for this repository from scratch and stay generic: they never copy,
paraphrase or describe an agent, profile, company, host, person, ticket or internal script of a
real installation, and an existing agent is never exported into a template (26 7.5). Examples use
the domain `example.com` and the people Alex, Sam and Robin.

`make templates-check` (`tools/templates/template_gate.py`, part of `make check` and of the
pull request CI) fails on:

- a file or folder outside the layout, among them `.claude/`, `.mcp.json`, `.lsp.json`,
  `CLAUDE.local.md`, a plugin manifest, `hooks/`, `commands/`, `agents/`, `output-styles/`,
  `monitors/`, `bin/`, an extra skill file and a symbolic link;
- a schema error of `template.yaml` or a `SKILL.md` frontmatter key other than `name` and
  `description`;
- any refusal of 26 4.3 on the draft (inline shell in both forms, `@` imports outside the agent
  folder, unknown members such as an MCP `headersHelper`, a plain secret value, a `stdio` server
  below `host`, limits);
- a host, URL or e-mail address outside `example.com`, `example.org` and `example.net`, an
  absolute or `~` path, an IP address, a person's name outside Alex, Sam and Robin in a person
  position, a value in a token format of `.gitleaks.toml`;
- a finding of the generic deny rules of `tools/denylist/`.

The gate needs no private deny list; when one is reachable (as for `denylist_scan.py
--allow-missing`) it runs too, and its findings never print the matched term. The private list
also runs over this folder in `make denylist`. The name rule is a heuristic: a name outside a
person position stays unseen, so review and the private list hold the rest
([decisions.d/A1-06.md](../../docs/decisions.d/A1-06.md)).

## Private templates

An installation adds its own templates in a folder outside the repository and names it in
`agents.templates_dir` (default empty: shipped templates only). The folder has the same layout:
one folder per template; a file at its root, such as a README, is ignored. The server reads it
once at start, so a change needs a restart. A private template whose slug a shipped one already
has is skipped and logged with `template_slug_taken`; one that breaks the layout, the schema, the
frontmatter or a rule of 26 4.3 is skipped and logged with `template_invalid`. The content rules
of the gate do not run there; the operator keeps private templates private, and they never enter
this repository (D-A1-25).

## Changing a template

Edit the files here, then run `make templates-check` and `go generate ./internal/agents/adapters/templates/`
in `services/server` (the copy the server embeds; `make codegen-check` fails without it). Raise
`version` when the content changes.
