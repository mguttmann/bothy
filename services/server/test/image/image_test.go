// SPDX-License-Identifier: Apache-2.0

//go:build imagetest

// Package image_test checks the structure of a built bothy-server image (brief M1-23,
// docs/spec/18-release.md "Server-Images"): configuration, file system, static binary, a run
// with a read-only root file system and without capabilities, the health check command and the
// platforms of the multi-arch archive.
//
// The tests need a Docker CLI with a reachable daemon and two inputs from the environment, set
// by `make image-test` after `make image`:
//
//	BOTHY_TEST_IMAGE      reference of the image loaded for the host platform
//	BOTHY_TEST_IMAGE_OCI  absolute path of the OCI archive with every platform
//
// Every container a test creates is removed by its ID in t.Cleanup.
package image_test

import (
	"archive/tar"
	"bytes"
	"context"
	"debug/elf"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	binaryPath = "usr/local/bin/bothy"
	// keyDir is the default of secrets.signing_key_dir (docs/operations/configuration.md).
	keyDir = "var/lib/bothy/signing-keys"
	// nonroot is uid and gid of the distroless user nonroot (18 "Server-Images" rule 3).
	nonroot = 65532
	// dockerTimeout bounds one Docker CLI call; an export of the small image takes seconds.
	dockerTimeout = 2 * time.Minute
	// composeFile is the Compose stack, relative to this package directory.
	composeFile = "../../../../infra/compose/compose.yaml"
	// versionFile holds the server version, relative to this package directory.
	versionFile = "../../VERSION"
)

// healthcheckCommand is the health check of the image and of x-server-healthcheck in
// infra/compose/compose.yaml (brief M1-23, design note 4).
var healthcheckCommand = []string{"CMD", "/usr/local/bin/bothy", "healthcheck", "--url", "http://127.0.0.1:8081/readyz"}

// shells are base names that must not exist anywhere in the image.
var shells = []string{"sh", "bash", "ash", "dash", "zsh", "busybox"}

// imageConfig is the part of `docker image inspect` the tests read.
type imageConfig struct {
	Architecture string `json:"Architecture"`
	Os           string `json:"Os"`
	Config       struct {
		User         string            `json:"User"`
		Entrypoint   []string          `json:"Entrypoint"`
		Cmd          []string          `json:"Cmd"`
		Env          []string          `json:"Env"`
		Labels       map[string]string `json:"Labels"`
		Volumes      map[string]any    `json:"Volumes"`
		ExposedPorts map[string]any    `json:"ExposedPorts"`
		Healthcheck  *struct {
			Test        []string      `json:"Test"`
			Interval    time.Duration `json:"Interval"`
			Timeout     time.Duration `json:"Timeout"`
			StartPeriod time.Duration `json:"StartPeriod"`
			Retries     int           `json:"Retries"`
		} `json:"Healthcheck"`
	} `json:"Config"`
}

// fsEntry is one entry of the exported container file system.
type fsEntry struct {
	header *tar.Header
	data   []byte // content, kept for the binary only
}

// env returns the value of the environment variable name and fails the test when it is empty.
// Only the inputs of make image-test are read; the server code reads no environment here.
func env(t *testing.T, name string) string {
	t.Helper()
	v := os.Getenv(name) //nolint:forbidigo // test input from make image-test, not server configuration
	if v == "" {
		t.Fatalf("%s is not set; run make image and then make image-test", name)
	}
	return v
}

// dockerCLI runs the Docker CLI with args, bounded by dockerTimeout and by the test context,
// writing to stdout and stderr; every Docker call of these tests goes through it.
func dockerCLI(t *testing.T, stdout, stderr io.Writer, args ...string) error {
	t.Helper()
	return dockerCLIContext(t.Context(), stdout, stderr, args...)
}

// dockerCLIContext is dockerCLI under the parent context ctx.
func dockerCLIContext(ctx context.Context, stdout, stderr io.Writer, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, dockerTimeout)
	defer cancel()
	//nolint:gosec // G204: the arguments are the image under test and fixed flags; running Docker is the test.
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	return cmd.Run()
}

// removeOnCleanup registers the removal of the container id(s) that ids returns once the test
// ends. The test context is canceled before cleanup functions run, so the removal uses a
// context without that cancellation; an id of a container that is already gone (docker run
// --rm) is fine, any other failure is logged.
func removeOnCleanup(t *testing.T, ids func() []string) {
	t.Helper()
	ctx := context.WithoutCancel(t.Context())
	t.Cleanup(func() {
		for _, id := range ids() {
			var stderr bytes.Buffer
			err := dockerCLIContext(ctx, io.Discard, &stderr, "rm", "--force", "--volumes", id)
			if err != nil && !strings.Contains(stderr.String(), "No such container") {
				t.Logf("remove container %s: %v: %s", id, err, strings.TrimSpace(stderr.String()))
			}
		}
	})
}

// docker runs the Docker CLI with args and returns stdout; stderr is part of the error.
func docker(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if err := dockerCLI(t, &stdout, &stderr, args...); err != nil {
		return stdout.String(), fmt.Errorf("docker %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// mustDocker is docker that fails the test on an error.
func mustDocker(t *testing.T, args ...string) string {
	t.Helper()
	out, err := docker(t, args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// removeContainer registers the removal of the container whose ID the cid file names, if
// Docker wrote one; `docker run --rm` usually removes it already.
func removeContainer(t *testing.T, cidFile string) {
	t.Helper()
	removeOnCleanup(t, func() []string {
		raw, err := os.ReadFile(cidFile) //nolint:gosec // path from t.TempDir
		if id := strings.TrimSpace(string(raw)); err == nil && id != "" {
			return []string{id}
		}
		return nil
	})
}

// run starts a container with `docker run --rm --pull=never` plus args (never from a registry)
// and returns its exit code, stdout and stderr. A start failure of the container (for example
// a missing entrypoint) is an exit code like any other; only a Docker CLI that cannot be
// started fails the test.
func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	cidFile := filepath.Join(t.TempDir(), "cid")
	removeContainer(t, cidFile)
	var out, errOut bytes.Buffer
	err := dockerCLI(t, &out, &errOut, append([]string{"run", "--rm", "--pull=never", "--cidfile", cidFile}, args...)...)
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0, out.String(), errOut.String()
	case errors.As(err, &exit):
		return exit.ExitCode(), out.String(), errOut.String()
	default:
		t.Fatalf("docker run: %v", err)
		return 0, "", ""
	}
}

// inspect returns the configuration of image.
func inspect(t *testing.T, image string) imageConfig {
	t.Helper()
	var configs []imageConfig
	if err := json.Unmarshal([]byte(mustDocker(t, "image", "inspect", image)), &configs); err != nil {
		t.Fatalf("docker image inspect: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("docker image inspect returned %d images, want 1", len(configs))
	}
	return configs[0]
}

// createContainer creates a container from image without starting it and registers its
// removal by ID.
func createContainer(t *testing.T, image string) string {
	t.Helper()
	id := strings.TrimSpace(mustDocker(t, "create", "--pull=never", image))
	removeOnCleanup(t, func() []string { return []string{id} })
	return id
}

// exportFS creates a container from image without starting it and returns its file system by
// path (without a leading "./" or "/"); the content is kept for the binary only.
func exportFS(t *testing.T, image string) map[string]fsEntry {
	t.Helper()
	id := createContainer(t, image)

	var archive, stderr bytes.Buffer
	if err := dockerCLI(t, &archive, &stderr, "export", id); err != nil {
		t.Fatalf("docker export: %v: %s", err, stderr.String())
	}
	entries := map[string]fsEntry{}
	tr := tar.NewReader(&archive)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("read exported file system: %v", err)
		}
		name := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(h.Name, "./"), "/"), "/")
		e := fsEntry{header: h}
		if name == binaryPath {
			if e.data, err = io.ReadAll(tr); err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
		}
		entries[name] = e
	}
	return entries
}

// serverArch returns the architecture of the Docker daemon, the platform `make image` loads.
func serverArch(t *testing.T) string {
	t.Helper()
	return strings.TrimSpace(mustDocker(t, "version", "--format", "{{.Server.Arch}}"))
}

// readFile returns the trimmed content of a file relative to this package directory.
func readFile(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(name) //nolint:gosec // fixed repository path
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(raw))
}

func TestImageConfig(t *testing.T) {
	cfg := inspect(t, env(t, "BOTHY_TEST_IMAGE"))
	c := cfg.Config

	if c.User != "65532:65532" {
		t.Errorf("User = %q, want 65532:65532", c.User)
	}
	if !slices.Equal(c.Entrypoint, []string{"/usr/local/bin/bothy"}) {
		t.Errorf("Entrypoint = %q, want [/usr/local/bin/bothy]", c.Entrypoint)
	}
	if !slices.Equal(c.Cmd, []string{"serve"}) {
		t.Errorf("Cmd = %q, want [serve]", c.Cmd)
	}
	if c.Healthcheck == nil || !slices.Equal(c.Healthcheck.Test, healthcheckCommand) {
		t.Errorf("Healthcheck = %+v, want test %q", c.Healthcheck, healthcheckCommand)
	}
	if len(c.Volumes) != 0 {
		t.Errorf("Volumes = %v, want none (no anonymous volumes)", c.Volumes)
	}
	if len(c.ExposedPorts) != 0 {
		t.Errorf("ExposedPorts = %v, want none (ports are published by Compose, not the image)", c.ExposedPorts)
	}

	wantLabels := map[string]string{
		"org.opencontainers.image.title":    "bothy-server",
		"org.opencontainers.image.version":  readFile(t, versionFile),
		"org.opencontainers.image.licenses": "Apache-2.0",
	}
	for k, want := range wantLabels {
		if got := c.Labels[k]; got != want {
			t.Errorf("label %s = %q, want %q", k, got, want)
		}
	}
	if src := c.Labels["org.opencontainers.image.source"]; !strings.HasPrefix(src, "https://") {
		t.Errorf("label org.opencontainers.image.source = %q, want an https URL", src)
	}
	rev := c.Labels["org.opencontainers.image.revision"]
	if !regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`).MatchString(rev) {
		t.Errorf("label org.opencontainers.image.revision = %q, want a commit SHA", rev)
	}

	for _, kv := range c.Env {
		name, _, _ := strings.Cut(kv, "=")
		upper := strings.ToUpper(name)
		for _, word := range []string{"SECRET", "PASSWORD", "TOKEN", "KEY"} {
			if strings.Contains(upper, word) {
				t.Errorf("environment variable %s looks like a secret; secrets never go into images", name)
			}
		}
	}
}

// TestHealthcheckMatchesCompose holds the health check of the image equal to the anchor
// x-server-healthcheck of the Compose stack, command and timings.
func TestHealthcheckMatchesCompose(t *testing.T) {
	hc := inspect(t, env(t, "BOTHY_TEST_IMAGE")).Config.Healthcheck
	if hc == nil {
		t.Fatal("the image has no health check")
	}
	block := composeAnchor(t, readFile(t, composeFile), "x-server-healthcheck")

	quoted := make([]string, len(hc.Test))
	for i, s := range hc.Test {
		quoted[i] = strconv.Quote(s)
	}
	if want := "[" + strings.Join(quoted, ", ") + "]"; block["test"] != want {
		t.Errorf("compose test = %s, image test = %s", block["test"], want)
	}
	for key, got := range map[string]time.Duration{"interval": hc.Interval, "timeout": hc.Timeout, "start_period": hc.StartPeriod} {
		want, err := time.ParseDuration(block[key])
		if err != nil || got != want {
			t.Errorf("health check %s: image %v, compose %q", key, got, block[key])
		}
	}
	if block["retries"] != strconv.Itoa(hc.Retries) {
		t.Errorf("health check retries: image %d, compose %q", hc.Retries, block["retries"])
	}
}

// composeAnchor returns the "key: value" lines of the top-level block name in a Compose file.
func composeAnchor(t *testing.T, compose, name string) map[string]string {
	t.Helper()
	values := map[string]string{}
	in := false
	for line := range strings.SplitSeq(compose, "\n") {
		if strings.HasPrefix(line, name+":") {
			in = true
			continue
		}
		if !in {
			continue
		}
		if !strings.HasPrefix(line, " ") {
			break
		}
		if k, v, ok := strings.Cut(strings.TrimSpace(line), ":"); ok {
			values[k] = strings.TrimSpace(v)
		}
	}
	if len(values) == 0 {
		t.Fatalf("%s: no block %s", composeFile, name)
	}
	return values
}

func TestImageHasNoShell(t *testing.T) {
	image := env(t, "BOTHY_TEST_IMAGE")
	entries := exportFS(t, image)

	bin, ok := entries[binaryPath]
	switch {
	case !ok:
		t.Errorf("%s is missing", binaryPath)
	case bin.header.Uid != 0 || bin.header.Gid != 0 || bin.header.Mode&0o7777 != 0o555:
		t.Errorf("%s: owner %d:%d mode %o, want 0:0 mode 555", binaryPath, bin.header.Uid, bin.header.Gid, bin.header.Mode&0o7777)
	}
	for name := range entries {
		if slices.Contains(shells, filepath.Base(name)) {
			t.Errorf("the image contains %s", name)
		}
	}

	if code, _, _ := run(t, "--entrypoint", "/bin/sh", image, "-c", "true"); code == 0 {
		t.Error("docker run --entrypoint /bin/sh succeeded; the image must have no shell")
	}
}

// TestSigningKeyDir checks the empty key store of secrets.signing_key_dir: a named volume
// mounted there takes owner and mode from the image (18 "Server-Images" rule 3, ADR 0066).
func TestSigningKeyDir(t *testing.T) {
	e, ok := exportFS(t, env(t, "BOTHY_TEST_IMAGE"))[keyDir]
	if !ok {
		t.Fatalf("%s is missing", keyDir)
	}
	h := e.header
	if h.Typeflag != tar.TypeDir || h.Uid != nonroot || h.Gid != nonroot || h.Mode&0o7777 != 0o700 {
		t.Errorf("%s: type %c owner %d:%d mode %o, want a directory 65532:65532 mode 700", keyDir, h.Typeflag, h.Uid, h.Gid, h.Mode&0o7777)
	}
}

func TestBinaryIsStatic(t *testing.T) {
	image := env(t, "BOTHY_TEST_IMAGE")
	bin, ok := exportFS(t, image)[binaryPath]
	if !ok {
		t.Fatalf("%s is missing", binaryPath)
	}
	f, err := elf.NewFile(bytes.NewReader(bin.data))
	if err != nil {
		t.Fatalf("parse %s: %v", binaryPath, err)
	}
	defer func() { _ = f.Close() }()

	for _, p := range f.Progs {
		if p.Type == elf.PT_INTERP || p.Type == elf.PT_DYNAMIC {
			t.Errorf("%s has program header %v; a static binary has none", binaryPath, p.Type)
		}
	}
	libs, err := f.ImportedLibraries()
	if err != nil || len(libs) != 0 {
		t.Errorf("imported libraries = %v (%v), want none", libs, err)
	}

	machines := map[string]elf.Machine{"amd64": elf.EM_X86_64, "arm64": elf.EM_AARCH64}
	arch := serverArch(t)
	want, known := machines[arch]
	if !known {
		t.Fatalf("Docker daemon architecture %q is neither amd64 nor arm64", arch)
	}
	if f.Machine != want {
		t.Errorf("machine = %v, want %v for the host platform linux/%s", f.Machine, want, arch)
	}
	if cfg := inspect(t, image); cfg.Os != "linux" || cfg.Architecture != arch {
		t.Errorf("image platform = %s/%s, want linux/%s", cfg.Os, cfg.Architecture, arch)
	}
}

func TestReadOnlyRunAsNonRoot(t *testing.T) {
	// config check needs no variable on this base; BOTHY_DATABASE__HOST proves that the
	// environment reaches the loader inside the hardened container.
	code, stdout, stderr := run(t,
		"--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true",
		"--tmpfs", "/tmp", "--network", "none",
		"-e", "BOTHY_DATABASE__HOST=postgres",
		env(t, "BOTHY_TEST_IMAGE"), "config", "check")
	if code != 0 || !strings.Contains(stdout, "configuration ok") {
		t.Errorf("config check: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestHealthcheckPresent(t *testing.T) {
	code, _, stderr := run(t, "--read-only", "--network", "none",
		env(t, "BOTHY_TEST_IMAGE"), "healthcheck", "--url", "http://127.0.0.1:8081/readyz")
	if code != 1 || !strings.Contains(stderr, "bothy healthcheck:") {
		t.Errorf("healthcheck without a listener: exit %d, stderr %q; want exit 1 from bothy healthcheck", code, stderr)
	}
}

// ociDescriptor is a descriptor of the OCI image layout (index.json and image indexes).
type ociDescriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Platform  *struct {
		OS           string `json:"os"`
		Architecture string `json:"architecture"`
	} `json:"platform"`
}

// ociIndex is an OCI image index; index.json of the layout has the same form.
type ociIndex struct {
	Manifests []ociDescriptor `json:"manifests"`
}

const (
	mediaIndex       = "application/vnd.oci.image.index.v1+json"
	mediaDockerList  = "application/vnd.docker.distribution.manifest.list.v2+json"
	mediaManifest    = "application/vnd.oci.image.manifest.v1+json"
	mediaDockerImage = "application/vnd.docker.distribution.manifest.v2+json"
)

// TestMultiArchArchive checks that the OCI archive of make image holds exactly linux/amd64 and
// linux/arm64, and that both carry the version and revision labels of the loaded image, so the
// archive and the image under test come from the same build input.
func TestMultiArchArchive(t *testing.T) {
	loaded := inspect(t, env(t, "BOTHY_TEST_IMAGE")).Config.Labels
	files := readOCIArchive(t, env(t, "BOTHY_TEST_IMAGE_OCI"))
	images := ociImages(t, files, "index.json", 0)
	platforms := make([]string, 0, len(images))
	for _, img := range images {
		platforms = append(platforms, img.platform)
		labels := ociLabels(t, files, img.digest)
		for _, k := range []string{"org.opencontainers.image.version", "org.opencontainers.image.revision"} {
			if labels[k] != loaded[k] {
				t.Errorf("%s: label %s = %q, loaded image has %q (rerun make image)", img.platform, k, labels[k], loaded[k])
			}
		}
	}
	slices.Sort(platforms)
	if want := []string{"linux/amd64", "linux/arm64"}; !slices.Equal(platforms, want) {
		t.Errorf("platforms = %v, want %v", platforms, want)
	}
}

// readOCIArchive returns the regular files of the OCI archive at path by name.
func readOCIArchive(t *testing.T, path string) map[string][]byte {
	t.Helper()
	f, err := os.Open(path) //nolint:gosec // path from make image-test
	if err != nil {
		t.Fatalf("open the OCI archive: %v (run make image)", err)
	}
	defer func() { _ = f.Close() }()
	files := map[string][]byte{}
	tr := tar.NewReader(f)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return files
		}
		if err != nil {
			t.Fatalf("read the OCI archive: %v", err)
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		if files[strings.TrimPrefix(h.Name, "./")], err = io.ReadAll(tr); err != nil {
			t.Fatalf("read %s: %v", h.Name, err)
		}
	}
}

// ociImage is one image manifest of the archive: its platform as os/architecture and digest.
type ociImage struct {
	platform, digest string
}

// blobPath returns the archive path of the blob with digest ("sha256:<hex>").
func blobPath(digest string) string {
	return "blobs/" + strings.Replace(digest, ":", "/", 1)
}

// ociImages follows the index in files[name] down to the image manifests and returns them;
// attestation manifests (platform unknown/unknown) are left out.
func ociImages(t *testing.T, files map[string][]byte, name string, depth int) []ociImage {
	t.Helper()
	if depth > 3 {
		t.Fatalf("OCI archive: indexes nested deeper than 3 levels at %s", name)
	}
	var idx ociIndex
	readJSON(t, files, name, &idx)
	var images []ociImage
	for _, d := range idx.Manifests {
		switch d.MediaType {
		case mediaIndex, mediaDockerList:
			images = append(images, ociImages(t, files, blobPath(d.Digest), depth+1)...)
		case mediaManifest, mediaDockerImage:
			if d.Platform == nil {
				t.Errorf("OCI archive: manifest %s has no platform", d.Digest)
				continue
			}
			if p := d.Platform.OS + "/" + d.Platform.Architecture; p != "unknown/unknown" {
				images = append(images, ociImage{platform: p, digest: d.Digest})
			}
		default:
			t.Errorf("OCI archive: unexpected media type %q of %s", d.MediaType, d.Digest)
		}
	}
	return images
}

// ociLabels returns the labels of the image configuration that the manifest with digest names.
func ociLabels(t *testing.T, files map[string][]byte, digest string) map[string]string {
	t.Helper()
	var manifest struct {
		Config struct {
			Digest string `json:"digest"`
		} `json:"config"`
	}
	readJSON(t, files, blobPath(digest), &manifest)
	var config struct {
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"config"`
	}
	readJSON(t, files, blobPath(manifest.Config.Digest), &config)
	return config.Config.Labels
}

// readJSON decodes files[name] into v and fails the test when it is missing or invalid.
func readJSON(t *testing.T, files map[string][]byte, name string, v any) {
	t.Helper()
	raw, ok := files[name]
	if !ok {
		t.Fatalf("OCI archive: %s is missing", name)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("OCI archive: %s: %v", name, err)
	}
}

// containerExists reports whether a container with id exists, in any state.
func containerExists(t *testing.T, id string) bool {
	t.Helper()
	return strings.TrimSpace(mustDocker(t, "ps", "--all", "--quiet", "--no-trunc", "--filter", "id="+id)) != ""
}

// TestCleanupRemovesContainers proves that the helpers remove what they create once the test
// that created it has ended (brief M1-23: every container is removed by its ID in t.Cleanup).
func TestCleanupRemovesContainers(t *testing.T) {
	image := env(t, "BOTHY_TEST_IMAGE")

	var created string
	t.Run("create", func(t *testing.T) { created = createContainer(t, image) })
	if created == "" || containerExists(t, created) {
		t.Errorf("container %q from createContainer still exists after its test", created)
	}

	var cidFile string
	t.Run("cidfile", func(t *testing.T) {
		cidFile = filepath.Join(t.TempDir(), "cid")
		removeContainer(t, cidFile)
		id := strings.TrimSpace(mustDocker(t, "create", "--pull=never", "--cidfile", cidFile, image))
		created = id
	})
	if containerExists(t, created) {
		t.Errorf("container %s named in a cid file still exists after its test", created)
	}
}
