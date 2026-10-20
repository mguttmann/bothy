// SPDX-License-Identifier: Apache-2.0

// Package stack runs the Compose stack of the terminal conformance suite (brief M2-48 Design
// notes 3 and 5): the unchanged installation file infra/compose/compose.yaml with the smoke
// overlay of M1-73 and the test overlays infra/compose/test/compose.terminal.yaml (and, for the
// profile twonode, M2-47's compose.multi-node.yaml and compose.terminal-twonode.yaml), the server
// image built from the worktree with services/server/Dockerfile, a test CA and the TLS front of
// the instance origin https://chat.example.com:8443, and the host containers with systemd in
// which the real host service bothy-host runs. It belongs to the architecture component
// testsupport and is never imported by production code.
//
// Everything it creates carries the project name termsuite-<package>-<8 hex>; Down removes the
// project and asserts that nothing of it remains. Keys, tokens and codes of the throwaway stack
// stay in memory or in files of mode 0600 in the temporary directory and are never printed.
package stack
