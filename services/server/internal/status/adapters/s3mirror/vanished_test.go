// SPDX-License-Identifier: Apache-2.0

package s3mirror

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// fakeAPI is an object store of the unit test: list returns keys, get answers 404 for gone,
// head answers 404 for every key, put records the keys.
type fakeAPI struct {
	keys []string
	gone map[string]bool
	put  []string
}

func notFound() error {
	return &awshttp.ResponseError{ResponseError: &smithyhttp.ResponseError{
		Response: &smithyhttp.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}}}
}

func (f *fakeAPI) ListObjectsV2(context.Context, *awss3.ListObjectsV2Input, ...func(*awss3.Options)) (*awss3.ListObjectsV2Output, error) {
	out := &awss3.ListObjectsV2Output{}
	for _, k := range f.keys {
		out.Contents = append(out.Contents, types.Object{Key: aws.String(k)})
	}
	return out, nil
}

func (f *fakeAPI) HeadObject(context.Context, *awss3.HeadObjectInput, ...func(*awss3.Options)) (*awss3.HeadObjectOutput, error) {
	return nil, notFound()
}

func (f *fakeAPI) GetObject(_ context.Context, in *awss3.GetObjectInput, _ ...func(*awss3.Options)) (*awss3.GetObjectOutput, error) {
	if f.gone[aws.ToString(in.Key)] {
		return nil, notFound()
	}
	return &awss3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader([]byte("EXAMPLE")))}, nil
}

func (f *fakeAPI) PutObject(_ context.Context, in *awss3.PutObjectInput, _ ...func(*awss3.Options)) (*awss3.PutObjectOutput, error) {
	f.put = append(f.put, aws.ToString(in.Key))
	return &awss3.PutObjectOutput{}, nil
}

// TestMirror_SourceDeletedDuringPass: an object deleted from the file bucket between the list
// and its read is skipped, and the pass copies the objects after it.
func TestMirror_SourceDeletedDuringPass(t *testing.T) {
	src := &fakeAPI{keys: []string{"files/a", "files/b", "files/c"}, gone: map[string]bool{"files/b": true}}
	dst := &fakeAPI{}
	m := &Mirror{src: src, dst: dst, srcBucket: "bothy-files", dstBkt: "bothy-backup", srcPrefix: "files/", dstBase: "objects/", scratch: t.TempDir()}
	res, err := m.S3Mirror(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Copied != 2 || len(dst.put) != 2 || dst.put[0] != "objects/a" || dst.put[1] != "objects/c" {
		t.Errorf("result %+v, puts %q; want a and c copied", res, dst.put)
	}
}
