// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestWindowsRequirementIsTheSubjectDN ties the requirement the Windows package carries to the
// form the signature check compares: hostdist bakes HELPER_CERT_SUBJECT ("CN=...") into
// buildinfo.ComputerHelperRequirement, and signerSubjectIs reads the signer's subject as an X.500
// string. Comparing the simple display name ("bothy-computer-helper (local)") refused every helper
// on a Windows 11 test machine (reason signature). The test reads both sources, so it runs on
// every platform.
func TestWindowsRequirementIsTheSubjectDN(t *testing.T) {
	py, err := os.ReadFile("../../../../tools/distribution/hostdist.py")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^HELPER_CERT_SUBJECT = "([^"]*)"`).FindSubmatch(py)
	if m == nil || !strings.HasPrefix(string(m[1]), "CN=") {
		t.Fatalf("HELPER_CERT_SUBJECT is no distinguished name: %q", m)
	}
	src, err := os.ReadFile("procexe_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "windows.CERT_NAME_RDN_TYPE") || !strings.Contains(string(src), "certX500NameStr") ||
		strings.Contains(string(src), "CERT_NAME_SIMPLE_DISPLAY_TYPE") {
		t.Error("signerSubjectIs does not compare the subject as an X.500 string")
	}
}
