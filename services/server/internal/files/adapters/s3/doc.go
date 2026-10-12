// SPDX-License-Identifier: Apache-2.0

// Package s3 is the S3 driver of the object storage port app.ObjectStore of the files module
// (docs/spec/12-dateien.md "4.3", ADR 0010), selected with file.storage.driver = s3. It speaks
// only core S3 to any S3-compatible service: PUT, GET with a range, HEAD and DELETE of single
// objects, multipart upload and presigned URLs for upload parts. It is the only package of the
// server that imports the AWS SDK (depguard rule in .golangci.yml, TestS3SDKOnlyInAdapter).
//
// Rules (docs/DECISIONS.md M3-02/1 to M3-02/13):
//
//   - The driver holds only the narrow interfaces coreAPI (eight operations) and presignAPI
//     (PresignUploadPart), so DeleteObjects, listing, copying, bucket calls, versioning and
//     presigned GET are unreachable; forbidigo forbids DeleteObjects and PresignGetObject.
//   - Access keys come only from file.s3.access_key_ref and file.s3.secret_key_ref through the
//     secret provider, resolved again on every refresh of the SDK credential cache, so a rotation
//     works without restart after secrets.cache_ttl. The default configuration and credential
//     chain of the SDK (environment, shared files, instance metadata) are never used.
//   - Every request goes through one egress client with purpose objectstore whose host policy
//     names exactly the request host and port; http and private addresses need an entry of
//     egress.allowed_hosts with allow_private. No redirects, no SDK retries (02 "14.3"), the time
//     limit http.transfer_timeout and the body limit file.max_upload_bytes.
//   - Checksums of the SDK follow file.s3.checksum_mode (default when_required); streamed bodies
//     are signed as UNSIGNED-PAYLOAD, and the own SHA-256 of 12 "3.4" checks the content.
//   - Every key passes domain.ValidateKey before any request; file.s3.prefix is joined with "/".
//   - Errors keep the contract of the port and never contain a request URL, a presigned URL, the
//     endpoint or credentials; the driver adds no metric, span or log line.
//
// Example (internal/wire builds the store this way):
//
//	store, err := s3.New(ctx, cfg, s3.Deps{
//		Egress:          egressCfg,          // wire.EgressConfig(v)
//		EgressDeps:      egress.Deps{Logger: logger, Metrics: metrics},
//		Secrets:         provider,           // *secrets.Provider of the process
//		TransferTimeout: transferTimeout,    // http.transfer_timeout
//		MaxObjectBytes:  maxUploadBytes,     // file.max_upload_bytes
//	})
package s3
