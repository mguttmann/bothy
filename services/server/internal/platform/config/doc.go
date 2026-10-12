// SPDX-License-Identifier: Apache-2.0

// Package config is the only way process configuration enters the server
// (docs/spec/02-architecture.md "Configuration loading", docs/spec/16-operations.md 5.1 with ADR 0011,
// docs/operations/configuration.md).
//
// A Schema lists every key the server reads, each with kind, default, documentation and owning
// chapter; internal/wire builds it from one key group per owning area and registers the
// cross-checks. Load reads the built-in defaults, the YAML file (--config or BOTHY_CONFIG_FILE),
// the BOTHY_ environment variables and the --role flag in that order of precedence and returns
// Values, or a *LoadError with every problem: unknown variables and file keys, type, range and
// enum errors, missing required keys and failed cross-checks. Problems name the key and the
// source, never a value, because a value can be a secret in the wrong key. Bind copies Values
// into the Config struct a consumer declares in its own package.
//
// The package never reads the process environment: cmd/bothy reads it once and passes it in
// Sources.Env (a forbidigo rule in .golangci.yml keeps it that way). Secrets appear only as
// references in keys ending in _ref (kind secret_ref); this package expands <secrets.dir> in
// them and checks their form, file: paths with the same rules as the secret provider
// (internal/platform/secrets), but never resolves them. Configuration is read once at start;
// only secret rotation (secret provider) changes anything at run time. Owner settings in the
// database (instance_settings, feature flags) are not configuration and are not loaded here.
package config
