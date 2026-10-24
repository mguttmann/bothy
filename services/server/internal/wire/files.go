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
	"github.com/OWNER/bothy/services/server/internal/files"
	fileschannels "github.com/OWNER/bothy/services/server/internal/files/adapters/channels"
	filesclamd "github.com/OWNER/bothy/services/server/internal/files/adapters/clamd"
	filesextract "github.com/OWNER/bothy/services/server/internal/files/adapters/extract"
	filesfs "github.com/OWNER/bothy/services/server/internal/files/adapters/fs"
	fileshttp "github.com/OWNER/bothy/services/server/internal/files/adapters/httpapi"
	filesimaging "github.com/OWNER/bothy/services/server/internal/files/adapters/imaging"
	filesisobmff "github.com/OWNER/bothy/services/server/internal/files/adapters/isobmff"
	filesjobs "github.com/OWNER/bothy/services/server/internal/files/adapters/jobs"
	filesmimetype "github.com/OWNER/bothy/services/server/internal/files/adapters/mimetype"
	filesmetrics "github.com/OWNER/bothy/services/server/internal/files/adapters/otelmetrics"
	filespg "github.com/OWNER/bothy/services/server/internal/files/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/profileimages"
	filess3 "github.com/OWNER/bothy/services/server/internal/files/adapters/s3"
	filesunicode "github.com/OWNER/bothy/services/server/internal/files/adapters/unicode"
	filesapp "github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
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
// the driver and their REST handlers; in the roles worker and all also the scan pass and its
// health monitor (M3-04, EnableScan).
type Files struct {
	// Service implements files.Uploads and files.Files.
	Service *filesapp.Service
	// Handlers are the REST handlers.
	Handlers *fileshttp.Files
	// Queue inserts files.scan; BindJobs gives it the inserter of the job runtime.
	Queue *filesjobs.Queue
	// Pass is the scan pass and Health its monitor; nil until EnableScan (role api).
	Pass   *filesapp.ScanPass
	Health *filesapp.ScanHealth
	// Images is the use case of files.thumbnail (M3-06); nil until EnableImages (role api).
	Images *filesapp.ImageProcessor
	// ExtractQueue inserts files.extract_text and files.extract_text_refresh (M3-15); BindJobs
	// gives it the inserter of the job runtime.
	ExtractQueue *filesjobs.ExtractQueue
	// Extraction are the use cases of the text extraction (M3-15); nil until EnableExtraction
	// (role api).
	Extraction *filesapp.Extraction
	// store is the object store; Close releases it.
	store filesapp.ObjectStore
	// The collaborators the scan pass shares with the upload use cases.
	repo        *filespg.Repository
	uow         txn.UnitOfWork
	events      eventlog.Appender
	channels    filesapp.ChannelLocator
	scanMetrics filesapp.ScanMetrics
}

// FilesWrites are the write ports of other modules the files module receives: the appender of
// sync for file.updated, and the metrics (nil means the instruments of files/adapters/otelmetrics
// on the global meter provider of platform/otel; a Metrics that also implements
// filesapp.ScanMetrics records the scan signals, any other records none).
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
	scanMetrics, _ := w.Metrics.(filesapp.ScanMetrics)
	repo := filespg.NewRepository(pool)
	queue := &filesjobs.Queue{}
	channels := fileschannels.Locator{Structure: structure}
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
		Repo: repo, Parts: parts, UoW: uow, Authorizer: az, Events: w.Events, Channels: channels,
		Names: filesunicode.Normalizer{}, Types: filesmimetype.Extensions{}, FreeSpace: free, ScanQueue: queue, Metrics: w.Metrics,
		Store: store, Clock: clk, Log: log,
	})
	if err != nil {
		return nil, err
	}
	return &Files{Service: svc, Handlers: fileshttp.New(svc, svc), Queue: queue, ExtractQueue: &filesjobs.ExtractQueue{}, store: store,
		repo: repo, uow: uow, events: w.Events, channels: channels, scanMetrics: scanMetrics}, nil
}

// ScanConfigs binds the keys of the scan pass (file.scan.mode, file.scan.timeout,
// file.scan.health_interval, file.scan.retry_backoff_cap, file.sniff_bytes,
// file.denied_mime_types) and of the clamd client (file.scan.clamd_address,
// file.scan.chunk_bytes, file.scan.timeout, file.scan.max_concurrent). The error names the key
// whose binding failed, never a value.
func ScanConfigs(v *config.Values) (filesapp.ScanConfig, filesclamd.Config, error) {
	var c filesapp.ScanConfig
	if err := config.Bind(v, &c); err != nil {
		return filesapp.ScanConfig{}, filesclamd.Config{}, err
	}
	var cc filesclamd.Config
	if err := config.Bind(v, &cc); err != nil {
		return filesapp.ScanConfig{}, filesclamd.Config{}, err
	}
	return c, cc, nil
}

// ExtractConfigs binds the keys of the text extraction (search.extract.max_input_bytes,
// search.extract.timeout, search.extract.refresh_batch_size for the use cases;
// search.extract.max_text_bytes, search.extract.max_uncompressed_bytes and
// search.extract.max_archive_entries for the adapter). The error names the key whose binding
// failed, never a value.
func ExtractConfigs(v *config.Values) (filesapp.ExtractConfig, filesextract.Config, error) {
	var c filesapp.ExtractConfig
	if err := config.Bind(v, &c); err != nil {
		return filesapp.ExtractConfig{}, filesextract.Config{}, err
	}
	var xc filesextract.Config
	if err := config.Bind(v, &xc); err != nil {
		return filesapp.ExtractConfig{}, filesextract.Config{}, err
	}
	return c, xc, nil
}

// EnableExtraction builds the text extraction (roles worker and all, M3-15): the pure Go
// adapters of adapters/extract with xc, the use cases over the repository, the object store and
// the unit of work of the module, the extraction queue, the search language of the instance
// (identity.Directory), the clock (nil means clock.System()) and log (nil means slog.Default()).
// EnableScan and EnableImages take its trigger, so it runs before them. Errors: the adapter
// refuses xc or the use cases refuse cfg.
func (f *Files) EnableExtraction(cfg filesapp.ExtractConfig, xc filesextract.Config, langs filesapp.SearchLanguages, clk clock.Clock,
	log *slog.Logger) error {
	reg, err := filesextract.New(xc)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	ext, err := filesapp.NewExtraction(cfg, filesapp.ExtractDeps{Repo: f.repo, Store: f.store, UoW: f.uow, Extractor: reg,
		Queue: f.ExtractQueue, Languages: langs, Clock: clk, Log: log})
	if err != nil {
		return err
	}
	f.Extraction = ext
	return nil
}

// trigger returns the trigger of the text extraction; nil before EnableExtraction, which the scan
// pass and the image job refuse.
func (f *Files) trigger() *filesapp.ExtractTrigger {
	if f.Extraction == nil {
		return nil
	}
	return f.Extraction.Trigger()
}

// ExtractRunners returns the use cases of files.extract_text and files.extract_text_refresh for
// the job registry: the use cases in a role that runs jobs, nil otherwise.
func (f *Files) ExtractRunners() (filesjobs.TextRunner, filesjobs.RefreshRunner) {
	if f == nil || f.Extraction == nil {
		return nil, nil
	}
	return f.Extraction.Text, f.Extraction.Refresh
}

// EnableScan builds the scan pass and its health monitor (roles worker and all, M3-04): the clamd
// client of cc with file.scan.mode = required (none with off), the type detection of
// adapters/mimetype, the audit recorder for the malware entry, the clock (nil means
// clock.System()) and log (nil means slog.Default()). It opens no connection. Errors: the
// client refuses cc, or the pass refuses cfg.
func (f *Files) EnableScan(cfg filesapp.ScanConfig, cc filesclamd.Config, audit auditing.Recorder, clk clock.Clock, log *slog.Logger) error {
	var scanner filesapp.Scanner
	if cfg.Mode == filesapp.ScanModeRequired {
		c, err := filesclamd.New(cc)
		if err != nil {
			return fmt.Errorf("config: %w", err)
		}
		scanner = c
	}
	pass, err := filesapp.NewScanPass(cfg, filesapp.ScanDeps{
		Repo: f.repo, Scans: f.repo, Store: f.store, UoW: f.uow, Events: f.events, Audit: audit, Channels: f.channels,
		Scanner: scanner, Types: filesmimetype.NewDetector(cfg.SniffBytes), Queue: f.Queue, Extracts: f.trigger(), Metrics: f.scanMetrics,
		Clock: clk, Log: log,
	})
	if err != nil {
		return err
	}
	health, err := filesapp.NewScanHealth(cfg, scanner, f.repo, f.scanMetrics, clk, log)
	if err != nil {
		return err
	}
	f.Pass, f.Health = pass, health
	return nil
}

// ImageConfig binds the keys of the job files.thumbnail (file.image.max_pixels,
// file.image.jpeg_quality, file.image.processing_timeout, file.image.max_concurrent and
// file.thumbnail.*). The error names the key whose binding failed, never a value.
func ImageConfig(v *config.Values) (filesapp.ImageConfig, error) {
	var c filesapp.ImageConfig
	if err := config.Bind(v, &c); err != nil {
		return filesapp.ImageConfig{}, err
	}
	return c, nil
}

// EnableImages builds the use case of files.thumbnail (roles worker and all, M3-06): the pure Go
// codec of adapters/imaging, the box reader of adapters/isobmff, the per-process limit
// file.image.max_concurrent, log (nil means slog.Default()) and the test seams hooks (nil in
// production, ADR 0071). Errors: the use case refuses cfg.
func (f *Files) EnableImages(cfg filesapp.ImageConfig, log *slog.Logger, hooks *filesapp.ImageHooks) error {
	p, err := filesapp.NewImageProcessor(cfg, filesapp.ImageDeps{
		Repo: f.repo, Store: f.store, UoW: f.uow, Events: f.events, Channels: f.channels, Codec: filesimaging.Codec{},
		Movies: filesisobmff.Reader{}, Extracts: f.trigger(), Log: log, Hooks: hooks,
	})
	if err != nil {
		return err
	}
	f.Images = p
	return nil
}

// ImageRunner returns the use case of files.thumbnail for the job registry: the processor in a
// role that runs jobs, nil otherwise (the api role never runs the job).
func (f *Files) ImageRunner() filesjobs.ImageRunner {
	if f == nil || f.Images == nil {
		return nil
	}
	return f.Images
}

// thumbnailSizesOrdered is the cross-check thumbnail_sizes_ordered (docs/DECISIONS.md D-M3-06-11):
// file.thumbnail.small_px is below file.thumbnail.medium_px, which is below
// file.thumbnail.large_px, so the three variants of 12 7.3 differ.
func thumbnailSizesOrdered(v *config.Values) error {
	if v.Int(keyThumbnailSmallPx) < v.Int(keyThumbnailMediumPx) && v.Int(keyThumbnailMediumPx) < v.Int(keyThumbnailLargePx) {
		return nil
	}
	return errors.New(keyThumbnailSmallPx + " must be below " + keyThumbnailMediumPx + ", and that below " + keyThumbnailLargePx)
}

// ScanRunner returns the use case of files.scan for the job registry: the pass in a role that
// runs jobs, nil otherwise (the api role only inserts the job).
func (f *Files) ScanRunner() filesjobs.Runner {
	if f == nil || f.Pass == nil {
		return nil
	}
	return f.Pass
}

// ScannerHealth returns the health of the virus scanner as this worker sees it (12 5.3); nil in
// the role api. M7-09 reads it for the status component.
func (f *Files) ScannerHealth() files.ScannerHealth {
	if f == nil || f.Health == nil {
		return nil
	}
	return f.Health
}

// BindJobs gives the scan queue and the extraction queue the inserter of the job runtime of the
// role; nil-safe.
func (f *Files) BindJobs(ins jobs.Inserter) {
	if f != nil {
		f.Queue.Bind(ins)
		f.ExtractQueue.Bind(ins)
	}
}

// scanStreamLimit is the cross-check scan_stream_limit (12 5.4, ADR 0033 Confirmation): with
// file.scan.mode = required, file.max_upload_bytes must not exceed file.scan.clamd_stream_max_bytes,
// the mirror of the clamd option StreamMaxLength, else clamd would cut every large upload off.
func scanStreamLimit(v *config.Values) error {
	if v.String(keyScanMode) != filesapp.ScanModeRequired || v.Int(keyMaxUploadBytes) <= v.Int(keyClamdStreamMaxByte) {
		return nil
	}
	return errors.New("file.max_upload_bytes must not exceed file.scan.clamd_stream_max_bytes when file.scan.mode is required")
}

// clamdAddressUnix is the cross-check clamd_address_unix (19 SEC-TLS-5): file.scan.clamd_address
// is unix:// followed by an absolute path, the rule of the clamd client (clamd.SocketPath).
func clamdAddressUnix(v *config.Values) error {
	_, err := filesclamd.SocketPath(v.String(keyClamdAddress))
	return err
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

// BindProfileImages binds the avatar and logo use cases of f into identity and instance through
// their source ports identity.AvatarFiles and instance.LogoFiles (M3-10, adapters/profileimages);
// files starts after both, so they take the ports late. A nil module is skipped.
func (f *Files) BindProfileImages(id *Identity, st *InstanceSettings) {
	if f == nil {
		return
	}
	if id != nil && id.People != nil {
		id.People.BindAvatarFiles(profileimages.Avatar{Images: f.Service})
	}
	if st != nil && st.Admin != nil {
		st.Admin.BindLogoFiles(profileimages.Logo{Images: f.Service})
	}
}

// fillHandlers sets the nil files handler of h to the one f built; a nil f leaves h unchanged.
func (f *Files) fillHandlers(h *Handlers) {
	if f != nil && h.Files == nil {
		h.Files = f.Handlers
	}
}
