// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"os"
	"regexp"
	"strconv"
	"testing"
)

// TestPipeClientsAskOnlyForTheInteractiveGrant ties every client of the helper pipes to their
// DACL (docs/spec/27-computer-use.md 9.4): a client's CreateFile asks for no right that pipeSDDL
// grants interactive users (IU) not, or Windows refuses it with ERROR_ACCESS_DENIED. The computer
// helper asked for GENERIC_READ | GENERIC_WRITE, which includes FILE_APPEND_DATA (on a pipe
// FILE_CREATE_PIPE_INSTANCE), FILE_WRITE_EA and FILE_WRITE_ATTRIBUTES; on a Windows 11 test machine it never
// connected and the host reported it absent. The test reads the sources
// themselves, so it runs on every platform: the Windows file of this package and the helper's
// header that its pipe client uses.
func TestPipeClientsAskOnlyForTheInteractiveGrant(t *testing.T) {
	goSrc := read(t, "peer_windows.go")
	iu := hexOf(t, goSrc, `\(A;;(0x[0-9a-fA-F]+);;;IU\)`, "the IU entry of pipeSDDL")
	agent := hexOf(t, goSrc, `clientAccess\s*=\s*(0x[0-9a-fA-F]+)`, "clientAccess")
	helper := hexOf(t, read(t, "../../desktop/windows/include/bothy/helper/trust.hpp"),
		`kPipeClientAccess\s*=\s*(0x[0-9a-fA-F]+)`, "kPipeClientAccess of the helper")
	for name, mask := range map[string]uint64{"confirmation agent": agent, "computer helper": helper} {
		if extra := mask &^ iu; extra != 0 {
			t.Errorf("%s asks for %#x, beyond the IU grant %#x by %#x", name, mask, iu, extra)
		}
		// FILE_READ_DATA and FILE_WRITE_DATA carry the channel; READ_CONTROL reads the pipe's owner
		// (the helper's server check, D-CU-02-5).
		if need := uint64(0x1 | 0x2 | 0x20000); mask&need != need {
			t.Errorf("%s asks for %#x without read, write and READ_CONTROL", name, mask)
		}
	}
	src := read(t, "../../desktop/windows/src/win32/pipe_channel.cpp")
	if !regexp.MustCompile(`CreateFileW\(pipe_name_\.c_str\(\),\s*kPipeClientAccess,`).MatchString(src) {
		t.Error("the helper's pipe client does not open the pipe with kPipeClientAccess")
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // G304: fixed source paths of this repository
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func hexOf(t *testing.T, src, pattern, what string) uint64 {
	t.Helper()
	m := regexp.MustCompile(pattern).FindStringSubmatch(src)
	if m == nil {
		t.Fatalf("%s not found", what)
	}
	v, err := strconv.ParseUint(m[1][2:], 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
