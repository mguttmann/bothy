// SPDX-License-Identifier: Apache-2.0

// Package guide runs the developer guide "Connect your own agent" (docs/guides/connect-agent.md)
// as a test against a fresh server (M2-37, exit criterion 7 of milestone M2).
//
// The page is the script: Parse reads its sections and fenced blocks, Validate holds them
// against chapter 11 section 13 (step titles), docs/api/openapi.yaml (operations), the example
// adapter in sdk/python/examples/guide (shown files) and sdk/python/VERSION (the pin), and Run
// executes every block in order: requests as the owner or a person, shell commands, the example
// adapter as a background process, and compares every answer and output line with the text. The
// first block that differs from the text fails the run with its step and title; nothing after
// it runs. The block grammar is described in README.md.
//
// The package is test support (component testsupport); production code never imports it.
package guide
