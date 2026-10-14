// SPDX-License-Identifier: Apache-2.0

package s3_test

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

// seaweedImage is the SeaweedFS image of the test container: the repository and version of the
// Compose service seaweedfs (BOTHY_SEAWEEDFS_VERSION in infra/compose/.env.example and its default
// in infra/compose/compose.yaml), pinned by the digest of its multi-architecture index, so the
// tests prove the version the stack runs (docs/DECISIONS.md M3-02/13). Renovate updates tag and
// digest through the custom manager in renovate.json, which reads the comment on the next line.
//
// renovate: datasource=docker depName=chrislusf/seaweedfs
const seaweedImage = "chrislusf/seaweedfs:4.48@sha256:4e61d15fd35994cb1e43e1e553dff106794841fd9a99ade2fc8c8bfce4d7872d"

// The label of every SeaweedFS test container; `docker ps --filter label=org.bothy.test=seaweedfs`
// finds one that a killed test binary left behind.
const (
	seaweedLabel      = "org.bothy.test"
	seaweedLabelValue = "seaweedfs"
)

// The volume size and count of the test server: small volumes, enough for one bucket per test.
const (
	seaweedVolumeMB   = "32"
	seaweedMaxVolumes = "1000"
)

// seaweedS3Port is the port of the S3 gateway inside the container.
const seaweedS3Port = "8333/tcp"

const (
	seaweedStartTimeout = 3 * time.Minute
	seaweedRuntimeWait  = 30 * time.Second
)

// TestMain checks for a container runtime and stops the SeaweedFS container after the tests.
// Without a Docker-compatible runtime the binary fails with the reason; the integration tests
// never skip (plan P-09).
func TestMain(m *testing.M) { os.Exit(seaweedMain(m)) }

func seaweedMain(m *testing.M) int {
	// testcontainers reads its configuration once; only the exact value "true" disables Ryuk.
	if err := os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true"); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "s3 tests: %v\n", err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), seaweedRuntimeWait)
	err := runtimeReachable(ctx)
	cancel()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "s3 tests: no Docker-compatible runtime reachable (%v); the SeaweedFS tests need one (plan P-09)\n", err)
		return 1
	}
	code := m.Run()
	if err := stopSeaweed(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "s3 tests: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	return code
}

// runtimeReachable asks the container runtime for its info; a panic of testcontainers on a
// broken setup counts as unreachable.
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

// seaweedServer is the started container with its S3 endpoint and keys.
type seaweedServer struct {
	ctr            *testcontainers.DockerContainer
	endpoint       string // http://127.0.0.1:<port>
	access, secret string
	admin          *awss3.Client // the full SDK client of the tests; the driver never has one
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

// startSeaweed starts the pinned image like the Compose service seaweedfs (weed server with the
// S3 gateway, keys as AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY) on 127.0.0.1 with a random
// host port.
func startSeaweed(ctx context.Context, t *testing.T) (*seaweedServer, error) {
	access, secret := testKeys(t)
	port := network.MustParsePort(seaweedS3Port)
	ctr, err := testcontainers.Run(ctx, seaweedImage,
		testcontainers.WithLogger(tclog.NewNoopLogger()),
		testcontainers.WithName("bothy-seaweedfs-"+randomHex(t, 6)),
		testcontainers.WithLabels(map[string]string{seaweedLabel: seaweedLabelValue}),
		testcontainers.WithEnv(map[string]string{"AWS_ACCESS_KEY_ID": access, "AWS_SECRET_ACCESS_KEY": secret}),
		testcontainers.WithEntrypoint("weed"),
		// The command of the Compose service plus two sizing flags: SeaweedFS keeps every bucket
		// in its own volumes, and with the default volume size of 30 GB the free disk of a test
		// runtime holds one bucket only, while every test here creates its own
		// (docs/DECISIONS.md M3-02/13).
		testcontainers.WithCmd("server", "-dir=/data", "-s3", "-ip.bind=0.0.0.0",
			"-master.volumeSizeLimitMB="+seaweedVolumeMB, "-volume.max="+seaweedMaxVolumes),
		testcontainers.WithExposedPorts(seaweedS3Port),
		testcontainers.WithHostConfigModifier(func(hc *container.HostConfig) {
			hc.PortBindings = network.PortMap{port: {{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: "0"}}}
			hc.AutoRemove = true
		}),
		// An unsigned request gets 403 once the gateway checks signatures.
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
	srv := &seaweedServer{ctr: ctr, endpoint: endpoint, access: access, secret: secret, admin: adminClient(endpoint, access, secret)}
	return srv, nil
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
	// TerminateContainer ignores the errors of a container that AutoRemove already removes.
	if err := testcontainers.TerminateContainer(srv.ctr); err != nil {
		return fmt.Errorf("terminate container %s: %w", srv.ctr.GetContainerID(), err)
	}
	return nil
}

// adminClient is the full SDK client of the tests: it creates buckets and inspects state. Only
// test code builds one (docs/DECISIONS.md M3-02/13).
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

// newBucket creates a bucket with a random name for t. The filer may still be starting when the
// first test runs, so creation is tried a few times.
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

// TestSeaweedImagePinned holds docs/DECISIONS.md M3-02/13: the image constant has the repository
// of the Compose service seaweedfs, a version and a sha256 digest, and the version is the one the
// stack runs.
func TestSeaweedImagePinned(t *testing.T) {
	t.Parallel()
	m := regexp.MustCompile(`^([a-z0-9][a-z0-9./_-]*):([^@:]+)@sha256:[0-9a-f]{64}$`).FindStringSubmatch(seaweedImage)
	if m == nil {
		t.Fatalf("seaweedImage %q is not <repository>:<version>@sha256:<digest>", seaweedImage)
	}
	repo, version := m[1], m[2]
	compose := readRepoFile(t, "infra/compose/compose.yaml")
	want := "image: " + repo + ":${BOTHY_SEAWEEDFS_VERSION:-" + version + "}"
	if !strings.Contains(compose, want) {
		t.Errorf("infra/compose/compose.yaml has no line %q", want)
	}
	env := readRepoFile(t, "infra/compose/.env.example")
	if !regexp.MustCompile(`(?m)^# renovate: datasource=docker depName=` + regexp.QuoteMeta(repo) + `\nBOTHY_SEAWEEDFS_VERSION=` + regexp.QuoteMeta(version) + `$`).MatchString(env) {
		t.Errorf("infra/compose/.env.example does not set BOTHY_SEAWEEDFS_VERSION=%s under its renovate comment", version)
	}
	self := readRepoFile(t, "services/server/internal/files/adapters/s3/seaweed_test.go")
	if !strings.Contains(self, "// renovate: datasource=docker depName="+repo+"\nconst seaweedImage = ") {
		t.Error("the renovate comment does not stand directly above seaweedImage")
	}
}

// readRepoFile reads a file relative to the repository root, six levels above this package.
func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "..", rel)) // #nosec G304 -- a fixed file of the repository
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s not found from the package directory", rel)
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
