// SPDX-License-Identifier: Apache-2.0

package s3sink_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	tclog "github.com/testcontainers/testcontainers-go/log"
	"github.com/testcontainers/testcontainers-go/wait"
)

// The SeaweedFS test container of the sink (docs/DECISIONS.md M7-22/2): the image is the one the
// S3 driver of the files module pins (internal/files/adapters/s3/seaweed_test.go, the version of
// the Compose service seaweedfs), read from that file, so Renovate moves one pin and both test
// suites follow.
const (
	seaweedS3Port       = "8333/tcp"
	seaweedStartTimeout = 3 * time.Minute
	seaweedLabel        = "org.bothy.test"
	seaweedLabelValue   = "seaweedfs"
	testRegion          = "us-east-1"
)

// TestMain stops the SeaweedFS container after the tests. Without a Docker-compatible runtime
// the container start fails the tests that need it; they never skip (plan P-09).
func TestMain(m *testing.M) {
	code := m.Run()
	if err := stopSeaweed(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "s3sink tests: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

// seaweedServer is the started container with its S3 endpoint, keys and the full SDK client of
// the tests; the sink never has one.
type seaweedServer struct {
	ctr            *testcontainers.DockerContainer
	endpoint       string
	access, secret string
	admin          *awss3.Client
}

var seaweed struct {
	mu  sync.Mutex
	srv *seaweedServer
	err error
}

// sharedSeaweed returns the SeaweedFS of this test binary, starting it on first use.
func sharedSeaweed(t *testing.T) *seaweedServer {
	t.Helper()
	seaweed.mu.Lock()
	defer seaweed.mu.Unlock()
	if seaweed.srv == nil && seaweed.err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), seaweedStartTimeout)
		seaweed.srv, seaweed.err = startSeaweed(ctx, t)
		cancel()
	}
	if seaweed.err != nil {
		t.Fatalf("SeaweedFS: %v", seaweed.err)
	}
	return seaweed.srv
}

// seaweedImage reads the pinned image from the test of the files S3 driver.
func seaweedImage() (string, error) {
	_, self, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(self), "..", "..", "..", "files", "adapters", "s3", "seaweed_test.go")
	src, err := os.ReadFile(path) //nolint:gosec // G304: a fixed file of the repository
	if err != nil {
		return "", err
	}
	m := regexp.MustCompile(`(?m)^const seaweedImage = "([^"]+@sha256:[0-9a-f]{64})"$`).FindSubmatch(src)
	if m == nil {
		return "", fmt.Errorf("no pinned seaweedImage in %s", path)
	}
	return string(m[1]), nil
}

// startSeaweed starts the pinned image like the files driver tests do, on 127.0.0.1 with a
// random host port and generated keys.
func startSeaweed(ctx context.Context, t *testing.T) (*seaweedServer, error) {
	// testcontainers reads its configuration once; only the exact value "true" disables Ryuk.
	if err := os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true"); err != nil {
		return nil, err
	}
	image, err := seaweedImage()
	if err != nil {
		return nil, err
	}
	access, secret := "AKEXAMPLE"+strings.ToUpper(randomHex(t, 6)), "EXAMPLE"+randomHex(t, 16)
	port := network.MustParsePort(seaweedS3Port)
	ctr, err := testcontainers.Run(ctx, image,
		testcontainers.WithLogger(tclog.NewNoopLogger()),
		testcontainers.WithName("bothy-seaweedfs-"+randomHex(t, 6)),
		testcontainers.WithLabels(map[string]string{seaweedLabel: seaweedLabelValue}),
		testcontainers.WithEnv(map[string]string{"AWS_ACCESS_KEY_ID": access, "AWS_SECRET_ACCESS_KEY": secret}),
		testcontainers.WithEntrypoint("weed"),
		testcontainers.WithCmd("server", "-dir=/data", "-s3", "-ip.bind=0.0.0.0",
			"-master.volumeSizeLimitMB=32", "-volume.max=1000"),
		testcontainers.WithExposedPorts(seaweedS3Port),
		testcontainers.WithHostConfigModifier(func(hc *container.HostConfig) {
			hc.PortBindings = network.PortMap{port: {{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: "0"}}}
			hc.AutoRemove = true
		}),
		testcontainers.WithWaitStrategy(wait.ForHTTP("/").WithPort(seaweedS3Port).
			WithStatusCodeMatcher(func(status int) bool { return status == 403 }).
			WithStartupTimeout(seaweedStartTimeout)),
	)
	if err != nil {
		if ctr != nil {
			_ = testcontainers.TerminateContainer(ctr)
		}
		return nil, fmt.Errorf("start %s: %w", image, err)
	}
	mapped, err := ctr.MappedPort(ctx, seaweedS3Port)
	if err != nil {
		_ = testcontainers.TerminateContainer(ctr)
		return nil, fmt.Errorf("mapped port: %w", err)
	}
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", mapped.Num())
	admin := awss3.New(awss3.Options{
		Region: testRegion, BaseEndpoint: aws.String(endpoint), UsePathStyle: true,
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: access, SecretAccessKey: secret}, nil
		}),
		Retryer:                    aws.NopRetryer{},
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})
	return &seaweedServer{ctr: ctr, endpoint: endpoint, access: access, secret: secret, admin: admin}, nil
}

// stopSeaweed removes the container of the test binary, if one was started.
func stopSeaweed() error {
	seaweed.mu.Lock()
	defer seaweed.mu.Unlock()
	srv := seaweed.srv
	seaweed.srv, seaweed.err = nil, nil
	if srv == nil {
		return nil
	}
	if err := testcontainers.TerminateContainer(srv.ctr); err != nil {
		return fmt.Errorf("terminate container %s: %w", srv.ctr.GetContainerID(), err)
	}
	return nil
}

// bucketLock is the Object Lock setup of a test bucket.
type bucketLock struct {
	enabled bool                          // Object Lock on the bucket (and versioning)
	mode    types.ObjectLockRetentionMode // default retention mode; "" for none
	days    int32
	years   int32
}

// compliance is a bucket with a default retention of one day in mode Compliance.
var compliance = bucketLock{enabled: true, mode: types.ObjectLockRetentionModeCompliance, days: 1}

// newBucket creates a bucket with a random name and the lock setup. The filer may still be
// starting when the first test runs, so creation is tried a few times.
func (s *seaweedServer) newBucket(t *testing.T, lock bucketLock) string {
	t.Helper()
	name := "bothy-audit-" + randomHex(t, 6)
	in := &awss3.CreateBucketInput{Bucket: aws.String(name)}
	if lock.enabled {
		in.ObjectLockEnabledForBucket = aws.Bool(true)
	}
	var err error
	for range 30 {
		if _, err = s.admin.CreateBucket(t.Context(), in); err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	if lock.mode == "" {
		return name
	}
	rule := &types.DefaultRetention{Mode: lock.mode}
	if lock.days > 0 {
		rule.Days = aws.Int32(lock.days)
	}
	if lock.years > 0 {
		rule.Years = aws.Int32(lock.years)
	}
	if _, err := s.admin.PutObjectLockConfiguration(t.Context(), &awss3.PutObjectLockConfigurationInput{
		Bucket: aws.String(name),
		ObjectLockConfiguration: &types.ObjectLockConfiguration{
			ObjectLockEnabled: types.ObjectLockEnabledEnabled,
			Rule:              &types.ObjectLockRule{DefaultRetention: rule},
		},
	}); err != nil {
		t.Fatalf("PutObjectLockConfiguration: %v", err)
	}
	return name
}

// randomHex returns n random bytes as hex.
func randomHex(t testing.TB, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}
