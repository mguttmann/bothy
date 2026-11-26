// SPDX-License-Identifier: Apache-2.0

//go:build computere2e

package computer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/test/terminal/stack"
	"github.com/OWNER/bothy/services/server/test/terminal/suite"
)

// The runner machine: the account of the runner, its folders, and the copies of the runner, the
// SDK, their dependencies, Fake-Claude and bothy-computer-mcp under /opt/cu (owned by root, not
// writable by group or others, as runner.computer_mcp_bin requires).
const (
	runnerUser = "robin"
	runnerHome = "/home/" + runnerUser
	// workDir holds the marks and gates of the script, the capture of Fake-Claude and its output.
	workDir    = runnerHome + "/cu"
	stateDir   = runnerHome + "/state"
	secretsDir = runnerHome + "/secrets"
	optDir     = "/opt/cu"
	mcpBin     = optDir + "/bin/bothy-computer-mcp"
	claudeBin  = optDir + "/bin/claude"
	scenario   = optDir + "/claude.scenario.json"
	runnerUnit = "cu-runner"
	// callsFile is the output of the scripted agent, captureFile the capture of Fake-Claude.
	callsFile   = workDir + "/computer.jsonl"
	captureFile = workDir + "/capture.jsonl"
	// apiKey is the key of the organization billing the turn receives; Fake-Claude calls no API.
	apiKey = "sk-ant-cu-t24-0123456789" //nolint:gosec // G101: a fake key; Fake-Claude calls no API
)

// runnerMachine is the container of the runner.
type runnerMachine struct {
	h *stack.Host
}

// newRunnerMachine creates the runner's account and folders.
func newRunnerMachine(t *testing.T, h *stack.Host) *runnerMachine {
	t.Helper()
	shell(t, h, "useradd --create-home --shell /bin/bash "+runnerUser+
		" && install -d -o "+runnerUser+" -g "+runnerUser+" -m 0700 "+workDir+" "+stateDir+" "+secretsDir+" "+runnerHome+"/projects "+
		runnerHome+"/projects/desk "+runnerHome+"/projects/desk/.git"+
		" && install -d -o root -g root -m 0755 "+optDir+" "+optDir+"/py "+optDir+"/bin")
	return &runnerMachine{h: h}
}

// mark and gate name the files of a mark the script writes and of a gate the test opens.
func (r *runnerMachine) mark(name string) string { return workDir + "/" + name + ".reached" }

func (r *runnerMachine) gate(name string) string { return workDir + "/" + name + ".open" }

// install copies the runner of this tree with its dependencies, Fake-Claude with the script and
// bothy-computer-mcp into the machine and writes the credentials and runner.toml (files of mode
// 0600 of the runner's account, written through stdin, never on the disk of the test machine).
func (r *runnerMachine) install(t *testing.T, env *suite.Env, ag agent, steps []map[string]any) {
	t.Helper()
	ctx := t.Context()
	root := env.Repo.Root
	work := t.TempDir()
	py := pythonVersion(t, r.h)
	site := runnerSite(t, root, work, env.Stack.Arch, py)
	mcp := buildMCP(t, root, work, env.Stack.Arch)
	copies := map[string]string{
		filepath.Join(root, "plugins", "claude-code-runner", "src", "claude_code_runner"): optDir + "/py/claude_code_runner",
		filepath.Join(root, "sdk", "python", "src", "bothy_sdk"):                          optDir + "/py/bothy_sdk",
		filepath.Join(root, "plugins", "claude-code-runner", "tests", "fake_claude"):      optDir + "/fake_claude",
		site: optDir + "/site",
		mcp:  mcpBin,
	}
	for src, dst := range copies {
		if err := r.h.CopyIn(ctx, src, dst); err != nil {
			t.Fatal(err)
		}
	}
	scen, err := json.Marshal(map[string]any{"fixture": "plain", "computer": map[string]any{"out": callsFile, "steps": steps}})
	if err != nil {
		t.Fatal(err)
	}
	wrapper := "#!/usr/bin/python3\nimport sys\nsys.path.insert(0, " + quote(optDir+"/fake_claude") + ")\nimport fake_claude\n" +
		"sys.exit(fake_claude.main(" + quote(scenario) + ", " + quote(captureFile) + "))\n"
	write(t, r.h, "root", scenario, string(scen))
	write(t, r.h, "root", claudeBin, wrapper)
	shell(t, r.h, "chown -R root:root "+optDir+" && chmod -R a+rX,go-w "+optDir+" && chmod 0755 "+claudeBin+" "+mcpBin)
	write(t, r.h, runnerUser, secretsDir+"/agent_token", ag.token)
	write(t, r.h, runnerUser, secretsDir+"/dpop.pem", string(ag.dpopPEM))
	write(t, r.h, runnerUser, secretsDir+"/api_key", apiKey+"\n")
	values := []struct {
		key string
		val any
	}{
		{"server_url", stack.InstanceOrigin},
		{"instance_id", ag.instanceID},
		{"agent_token_ref", "file:" + secretsDir + "/agent_token"},
		{"dpop_key_ref", "file:" + secretsDir + "/dpop.pem"},
		{"anthropic_api_key_ref", "file:" + secretsDir + "/api_key"},
		{"consumer", "cu-t24"},
		{"default_model", "model-large"},
		{"projects_root", runnerHome + "/projects"},
		{"default_workspace", "desk"},
		{"state_dir", stateDir},
		{"tool_class", "host"},
		{"claude_bin", claudeBin},
		{"computer_use", true},
		{"computer_mcp_bin", mcpBin},
		{"computer_request_timeout", "30s"},
	}
	var toml strings.Builder
	toml.WriteString("[runner]\n")
	for _, v := range values {
		b, err := json.Marshal(v.val)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&toml, "%s = %s\n", v.key, b)
	}
	write(t, r.h, runnerUser, runnerHome+"/runner.toml", toml.String())
}

// start starts the runner as its account (a transient systemd unit); its JSON log lines go to the
// journal of the unit.
func (r *runnerMachine) start(t *testing.T) {
	t.Helper()
	shell(t, r.h, "systemd-run --quiet --unit="+runnerUnit+" --uid="+runnerUser+" --gid="+runnerUser+
		" --working-directory="+runnerHome+" --setenv=HOME="+runnerHome+" --setenv=USER="+runnerUser+
		" --setenv=LOGNAME="+runnerUser+" --setenv=PATH="+optDir+"/bin:/usr/bin:/bin --setenv=LANG=C.UTF-8"+
		" --setenv=PYTHONPATH="+optDir+"/py:"+optDir+"/site --setenv=SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt"+
		" /usr/bin/python3 -m claude_code_runner run --config "+runnerHome+"/runner.toml")
	t.Cleanup(func() {
		if t.Failed() {
			r.diagnose(t)
		}
	})
}

// diagnose prints the codes of the runner's last log lines (level, logger, message and code
// members only, never a value of another member) and whether the unit still runs.
func (r *runnerMachine) diagnose(t *testing.T) {
	// t.Context is cancelled before the cleanups run.
	ctx := context.WithoutCancel(t.Context())
	if st, err := r.h.Exec(ctx, nil, "systemctl", "is-active", runnerUnit); err == nil {
		t.Logf("runner unit: %s", strings.TrimSpace(string(st.Stdout)))
	}
	j, err := r.h.Exec(ctx, nil, "journalctl", "--no-pager", "-o", "cat", "-n", "60", "-u", runnerUnit)
	if err != nil {
		return
	}
	sc := bufio.NewScanner(bytes.NewReader(j.Stdout))
	for sc.Scan() {
		var line map[string]any
		if json.Unmarshal(sc.Bytes(), &line) != nil {
			t.Logf("runner: (not JSON, %d bytes)", len(sc.Bytes()))
			continue
		}
		var parts []string
		for _, k := range []string{"level", "logger", "msg", "message", "event", "code", "reason"} {
			if v, ok := line[k]; ok {
				parts = append(parts, fmt.Sprintf("%s=%v", k, v))
			}
		}
		t.Logf("runner: %s", strings.Join(parts, " "))
	}
}

// waitMark waits until the script wrote the mark name.
func (r *runnerMachine) waitMark(t *testing.T, name string) {
	t.Helper()
	until(t, "the mark "+name+" of the scripted agent", func() bool {
		res, err := r.h.Exec(t.Context(), nil, "test", "-e", r.mark(name))
		return err == nil && res.Code == 0
	})
}

// openGate opens the gate name of the script.
func (r *runnerMachine) openGate(t *testing.T, name string) {
	t.Helper()
	if res, err := r.h.ExecAs(t.Context(), runnerUser, nil, "touch", r.gate(name)); err != nil || res.Code != 0 {
		t.Fatalf("gate %s: %v", name, err)
	}
}

// calls returns the output lines of the scripted agent.
func (r *runnerMachine) calls(t *testing.T) []call {
	t.Helper()
	res, err := r.h.Exec(t.Context(), nil, "cat", callsFile)
	if err != nil || res.Code != 0 {
		t.Fatalf("the output of the scripted agent: %v", err)
	}
	var out []call
	for _, l := range bytes.Split(bytes.TrimSpace(res.Stdout), []byte("\n")) {
		var c call
		if err := json.Unmarshal(l, &c); err != nil {
			t.Fatalf("an output line of the scripted agent: %v", err)
		}
		out = append(out, c)
	}
	return out
}

// checkTurn checks the turn of the run on the runner side: Fake-Claude got --mcp-config directly
// after --permission-prompts none (27 10.1) and started the one server of the file with its five
// variables, and after the run neither the per-turn MCP file nor the request folder of the run is
// left (27 10.1, 10.2).
func (r *runnerMachine) checkTurn(t *testing.T, runID string) {
	t.Helper()
	ctx := t.Context()
	res, err := r.h.Exec(ctx, nil, "cat", captureFile)
	if err != nil || res.Code != 0 {
		t.Fatalf("the capture of Fake-Claude: %v", err)
	}
	var turns, servers int
	for _, l := range bytes.Split(bytes.TrimSpace(res.Stdout), []byte("\n")) {
		var rec struct {
			Kind    string   `json:"kind"`
			Argv    []string `json:"argv"`
			Command string   `json:"command"`
			EnvKeys []string `json:"env_keys"`
		}
		if err := json.Unmarshal(l, &rec); err != nil {
			t.Fatalf("a capture line: %v", err)
		}
		switch rec.Kind {
		case "turn_start":
			turns++
			i := indexOf(rec.Argv, "--permission-prompts")
			if i < 0 || i+3 >= len(rec.Argv) || rec.Argv[i+1] != "none" || rec.Argv[i+2] != "--mcp-config" {
				t.Errorf("the turn's arguments do not carry --mcp-config after --permission-prompts none")
			}
		case "computer_server":
			servers++
			want := []string{"BOTHY_COMPUTER_DIR", "BOTHY_COMPUTER_NONCE", "BOTHY_COMPUTER_PINS", "BOTHY_COMPUTER_SERVER", "BOTHY_COMPUTER_TIMEOUT_MS"}
			if rec.Command != mcpBin || strings.Join(rec.EnvKeys, ",") != strings.Join(want, ",") {
				t.Errorf("the MCP server of the turn: %s with %v", rec.Command, rec.EnvKeys)
			}
		}
	}
	t.Logf("runner: %d turn with --mcp-config after --permission-prompts none, %d computer server with the five variables", turns, servers)
	if turns != 1 || servers != 1 {
		t.Errorf("%d turns and %d computer servers, want one each", turns, servers)
	}
	until(t, "the runner removes the per-turn MCP file and the request folder", func() bool {
		ls, err := r.h.Exec(ctx, nil, "bash", "-c", "ls "+stateDir+"/turns/"+runID+".computer.mcp.json "+stateDir+"/computer-requests/"+runID+" 2>/dev/null | wc -l")
		return err == nil && ls.Code == 0 && strings.TrimSpace(string(ls.Stdout)) == "0"
	})
}

// indexOf returns the index of s in list or -1.
func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}

// write writes content into path in h as user (mode 0600 for the runner's account, 0644 for
// root), through stdin.
func write(t *testing.T, h *stack.Host, user, path, content string) {
	t.Helper()
	mode := "0644"
	if user != "root" {
		mode = "0600"
	}
	script := "umask 077 && cat > " + path + " && chmod " + mode + " " + path
	var res stack.ExecResult
	var err error
	if user == "root" {
		res, err = h.Exec(t.Context(), strings.NewReader(content), "bash", "-c", script)
	} else {
		res, err = h.ExecAs(t.Context(), user, strings.NewReader(content), "bash", "-c", script)
	}
	if err != nil || res.Code != 0 {
		t.Fatalf("write %s in %s: %v %d", path, h.Name, err, res.Code)
	}
}

// quote returns s as a Python string literal.
func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// pythonVersion returns major.minor of the python3 of h.
func pythonVersion(t *testing.T, h *stack.Host) string {
	t.Helper()
	r, err := h.Exec(t.Context(), nil, "python3", "-c", "import sys; print('%d.%d' % sys.version_info[:2])")
	if err != nil || r.Code != 0 {
		t.Fatalf("python3 of %s: %v", h.Name, err)
	}
	return strings.TrimSpace(string(r.Stdout))
}

// runnerSite installs the third-party dependencies of the runner (the locked versions of
// plugins/claude-code-runner/uv.lock without the development group and without the local
// packages, which the machine gets as source) for linux/arch and python into a folder: uv export,
// then uv pip install --target with the platform and the hashes of the lock.
func runnerSite(t *testing.T, root, work, arch, python string) string {
	t.Helper()
	platform := map[string]string{"amd64": "x86_64-manylinux_2_28", "arm64": "aarch64-manylinux_2_28"}[arch]
	if platform == "" {
		t.Fatalf("no manylinux platform for %s", arch)
	}
	reqs := filepath.Join(work, "requirements.txt")
	site := filepath.Join(work, "site")
	steps := [][]string{
		{"uv", "export", "--frozen", "--no-dev", "--no-emit-project", "--no-emit-local", "--format", "requirements-txt", "--quiet", "--output-file", reqs},
		{"uv", "pip", "install", "--quiet", "--target", site, "--python-platform", platform, "--python-version", python,
			"--only-binary", ":all:", "--require-hashes", "--requirement", reqs},
	}
	for _, args := range steps {
		cmd := exec.CommandContext(t.Context(), args[0], args[1:]...) //nolint:gosec // G204: fixed programs, paths of the test's temp dir
		cmd.Dir = filepath.Join(root, "plugins", "claude-code-runner")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", strings.Join(args[:2], " "), err, out)
		}
	}
	return site
}

// buildMCP builds bothy-computer-mcp of this tree for linux/arch without cgo.
func buildMCP(t *testing.T, root, work, arch string) string {
	t.Helper()
	bin := filepath.Join(work, "bothy-computer-mcp")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-o", bin, "./cmd/bothy-computer-mcp") //nolint:gosec // G204: fixed program, a path of the test's temp dir
	cmd.Dir = filepath.Join(root, "services", "host")
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+arch, "CGO_ENABLED=0", "GOTOOLCHAIN=local") //nolint:forbidigo // go needs PATH, HOME and the caches of the machine
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build bothy-computer-mcp: %v\n%s", err, out)
	}
	return bin
}

// waitTick is the pause between two looks of a wait (a test constant).
const waitTick = 250 * time.Millisecond
