# M1-07: Configuration schema and loader

Status: ready | Depends on: M1-05 | Etappe: M1

## Goal

`internal/platform/config` knows every process configuration key the server reads, each with kind,
default, documentation and owning chapter. It loads default < file < environment < command-line
flag, aborts on unknown keys and variables, type and range errors and broken cross-conditions, and
names key and source in every message without echoing values. `bothy config check` and
`bothy config print --redacted` let operators verify a configuration without starting the server.
This is the only way configuration enters the process; modules never read the environment.

## Read first

- `docs/PLAN.md`: section "Arbeitsweise je Ticket", section "M1 Fundament" (row M1-07, W1, W4),
  P-08, P-10, P-11.
- `docs/spec/02-architektur.md`: "Konfigurationsladen", "Befehle des Binarys", "Schichten im Modul",
  "Konfigurationswerte dieses Kapitels" (`http.max_json_body_bytes`).
- `docs/spec/16-betrieb.md`: "Netz" ("Zwei Listener", "Rand und Identität"), "Konfiguration und
  Geheimnisse" ("Konfiguration (ADR 0011)", "Secret-Provider (ADR 0011)"), "Konfigurationswerte
  dieses Kapitels" (`server.role`, `server.listen_addr`, `server.internal_listen_addr`).
- `docs/spec/03-datenmodell.md`: "Konfigurationswerte dieses Kapitels" (`message.max_content_bytes`).
- `docs/operations/configuration.md`: "Quellen", "Namensregel und Formate", "Prüfung" (section 4 is
  generated; do not edit it by hand).
- ADR `docs/adr/0011-12-factor-konfiguration-und-secret-provider.md`, ADR 0050 and
  `services/server/architecture/modules.yaml` (register format from M1-05).
- `services/server/cmd/bothy` (command table from M1-05), `services/server/.golangci.yml`,
  `.semgrep/bothy-go-no-model-credentials.yaml`.

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or doc
   comment (purpose, parameters, return value, errors, example where useful); comments explain why,
   not what. API documentation is generated from code (OpenAPI from the routes, dartdoc, Python API
   docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision, a
   CHANGELOG entry per release, docs site kept current. A change without matching documentation is
   not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the merge.
6. Test first: write the failing test, run it and keep its red output for the report, make it green,
   then clean up.
7. No en dash (U+2013) or em dash (U+2014) in any file you write; `grep -nP "[\x{2013}\x{2014}]"`
   over your changed files prints nothing.
8. White label: no company, person, host or instance names, no paths of a personal machine, no real
   data. Example addresses use 127.0.0.1 or RFC 5737 ranges, hosts example.com.
9. Configuration values only through named keys from `docs/operations/configuration.md`; every key
   this ticket registers has a row in its chapter table with the same default.
10. Configuration (operator, restart) and settings (owner, database) are separate; only
    configuration is loaded here (02 "Konfigurationsladen" item 1, 16 "Konfiguration" item 6).
11. Name rule `a.b_c` to `BOTHY_A__B_C`, reversible (16 "Konfiguration" item 2).
12. Unknown `BOTHY_` variables and unknown file keys abort the start; type and range errors abort
    the start (16 items 3 and 4, R-AR-15, R-OP-04).
13. Secrets only as references in keys ending in `_ref`; values never appear in logs, errors or
    `bothy config print` (02 "Konfigurationsladen" item 3, ADR 0011).
14. Typed per consumer: each module or platform package gets its own `Config` struct, filled by
    `internal/wire`; modules never read environment variables (02 "Konfigurationsladen" item 5).
15. No reload at runtime except secret rotation, which M1-09 adds (02 item 6).

## Scope

Create (paths below `services/server/`):

- `internal/platform/config/`: `kind.go`, `schema.go`, `envname.go`, `load.go`, `values.go`,
  `bind.go`, `errors.go`, `doc.go`, `README.md` (German, like the module READMEs), tests.
- `internal/wire/config.go` (`ConfigSchema`), `internal/wire/configkeys_server.go`,
  `internal/wire/configkeys_http.go`, `internal/wire/configkeys_messaging.go`,
  `internal/wire/configchecks.go`, tests.
- `cmd/bothy/config.go` (command `config` with `check` and `print`), tests.

Change:

- `architecture/modules.yaml`: `platform.config: {depends_on: [], vendors: [yaml]}` and
  `vendors.yaml: [go.yaml.in/yaml/v3]`; regenerate `.go-arch-lint.yml` (`go generate ./tools/archgen`).
- `.golangci.yml`: forbidigo rule (see Design notes).
- `go.mod`/`go.sum` only if needed (the YAML module is already required by M1-05).
- `docs/spec/02-architektur.md` "Konfigurationsladen" items 2 and 4; `docs/spec/16-betrieb.md`
  "Konfiguration (ADR 0011)" items 1 and 3; `docs/operations/configuration.md` sections 1 to 3.
- `services/server/README.md` (platform table row `config`), `services/server/CHANGELOG.md`, root
  `CHANGELOG.md`, `docs/DECISIONS.md`.

## Out of scope

- Generated configuration reference, the JSON export of the schema, chapter table comparison and the
  W1 name cleanup: M1-08.
- Resolving secret references, `secrets.*` keys, `bothy secrets`: M1-09. Telemetry keys: M1-10.
  Listeners, `bothy serve`, health: M1-11. Database keys: M1-12. Every other key: the ticket that
  builds its first consumer adds it to the schema and to its `Config` struct.
- The trusted header driver and its loopback rule: it is a database setting of the owner, checked at
  readiness (16 "Rand und Identität", W4, M1-43), not a configuration check.
- Instance settings and feature flags (M1-46, M1-47).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-07-config-schema-loader`.
- Conventional Commits with DCO sign-off (`git commit -s`), scope `server`, `spec` or `docs`.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside Scope, except docs and changelog entries named here.
- Follow the host rules supplied by the launcher. Stop only processes you started, by PID.
- Temporary directories only via `mktemp -d` or `t.TempDir()`; remove them. Tests pass environment
  maps explicitly and never read or set the process environment (`t.Setenv` is not used).
- Never read or print secrets.
- No network calls except package downloads for pinned tools and Go modules, and what the
  repository gates in `make check` do themselves. Run Go with `GOTOOLCHAIN=local`.

## Design notes

### Public API of `internal/platform/config`

```go
type Kind int // KindString, KindEnum, KindBool, KindInt, KindFloat, KindDuration, KindStringList,
              // KindCIDRList, KindListenAddr, KindCron, KindSemVer, KindSecretRef, KindURL
func (k Kind) String() string // "string", "enum", "bool", "int", "float", "duration", "list(string)",
                              // "list(cidr)", "listen address", "cron", "semver", "secret_ref", "url"

type Key struct {
    Name     string   // e.g. "server.listen_addr"
    Kind     Kind
    Default  string   // exactly the chapter table cell without backticks; "" for "leer"
    Required bool     // chapter cell "keiner, Pflicht"; Default must then be ""
    Enum     []string // only for KindEnum
    Min, Max string   // optional bounds in the kind's text form (int, float, duration)
    Doc      string   // one English sentence
    Chapter  string   // file name of the chapter whose table holds the key, e.g. "16-betrieb.md"
}

func NewSchema(keys []Key) (*Schema, error)
func (s *Schema) AddCheck(name string, keys []string, fn func(*Values) error) error
func (s *Schema) Keys() []Key // sorted by name
func EnvName(key string) string
func KeyFromEnv(variable string) (key string, ok bool)

type Sources struct {
    Env   []string          // "NAME=value" entries as from os.Environ
    File  string            // from --config; "" means BOTHY_CONFIG_FILE from Env, else no file
    Flags map[string]string // key -> raw value; only "server.role" (from --role)
}
func Load(s *Schema, src Sources) (*Values, error) // error is *LoadError with every problem

type Source struct{ Kind SourceKind; Name string } // default | file (path) | env (variable) | flag (--role)
func (v *Values) String(key string) string
func (v *Values) Int(key string) int64
func (v *Values) Float(key string) float64
func (v *Values) Bool(key string) bool
func (v *Values) Duration(key string) time.Duration
func (v *Values) Strings(key string) []string
func (v *Values) Prefixes(key string) []netip.Prefix
func (v *Values) Text(key string) string // canonical text form
func (v *Values) Source(key string) Source
func (v *Values) File() string           // effective file path or ""
func Bind(v *Values, dst any) error

type LoadError struct{ Problems []Problem }
type Problem struct{ Key, Source, Message string }
```

- Accessors panic on an unknown key or a kind mismatch: that is a programming error that tests
  catch, not an operator error.
- `Bind` fills a pointer to a struct whose exported fields carry `config:"<key>"` tags. Field types:
  `string` (string, enum, listen address, cron, semver, secret_ref, url), `bool`, `int64` or `int`
  (int), `float64`, `time.Duration`, `[]string`, `[]netip.Prefix`. An exported field without tag, an
  unknown key or a type that does not fit the kind is an error. Consumers declare their `Config`
  struct in their own package (a module port may declare it, it needs no import); `internal/wire`
  calls `Bind`. This ticket has no consumer yet, so it ships `Bind` with tests and no module struct.

### Schema rules (checked by `NewSchema`)

- Name: segments joined by `.`, at least two segments, each `^[a-z][a-z0-9]*(_[a-z0-9]+)*$`. This
  excludes `__`, leading or trailing `_`, upper case, so `EnvName` and `KeyFromEnv` are exact
  inverses.
- `EnvName(k) = "BOTHY_" + upper(k)` with `.` replaced by `__`.
- A name ends in `_ref` exactly when the kind is `secret_ref` (02 "Konfigurationsladen" item 3).
- Default parses with the kind's parser; empty default only for `string`, `list(string)`,
  `list(cidr)` and required keys; `Enum` non-empty exactly for enum; bounds parse and `Min <= Max`;
  `Doc` non-empty; `Chapter` matches `^[0-9]{2}-[a-z0-9-]+\.md$`; names unique.
- In `secret_ref` defaults a placeholder `<other.key>` is replaced by that key's effective value at
  load time (16: default `file:<secrets.dir>/<name>`); the placeholder must name a registered string
  key. Placeholders are expanded only in defaults.

### Parsers (identical for default, environment and file; the file parser hands the scalar text
### of the YAML node to the same parser, so YAML implicit typing never changes a value)

| Kind | Accepted text |
|---|---|
| string | any text |
| enum | one of `Enum`, exact |
| bool | `true` or `false` |
| int | base-10 `int64`, optional leading `-` |
| float | finite decimal (`strconv.ParseFloat`) |
| duration | `^(0\|[1-9][0-9]*)(ms\|s\|m\|h\|d)$`, `d` = 24 h; overflow is an error; no fractions, no composites |
| list(string) | environment and defaults: a JSON array of strings if the text starts with `[`, else comma-separated with spaces trimmed; empty text is the empty list; an empty element is an error. File: a YAML sequence of scalars, or a scalar parsed like the environment form |
| list(cidr) | list(string), each element `netip.ParsePrefix` with no host bits set |
| listen address | `host:port` (`net.SplitHostPort`); host empty or an IP literal; port 0 to 65535 |
| cron | five fields, UTC; each field `*`, `*/n`, `a`, `a-b`, `a-b/n` or a comma list of these; minute 0-59, hour 0-23, day 1-31, month 1-12, weekday 0-6; no names |
| semver | SemVer 2.0.0 without leading `v` |
| secret_ref | `<scheme>:<rest>`; `file` (absolute path after placeholder expansion) and `env` (variable name `^[A-Z_][A-Z0-9_]*$`); `azkv` and `awssm` give "scheme not supported by this server version"; anything else is invalid |
| url | `http` or `https`, non-empty host, no user info, no fragment |

### Loading

- Order, low to high: default, file, environment, flag. `Source` records the winner per key.
- File: `Sources.File`, else `BOTHY_CONFIG_FILE` from `Sources.Env`, else none. A named file that is
  missing or unreadable is a problem. The file is YAML nested like the names
  (`sync: {event_retention: 30d}`); decode with `go.yaml.in/yaml/v3` into `yaml.Node`. Duplicate
  keys, a mapping where a value is expected, a scalar where a mapping is expected and a dotted key
  (`sync.event_retention: 30d`) are problems naming the path and the file.
- Environment: variables without the prefix `BOTHY_` are ignored (`OTEL_*` belong to the telemetry
  SDK, 16 item 7). Every `BOTHY_` variable must be a key's `EnvName`, `BOTHY_CONFIG_FILE`, or the
  variable named by an `env:` reference in the effective value of a `secret_ref` key (16 "Secret-
  Provider" allows `env:BOTHY_SECRET_...`); anything else is `unknown variable`. Reason: a typo must
  never silently leave a security limit at its default.
- Required keys without value, bounds and enum violations are problems.
- After all keys parse, run the cross-checks registered with `AddCheck`; a check runs only when all
  its keys parsed.
- Problems are collected, sorted by key, and returned together. Message format:
  `config: <key>: <message> (source: <source>)`, unknown variables as
  `config: unknown variable BOTHY_X (source: environment)`, unknown file keys as
  `config: unknown key "a.b" (source: file <path>)`. Messages never contain the offending value of
  any key: values can be secrets placed in the wrong key.

### Keys and cross-checks this ticket registers (in `internal/wire`)

| Key | Kind | Default | Bounds | Chapter |
|---|---|---|---|---|
| `server.role` | enum `api`, `worker`, `all` | `all` | | `16-betrieb.md` |
| `server.listen_addr` | listen address | `127.0.0.1:8080` | | `16-betrieb.md` |
| `server.internal_listen_addr` | listen address | `127.0.0.1:8081` | | `16-betrieb.md` |
| `http.max_json_body_bytes` | int | `1048576` | min `1` | `02-architektur.md` |
| `message.max_content_bytes` | int | `262144` | min `1` | `03-datenmodell.md` |

If a chapter default differs from this table when you build, the chapter wins; use its value. Keys
enter the schema with the ticket that builds their first consumer; these five are here because the
loader's own flag (`--role`) and its cross-checks read them.

- `listeners_distinct`: `server.listen_addr` and `server.internal_listen_addr` conflict when the
  ports are equal and not 0, and the hosts are equal or either host is empty, `0.0.0.0` or `::`.
  Port 0 is allowed twice because tests bind port 0 (P-10).
- `json_limit_above_message`: `http.max_json_body_bytes > message.max_content_bytes`.
- `wire.ConfigSchema() (*config.Schema, error)` builds the schema from the key groups
  (one file per owning area, so parallel tickets add files instead of editing one list; the single
  list of groups in `config.go` is the only shared line) and registers the checks.

### Command `bothy config`

- `bothy config check [--config FILE] [--role ROLE]`: loads with `os.Environ()`, prints
  `configuration ok` and exits 0, or prints each problem on stderr and exits 1; usage errors exit 2.
- `bothy config print [--redacted] [--config FILE] [--role ROLE]`: first line
  `# configuration file: <path>` or `# configuration file: none`, then one line per key sorted by
  name: `<key> = <canonical text>  # <source>` (`default`, `file <path>`, `env BOTHY_X`,
  `flag --role`). `secret_ref` keys print the reference itself; nothing is ever resolved. Output is
  always redacted; `--redacted` is accepted so that scripts can state it, and no flag prints more.
- `cmd/bothy` is the only place that reads the process environment, once, and passes it as
  `Sources.Env`.

### Lint

In `.golangci.yml` enable `forbidigo` with `analyze-types: true` and forbid `os.Getenv`,
`os.LookupEnv`, `os.Environ`, `os.ExpandEnv` and `syscall.Getenv`, with one path exclusion for
`cmd/bothy/`. M1-09 adds its own exclusion for the `env` secret driver.

### Spec edits in this change (spec first)

- 02 "Konfigurationsladen" item 2: the mapping is now fixed (remove "diese ist der Vorschlag"),
  flags are `--role` and `--config`. Item 4: examples become exactly the two cross-checks above (JSON
  limit strictly greater than `message.max_content_bytes`; listeners distinct with the port 0
  exception); the trusted header example moves to a sentence pointing to 16 "Rand und Identität"
  (readiness check, not configuration).
- 16 "Konfiguration (ADR 0011)": item 1 precedence including the command line; item 3 names the
  exempt variables (`BOTHY_CONFIG_FILE`, variables named by `env:` references).
- `docs/operations/configuration.md` sections 1 to 3: precedence including the command line, the
  exact grammars of the table above, rows for `list(cidr)` and `url`, and "messages never show
  values".
- DECISIONS.md: precedence with flags; exempt variables; no values in messages; duration grammar;
  port 0 exception; trusted header rule moved to readiness; `Bind` with struct tags as the typed
  per-consumer mechanism; keys enter the schema with their first consumer.

## Tests (write them first)

`internal/platform/config` (each test builds its own small schema):

- `TestEnvNameRoundTrip` (`a.b_c` to `BOTHY_A__B_C` and back; also every key of
  `wire.ConfigSchema()`), `TestInvalidKeyNames` (`a__b.c`, `a._b`, `A.b`, `single`, `a.b.`).
- `TestDurationParse` (`250ms`, `5s`, `2m`, `24h`, `7d` = 168 h, `0s`; rejected `1.5h`, `1h30m`, `5`,
  `-1s`, `5S`, `99999999999999d`).
- `TestListFromEnvCommaAndJSON` (`a, b ,c`, `["a","b"]`, empty, `a,,b` rejected),
  `TestListFromYAMLSequence`.
- `TestUnknownFileKeyNamesKeyAndFile`, `TestUnknownEnvVariableNamesVariable`,
  `TestConfigFileVariableIsAllowed`, `TestEnvSecretRefVariableIsAllowed`.
- `TestPrecedenceDefaultFileEnvFlag` (`server.role`: file `worker`, env `api`, flag `all`; result
  `all` with source flag; without flag `api` from env; without env `worker` from file).
- `TestTypeErrorNeverEchoesValue` (value `canary-7f3a` appears in no message).
- `TestRequiredKeyMissing`, `TestBounds`, `TestEnum`.
- `TestYAMLDuplicateKeyRejected`, `TestYAMLDottedKeyRejected`, `TestYAMLMappingWhereScalarExpected`.
- `TestRefNamingRule` (`x.y_ref` must be `secret_ref` and back), `TestSecretRefSchemes` (file, env,
  azkv unsupported, missing scheme, relative file path), `TestSecretRefDefaultPlaceholder`.
- `TestCron`, `TestSemVer`, `TestURL`, `TestListenAddr`, `TestCIDRList` (host bits set rejected).
- `TestBind` (all field types; unknown key, untagged exported field and type mismatch rejected).
- `TestAccessorPanicsOnUnknownKeyOrKind`.
- `TestProblemsSortedAndComplete` (three problems reported together).

`internal/wire`: `TestConfigSchemaBuilds`, `TestDefaultsPassCrossChecks`,
`TestListenersDistinct` (table: same address, `:8080` vs `127.0.0.1:8080`, `[::]:8080` vs
`127.0.0.1:8080`, both port 0, different ports), `TestJSONLimitAboveMessage` (equal, smaller,
larger).

`cmd/bothy`: `TestConfigCheckOK`, `TestConfigCheckReportsProblemsExit1`, `TestConfigUsageExit2`,
`TestConfigPrintShowsSourcePerKey`, `TestConfigPrintShowsReferenceOnly` (a test schema with
`database.password_ref = env:BOTHY_SECRET_TEST`; the variable's content `canary-9b1c` is never
printed), `TestConfigPrintRedactedFlagAccepted`. The command code takes the schema and the
environment as parameters so tests inject both.

Commands that must pass:

```sh
cd services/server && GOTOOLCHAIN=local go build ./... && go vet ./...
make test arch codegen-check fmt-check lint typecheck coverage licenses osv semgrep docs linkcheck
make sonar
make check
```

## Acceptance criteria

- [ ] Every test above exists and passes with `-race`.
- [ ] `bothy config check` and `bothy config print --redacted` behave as specified; exit codes 0, 1,
      2.
- [ ] Unknown `BOTHY_` variable and unknown file key abort with key and source in the message.
- [ ] No message and no print output contains a value given for a `secret_ref` key or any invalid
      value.
- [ ] forbidigo rejects `os.Getenv` outside `cmd/bothy` (show the lint failure on a scratch file once
      in the report, then remove it).
- [ ] `make arch` passes with `platform.config` in the register; `.go-arch-lint.yml` regenerated.
- [ ] 02, 16 and `docs/operations/configuration.md` sections 1 to 3 describe exactly the implemented
      behaviour; `make docs` passes (section 4 untouched).
- [ ] Coverage of the Go module at or above `quality.coverage.go_min_percent`; `make sonar` passes.
- [ ] `make check` is green.

## Definition of done

- Code, tests and docs as above: `internal/platform/config/README.md`, doc comments on every public
  symbol (with an example for `Load` and `Bind`), DECISIONS.md entries, `services/server/CHANGELOG.md`
  (`Added`), root `CHANGELOG.md` (`Changed`: chapters 02, 16 and the configuration reference).
- `make check` green.
- Independent review over the diff (a fresh reviewer, `make review`) and the red team gate of
  `docs/quality/red-team-gate.md` over the diff, verdict recorded in the report; fix every finding
  with a test and have the delta checked again.
- Merge locally into the base branch only if review and red team are green and `make check` passes
  on the merged state; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then commits
(SHA and subject), files with line counts, test results with counts (including the red output of
the first test run), what is not proven, and follow-ups that could not be done in scope with the
reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<merge sha>` or `exit=1 <reason>`.
