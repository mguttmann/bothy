// SPDX-License-Identifier: Apache-2.0

package s3mirror

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/OWNER/bothy/services/server/internal/status/app"
)

// httpError is an SDK response error with the status code.
func httpError(code int) error {
	return &awshttp.ResponseError{ResponseError: &smithyhttp.ResponseError{
		Response: &smithyhttp.Response{Response: &http.Response{StatusCode: code}}}}
}

// flakyAPI is an object store of the unit tests whose calls fail as scripted: fail[op+" "+key]
// lists the errors of the first calls of op on key (nil passes), always[op+" "+key] fails every
// call. Head answers 404 once the script is used up, so every object is copied; put records the
// keys and the bodies it received.
type flakyAPI struct {
	mu     sync.Mutex
	keys   []string
	fail   map[string][]error
	always map[string]error
	calls  map[string]int
	put    []string
	bodies map[string]string
}

func newFlaky(keys ...string) *flakyAPI {
	return &flakyAPI{keys: keys, fail: map[string][]error{}, always: map[string]error{}, calls: map[string]int{}, bodies: map[string]string{}}
}

// next returns the scripted error of the next call of op on key.
func (f *flakyAPI) next(op, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := op + " " + key
	f.calls[id]++
	if err, ok := f.always[id]; ok {
		return err
	}
	if s := f.fail[id]; len(s) > 0 {
		f.fail[id] = s[1:]
		return s[0]
	}
	return nil
}

func (f *flakyAPI) ListObjectsV2(context.Context, *awss3.ListObjectsV2Input, ...func(*awss3.Options)) (*awss3.ListObjectsV2Output, error) {
	if err := f.next("list", ""); err != nil {
		return nil, err
	}
	out := &awss3.ListObjectsV2Output{}
	for _, k := range f.keys {
		out.Contents = append(out.Contents, types.Object{Key: aws.String(k)})
	}
	return out, nil
}

func (f *flakyAPI) HeadObject(_ context.Context, in *awss3.HeadObjectInput, _ ...func(*awss3.Options)) (*awss3.HeadObjectOutput, error) {
	if err := f.next("head", aws.ToString(in.Key)); err != nil {
		return nil, err
	}
	return nil, httpError(http.StatusNotFound)
}

func (f *flakyAPI) GetObject(_ context.Context, in *awss3.GetObjectInput, _ ...func(*awss3.Options)) (*awss3.GetObjectOutput, error) {
	if err := f.next("get", aws.ToString(in.Key)); err != nil {
		return nil, err
	}
	return &awss3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader([]byte("EXAMPLE")))}, nil
}

func (f *flakyAPI) PutObject(_ context.Context, in *awss3.PutObjectInput, _ ...func(*awss3.Options)) (*awss3.PutObjectOutput, error) {
	key := aws.ToString(in.Key)
	b, err := io.ReadAll(in.Body)
	if err != nil {
		return nil, err
	}
	if err := f.next("put", key); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.put = append(f.put, key)
	f.bodies[key] = string(b)
	return &awss3.PutObjectOutput{}, nil
}

// flakyMirror is a mirror from src to dst whose pauses are recorded instead of slept.
func flakyMirror(t *testing.T, src, dst *flakyAPI, pauses *[]time.Duration) *Mirror {
	t.Helper()
	return &Mirror{src: src, dst: dst, srcBucket: "bothy-files", dstBkt: "bothy-backup", srcPrefix: "files/", dstBase: "objects/",
		scratch: t.TempDir(), sleep: func(_ context.Context, d time.Duration) bool { *pauses = append(*pauses, d); return true }}
}

// TestMirror_RetriesTransientErrors holds the retry of AUD srv-stats-status-7: a list, head, get
// and put that fail with a network error, 429 or a 5xx are tried again with a doubling pause, a
// retried put sends the whole object again, and the pass copies everything without an error.
func TestMirror_RetriesTransientErrors(t *testing.T) {
	t.Parallel()
	src, dst := newFlaky("files/a", "files/b"), newFlaky()
	src.fail["list "] = []error{httpError(http.StatusServiceUnavailable)}
	dst.fail["head objects/a"] = []error{httpError(http.StatusInternalServerError), httpError(http.StatusTooManyRequests)}
	src.fail["get files/b"] = []error{errors.New("connection reset")}
	dst.fail["put objects/b"] = []error{httpError(http.StatusBadGateway)}
	var pauses []time.Duration
	res, err := flakyMirror(t, src, dst, &pauses).S3Mirror(context.Background())
	if err != nil {
		t.Fatalf("S3Mirror: %v", err)
	}
	if res.Copied != 2 || !slices.Equal(dst.put, []string{"objects/a", "objects/b"}) || dst.bodies["objects/b"] != "EXAMPLE" {
		t.Errorf("result %+v, puts %q, body %q; want a and b copied whole", res, dst.put, dst.bodies["objects/b"])
	}
	want := []time.Duration{firstBackoff, firstBackoff, 2 * firstBackoff, firstBackoff, firstBackoff}
	if !slices.Equal(pauses, want) {
		t.Errorf("pauses %v, want %v", pauses, want)
	}
}

// TestMirror_FailedObjectDoesNotStopThePass: an object that fails on every try, or with a status
// a retry cannot change, is skipped after its tries; the keys after it are still copied and the
// pass fails at the end with the number of objects it could not copy.
func TestMirror_FailedObjectDoesNotStopThePass(t *testing.T) {
	t.Parallel()
	src, dst := newFlaky("files/a", "files/b", "files/c", "files/d"), newFlaky()
	dst.always["put objects/b"] = httpError(http.StatusServiceUnavailable)
	dst.always["head objects/c"] = httpError(http.StatusForbidden)
	var pauses []time.Duration
	res, err := flakyMirror(t, src, dst, &pauses).S3Mirror(context.Background())
	var ee *app.ExecError
	if !errors.As(err, &ee) || ee.Code != "s3_mirror_objects_failed_2" {
		t.Fatalf("S3Mirror error %v, want s3_mirror_objects_failed_2", err)
	}
	if res.Copied != 2 || !slices.Equal(dst.put, []string{"objects/a", "objects/d"}) {
		t.Errorf("result %+v, puts %q; want a and d copied", res, dst.put)
	}
	if got := dst.calls["put objects/b"]; got != attempts {
		t.Errorf("put of b tried %d times, want %d", got, attempts)
	}
	if got := dst.calls["head objects/c"]; got != 1 {
		t.Errorf("head of c answered 403 and was tried %d times, want 1", got)
	}
}

// TestMirror_ListFailsAfterRetries: a list page that fails on every try ends the pass with
// s3_mirror_failed; a canceled context ends it with canceled and no further try.
func TestMirror_ListFailsAfterRetries(t *testing.T) {
	t.Parallel()
	src := newFlaky("files/a")
	src.always["list "] = httpError(http.StatusServiceUnavailable)
	var pauses []time.Duration
	_, err := flakyMirror(t, src, newFlaky(), &pauses).S3Mirror(context.Background())
	var ee *app.ExecError
	if !errors.As(err, &ee) || ee.Code != codeFailed || src.calls["list "] != attempts {
		t.Errorf("S3Mirror error %v after %d lists, want %s after %d", err, src.calls["list "], codeFailed, attempts)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := newFlaky("files/a")
	c.always["list "] = errors.New("context canceled")
	_, err = flakyMirror(t, c, newFlaky(), &pauses).S3Mirror(ctx)
	if !errors.As(err, &ee) || ee.Code != codeCanceled || c.calls["list "] != 1 {
		t.Errorf("canceled: error %v after %d lists, want canceled after 1", err, c.calls["list "])
	}
}
