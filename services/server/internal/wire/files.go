// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	filesfs "github.com/OWNER/bothy/services/server/internal/files/adapters/fs"
	filess3 "github.com/OWNER/bothy/services/server/internal/files/adapters/s3"
	filesapp "github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// The values of file.storage.driver.
const (
	driverFS = "fs"
	driverS3 = "s3"
)

// filesStorageValues are the keys of the object storage as config.Bind fills them.
type filesStorageValues struct {
	Driver          string        `config:"file.storage.driver"`
	FSRoot          string        `config:"file.fs.root"`
	TransferTimeout time.Duration `config:"http.transfer_timeout"`
	MaxUploadBytes  int64         `config:"file.max_upload_bytes"`
}

// FilesDeps are what the S3 driver needs beyond the loaded values; the driver fs needs none.
type FilesDeps struct {
	// Secrets resolves file.s3.access_key_ref and file.s3.secret_key_ref: the provider of the
	// process (NewSecretsProvider).
	Secrets filess3.SecretResolver
	// Egress are the resolver, roots, logger and metrics of the egress client of the driver,
	// the same as every other egress consumer of the process gets.
	Egress egress.Deps
}

// FilesObjectStore opens the object storage of the files module from the loaded values: the
// driver fs gives the file system driver on file.fs.root (fs.Open checks the root), the driver s3
// the S3 driver with the keys file.s3.*, egress.* (EgressConfig), http.transfer_timeout,
// file.max_upload_bytes and deps (docs/DECISIONS.md M3-02/14). No role calls it yet: M3-03 opens
// the store in the roles together with the directory in the server image, because the image has
// no writable file.fs.root so far and opening it at start would stop the stack.
//
// The error reports a schema without the keys (config.Bind), a root that fs.Open refuses, or a
// configuration, secret or egress setting that s3.New refuses (a key file that cannot be read
// stops the start); it names keys, never values. A store of the driver fs holds a directory
// handle and implements io.Closer. The S3 driver sends no request here.
func FilesObjectStore(ctx context.Context, v *config.Values, deps FilesDeps) (filesapp.ObjectStore, error) {
	var raw filesStorageValues
	if err := config.Bind(v, &raw); err != nil {
		return nil, err
	}
	if raw.Driver == driverFS {
		store, err := filesfs.Open(filesfs.Config{Root: raw.FSRoot})
		if err != nil {
			return nil, err // never a typed nil in the interface
		}
		return store, nil
	}
	var cfg filess3.Config
	if err := config.Bind(v, &cfg); err != nil {
		return nil, err
	}
	eg, err := EgressConfig(v)
	if err != nil {
		return nil, err
	}
	if deps.Secrets == nil {
		return nil, errors.New("config: file.s3.access_key_ref: no secret provider to resolve it")
	}
	store, err := filess3.New(ctx, cfg, filess3.Deps{
		Egress:          eg,
		EgressDeps:      deps.Egress,
		Secrets:         deps.Secrets,
		TransferTimeout: raw.TransferTimeout,
		MaxObjectBytes:  raw.MaxUploadBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return store, nil
}

// fileFSRootAbsolute is the cross-check file_fs_root_absolute (docs/DECISIONS.md M3-01/12): with
// the driver fs, file.fs.root is absolute and clean, because a relative root would depend on the
// working directory of the process. With s3 the root is unused and not checked.
func fileFSRootAbsolute(v *config.Values) error {
	if v.String(keyStorageDriver) != driverFS {
		return nil
	}
	root := v.String("file.fs.root")
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return errors.New("file.fs.root must be an absolute path without . or .. elements and without a trailing slash when file.storage.driver is fs")
	}
	return nil
}

// fileS3 is the cross-check file_s3 (docs/DECISIONS.md M3-02/11): with the driver s3 it fills
// s3.Config from the six non-secret keys file.s3.* and runs Config.Validate against the parsed
// entries of egress.allowed_hosts, so the rules live in one place and an http endpoint without an
// allow_private entry fails at load instead of at the first request. A check sees only the keys
// it names, so the two references stay out (they are read only by the secret provider). With fs
// no file.s3.* value is read. Entries that do not parse are the problem of the check
// egress_allowed_hosts and are not reported twice.
func fileS3(v *config.Values) error {
	if v.String(keyStorageDriver) != driverS3 {
		return nil
	}
	cfg := filess3.Config{
		Endpoint:     v.String("file.s3.endpoint"),
		Region:       v.String("file.s3.region"),
		Bucket:       v.String("file.s3.bucket"),
		Prefix:       v.String("file.s3.prefix"),
		UsePathStyle: v.Bool("file.s3.use_path_style"),
		ChecksumMode: v.String("file.s3.checksum_mode"),
	}
	rules, err := egressRules(v.Strings(keyEgressAllowedHosts))
	if err != nil {
		return nil //nolint:nilerr // reported by egress_allowed_hosts
	}
	return cfg.Validate(rules)
}
