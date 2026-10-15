// SPDX-License-Identifier: Apache-2.0

package s3sink

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/logging"

	"github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// objectAPI is the only part of the S3 SDK client the sink holds: the four operations it needs
// (docs/spec/12-dateien.md 4.3, ADR 0048). Deleting, copying, bucket changes and retention
// changes are therefore unreachable from the sink; TestAPIMethodSet holds the method set.
// *awss3.Client implements it.
type objectAPI interface {
	PutObject(ctx context.Context, params *awss3.PutObjectInput, optFns ...func(*awss3.Options)) (*awss3.PutObjectOutput, error)
	GetObject(ctx context.Context, params *awss3.GetObjectInput, optFns ...func(*awss3.Options)) (*awss3.GetObjectOutput, error)
	ListObjectVersions(ctx context.Context, params *awss3.ListObjectVersionsInput, optFns ...func(*awss3.Options)) (*awss3.ListObjectVersionsOutput, error)
	GetObjectLockConfiguration(ctx context.Context, params *awss3.GetObjectLockConfigurationInput, optFns ...func(*awss3.Options)) (*awss3.GetObjectLockConfigurationOutput, error)
}

var _ objectAPI = (*awss3.Client)(nil)

// egressPurpose is the label of the egress client of the sink in logs (bothy.egress.purpose).
const egressPurpose = "audit_checkpoint"

// seqDigits is the width of the seq in an object name: 20 digits hold every int64, so the
// lexicographic order of the keys is the seq order (as the file sink).
const seqDigits = 20

// defaultPageSize is the page size of ListObjectVersions, the S3 maximum.
const defaultPageSize = 1000

// maxCheckpointBytes bounds the body of one checkpoint the sink reads: a compact JWS over the
// small payload of 19 SEC-AUD-7 is a few hundred bytes.
const maxCheckpointBytes = 64 << 10

// objectName matches the name of a checkpoint object below the directory of the instance.
var objectName = regexp.MustCompile(`^[0-9]{20}\.jws$`)

// SecretResolver resolves a secret reference such as "file:<secrets.dir>/audit_checkpoint_s3_access_key";
// *secrets.Provider implements it.
type SecretResolver interface {
	Resolve(ctx context.Context, raw string) (secrets.Value, secrets.Version, error)
}

// Deps are the dependencies of the sink; internal/wire fills them.
type Deps struct {
	// Egress is the configuration egress.* (wire.EgressConfig); every request of the sink goes
	// through one egress client built from it, with its time and size limits (19 SEC-NET-1).
	Egress egress.Config
	// EgressDeps are the resolver, roots, logger and metrics of that client.
	EgressDeps egress.Deps
	// Secrets resolves audit.checkpoint_s3_access_key_ref and audit.checkpoint_s3_secret_key_ref.
	Secrets SecretResolver
}

// Sink is the object store implementation of app.CheckpointSink, safe for concurrent use. It
// holds only objectAPI, so it can send nothing but the four operations of the interface.
type Sink struct {
	api      objectAPI
	bucket   string
	dir      string // "[<prefix>/]<instance_id>/"
	pageSize int32

	// bodies caches the body of every version read, by key and version ID. A version of an
	// object never changes, so the job audit.chain_verify, which lists the sink on every run,
	// reads each checkpoint once per process instead of once per run (docs/DECISIONS.md M7-22/7).
	mu     sync.Mutex
	bodies map[versionRef][]byte
}

// versionRef names one version of one object.
type versionRef struct{ key, version string }

// readParallel bounds the concurrent GETs of one List: a cold List of a year of hourly
// checkpoints from a remote store then takes seconds, not the minutes of sequential reads.
const readParallel = 16

// Compile-time check: Sink is the outgoing port of app.
var _ app.CheckpointSink = (*Sink)(nil)

// New builds the sink for the checkpoints of instance (docs/DECISIONS.md M7-22/4), in this order:
// cfg.Validate against deps.Egress.AllowedHosts; both secret references resolved once, so a key
// that cannot be read stops the start; one egress client with purpose audit_checkpoint, the host
// policy of cfg and no redirects; then the SDK client built by hand: region, the credentials of
// the secret provider (read again on every request), the HTTP client of the egress client, no
// retries, checksums only where an operation requires them, no SDK log, the endpoint of cfg and
// its path style. It never loads the default configuration or credential chain of the SDK and
// sends no request.
//
// The error names keys, never values: a Config that Validate refuses, a secret reference that
// cannot be resolved, or Deps the egress client refuses.
//
// Example:
//
//	sink, err := s3sink.New(ctx, s3sink.Config{
//		Bucket: "bothy-audit", Endpoint: "https://s3.example.com", Region: "us-east-1", UsePathStyle: true,
//		AccessKeyRef: "file:<secrets.dir>/audit_checkpoint_s3_access_key",
//		SecretKeyRef: "file:<secrets.dir>/audit_checkpoint_s3_secret_key",
//	}, instanceID, s3sink.Deps{Egress: egressCfg, Secrets: provider})
func New(ctx context.Context, cfg Config, instance ids.ID, deps Deps) (*Sink, error) {
	if err := cfg.Validate(deps.Egress.AllowedHosts); err != nil {
		return nil, err
	}
	if deps.Secrets == nil {
		return nil, errors.New(keyAccessRef + ": no secret provider")
	}
	creds := credentials{resolver: deps.Secrets, accessRef: cfg.AccessKeyRef, secretRef: cfg.SecretKeyRef}
	if _, err := creds.Retrieve(ctx); err != nil {
		return nil, err
	}
	client, err := egress.New(deps.Egress, egress.Options{
		Purpose: egressPurpose, Hosts: cfg.HostPolicy(), Redirects: egress.RedirectNever,
	}, deps.EgressDeps)
	if err != nil {
		return nil, fmt.Errorf("%s: egress client: %w", keySink, err)
	}
	// awss3.New instead of NewFromConfig: NewFromConfig looks at AWS_ENDPOINT_URL and related
	// variables of the environment; New takes only these options.
	api := awss3.New(awss3.Options{
		Region:                      cfg.Region,
		BaseEndpoint:                aws.String(strings.TrimSuffix(cfg.Endpoint, "/")),
		UsePathStyle:                cfg.UsePathStyle,
		Credentials:                 creds,
		HTTPClient:                  client.HTTPClient(),
		Retryer:                     aws.NopRetryer{},
		RequestChecksumCalculation:  aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation:  aws.ResponseChecksumValidationWhenRequired,
		DisableS3ExpressSessionAuth: aws.Bool(true),
		Logger:                      logging.Nop{},
	})
	dir := instance.String() + "/"
	if cfg.Prefix != "" {
		dir = cfg.Prefix + "/" + dir
	}
	return &Sink{api: api, bucket: cfg.Bucket, dir: dir, pageSize: defaultPageSize, bodies: map[versionRef][]byte{}}, nil
}

// Configured reports true: an object store sink is a sink.
func (s *Sink) Configured() bool { return true }

// key returns the object key of seq.
func (s *Sink) key(seq int64) string {
	return fmt.Sprintf("%s%0*d.jws", s.dir, seqDigits, seq)
}

// errOtherContent is the error of a Put whose seq holds other content.
var errOtherContent = fmt.Errorf("audit/s3sink: %w", app.ErrCheckpointConflict)

// Put stores c once (app.CheckpointSink, 19 SEC-AUD-2, docs/DECISIONS.md M7-22/3). It first
// reads the oldest version of the key: the same bytes there count as stored, other bytes are
// ErrCheckpointConflict. Otherwise it writes with If-None-Match: *, so the service refuses an
// existing key; on that refusal (a concurrent writer) the stored bytes decide the same way. On a
// service that ignores the header a second writer only adds a version on the locked bucket, and
// List keeps reading the oldest one.
//
// Errors: a seq below 1, ErrCheckpointConflict, and storage failures with the operation and the
// S3 code or HTTP status, never the endpoint or credentials.
func (s *Sink) Put(ctx context.Context, c app.SignedCheckpoint) error {
	if c.Seq < 1 {
		return errors.New("audit/s3sink: a checkpoint has a seq of at least 1")
	}
	key := s.key(c.Seq)
	if err := s.compare(ctx, key, c.JWS); !errors.Is(err, errNotStored) {
		return err
	}
	_, err := s.api.PutObject(ctx, &awss3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(c.JWS),
		ContentLength: aws.Int64(int64(len(c.JWS))),
		ContentType:   aws.String("application/jose"),
		IfNoneMatch:   aws.String("*"),
		// A bucket with a default retention requires an integrity header on every upload.
		ChecksumAlgorithm: types.ChecksumAlgorithmSha256,
	})
	if err == nil {
		return nil
	}
	if code, status := apiStatus(err); code == "PreconditionFailed" || code == "ConditionalRequestConflict" || status == 412 || status == 409 {
		// Another writer created the key first; its content decides.
		if cerr := s.compare(ctx, key, c.JWS); !errors.Is(cerr, errNotStored) {
			return cerr
		}
	}
	return opError("put", err)
}

// errNotStored is the answer of compare for a key without a version.
var errNotStored = errors.New("audit/s3sink: not stored")

// compare returns nil when the oldest version of key holds want, errOtherContent when it holds
// other bytes, errNotStored when the key has no version, and a storage failure otherwise.
func (s *Sink) compare(ctx context.Context, key string, want []byte) error {
	versions, err := s.versions(ctx, key)
	if err != nil {
		return err
	}
	version, ok := versions[key]
	if !ok {
		return errNotStored
	}
	got, err := s.read(ctx, key, version)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, want) {
		return errOtherContent
	}
	return nil
}

// versions lists every version below prefix, following every page, and returns the oldest
// version ID of every key (S3 lists the versions of a key newest first, so the last one seen
// wins). Delete markers are skipped: they hide a key from a plain GET, never its versions.
func (s *Sink) versions(ctx context.Context, prefix string) (map[string]string, error) {
	out := map[string]string{}
	in := &awss3.ListObjectVersionsInput{Bucket: aws.String(s.bucket), Prefix: aws.String(prefix), MaxKeys: aws.Int32(s.pageSize)}
	for {
		page, err := s.api.ListObjectVersions(ctx, in)
		if err != nil {
			return nil, opError("list", err)
		}
		for _, v := range page.Versions {
			out[aws.ToString(v.Key)] = aws.ToString(v.VersionId)
		}
		if !aws.ToBool(page.IsTruncated) {
			return out, nil
		}
		if page.NextKeyMarker == nil && page.NextVersionIdMarker == nil {
			return nil, errors.New("audit/s3sink: list: a truncated page without a marker")
		}
		in.KeyMarker, in.VersionIdMarker = page.NextKeyMarker, page.NextVersionIdMarker
	}
}

// read returns the body of one version of key, at most maxCheckpointBytes, from the cache when
// it was read before. Only versions with a real version ID are cached: the object of "" or "null"
// (written while versioning was off) can be replaced.
func (s *Sink) read(ctx context.Context, key, version string) ([]byte, error) {
	ref := versionRef{key: key, version: version}
	cacheable := version != "" && version != "null"
	if cacheable {
		s.mu.Lock()
		body, ok := s.bodies[ref]
		s.mu.Unlock()
		if ok {
			return body, nil
		}
	}
	in := &awss3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)}
	if cacheable {
		in.VersionId = aws.String(version)
	}
	out, err := s.api.GetObject(ctx, in)
	if err != nil {
		return nil, opError("get", err)
	}
	defer func() { _ = out.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(out.Body, maxCheckpointBytes+1))
	if err != nil {
		return nil, opError("get", err)
	}
	if len(body) > maxCheckpointBytes {
		return nil, errors.New("audit/s3sink: get: a checkpoint object is larger than a checkpoint")
	}
	if cacheable {
		s.mu.Lock()
		s.bodies[ref] = body
		s.mu.Unlock()
	}
	return body, nil
}

// List returns every checkpoint of the instance in seq order (app.CheckpointSink), the oldest
// version of every key, following every page. Objects outside the directory of the instance and
// names that are no checkpoint are skipped. Bodies not read before are read with at most
// readParallel concurrent GETs. It verifies nothing; verification stays in audit/domain.
// Errors: storage failures, named like those of Put; the first one ends the List.
func (s *Sink) List(ctx context.Context) ([]app.SignedCheckpoint, error) {
	versions, err := s.versions(ctx, s.dir)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(versions))
	for k := range versions {
		if objectName.MatchString(strings.TrimPrefix(k, s.dir)) && strings.HasPrefix(k, s.dir) {
			keys = append(keys, k)
		}
	}
	// Same width, so the lexicographic order is the seq order.
	slices.Sort(keys)
	out := make([]app.SignedCheckpoint, 0, len(keys))
	stored := make([]versionRef, 0, len(keys))
	for _, k := range keys {
		seq, err := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(k, s.dir), ".jws"), 10, 64)
		if err != nil {
			continue // 20 digits above the int64 range: no checkpoint of this server
		}
		out = append(out, app.SignedCheckpoint{Seq: seq})
		stored = append(stored, versionRef{key: k, version: versions[k]})
	}
	if err := s.readAll(ctx, out, stored); err != nil {
		return nil, err
	}
	return out, nil
}

// readAll fills the JWS of out[i] from the version stored[i], with at most readParallel GETs at a
// time; it returns the first error and cancels the other reads.
func (s *Sink) readAll(ctx context.Context, out []app.SignedCheckpoint, stored []versionRef) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		wg    sync.WaitGroup
		once  sync.Once
		first error
		slots = make(chan struct{}, readParallel)
	)
	for i := range out {
		slots <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() { <-slots; wg.Done() }()
			body, err := s.read(ctx, stored[i].key, stored[i].version)
			if err != nil {
				once.Do(func() { first = err; cancel() })
				return
			}
			out[i].JWS = body
		}()
	}
	wg.Wait()
	return first
}

// credentials is the credentials provider of the SDK client. It resolves both references through
// the secret provider on every Retrieve and marks the result as expired at once, so the SDK asks
// again for the next request and a rotated key is used after secrets.cache_ttl without restart.
type credentials struct {
	resolver             SecretResolver
	accessRef, secretRef string
}

// Retrieve implements aws.CredentialsProvider. Errors name the key and the error code of the
// secret provider, never a value or the reference.
func (c credentials) Retrieve(ctx context.Context) (aws.Credentials, error) {
	access, _, err := c.resolver.Resolve(ctx, c.accessRef)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("%s: %s: %w", keyAccessRef, secrets.ErrorCode(err), err)
	}
	secret, _, err := c.resolver.Resolve(ctx, c.secretRef)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("%s: %s: %w", keySecretRef, secrets.ErrorCode(err), err)
	}
	return aws.Credentials{
		AccessKeyID: string(access.Bytes()), SecretAccessKey: string(secret.Bytes()),
		Source: "server secret provider", CanExpire: true, Expires: time.Now(),
	}, nil
}

var _ aws.CredentialsProvider = credentials{}

// apiStatus returns the S3 error code and the HTTP status of err; either may be empty or 0.
func apiStatus(err error) (string, int) {
	code, status := "", 0
	var ae smithy.APIError
	if errors.As(err, &ae) {
		code = ae.ErrorCode()
	}
	var re *awshttp.ResponseError
	if errors.As(err, &re) {
		status = re.HTTPStatusCode()
	}
	return code, status
}

// sinkError is a storage failure of the sink. Its text is "audit/s3sink: <operation>: <detail>",
// where detail is the S3 code, the HTTP status, a fixed text for a known cause or "transport";
// net/http errors carry the request URL, so the text never includes the cause's text. Unwrap
// keeps errors.Is working for the cause.
type sinkError struct {
	op, detail string
	cause      error
}

func (e *sinkError) Error() string { return "audit/s3sink: " + e.op + ": " + e.detail }

func (e *sinkError) Unwrap() error { return e.cause }

// opError wraps an SDK error of op.
func opError(op string, err error) error {
	code, status := apiStatus(err)
	var blocked *egress.BlockedError
	d := "transport"
	switch {
	case code != "" && len(code) <= 64 && strings.Trim(code, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.") == "":
		d = code
		if status != 0 {
			d += " (status " + strconv.Itoa(status) + ")"
		}
	case status != 0:
		d = "status " + strconv.Itoa(status)
	case errors.As(err, &blocked):
		d = "blocked by the egress client (" + string(blocked.Reason) + ")"
	case errors.Is(err, egress.ErrResponseTooLarge):
		d = "response too large"
	case errors.Is(err, context.Canceled):
		d = "context canceled"
	case errors.Is(err, context.DeadlineExceeded):
		d = "deadline exceeded"
	case strings.Contains(err.Error(), keyAccessRef) || strings.Contains(err.Error(), keySecretRef):
		d = "credentials"
	}
	return &sinkError{op: op, detail: d, cause: err}
}
