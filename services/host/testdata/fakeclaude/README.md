# fakeclaude

A stand-in for `claude auth status` in the tests of check HD6 (brief M2-42, design note 9). It
never calls a model, never reads a login and needs no account.

## Behaviour

- Prints the fixture named by the file `fake-auth` in `CLAUDE_CONFIG_DIR`, or in `$HOME/.claude`
  (`%USERPROFILE%\.claude`) when the variable is unset: `none`, `claude.ai`, `oauth_token`,
  `api_key`, `unknown_method`, `garbage` or `hang` (sleeps, so the check runs into
  `host_service.auth_status_timeout`).
- Exits `1` for `none` and `0` otherwise, like the CLI ("Exits with code 0 if logged in, 1 if not").
- Appends the path of its own executable to the start log set at build time with
  `-ldflags "-X main.startLog=<path>"`; the platform suite reads it to prove which program HD6
  started (TM-T27).
- A second build named `node` is the interpreter of `#!/usr/bin/env node` scripts and the wrong
  `node` of TM-T27.

## Fixtures

- `none.json` was recorded on 2026-10-17 from Claude Code 2.1.288 without a login, in a
  throwaway directory with an empty `HOME`, an empty `CLAUDE_CONFIG_DIR` and
  `DISABLE_TELEMETRY`, `DISABLE_ERROR_REPORTING` and `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC`
  set; the command needed no network and exited `1`. The two directory members held the
  throwaway path; they are replaced by an example path of a session user.
- The other files are derived from the documented format: "Show authentication status as JSON
  [...] Exits with code 0 if logged in, 1 if not [...] The JSON's `authMethod` field is one of
  `none`, `claude.ai`, `oauth_token`, `api_key`, `api_key_helper`, or `third_party`"
  (https://code.claude.com/docs/en/cli-reference, retrieved 2026-10-17). `unknown_method.json`
  carries a value outside that list; `garbage.txt` is not JSON.
