// SPDX-License-Identifier: Apache-2.0

// Package stack prepares the secrets of the Compose stack (docs/spec/16-betrieb.md 5.3 and 5.4):
// bothy secrets init creates and copies them into one directory per service, and bothy secrets
// export writes them as an age-encrypted escrow bundle.
//
// Catalog is the Go form of the table "Geheimnisse" in docs/operations/configuration.md; a test
// keeps both equal. Init keeps the one canonical copy of every secret in <target>/_store and
// writes the copies each service mounts with a Compose subpath to <target>/<service>. It
// reconciles _store with the owner's source directory by the table of the brief (generate,
// adopt, keep, update, conflict), runs every check before the first write, writes atomically and
// changes nothing on a second run with the same inputs. Export streams _store as tar through age
// to the recipients; plaintext never reaches a disk.
//
// The package reads only its options and the directories they name: no configuration, no
// environment, no database, because the secrets-init container has none of them.
package stack
