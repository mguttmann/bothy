// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// keyStorageDriver selects the driver of the object storage; the checks file_fs_root_absolute,
// file_s3 and upload_part_size_s3 and FilesObjectStore read it.
const keyStorageDriver = "file.storage.driver"

// The keys of the scan checks of M3-04 (docs/DECISIONS.md D-M3-04-15).
const (
	keyScanMode           = "file.scan.mode"
	keyClamdAddress       = "file.scan.clamd_address"
	keyClamdStreamMaxByte = "file.scan.clamd_stream_max_bytes"
)

// The keys of the thumbnail check of M3-06 (docs/DECISIONS.md D-M3-06-11).
const (
	keyThumbnailSmallPx  = "file.thumbnail.small_px"
	keyThumbnailMediumPx = "file.thumbnail.medium_px"
	keyThumbnailLargePx  = "file.thumbnail.large_px"
)

// jpegQualityMin and jpegQualityMax are the quality range of Go's image/jpeg Options (1 to 100), the
// bounds of file.image.jpeg_quality and file.thumbnail.jpeg_quality.
const (
	jpegQualityMin = "1"
	jpegQualityMax = "100"
)

// The keys of the download tickets (M3-05): the start check ticket key and DownloadKeys read them.
const (
	keyTicketTTL    = "file.ticket_ttl"
	keyTicketKeyRef = "file.ticket_key_ref"
)

// The keys of the upload checks of M3-03 (docs/DECISIONS.md D-M3-03-19).
const (
	keyMaxUploadBytes  = "file.max_upload_bytes"
	keyUploadPartBytes = "file.upload_part_bytes"
	keyPresignedUpload = "file.s3.presigned_upload"
)

// filesConfigKeys returns the keys of the object storage from docs/spec/12-dateien.md
// "18. Konfigurationswerte dieses Kapitels" that FilesObjectStore reads. The cross-check
// file_fs_root_absolute holds file.fs.root to an absolute, clean path when the driver is fs, the
// cross-check file_s3 the keys file.s3.* when the driver is s3 (docs/DECISIONS.md M3-02/11).
// The keys of the upload sessions (M3-03) follow: the reserve of the file system driver, the part
// size, the session lifetime, the open sessions and the quota per principal, the name length, the
// denied types, and file.s3.presigned_upload, which the check presigned_mode_unavailable holds
// false because the mode is not built (12 3.3). file.s3.presign_ttl enters with the presigned
// mode, its consumer (02 "13" item 5).
//
// The keys of the scan pass (M3-04) follow: file.scan.* without file.scan.pending_alert_after,
// which enters with the status component of M7-09, and file.sniff_bytes. The cross-check
// scan_stream_limit holds file.max_upload_bytes within the mirror file.scan.clamd_stream_max_bytes
// while the scan is required, the cross-check clamd_address_unix the address to a Unix socket
// (19 SEC-TLS-5).
//
// The keys of the job files.thumbnail (M3-06) follow: file.image.* without
// file.image.libvips_enabled, whose build does not exist, and file.thumbnail.*; the cross-check
// thumbnail_sizes_ordered holds small below medium below large. file.poster_max_bytes enters with
// the poster of a video, which UploadCreate cannot carry yet.
//
// The keys of the downloads (M3-05) follow: file.ticket_ttl and file.ticket_key_ref of the
// download tickets, file.not_ready_retry_after of 409 file_not_ready, and client.cache_max_age of
// chapter 04, the max-age of an authenticated download (12 9.3).
//
// voice.max_duration is here because getInstance reports it as a client limit (M1-46) before the
// voice messages of M3 read it.
func filesConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "voice.max_duration", Kind: config.KindDuration, Default: defaultQuarterHour, Min: "1s",
			Doc:     "Longest voice message.",
			Chapter: chapter12,
		},
		{
			Name: keyStorageDriver, Kind: config.KindEnum, Default: "fs", Enum: []string{"fs", "s3"},
			Doc:     "Driver of the object storage of files: fs for a directory of the server, s3 for an S3-compatible service.",
			Chapter: chapter12,
		},
		{
			Name: "file.fs.root", Kind: config.KindString, Default: "/var/lib/bothy/files",
			Doc:     "Root directory of the file system driver, used only by the server and owned by its user with mode 0700.",
			Chapter: chapter12,
		},
		{
			// A string, not a URL key: the schema allows an empty default only for string and list
			// keys; the cross-check file_s3 parses it.
			Name: "file.s3.endpoint", Kind: config.KindString, Default: "",
			Doc:     "Endpoint of the S3-compatible service, an absolute http or https URL without path, required with the driver s3.",
			Chapter: chapter12,
		},
		{
			Name: "file.s3.region", Kind: config.KindString, Default: "us-east-1",
			Doc:     "Region of the S3 signature.",
			Chapter: chapter12,
		},
		{
			Name: "file.s3.bucket", Kind: config.KindString, Default: "",
			Doc:     "Private bucket of the objects, required with the driver s3.",
			Chapter: chapter12,
		},
		{
			Name: "file.s3.prefix", Kind: config.KindString, Default: "",
			Doc:     "Optional prefix in the key grammar before every object name, so one bucket can hold several environments.",
			Chapter: chapter12,
		},
		{
			Name: "file.s3.use_path_style", Kind: config.KindBool, Default: defaultTrue,
			Doc:     "Path-style requests on the endpoint instead of virtual-hosted requests on <bucket>.<endpoint host>.",
			Chapter: chapter12,
		},
		{
			Name: "file.s3.checksum_mode", Kind: config.KindEnum, Default: "when_required", Enum: []string{"when_required", "when_supported"},
			Doc:     "When the S3 SDK computes and checks its own checksums for requests and responses.",
			Chapter: chapter12,
		},
		{
			Name: "file.fs.min_free_bytes", Kind: config.KindInt, Default: "2147483648", Min: "0",
			Doc:     "Free space the file system driver keeps; an upload that would leave less is refused with 503 storage_full.",
			Chapter: chapter12,
		},
		{
			Name: keyUploadPartBytes, Kind: config.KindInt, Default: "8388608", Min: "1",
			Doc:     "Size of every part of an upload but the last.",
			Chapter: chapter12,
		},
		{
			Name: "file.upload_session_ttl", Kind: config.KindDuration, Default: "24h", Min: "1s",
			Doc:     "Lifetime of an open upload session.",
			Chapter: chapter12,
		},
		{
			Name: "file.upload_max_open_sessions", Kind: config.KindInt, Default: "20", Min: "1",
			Doc:     "Open upload sessions of one principal.",
			Chapter: chapter12,
		},
		{
			Name: "file.quota_bytes_per_principal", Kind: config.KindInt, Default: "0", Min: "0",
			Doc:     "Bytes of files one principal may keep; 0 switches the quota off.",
			Chapter: chapter12,
		},
		{
			Name: "file.name_max_bytes", Kind: config.KindInt, Default: "255", Min: "4",
			Doc:     "Longest file name in UTF-8 bytes; longer names are shortened before their extension.",
			Chapter: chapter12,
		},
		{
			Name: "file.denied_mime_types", Kind: config.KindStringList, Default: "",
			Doc:     "Media types refused at upload, by declared type and by file extension; empty refuses none.",
			Chapter: chapter12,
		},
		{
			Name: keyPresignedUpload, Kind: config.KindBool, Default: "false",
			Doc:     "Allow the upload mode presigned; it is not built, so the check presigned_mode_unavailable refuses true.",
			Chapter: chapter12,
		},
		{
			Name: keyScanMode, Kind: config.KindEnum, Default: "required", Enum: []string{"required", "off"},
			Doc:     "Virus scan of uploads: required keeps every file undeliverable until clamd checked it (fail closed), off marks files skipped.",
			Chapter: chapter12,
		},
		{
			Name: keyClamdAddress, Kind: config.KindString, Default: "unix:///run/clamav/clamd.sock",
			Doc:     "Unix socket of clamd as unix:// followed by an absolute path; no other transport is accepted.",
			Chapter: chapter12,
		},
		{
			Name: "file.scan.chunk_bytes", Kind: config.KindInt, Default: "65536", Min: "1", Max: "2147483647",
			Doc:     "Largest chunk of an INSTREAM to clamd, and the bound of its reply.",
			Chapter: chapter12,
		},
		{
			Name: "file.scan.timeout", Kind: config.KindDuration, Default: "120s", Min: "1s",
			Doc:     "Deadline of the read pass with the scan of one file.",
			Chapter: chapter12,
		},
		{
			Name: "file.scan.max_concurrent", Kind: config.KindInt, Default: "4", Min: "1",
			Doc:     "Scans that one worker process runs at the same time.",
			Chapter: chapter12,
		},
		{
			Name: "file.scan.retry_backoff_cap", Kind: config.KindDuration, Default: "5m", Min: "1s",
			Doc:     "Longest pause between two attempts to scan a file while clamd is not reachable.",
			Chapter: chapter12,
		},
		{
			Name: "file.scan.health_interval", Kind: config.KindDuration, Default: thirtySeconds, Min: "1s",
			Doc:     "Interval of the PING to clamd, the first pause of a retried scan and the deadline of a PING.",
			Chapter: chapter12,
		},
		{
			Name: keyClamdStreamMaxByte, Kind: config.KindInt, Default: "104857600", Min: "1",
			Doc:     "Mirror of the clamd option StreamMaxLength; the server does not start when file.max_upload_bytes is larger while the scan is required.",
			Chapter: chapter12,
		},
		{
			Name: "file.sniff_bytes", Kind: config.KindInt, Default: "1048576", Min: "1",
			Doc:     "Bytes at the start of a file the type detection reads.",
			Chapter: chapter12,
		},
		{
			Name: "file.image.max_pixels", Kind: config.KindInt, Default: "50000000", Min: "1",
			Doc:     "Largest number of pixels (width times height, for GIF times frames) the server decodes; larger images get no thumbnails.",
			Chapter: chapter12,
		},
		{
			Name: "file.image.jpeg_quality", Kind: config.KindInt, Default: "90", Min: jpegQualityMin, Max: jpegQualityMax,
			Doc:     "JPEG quality of a still WebP without alpha that the server stores as JPEG.",
			Chapter: chapter12,
		},
		{
			Name: "file.image.processing_timeout", Kind: config.KindDuration, Default: sixtySeconds, Min: "1s",
			Doc:     "Time limit of one run of the image job files.thumbnail.",
			Chapter: chapter12,
		},
		{
			Name: "file.image.max_concurrent", Kind: config.KindInt, Default: "2", Min: "1",
			Doc:     "Image jobs that one worker process runs at the same time.",
			Chapter: chapter12,
		},
		{
			Name: keyThumbnailSmallPx, Kind: config.KindInt, Default: "320", Min: "1",
			Doc:     "Longest edge of the thumbnail variant small in pixels.",
			Chapter: chapter12,
		},
		{
			Name: keyThumbnailMediumPx, Kind: config.KindInt, Default: "1024", Min: "1",
			Doc:     "Longest edge of the thumbnail variant medium in pixels.",
			Chapter: chapter12,
		},
		{
			Name: keyThumbnailLargePx, Kind: config.KindInt, Default: "2048", Min: "1",
			Doc:     "Longest edge of the thumbnail variant large in pixels.",
			Chapter: chapter12,
		},
		{
			Name: "file.thumbnail.jpeg_quality", Kind: config.KindInt, Default: "82", Min: jpegQualityMin, Max: jpegQualityMax,
			Doc:     "JPEG quality of the thumbnails of images without alpha.",
			Chapter: chapter12,
		},
		{
			Name: keyTicketTTL, Kind: config.KindDuration, Default: "5m", Min: "1s",
			Doc:     "Validity of a download ticket (createFileTicket).",
			Chapter: chapter12,
		},
		{
			Name: keyTicketKeyRef, Kind: config.KindSecretRef, Default: "file:<secrets.dir>/file_ticket_key",
			Doc:     "Secret reference of the HMAC key of the download tickets; 32 bytes as hex. Roles api and all do not start without it.",
			Chapter: chapter12,
		},
		{
			Name: "file.not_ready_retry_after", Kind: config.KindDuration, Default: "5s", Min: "1s", Max: "1h",
			Doc:     "Retry-After of the 409 file_not_ready responses, in whole seconds.",
			Chapter: chapter12,
		},
		{
			Name: "client.cache_max_age", Kind: config.KindDuration, Default: sessionMaxAgeDefault, Min: "1s",
			Doc:     "Age after which clients drop unused cache entries; the max-age of an authenticated file download.",
			Chapter: chapter04,
		},
		{
			Name: "file.s3.access_key_ref", Kind: config.KindSecretRef, Default: "file:<secrets.dir>/s3_access_key",
			Doc:     "Secret reference of the S3 access key ID, read only with the driver s3.",
			Chapter: chapter12,
		},
		{
			Name: "file.s3.secret_key_ref", Kind: config.KindSecretRef, Default: "file:<secrets.dir>/s3_secret_key",
			Doc:     "Secret reference of the S3 secret access key, read only with the driver s3.",
			Chapter: chapter12,
		},
	}
}
