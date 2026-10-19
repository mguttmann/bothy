// SPDX-License-Identifier: Apache-2.0

// Package s3mirror is the object backup of the driver s3 (brief M7-02 design note 5): a mirror
// of the file bucket into the off-site bucket below <prefix>/objects/ that copies every object
// the destination lacks and never deletes or overwrites. Objects deleted from the file bucket
// stay in the destination until the owner's off-site expiry. It implements the S3Mirror step of
// app.Executor (adapters/executor delegates to it).
//
// The client holds only the four calls a mirror needs (list, head, get, put); it has no delete.
// Every error is the *app.ExecError s3_mirror_failed (or canceled), without keys, endpoint or
// the text of the SDK.
package s3mirror

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/logging"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/status/app"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// egressPurpose labels the egress clients of the mirror in logs (bothy.egress.purpose).
const egressPurpose = "backup_mirror"

// Error codes of the mirror.
const (
	codeFailed   = "s3_mirror_failed"
	codeCanceled = "canceled"
)

// Credentials returns the access key ID and the secret access key on every call, so rotated
// keys are used without restart.
type Credentials func(ctx context.Context) (accessKeyID, secretAccessKey string, err error)

// Source is the file bucket: the values of file.s3.* (endpoint, region, bucket, prefix, path
// style) and its keys.
type Source struct {
	// Endpoint is file.s3.endpoint, an http or https URL.
	Endpoint string
	Region   string
	Bucket   string
	// Prefix is file.s3.prefix; "" for the bucket root.
	Prefix       string
	UsePathStyle bool
	Credentials  Credentials
}

// Destination is the off-site bucket: backup.offsite.target, backup.offsite.region,
// backup.offsite.use_path_style, the keys of <secrets.dir>/offsite/ and the optional
// offsite/ca.crt.
type Destination struct {
	Target       domain.OffsiteTarget
	Region       string
	UsePathStyle bool
	Credentials  Credentials
	// CAPEM is a PEM CA bundle that replaces the system roots for a private CA; nil uses the
	// system roots.
	CAPEM []byte
}

// Config is the configuration of the mirror.
type Config struct {
	Source      Source
	Destination Destination
	// ScratchDir holds one object at a time while it is copied, so its SHA-256 is known before
	// the upload; "" means the temporary directory of the system.
	ScratchDir string
	// Clock stamps the expiry of the credentials; nil means clock.System().
	Clock clock.Clock
	// Egress is egress.* (wire.EgressConfig): every request of the mirror goes through an
	// egress client (19 SEC-NET-1) that allows the host of its own side only; an http or private
	// endpoint (the SeaweedFS of the Compose profile s3) needs an egress.allowed_hosts entry with
	// allow_private, as for the files driver.
	Egress egress.Config
	// EgressDeps are the resolver, logger and metrics of the egress clients; Destination.CAPEM
	// replaces RootCAs on the destination side when set.
	EgressDeps egress.Deps
	// MaxObjectBytes bounds the body of one object read (file.max_upload_bytes); 0 keeps
	// egress.max_response_bytes.
	MaxObjectBytes int64
	// TransferTimeout bounds one request (http.transfer_timeout); 0 keeps egress.timeout.
	TransferTimeout time.Duration
}

// objectAPI is the part of the SDK client the mirror holds: no delete, no copy, no bucket call.
// *awss3.Client implements it.
type objectAPI interface {
	ListObjectsV2(ctx context.Context, params *awss3.ListObjectsV2Input, optFns ...func(*awss3.Options)) (*awss3.ListObjectsV2Output, error)
	HeadObject(ctx context.Context, params *awss3.HeadObjectInput, optFns ...func(*awss3.Options)) (*awss3.HeadObjectOutput, error)
	GetObject(ctx context.Context, params *awss3.GetObjectInput, optFns ...func(*awss3.Options)) (*awss3.GetObjectOutput, error)
	PutObject(ctx context.Context, params *awss3.PutObjectInput, optFns ...func(*awss3.Options)) (*awss3.PutObjectOutput, error)
}

var _ objectAPI = (*awss3.Client)(nil)

// Mirror copies the file bucket into the off-site bucket. It is safe for concurrent use, but
// bothy-backupd runs at most one object backup at a time.
type Mirror struct {
	src, dst           objectAPI
	srcBucket, dstBkt  string
	srcPrefix, dstBase string
	scratch            string
}

// New builds the mirror of cfg; it sends no request. The error names a field, never a value.
//
// Example:
//
//	m, err := s3mirror.New(s3mirror.Config{
//		Source: s3mirror.Source{Endpoint: "https://s3.example.com", Region: "us-east-1",
//			Bucket: "bothy-files", UsePathStyle: true, Credentials: fileKeys},
//		Destination: s3mirror.Destination{Target: target, Region: "us-east-1",
//			UsePathStyle: true, Credentials: offsiteKeys},
//	})
func New(cfg Config) (*Mirror, error) {
	return newMirror(cfg, "")
}

// newMirror is New with an optional destination endpoint that replaces Target.Endpoint (tests).
func newMirror(cfg Config, dstEndpoint string) (*Mirror, error) {
	if err := checkSource(cfg.Source); err != nil {
		return nil, err
	}
	d := cfg.Destination
	switch {
	case d.Target.Bucket == "" || d.Target.Host == "":
		return nil, errors.New("s3mirror: Destination.Target has no bucket or host")
	case d.Region == "":
		return nil, errors.New("s3mirror: Destination.Region is empty")
	case d.Credentials == nil:
		return nil, errors.New("s3mirror: Destination.Credentials is nil")
	}
	if dstEndpoint == "" {
		dstEndpoint = d.Target.Endpoint()
	}
	dstHTTP, err := sideClient(cfg, dstEndpoint, d.UsePathStyle, d.Target.Bucket, d.CAPEM)
	if err != nil {
		return nil, err
	}
	srcHTTP, err := sideClient(cfg, cfg.Source.Endpoint, cfg.Source.UsePathStyle, cfg.Source.Bucket, nil)
	if err != nil {
		return nil, err
	}
	clk := cfg.Clock
	if clk == nil {
		clk = clock.System()
	}
	srcPrefix := strings.Trim(cfg.Source.Prefix, "/")
	if srcPrefix != "" {
		srcPrefix += "/"
	}
	return &Mirror{
		src:       newClient(strings.TrimSuffix(cfg.Source.Endpoint, "/"), cfg.Source.Region, cfg.Source.UsePathStyle, cfg.Source.Credentials, srcHTTP, clk),
		dst:       newClient(dstEndpoint, d.Region, d.UsePathStyle, d.Credentials, dstHTTP, clk),
		srcBucket: cfg.Source.Bucket, dstBkt: d.Target.Bucket,
		srcPrefix: srcPrefix, dstBase: d.Target.Key("objects") + "/",
		scratch: cfg.ScratchDir,
	}, nil
}

// checkSource checks the fields of the file bucket.
func checkSource(s Source) error {
	u, err := url.Parse(s.Endpoint)
	switch {
	case err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "":
		return errors.New("s3mirror: Source.Endpoint must be an http or https URL with a host")
	case s.Bucket == "":
		return errors.New("s3mirror: Source.Bucket is empty")
	case s.Region == "":
		return errors.New("s3mirror: Source.Region is empty")
	case s.Credentials == nil:
		return errors.New("s3mirror: Source.Credentials is nil")
	}
	return nil
}

// sideClient is the HTTP client of one side: an egress client that allows exactly the request
// host of endpoint (the endpoint host with path style, <bucket>.<host> with virtual hosts) and
// its port, follows no redirect and trusts caPEM instead of the system roots when it is set.
func sideClient(cfg Config, endpoint string, pathStyle bool, bucket string, caPEM []byte) (*http.Client, error) {
	deps := cfg.EgressDeps
	if caPEM != nil {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, errors.New("s3mirror: Destination.CAPEM holds no PEM certificate")
		}
		deps.RootCAs = pool
	}
	policy, err := hostPolicy(endpoint, pathStyle, bucket)
	if err != nil {
		return nil, err
	}
	c, err := egress.New(cfg.Egress, egress.Options{
		Purpose: egressPurpose, Hosts: policy, Redirects: egress.RedirectNever,
		Timeout: cfg.TransferTimeout, MaxResponseBytes: cfg.MaxObjectBytes,
	}, deps)
	if err != nil {
		return nil, errors.New("s3mirror: egress client: " + err.Error())
	}
	return c.HTTPClient(), nil
}

// hostPolicy allows the scheme, request host and port of one side.
func hostPolicy(endpoint string, pathStyle bool, bucket string) (egress.HostPolicy, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Hostname() == "" {
		return nil, errors.New("s3mirror: an endpoint has no host")
	}
	host := strings.ToLower(u.Hostname())
	if !pathStyle {
		host = bucket + "." + host
	}
	port := uint16(443)
	if u.Scheme == "http" {
		port = 80
	}
	if p := u.Port(); p != "" {
		n, err := strconv.ParseUint(p, 10, 16)
		if err != nil || n == 0 {
			return nil, errors.New("s3mirror: an endpoint has an invalid port")
		}
		port = uint16(n)
	}
	scheme := u.Scheme
	return func(_ context.Context, sch, h string, pt uint16) bool {
		return sch == scheme && h == host && pt == port
	}, nil
}

// newClient builds an SDK client by hand, like the files driver: no default configuration of
// the SDK (environment, shared files, instance metadata), no retries, no SDK log, checksums only
// where an operation requires them.
func newClient(endpoint, region string, pathStyle bool, creds Credentials, hc *http.Client, clk clock.Clock) *awss3.Client {
	return awss3.New(awss3.Options{
		Region:       region,
		BaseEndpoint: aws.String(endpoint),
		UsePathStyle: pathStyle,
		// The keys expire at once, so the credential cache of the SDK asks creds again and a
		// rotated key file is used.
		Credentials: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			access, secret, err := creds(ctx)
			if err != nil {
				return aws.Credentials{}, err
			}
			return aws.Credentials{AccessKeyID: access, SecretAccessKey: secret, CanExpire: true, Expires: clk.Now()}, nil
		}),
		HTTPClient:                  hc,
		Retryer:                     aws.NopRetryer{},
		RequestChecksumCalculation:  aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation:  aws.ResponseChecksumValidationWhenRequired,
		DisableS3ExpressSessionAuth: aws.Bool(true),
		Logger:                      logging.Nop{},
	})
}

// fail returns the error of a failed pass: canceled when ctx ended, else s3_mirror_failed. The
// cause is dropped, because the texts of the SDK carry endpoints and keys of requests.
func fail(ctx context.Context) error {
	if ctx.Err() != nil {
		return &app.ExecError{Code: codeCanceled}
	}
	return &app.ExecError{Code: codeFailed}
}

// S3Mirror lists the source objects below the source prefix page by page and copies every one
// whose destination key <target prefix>/objects/<key without source prefix> does not exist yet.
// It never deletes and never overwrites: the destination is checked with HeadObject and written
// with If-None-Match: *, and a 412 means another pass copied the object first.
func (m *Mirror) S3Mirror(ctx context.Context) (app.MirrorResult, error) {
	var res app.MirrorResult
	in := &awss3.ListObjectsV2Input{Bucket: aws.String(m.srcBucket)}
	if m.srcPrefix != "" {
		in.Prefix = aws.String(m.srcPrefix)
	}
	pages := awss3.NewListObjectsV2Paginator(m.src, in)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return res, fail(ctx)
		}
		if err := m.copyPage(ctx, page, &res); err != nil {
			return res, fail(ctx)
		}
	}
	return res, nil
}

// copyPage copies the missing objects of one list page and adds them to res.
func (m *Mirror) copyPage(ctx context.Context, page *awss3.ListObjectsV2Output, res *app.MirrorResult) error {
	for _, obj := range page.Contents {
		key := aws.ToString(obj.Key)
		rel := strings.TrimPrefix(key, m.srcPrefix)
		if rel == "" || strings.HasSuffix(key, "/") {
			continue // a directory marker, no object of the driver
		}
		copied, size, err := m.copyMissing(ctx, key, m.dstBase+rel)
		if err != nil {
			return err
		}
		if copied {
			res.Copied++
			res.SizeBytes += size
		}
	}
	return nil
}

// copyMissing copies src to dst unless dst exists; it reports whether it wrote the object and
// its size.
func (m *Mirror) copyMissing(ctx context.Context, src, dst string) (bool, int64, error) {
	_, err := m.dst.HeadObject(ctx, &awss3.HeadObjectInput{Bucket: aws.String(m.dstBkt), Key: aws.String(dst)})
	if err == nil {
		return false, 0, nil
	}
	if status(err) != http.StatusNotFound {
		return false, 0, err
	}
	f, sum, size, err := m.spool(ctx, src)
	if status(err) == http.StatusNotFound {
		return false, 0, nil // deleted from the file bucket after the list: nothing to back up
	}
	if err != nil {
		return false, 0, err
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()
	_, err = m.dst.PutObject(ctx, &awss3.PutObjectInput{
		Bucket:         aws.String(m.dstBkt),
		Key:            aws.String(dst),
		Body:           f,
		ContentLength:  aws.Int64(size),
		IfNoneMatch:    aws.String("*"),
		ChecksumSHA256: aws.String(sum),
	})
	switch {
	case err == nil:
		return true, size, nil
	case status(err) == http.StatusPreconditionFailed:
		return false, 0, nil
	default:
		return false, 0, err
	}
}

// spool reads the source object into a temporary file and returns it rewound, its SHA-256 as
// base64 and its size.
func (m *Mirror) spool(ctx context.Context, key string) (*os.File, string, int64, error) {
	out, err := m.src.GetObject(ctx, &awss3.GetObjectInput{Bucket: aws.String(m.srcBucket), Key: aws.String(key)})
	if err != nil {
		return nil, "", 0, err
	}
	defer func() { _ = out.Body.Close() }()
	f, err := os.CreateTemp(m.scratch, ".s3mirror-")
	if err != nil {
		return nil, "", 0, err
	}
	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(f, h), out.Body)
	if err == nil {
		_, err = f.Seek(0, io.SeekStart)
	}
	if err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return nil, "", 0, err
	}
	return f, base64.StdEncoding.EncodeToString(h.Sum(nil)), size, nil
}

// status returns the HTTP status of an SDK error, 0 when there is none.
func status(err error) int {
	var re *awshttp.ResponseError
	if errors.As(err, &re) {
		return re.HTTPStatusCode()
	}
	return 0
}
