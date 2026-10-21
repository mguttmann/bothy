// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	fileschannels "github.com/OWNER/bothy/services/server/internal/files/adapters/channels"
	filesfs "github.com/OWNER/bothy/services/server/internal/files/adapters/fs"
	fileshttp "github.com/OWNER/bothy/services/server/internal/files/adapters/httpapi"
	filesmimetype "github.com/OWNER/bothy/services/server/internal/files/adapters/mimetype"
	filesmetrics "github.com/OWNER/bothy/services/server/internal/files/adapters/otelmetrics"
	filespg "github.com/OWNER/bothy/services/server/internal/files/adapters/postgres"
	filess3 "github.com/OWNER/bothy/services/server/internal/files/adapters/s3"
	filesunicode "github.com/OWNER/bothy/services/server/internal/files/adapters/unicode"
	filesapp "github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
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
// file.max_upload_bytes and deps (docs/DECISIONS.md M3-02/14). Every role opens it in the start
// step files (M3-03); the server image holds the directory file.fs.root of the driver fs.
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

// The S3 limits of the part plan (docs/spec/12-files.md 3.1, source Q1): at most 10,000 parts
// per multipart upload and at least 5 MiB per part except the last. They are protocol constants,
// not settings.
const (
	s3MaxParts        = 10000
	s3MinPartSizeByte = 5 << 20
)

// uploadPartPlan is the cross-check upload_part_plan (D-M3-03-19): a file of file.max_upload_bytes
// in parts of file.upload_part_bytes has at most the S3 part-count limit of parts, so the same plan
// works on both drivers.
func uploadPartPlan(v *config.Values) error {
	maxBytes, part := v.Int(keyMaxUploadBytes), v.Int(keyUploadPartBytes)
	if part < 1 || (maxBytes+part-1)/part > s3MaxParts {
		return errors.New("file.max_upload_bytes divided by file.upload_part_bytes must not exceed the S3 multipart part-count limit")
	}
	return nil
}

// uploadPartSizeS3 is the cross-check upload_part_size_s3 (D-M3-03-19): with the driver s3,
// file.upload_part_bytes is not below the S3 minimum part size, else every upload of more than one
// part would fail at CompleteMultipart.
func uploadPartSizeS3(v *config.Values) error {
	if v.String(keyStorageDriver) != driverS3 || v.Int(keyUploadPartBytes) >= s3MinPartSizeByte {
		return nil
	}
	return errors.New("file.upload_part_bytes must not be below the S3 minimum part size when file.storage.driver is s3")
}

// presignedModeUnavailable is the cross-check presigned_mode_unavailable (D-M3-03-19): the mode
// presigned is not built and stays off until the proofs of 12 3.3 exist (R-FI-02, RK-42).
func presignedModeUnavailable(v *config.Values) error {
	if v.Bool(keyPresignedUpload) {
		return errors.New("file.s3.presigned_upload must be false: the upload mode presigned is not available in this server version")
	}
	return nil
}

// FilesConfig binds the keys of the upload use cases (file.max_upload_bytes,
// file.upload_part_bytes, file.upload_session_ttl, file.upload_max_open_sessions,
// file.quota_bytes_per_principal, file.name_max_bytes, file.denied_mime_types,
// file.fs.min_free_bytes). The error names the key whose binding failed, never a value.
func FilesConfig(v *config.Values) (filesapp.Config, error) {
	var c filesapp.Config
	if err := config.Bind(v, &c); err != nil {
		return filesapp.Config{}, err
	}
	return c, nil
}

// Files is the files module of the process (M3-03): the upload use cases over the object store of
// the driver and their REST handlers.
type Files struct {
	// Service implements files.Uploads and files.Files.
	Service *filesapp.Service
	// Handlers are the REST handlers.
	Handlers *fileshttp.Files
	// store is the object store; Close releases it.
	store filesapp.ObjectStore
}

// FilesWrites are the write ports of other modules the files module receives: the appender of
// sync for file.updated, and the metrics (nil means bothy.file.upload.size of
// files/adapters/otelmetrics on the global meter provider of platform/otel).
type FilesWrites struct {
	Events  eventlog.Appender
	Metrics filesapp.Metrics
}

// NewFiles builds the files module over the object store of driver (file.storage.driver): the part
// strategy fs with the free space of the file system driver, or s3 (docs/DECISIONS.md D-M3-03-2),
// the repository over pool, the unit of work, the Authorizer, the write ports, the structure port
// of chat for the scope of file.updated, the clock (nil means clock.System()) and log (nil means
// slog.Default()). It opens no connection. Errors: a missing collaborator, a limit the use cases
// refuse, an unknown driver or the histogram.
func NewFiles(cfg filesapp.Config, driver string, store filesapp.ObjectStore, pool *pg.Pool, uow txn.UnitOfWork, az authorize.Authorizer,
	w FilesWrites, structure chat.Structure, clk clock.Clock, log *slog.Logger) (*Files, error) {
	if store == nil || az == nil || structure == nil || w.Events == nil {
		return nil, errors.New("wire: the files module needs the object store, the Authorizer, the appender and the structure of chat")
	}
	if w.Metrics == nil {
		m, err := filesmetrics.New(nil)
		if err != nil {
			return nil, err
		}
		w.Metrics = m
	}
	repo := filespg.NewRepository(pool)
	var parts filesapp.PartStore
	var free filesapp.FreeSpace
	switch driver {
	case driverFS:
		parts = filesapp.NewFSPartStore(store)
		if fs, ok := store.(filesapp.FreeSpace); ok {
			free = fs
		}
	case driverS3:
		parts = filesapp.NewS3PartStore(store, repo, uow)
	default:
		return nil, errors.New("config: file.storage.driver: unknown driver")
	}
	svc, err := filesapp.New(cfg, filesapp.Deps{
		Repo: repo, Parts: parts, UoW: uow, Authorizer: az, Events: w.Events, Channels: fileschannels.Locator{Structure: structure},
		Names: filesunicode.Normalizer{}, Types: filesmimetype.Extensions{}, FreeSpace: free, Metrics: w.Metrics, Clock: clk, Log: log,
	})
	if err != nil {
		return nil, err
	}
	return &Files{Service: svc, Handlers: fileshttp.New(svc, svc), store: store}, nil
}

// Close releases the object store (the directory handle of the driver fs); nil-safe.
func (f *Files) Close() error {
	if f == nil {
		return nil
	}
	if c, ok := f.store.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// fillHandlers sets the nil files handler of h to the one f built; a nil f leaves h unchanged.
func (f *Files) fillHandlers(h *Handlers) {
	if f != nil && h.Files == nil {
		h.Files = f.Handlers
	}
}
