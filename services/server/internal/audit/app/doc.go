// SPDX-License-Identifier: Apache-2.0

// Package app holds the use cases of the audit module (ADR 0021): the implementation of the
// kernel port auditing.Recorder, the audit phase hook that continues the hash chain before the
// commit (Chain, phase 3.3 of 02 "Flow"), the aggregation of identical denials (Aggregator) and
// the keyring of the IP pseudonyms (Keyring). Storage, the key-encryption key and the error
// reports are outgoing ports (Store, KEKSource, Reporter) that the adapters and internal/wire
// implement. It imports only the own domain, kernel and the port of instance (audit_seq).
package app
