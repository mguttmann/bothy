# Package `platform/config`

Status: built (M1-07), references extended (M1-09). The normative sources are
[02-architecture.md](../../../../../docs/spec/02-architecture.md) "Configuration loading",
[16-operations.md](../../../../../docs/spec/16-operations.md) "Configuration (ADR 0011)" and the
[configuration reference](../../../../../docs/operations/configuration.md) sections 1 to 3; if
this file differs, the chapter applies.

## Purpose

The only way process configuration gets into the server: schema of all keys, parser per kind,
loading with precedence, check at start and binding to the `Config` structs of the users.
Settings of the owner (`instance_settings`, feature flags) are no configuration and do not belong
here.

## Code

| File | Content |
|---|---|
| `kind.go` | `Kind` and the parsers per kind with canonical text |
| `envname.go` | naming rule `a.b_c` to `BOTHY_A__B_C` and back |
| `schema.go` | `Key`, `Schema`, `NewSchema` with all check rules, `AddCheck` |
| `load.go` | `Sources`, `Load`: file, environment, flag, unknown names, cross checks |
| `values.go` | `Values` with accessors per kind, `Source` |
| `bind.go` | `Bind`: values into a struct with tags `config:"<key>"` |
| `errors.go` | `LoadError` and `Problem` |
| `doc.go` | package description |

## Interface

- `NewSchema(keys)` checks the keys; `(*Schema).AddCheck(name, keys, fn)` registers a cross
  check; `(*Schema).Keys()` returns the keys sorted by name.
- `Load(schema, Sources{Env, File, Flags})` returns `*Values` or `*LoadError` with all
  problems.
- `Values`: `String`, `Int`, `Float`, `Bool`, `Duration`, `Strings`, `Prefixes`, `Text`,
  `Source`, `File`. An unknown key or a wrong kind is a programming error and leads to a panic,
  which tests find.
- `Bind(values, &cfg)` fills a struct of the using package; `internal/wire` calls it.
- `EnvName` and `KeyFromEnv` are exactly reversible for valid names.

## Rules

1. Precedence from low to high: default value, file (`--config` or `BOTHY_CONFIG_FILE`),
   environment (`BOTHY_*`), flag (only `--role`). `Values.Source` names the winner per key.
2. Every value from file, environment or flag is checked, even if a higher source overrides it;
   this way an error in the file does not stay hidden until the variable goes away.
3. Unknown `BOTHY_` variables and unknown file keys abort. Exempt are `BOTHY_CONFIG_FILE` and
   variables that a valid reference `env:` in the effective value of a `*_ref` key names.
   Variables without the prefix `BOTHY_` are ignored.
4. The file is one YAML document, nested like the names. Duplicate and dotted keys, anchors,
   aliases, merge keys and explicit tags are problems. Scalars go as the written text to the same
   parser as the environment, so the implicit typing of YAML changes no value (`yes` stays `yes`,
   `1.10` stays `1.10`).
5. Required keys need a non-empty value. Limits and enumerations apply to every source. Cross
   checks run only when all their keys are loaded without a problem (also none in a rejected
   file or under a rejected section), and see only these keys.
6. Problems are collected, sorted by key and reported together:
   `config: <key>: <message> (source: <source>)`. No message contains a value, not even an
   invalid one: a secret can be in the wrong key. That is why a message names a file key without
   the form of a key only with its line, and paths or names with control characters stand in
   quotation marks.
7. Secrets appear only as a reference in keys ending in `_ref` (kind `secret_ref`). This package
   checks only the form; nothing is resolved. A reference `file:` names an absolute, clean path
   (no element `.` or `..`, no double and no trailing slash, no NUL), the same rule as in the
   secret provider (`internal/platform/secrets`), so that `bothy config check` and the start
   accept the same references. A reference `env:` names neither `BOTHY_CONFIG_FILE` nor a variable
   in the form of a key variable. In default values of `secret_ref` keys, `<key>` stands for the
   effective value of a string key; in set values from file and environment only `<secrets.dir>`
   is replaced, with the effective value of `secrets.dir` (M1-09). Inserted text is not searched
   again. A problem of the replaced text starts with `after expanding <secrets.dir>:`; if
   `secrets.dir` itself has a problem, the reference reports no second one. The loader checks
   `secrets.dir` itself (a rule that no kind expresses, M1-09/15): absolute, clean, not `/`,
   without NUL and without `<`. A `secret_ref` key with an empty default (chapter cell "empty", for
   example `server.tls_cert_ref`) is optional: the empty text means "no reference" and switches
   its feature off; a key with a reference as default still refuses the empty text
   (docs/DECISIONS.md M1-21/1).
8. The package never reads the process environment. `cmd/bothy` reads it once and passes it on; a
   forbidigo rule in `.golangci.yml` forbids the reading functions (also `exec.Cmd.Environ` and
   `http.ProxyFromEnvironment`) outside `cmd/bothy` and the tests under `tools/`.

Grammars of the kinds: configuration reference, section "Naming rule and formats".

## New key

A key enters the schema with the ticket that builds its first user (02, 13 item 5): a row in the
table "Configuration values of this chapter" of its chapter, an entry in the file of its area
`internal/wire/configkeys_<area>.go` with the same default and the chapter as a constant from
`internal/wire/configchapters.go`, and a field in the `Config` struct of the using package. A new
cross check belongs in the list `configChecks` in `internal/wire/configchecks.go`. Parallel
tickets therefore share four places: the list of key groups in `internal/wire/config.go`, the list
`configChecks`, the chapter constants and the expected keys in `TestConfigSchemaBuilds`.

## Dependencies

Standard library and `go.yaml.in/yaml/v3` (register: `platform.config` with the vendor group
`yaml`). No other project packages; `wire` and `cmd` use this package.

## Chapters

[02-architecture.md](../../../../../docs/spec/02-architecture.md),
[16-operations.md](../../../../../docs/spec/16-operations.md),
[ADR 0011](../../../../../docs/adr/0011-12-factor-configuration-and-secret-provider.md),
[configuration.md](../../../../../docs/operations/configuration.md)
