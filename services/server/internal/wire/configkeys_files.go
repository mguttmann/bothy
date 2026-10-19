// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// keyStorageDriver selects the driver of the object storage; the checks file_fs_root_absolute,
// file_s3 and upload_part_size_s3 and FilesObjectStore read it.
const keyStorageDriver = "file.storage.driver"

// The keys of the upload checks of M3-03 (docs/DECISIONS.md D-M3-03-19).
const (
	keyMaxUploadBytes  = "file.max_upload_bytes"
	keyUploadPartBytes = "file.upload_part_bytes"
	keyPresignedUpload = "file.s3.presigned_upload"
)

// filesConfigKeys returns the keys of the object storage from docs/spec/12-files.md
// "18. Konfigurationswerte dieses Kapitels" that FilesObjectStore reads. The cross-check
// file_fs_root_absolute holds file.fs.root to an absolute, clean path when the driver is fs, the
// cross-check file_s3 the keys file.s3.* when the driver is s3 (docs/DECISIONS.md M3-02/11).
// The keys of the upload sessions (M3-03) follow: the reserve of the file system driver, the part
// size, the session lifetime, the open sessions and the quota per principal, the name length, the
// denied types, and file.s3.presigned_upload, which the check presigned_mode_unavailable holds
// false because the mode is not built (12 3.3). file.s3.presign_ttl enters with the presigned
// mode, its consumer (02 "13" item 5).
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
