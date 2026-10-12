// SPDX-License-Identifier: Apache-2.0

package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go/logging"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// egressPurpose is the label of the egress client of the driver in logs (bothy.egress.purpose).
const egressPurpose = "objectstore"

// Deps are the dependencies of the S3 driver; internal/wire fills them.
type Deps struct {
	// Egress is the configuration egress.* (wire.EgressConfig); every request of the driver goes
	// through one egress client built from it (19 SEC-NET-1).
	Egress egress.Config
	// EgressDeps are the resolver, roots, logger and metrics of that client.
	EgressDeps egress.Deps
	// Secrets resolves file.s3.access_key_ref and file.s3.secret_key_ref.
	Secrets SecretResolver
	// TransferTimeout is http.transfer_timeout, the time limit of one request including its
	// body (02 "14.2" row "upload and download").
	TransferTimeout time.Duration
	// MaxObjectBytes is file.max_upload_bytes, the largest body the driver reads from one
	// response; no stored object is larger (03 files.size_bytes, 19 SEC-NET-5).
	MaxObjectBytes int64
}

// Store is the S3 driver of app.ObjectStore (docs/spec/12-files.md "4.3"). It holds only the
// narrow interfaces coreAPI and presignAPI, so it can send nothing but core S3 requests. It is
// safe for concurrent use and keeps no state of its own besides the SDK clients.
type Store struct {
	api     coreAPI
	presign presignAPI
	bucket  string
	prefix  string
	now     func() time.Time
}

var _ app.ObjectStore = (*Store)(nil)

// New builds the S3 driver (docs/DECISIONS.md M3-02/1 to M3-02/3 and M3-02/6), in this order:
// cfg.Validate against deps.Egress.AllowedHosts; both secret references resolved once, so a key
// that cannot be read stops the start; one egress client with purpose objectstore, the host
// policy of cfg, no redirects, deps.TransferTimeout and deps.MaxObjectBytes; then the SDK client
// built by hand: region, the credentials of the secret provider, the HTTP client of the egress
// client, no retries, the checksum mode of cfg, no SDK log, tracer or meter, the endpoint of cfg
// and its path style. It never loads the default configuration of the SDK (environment, shared
// files, instance metadata) and sends no request.
//
// The error names keys, never values: a Config that Validate refuses, a secret reference that
// cannot be resolved (wrapping the error of the provider, for example secrets.ErrInsecureFile), or
// Deps the egress client refuses.
//
// Example:
//
//	store, err := s3.New(ctx, s3.Config{
//		Endpoint: "https://s3.example.com", Region: "us-east-1", Bucket: "bothy-files",
//		UsePathStyle: true, ChecksumMode: s3.ChecksumWhenRequired,
//		AccessKeyRef: "file:<secrets.dir>/s3_access_key", SecretKeyRef: "file:<secrets.dir>/s3_secret_key",
//	}, s3.Deps{Egress: egressCfg, Secrets: provider, TransferTimeout: 10 * time.Minute, MaxObjectBytes: 100 << 20})
func New(ctx context.Context, cfg Config, deps Deps) (*Store, error) {
	return newStore(ctx, cfg, deps, nil)
}

// newStore is New with an optional wrapper around the HTTP client (the recorder of the tests).
func newStore(ctx context.Context, cfg Config, deps Deps, wrap func(httpDoer) httpDoer) (*Store, error) {
	if err := cfg.Validate(deps.Egress.AllowedHosts); err != nil {
		return nil, err
	}
	if deps.Secrets == nil {
		return nil, errors.New("file.s3.access_key_ref: no secret provider")
	}
	creds := secretCredentials{resolver: deps.Secrets, accessRef: cfg.AccessKeyRef, secretRef: cfg.SecretKeyRef, now: time.Now}
	if err := creds.check(ctx); err != nil {
		return nil, err
	}
	client, err := egress.New(deps.Egress, egress.Options{
		Purpose:          egressPurpose,
		Hosts:            cfg.HostPolicy(),
		Redirects:        egress.RedirectNever,
		Timeout:          deps.TransferTimeout,
		MaxResponseBytes: deps.MaxObjectBytes,
	}, deps.EgressDeps)
	if err != nil {
		return nil, fmt.Errorf("file.s3: egress client: %w", err)
	}
	var hc httpDoer = client.HTTPClient()
	if wrap != nil {
		hc = wrap(hc)
	}
	reqMode, respMode, _ := checksumModes(cfg.ChecksumMode) // checked by Validate
	// awss3.New instead of NewFromConfig: NewFromConfig looks at AWS_ENDPOINT_URL and related
	// variables of the environment; New takes only these options.
	api := awss3.New(awss3.Options{
		Region:                      cfg.Region,
		BaseEndpoint:                aws.String(strings.TrimSuffix(cfg.Endpoint, "/")),
		UsePathStyle:                cfg.UsePathStyle,
		Credentials:                 creds,
		HTTPClient:                  hc,
		Retryer:                     aws.NopRetryer{},
		RequestChecksumCalculation:  reqMode,
		ResponseChecksumValidation:  respMode,
		DisableS3ExpressSessionAuth: aws.Bool(true),
		Logger:                      logging.Nop{},
	})
	return &Store{
		api:     api,
		presign: awss3.NewPresignClient(api),
		bucket:  cfg.Bucket,
		prefix:  cfg.Prefix,
		now:     time.Now,
	}, nil
}

// unsignedPayload signs streamed bodies as UNSIGNED-PAYLOAD (docs/DECISIONS.md M3-02/4): the body
// is not seekable and the endpoint may be plain http, where the SDK would otherwise need to hash
// the payload before sending. Integrity comes from the part digest (12 "3.2") and the file
// SHA-256 of the read pass (12 "3.4").
var unsignedPayload = awss3.WithAPIOptions(v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware)

// name maps key to its object name or refuses it for op.
func (s *Store) name(op, key string) (string, error) {
	n, err := objectName(s.prefix, key)
	if err != nil {
		return "", invalidKey(op, err)
	}
	return n, nil
}

// Put streams exactly size bytes of body to the object key with PutObject; an existing key is
// replaced (M3-01 note 7f). A body of another length gives app.ErrSizeMismatch and stores nothing.
func (s *Store) Put(ctx context.Context, key string, body io.Reader, size int64) error {
	name, err := s.name(opPut, key)
	if err != nil {
		return err
	}
	if size < 0 {
		return refused(opPut, key, app.ErrSizeMismatch)
	}
	eb, rd, err := s.body(opPut, key, body, size)
	if err != nil {
		return err
	}
	_, err = s.api.PutObject(ctx, &awss3.PutObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(name), Body: rd, ContentLength: aws.Int64(size),
	}, unsignedPayload)
	if serr := eb.sizeError(opPut, key); serr != nil {
		return serr
	}
	if err != nil {
		return fail(opPut, key, err, mapping{})
	}
	return nil
}

// body wraps the caller's reader; with size 0 it proves the reader empty before any request.
func (s *Store) body(op, key string, body io.Reader, size int64) (*exactBody, io.Reader, error) {
	eb := newExactBody(body, size)
	if size > 0 {
		return eb, eb, nil
	}
	if _, err := eb.atEnd(); !errors.Is(err, io.EOF) {
		if eb.failed() {
			return nil, nil, refused(op, key, app.ErrSizeMismatch)
		}
		return nil, nil, &opError{op: op, key: key, detail: "read body", cause: err}
	}
	return eb, nil, nil
}

// Get opens the object key, the whole object or one byte range, with GetObject. The reader keeps
// the egress limits: at most MaxObjectBytes and http.transfer_timeout for the whole read.
func (s *Store) Get(ctx context.Context, key string, rng *app.ByteRange) (io.ReadCloser, error) {
	name, err := s.name(opGet, key)
	if err != nil {
		return nil, err
	}
	in := &awss3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(name)}
	if rng != nil {
		if rng.Offset < 0 || rng.Length < 1 || rng.Offset > (1<<63-1)-rng.Length {
			return nil, refused(opGet, key, app.ErrInvalidRange)
		}
		in.Range = aws.String("bytes=" + strconv.FormatInt(rng.Offset, 10) + "-" + strconv.FormatInt(rng.Offset+rng.Length-1, 10))
	}
	out, err := s.api.GetObject(ctx, in)
	if err != nil {
		return nil, fail(opGet, key, err, mapping{notFound: true, invalidRange: true})
	}
	return &getBody{rc: out.Body, key: key}, nil
}

// getBody maps read errors of a Get body to the text rule of opError.
type getBody struct {
	rc  io.ReadCloser
	key string
}

func (b *getBody) Read(p []byte) (int, error) {
	n, err := b.rc.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		return n, readError(b.key, err)
	}
	return n, err
}

func (b *getBody) Close() error {
	if err := b.rc.Close(); err != nil {
		return &opError{op: opGet, key: b.key, detail: "close: transport", cause: err}
	}
	return nil
}

// Head returns the size of the object key with HeadObject. A HEAD answer has no body, so a
// missing bucket also reads as app.ErrNotFound here.
func (s *Store) Head(ctx context.Context, key string) (app.ObjectInfo, error) {
	name, err := s.name(opHead, key)
	if err != nil {
		return app.ObjectInfo{}, err
	}
	out, err := s.api.HeadObject(ctx, &awss3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(name)})
	if err != nil {
		return app.ObjectInfo{}, fail(opHead, key, err, mapping{notFound: true})
	}
	return app.ObjectInfo{Size: aws.ToInt64(out.ContentLength)}, nil
}

// Delete removes the object key with DeleteObject; S3 answers a missing key with success, so a
// missing object is no error (12 "12").
func (s *Store) Delete(ctx context.Context, key string) error {
	name, err := s.name(opDelete, key)
	if err != nil {
		return err
	}
	if _, err := s.api.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(name)}); err != nil {
		return fail(opDelete, key, err, mapping{})
	}
	return nil
}

// CreateMultipart opens a multipart upload for an original key and returns the S3 upload ID;
// other keys give app.ErrInvalidKey, as on the file system driver (M3-01 note 10).
func (s *Store) CreateMultipart(ctx context.Context, key string) (string, error) {
	name, err := s.name(opCreate, key)
	if err != nil {
		return "", err
	}
	if _, _, err := domain.ParseOriginalKey(key); err != nil {
		return "", refused(opCreate, key, app.ErrInvalidKey)
	}
	out, err := s.api.CreateMultipartUpload(ctx, &awss3.CreateMultipartUploadInput{Bucket: aws.String(s.bucket), Key: aws.String(name)})
	if err != nil {
		return "", fail(opCreate, key, err, mapping{})
	}
	id := aws.ToString(out.UploadId)
	if id == "" {
		return "", &opError{op: opCreate, key: key, detail: "no upload ID in the answer"}
	}
	return id, nil
}

// UploadPart streams part n of exactly size bytes as a multipart part, without a buffer in the
// server (12 "3.2"), and returns its ETag exactly as S3 sent it, quotes included.
func (s *Store) UploadPart(ctx context.Context, key, uploadID string, n int, body io.Reader, size int64) (app.Part, error) {
	name, err := s.name(opPart, key)
	if err != nil {
		return app.Part{}, err
	}
	if n < 1 || n > maxPartNumber {
		return app.Part{}, refused(opPart, key, app.ErrInvalidPart)
	}
	if size < 0 {
		return app.Part{}, refused(opPart, key, app.ErrSizeMismatch)
	}
	if uploadID == "" {
		return app.Part{}, refused(opPart, key, app.ErrUploadNotFound)
	}
	eb, rd, err := s.body(opPart, key, body, size)
	if err != nil {
		return app.Part{}, err
	}
	out, err := s.api.UploadPart(ctx, &awss3.UploadPartInput{
		Bucket: aws.String(s.bucket), Key: aws.String(name), UploadId: aws.String(uploadID),
		PartNumber: aws.Int32(int32(n)), Body: rd, ContentLength: aws.Int64(size), // #nosec G115 -- n is at most maxPartNumber
	}, unsignedPayload)
	if serr := eb.sizeError(opPart, key); serr != nil {
		return app.Part{}, serr
	}
	if err != nil {
		return app.Part{}, fail(opPart, key, err, mapping{uploadMissing: true})
	}
	etag := aws.ToString(out.ETag)
	if etag == "" {
		return app.Part{}, &opError{op: opPart, key: key, detail: "no ETag in the answer"}
	}
	return app.Part{Number: n, ETag: etag}, nil
}

// maxPartNumber is the largest part number S3 accepts (docs/spec/12-files.md "3.1" [Q1]);
// a larger number would not fit the int32 of the SDK either.
const maxPartNumber = 10000

// CompleteMultipart checks the list shape (not empty, numbers strictly ascending from 1), then
// joins the listed parts with CompleteMultipartUpload. S3 keeps the upload open after
// InvalidPart, InvalidPartOrder and EntityTooSmall, so a later correct call succeeds.
func (s *Store) CompleteMultipart(ctx context.Context, key, uploadID string, parts []app.Part) error {
	name, err := s.name(opComplete, key)
	if err != nil {
		return err
	}
	if len(parts) == 0 {
		return refused(opComplete, key, app.ErrInvalidPart)
	}
	completed := make([]types.CompletedPart, 0, len(parts))
	prev := 0
	for _, p := range parts {
		if p.Number <= prev || p.Number > maxPartNumber {
			return refused(opComplete, key, app.ErrInvalidPart)
		}
		prev = p.Number
		completed = append(completed, types.CompletedPart{PartNumber: aws.Int32(int32(p.Number)), ETag: aws.String(p.ETag)}) // #nosec G115 -- at most maxPartNumber
	}
	if uploadID == "" {
		return refused(opComplete, key, app.ErrUploadNotFound)
	}
	_, err = s.api.CompleteMultipartUpload(ctx, &awss3.CompleteMultipartUploadInput{
		Bucket: aws.String(s.bucket), Key: aws.String(name), UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: completed},
	})
	if err != nil {
		return fail(opComplete, key, err, mapping{uploadMissing: true, invalidPart: true})
	}
	return nil
}

// AbortMultipart ends the upload with AbortMultipartUpload; an unknown, aborted or completed
// upload (NoSuchUpload) is no error.
func (s *Store) AbortMultipart(ctx context.Context, key, uploadID string) error {
	name, err := s.name(opAbort, key)
	if err != nil {
		return err
	}
	if uploadID == "" {
		return nil
	}
	_, err = s.api.AbortMultipartUpload(ctx, &awss3.AbortMultipartUploadInput{
		Bucket: aws.String(s.bucket), Key: aws.String(name), UploadId: aws.String(uploadID),
	})
	if err != nil && !isNoSuchUpload(err) {
		return fail(opAbort, key, err, mapping{})
	}
	return nil
}

// PresignUploadPart presigns UploadPart of part n with ContentLength size and the expiry ttl
// (callers pass file.s3.presign_ttl), so the signature binds the part length (12 "3.3", R-FI-02,
// docs/DECISIONS.md M3-02/9). ExpiresAt is the signing time plus ttl. The URL goes to the caller
// only; it is never part of an error. No request is sent.
func (s *Store) PresignUploadPart(ctx context.Context, key, uploadID string, n int, size int64, ttl time.Duration) (app.PresignedPart, error) {
	name, err := s.name(opPresign, key)
	if err != nil {
		return app.PresignedPart{}, err
	}
	switch {
	case n < 1 || n > maxPartNumber:
		return app.PresignedPart{}, refused(opPresign, key, app.ErrInvalidPart)
	case size < 1:
		return app.PresignedPart{}, refused(opPresign, key, app.ErrSizeMismatch)
	case ttl <= 0 || ttl%time.Second != 0:
		return app.PresignedPart{}, &opError{op: opPresign, key: key, detail: "the validity must be a positive whole number of seconds"}
	case uploadID == "":
		return app.PresignedPart{}, refused(opPresign, key, app.ErrUploadNotFound)
	}
	// Taken before signing, so ExpiresAt never lies after the end the URL carries.
	signedAt := s.now()
	req, err := s.presign.PresignUploadPart(ctx, &awss3.UploadPartInput{
		Bucket: aws.String(s.bucket), Key: aws.String(name), UploadId: aws.String(uploadID),
		PartNumber: aws.Int32(int32(n)), ContentLength: aws.Int64(size), // #nosec G115 -- n is at most maxPartNumber
	}, awss3.WithPresignExpires(ttl))
	if err != nil {
		return app.PresignedPart{}, fail(opPresign, key, err, mapping{})
	}
	return app.PresignedPart{URL: req.URL, ExpiresAt: signedAt.Add(ttl)}, nil
}
