// SPDX-License-Identifier: Apache-2.0

// Package domain holds the pure rules of the apps module (docs/spec/11-extensions.md): the
// typed manifest app_manifest.v1 and its check rules (section 3), SemVer 2.0.0 precedence, the
// consent rules for granted scopes and event subscriptions at install, upgrade and scope change
// (section 4), the manifest difference of an upgrade and the states of an installation. It imports
// only the kernel core (02 section 5); the application layer turns catalogue facts and
// configuration into Env.
package domain
