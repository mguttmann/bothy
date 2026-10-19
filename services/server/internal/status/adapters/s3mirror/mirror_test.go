// SPDX-License-Identifier: Apache-2.0

package s3mirror_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/OWNER/bothy/services/server/internal/status/adapters/s3mirror"
	"github.com/OWNER/bothy/services/server/internal/status/app"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// keys returns the credentials function of access and secret.
func keys(access, secret string) s3mirror.Credentials {
	return func(context.Context) (string, string, error) { return access, secret, nil }
}

// mirrorConfig is a mirror from srcBucket below "files" into dstBucket below "instance-1".
func mirrorConfig(s *seaweedServer, srcBucket, dstBucket string) s3mirror.Config {
	return s3mirror.Config{
		Source: s3mirror.Source{
			Endpoint: s.endpoint, Region: testRegion, Bucket: srcBucket, Prefix: "files", UsePathStyle: true,
			Credentials: keys(s.access, s.secret),
		},
		Destination: s3mirror.Destination{
			Target: domain.OffsiteTarget{Host: "s3.example.com", Port: 443, Bucket: dstBucket, Prefix: "instance-1"},
			Region: testRegion, UsePathStyle: true, Credentials: keys(s.access, s.secret),
		},
	}
}

func put(t *testing.T, s *seaweedServer, bucket, key, body string) {
	t.Helper()
	if _, err := s.admin.PutObject(t.Context(), &awss3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(key), Body: bytes.NewReader([]byte(body))}); err != nil {
		t.Fatalf("PutObject %s: %v", key, err)
	}
}

// get returns the content of key, "" when it cannot be read.
func get(t *testing.T, s *seaweedServer, bucket, key string) string {
	t.Helper()
	out, err := s.admin.GetObject(t.Context(), &awss3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return ""
	}
	defer func() { _ = out.Body.Close() }()
	b, err := io.ReadAll(out.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func listKeys(t *testing.T, s *seaweedServer, bucket string) []string {
	t.Helper()
	out, err := s.admin.ListObjectsV2(t.Context(), &awss3.ListObjectsV2Input{Bucket: aws.String(bucket)})
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, o := range out.Contents {
		keys = append(keys, aws.ToString(o.Key))
	}
	slices.Sort(keys)
	return keys
}

// TestMirror_CopiesNewNeverOverwritesNeverDeletes: new objects below the source prefix are
// copied below <target prefix>/objects/, an existing destination object is not touched, an
// object deleted at the source stays in the destination, and a pass without new objects copies
// nothing.
func TestMirror_CopiesNewNeverOverwritesNeverDeletes(t *testing.T) {
	s := sharedSeaweed(t)
	src, dst := s.newBucket(t), s.newBucket(t)
	put(t, s, src, "files/a/1", "alpha")
	put(t, s, src, "files/b/2", "bravo-bravo")
	put(t, s, src, "other/x", "outside the prefix")
	put(t, s, dst, "instance-1/objects/a/1", "already there")

	m, err := s3mirror.NewWithDestinationEndpoint(mirrorConfig(s, src, dst), s.endpoint)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	res, err := m.S3Mirror(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Copied != 1 || res.SizeBytes != int64(len("bravo-bravo")) {
		t.Errorf("first pass %+v, want 1 object of 11 bytes", res)
	}
	if got := get(t, s, dst, "instance-1/objects/a/1"); got != "already there" {
		t.Errorf("existing destination object overwritten: %q", got)
	}
	if got := get(t, s, dst, "instance-1/objects/b/2"); got != "bravo-bravo" {
		t.Errorf("copied object %q", got)
	}

	if _, err := s.admin.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: aws.String(src), Key: aws.String("files/b/2")}); err != nil {
		t.Fatal(err)
	}
	put(t, s, src, "files/c/3", "charlie")
	res, err = m.S3Mirror(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Copied != 1 || res.SizeBytes != int64(len("charlie")) {
		t.Errorf("second pass %+v, want 1 object of 7 bytes", res)
	}
	want := []string{"instance-1/objects/a/1", "instance-1/objects/b/2", "instance-1/objects/c/3"}
	if got := listKeys(t, s, dst); !slices.Equal(got, want) {
		t.Errorf("destination %q, want %q", got, want)
	}

	res, err = m.S3Mirror(ctx)
	if err != nil || res.Copied != 0 || res.SizeBytes != 0 {
		t.Errorf("third pass %+v, %v, want nothing copied", res, err)
	}
}

// TestMirror_ErrorIsCode: refused credentials of the destination are s3_mirror_failed, an error
// code without the keys or the endpoint.
func TestMirror_ErrorIsCode(t *testing.T) {
	s := sharedSeaweed(t)
	src, dst := s.newBucket(t), s.newBucket(t)
	put(t, s, src, "files/a/1", "alpha")
	cfg := mirrorConfig(s, src, dst)
	cfg.Destination.Credentials = keys("EXAMPLEWRONG", "EXAMPLE-wrong-secret")
	m, err := s3mirror.NewWithDestinationEndpoint(cfg, s.endpoint)
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.S3Mirror(context.Background())
	var ee *app.ExecError
	if !errors.As(err, &ee) || ee.Code != "s3_mirror_failed" || err.Error() != "status: s3_mirror_failed" {
		t.Fatalf("error %v, want ExecError s3_mirror_failed", err)
	}
}

// TestNew_Validation: New refuses a configuration without bucket, credentials or a usable
// source endpoint, and an unparsable CA bundle.
func TestNew_Validation(t *testing.T) {
	good := s3mirror.Config{
		Source: s3mirror.Source{Endpoint: "https://s3.example.com", Region: "us-east-1", Bucket: "bothy-files", Credentials: keys("EXAMPLE", "EXAMPLE")},
		Destination: s3mirror.Destination{
			Target: domain.OffsiteTarget{Host: "backup.example.com", Port: 443, Bucket: "bothy-backup"},
			Region: "us-east-1", Credentials: keys("EXAMPLE", "EXAMPLE"),
		},
	}
	if _, err := s3mirror.New(good); err != nil {
		t.Fatalf("New(good) = %v", err)
	}
	for name, mutate := range map[string]func(*s3mirror.Config){
		"source endpoint":    func(c *s3mirror.Config) { c.Source.Endpoint = "ftp://s3.example.com" },
		"source bucket":      func(c *s3mirror.Config) { c.Source.Bucket = "" },
		"source credentials": func(c *s3mirror.Config) { c.Source.Credentials = nil },
		"destination bucket": func(c *s3mirror.Config) { c.Destination.Target.Bucket = "" },
		"destination keys":   func(c *s3mirror.Config) { c.Destination.Credentials = nil },
		"destination CA":     func(c *s3mirror.Config) { c.Destination.CAPEM = []byte("EXAMPLE not a PEM") },
		"destination region": func(c *s3mirror.Config) { c.Destination.Region = "" },
		"source region":      func(c *s3mirror.Config) { c.Source.Region = "" },
	} {
		cfg := good
		mutate(&cfg)
		if _, err := s3mirror.New(cfg); err == nil {
			t.Errorf("%s: New accepted the configuration", name)
		}
	}
}
