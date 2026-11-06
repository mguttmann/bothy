# tests/agent-definitions

Status: built (A1-02). The shared corpus of definitions of built-in agents: the conformance set of
[26-builtin-agents.md](../../docs/spec/26-builtin-agents.md) 4.2 and 4.3 for every writer and
reader of a definition. The server's validator (`services/server/internal/agents/domain/definition`,
A1-02), the runner's own check before every turn (A1-05), the template gate (A1-06) and the
interview's draft check (A1-07) each run every file of this folder and must give exactly its
expected findings. A rule changes here first; a writer that disagrees with a fixture is wrong.

## Files

| Path | Content |
|---|---|
| `valid/<name>.json` | a definition that passes, with `"expect": []` |
| `invalid/<code>-<name>.json` | a definition with at least one finding; `<code>` is the main field code it holds |
| `limits.json` | the limits the corpus runs with, by configuration key: the defaults of 26 14 |

Each fixture is one JSON object with exactly these members:

```json
{"tool_class": "chat", "definition": {}, "expect": [{"pointer": "/skills/0/body", "code": "skill_frontmatter"}]}
```

- `tool_class` is the configured tool class of the agent the definition is checked for (`chat`,
  `scoped` or `host`); it decides `tool_class_too_low`.
- `definition` is the document as a client sends it (schema `agent_definition.v1`, OpenAPI
  `AgentDefinition`), also when it breaks the schema.
- `expect` lists every finding as JSON Pointer and field code; the comparison ignores the order,
  and one pointer has each code at most once.

The field codes are those of 26 4.3, the shape codes of the API's request validation and the
character codes of the text check; the exact rules and patterns are the rulings of
[decisions.d/A1-02.md](../../docs/decisions.d/A1-02.md).

## Coverage

At least one invalid fixture per surface of 26 5.5 (frontmatter keys of `SKILL.md`, both inline
shell forms, settings files, folder content, plugin parts, an MCP `headersHelper`, hooks and `@`
imports with an absolute, a `~`, a `..` and a backslash path) and per field code of 26 4.3. The Go
test `TestCorpusCoversEveryCode` fails when a code has no fixture; the wire test
`TestAgentDefinitionLimitsEqualCorpus` fails when `limits.json` differs from the server defaults.

What the corpus does not prove: whether a later version of the Claude Code CLI reads a surface
that 26 5.5 does not name (RK-14). The runner's probes of 26 5.7 (A1-05) cover the pinned version
only.

## Running

```sh
(cd services/server && go test ./internal/agents/domain/definition/)
```
