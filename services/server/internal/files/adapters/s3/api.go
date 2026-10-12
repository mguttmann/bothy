// SPDX-License-Identifier: Apache-2.0

package s3

import (
	"context"
	"net/http"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

// coreAPI is the only part of the S3 SDK client the driver holds: exactly the eight core S3
// operations behind the port (docs/spec/12-dateien.md "4.1", ADR 0010 "Kern-S3"). Batch deletion
// (DeleteObjects), listing, copying, bucket calls and versioning are therefore unreachable from the
// driver; TestCoreAPIMethodSet holds the method set, forbidigo the two calls that must never
// appear (docs/DECISIONS.md M3-02/12). *awss3.Client implements it.
type coreAPI interface {
	PutObject(ctx context.Context, params *awss3.PutObjectInput, optFns ...func(*awss3.Options)) (*awss3.PutObjectOutput, error)
	GetObject(ctx context.Context, params *awss3.GetObjectInput, optFns ...func(*awss3.Options)) (*awss3.GetObjectOutput, error)
	HeadObject(ctx context.Context, params *awss3.HeadObjectInput, optFns ...func(*awss3.Options)) (*awss3.HeadObjectOutput, error)
	DeleteObject(ctx context.Context, params *awss3.DeleteObjectInput, optFns ...func(*awss3.Options)) (*awss3.DeleteObjectOutput, error)
	CreateMultipartUpload(ctx context.Context, params *awss3.CreateMultipartUploadInput, optFns ...func(*awss3.Options)) (*awss3.CreateMultipartUploadOutput, error)
	UploadPart(ctx context.Context, params *awss3.UploadPartInput, optFns ...func(*awss3.Options)) (*awss3.UploadPartOutput, error)
	CompleteMultipartUpload(ctx context.Context, params *awss3.CompleteMultipartUploadInput, optFns ...func(*awss3.Options)) (*awss3.CompleteMultipartUploadOutput, error)
	AbortMultipartUpload(ctx context.Context, params *awss3.AbortMultipartUploadInput, optFns ...func(*awss3.Options)) (*awss3.AbortMultipartUploadOutput, error)
}

// presignAPI is the only part of the presign client the driver holds: presigned part uploads.
// There is no presigned GET, HEAD, DELETE or whole-object PUT, because clients read only through
// the server (R-FI-03, ADR 0048). *awss3.PresignClient implements it.
type presignAPI interface {
	PresignUploadPart(ctx context.Context, params *awss3.UploadPartInput, optFns ...func(*awss3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

// httpDoer is the HTTP client the SDK sends with: the *http.Client of the egress client
// (awss3.HTTPClient has the same method).
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

var (
	_ coreAPI    = (*awss3.Client)(nil)
	_ presignAPI = (*awss3.PresignClient)(nil)
	_ httpDoer   = awss3.HTTPClient(nil)
)
