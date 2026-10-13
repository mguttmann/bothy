// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// filesConfigKeys returns the keys of the object storage from docs/spec/12-files.md
// "18. Konfigurationswerte dieses Kapitels" that FilesObjectStore reads. The cross-check
// file_fs_root_absolute holds file.fs.root to an absolute, clean path when the driver is fs.
// file.fs.min_free_bytes enters the schema with the upload check of M3-03, the keys file.s3.*
// with the S3 driver of M3-02.
func filesConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "file.storage.driver", Kind: config.KindEnum, Default: "fs", Enum: []string{"fs", "s3"},
			Doc:     "Driver of the object storage of files: fs for a directory of the server, s3 for an S3-compatible service.",
			Chapter: chapter12,
		},
		{
			Name: "file.fs.root", Kind: config.KindString, Default: "/var/lib/bothy/files",
			Doc:     "Root directory of the file system driver, used only by the server and owned by its user with mode 0700.",
			Chapter: chapter12,
		},
	}
}
