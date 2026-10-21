// SPDX-License-Identifier: Apache-2.0

package s3mirror_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/netip"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	tclog "github.com/testcontainers/testcontainers-go/log"
	"github.com/testcontainers/testcontainers-go/wait"
)

// seaweedImage is the SeaweedFS image of the files driver tests
// (internal/files/adapters/s3/seaweed_test.go, which holds the pin against the Compose stack).
const seaweedImage = "chrislusf/seaweedfs:4.48@sha256:4e61d15fd35994cb1e43e1e553dff106794841fd9a99ade2fc8c8bfce4d7872d"

// The label of the test container; `docker ps --filter label=org.bothy.test=seaweedfs-mirror`
// finds one that a killed test binary left behind.
const (
	seaweedLabel      = "org.bothy.test"
	seaweedLabelValue = "seaweedfs-mirror"
	seaweedS3Port     = "8333/tcp"
	testRegion        = "us-east-1"
)

const (
	seaweedStartTimeout = 3 * time.Minute
	seaweedRuntimeWait  = 30 * time.Second
)

// TestMain checks for a container runtime and stops the SeaweedFS container after the tests;
// without a runtime the binary fails with the reason instead of skipping.
func TestMain(m *testing.M) { os.Exit(seaweedMain(m)) }

func seaweedMain(m *testing.M) int {
	if err := os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true"); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "s3mirror tests: %v\n", err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), seaweedRuntimeWait)
	err := runtimeReachable(ctx)
	cancel()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "s3mirror tests: no Docker-compatible runtime reachable (%v)\n", err)
		return 1
	}
	code := m.Run()
	if err := stopSeaweed(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "s3mirror tests: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	return code
}

// runtimeReachable asks the container runtime for its info; a panic counts as unreachable.
func runtimeReachable(ctx context.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	p, err := testcontainers.NewDockerProvider(testcontainers.WithLogger(tclog.NewNoopLogger()))
	if err != nil {
		return err
	}
	return p.Health(ctx)
}

// seaweedServer is the started container with its S3 endpoint and throwaway keys.
type seaweedServer struct {
	ctr            *testcontainers.DockerContainer
	endpoint       string // http://127.0.0.1:<port>
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

// startSeaweed starts the pinned image with the S3 gateway on 127.0.0.1 and a random host port.
func startSeaweed(ctx context.Context, t *testing.T) (*seaweedServer, error) {
	access, secret := "EXAMPLE"+randomHex(t, 8), "EXAMPLE"+randomHex(t, 16)
	port := network.MustParsePort(seaweedS3Port)
	ctr, err := testcontainers.Run(ctx, seaweedImage,
		testcontainers.WithLogger(tclog.NewNoopLogger()),
		testcontainers.WithName("bothy-seaweedfs-mirror-"+randomHex(t, 6)),
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
		return nil, fmt.Errorf("start %s: %w", seaweedImage, err)
	}
	mapped, err := ctr.MappedPort(ctx, seaweedS3Port)
	if err != nil {
		_ = testcontainers.TerminateContainer(ctr)
		return nil, fmt.Errorf("mapped port: %w", err)
	}
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", mapped.Num())
	return &seaweedServer{ctr: ctr, endpoint: endpoint, access: access, secret: secret, admin: adminClient(endpoint, access, secret)}, nil
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

// adminClient is the full SDK client of the tests: it creates buckets and inspects state.
func adminClient(endpoint, access, secret string) *awss3.Client {
	return awss3.New(awss3.Options{
		Region:       testRegion,
		BaseEndpoint: aws.String(endpoint),
		UsePathStyle: true,
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: access, SecretAccessKey: secret}, nil
		}),
		Retryer:                    aws.NopRetryer{},
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})
}

// newBucket creates a bucket with a random name. The filer may still be starting when the first
// test runs, so creation is tried a few times, as in the files driver tests.
func (s *seaweedServer) newBucket(t *testing.T) string {
	t.Helper()
	name := "bothy-" + randomHex(t, 8)
	var err error
	for range 30 {
		_, err = s.admin.CreateBucket(t.Context(), &awss3.CreateBucketInput{Bucket: aws.String(name)})
		if err == nil {
			return name
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("CreateBucket: %v", err)
	return ""
}

func randomHex(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}
