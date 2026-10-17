// SPDX-License-Identifier: Apache-2.0

// Package s3sink is the object store implementation of app.CheckpointSink, the value
// s3://<bucket>[/<prefix>] of audit.checkpoint_sink (19 SEC-AUD-2 and section 16, runbook
// backup-restore 8, docs/DECISIONS.md M7-22/1 to M7-22/6): every signed checkpoint is the object
// [<prefix>/]<instance_id>/<seq with 20 digits, leading zeros>.jws in a bucket with Object Lock
// and a default retention in mode COMPLIANCE, written once and never overwritten.
//
// Rules:
//
//   - The sink holds only the narrow interface objectAPI (PutObject with If-None-Match: *,
//     GetObject, ListObjectVersions, GetObjectLockConfiguration). It is the second and last
//     package of the server that imports the AWS SDK, after internal/files/adapters/s3 (depguard
//     rule aws-sdk-only-in-s3-driver, TestS3SDKOnlyInAdapter).
//   - Own endpoint, bucket and credentials (audit.checkpoint_s3_*), separate from file.s3.* and
//     the backup. Access keys come only from the two references through the secret provider; the
//     default configuration and credential chain of the SDK are never used.
//   - Every request goes through one egress client with purpose audit_checkpoint whose host
//     policy names exactly the request host and port; no redirects, no SDK retries, the limits
//     egress.timeout and egress.max_response_bytes.
//   - List reads the oldest version of every key, so an overwrite that a service lets through
//     only adds a version that the sink never reads; it returns the checkpoints in seq order and
//     verifies nothing (verification stays in audit/domain).
//   - CheckLock refuses a bucket without an effective retention lock; bothy config check and the
//     server start call it.
//   - Errors never contain a request URL, the endpoint or credentials.
//
// Example (internal/wire builds the sink this way):
//
//	sink, err := s3sink.New(ctx, cfg, instanceID, s3sink.Deps{Egress: egressCfg, Secrets: provider})
//	err = sink.CheckLock(ctx, retentionAudit)
//	err = sink.Put(ctx, app.SignedCheckpoint{Seq: 5301, JWS: jws})
package s3sink
