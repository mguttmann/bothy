// SPDX-License-Identifier: Apache-2.0

package hostos

import (
	"errors"
	"slices"
	"testing"
)

// TestDarwinGroupIDs: the IDs come from "dscl . -list /Groups PrimaryGroupID" of the local node;
// a failed or refused listing is an error, never an empty list (install would then pick an ID
// that is taken).
func TestDarwinGroupIDs(t *testing.T) {
	a := &answer{out: Output{Stdout: []byte("com.apple.access_remote_ae       400\nstaff 20\n_amavisd   83\nbroken\nnoid  x\n")}}
	ids, err := DarwinGroupIDs(a)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids, []int{400, 20, 83}) {
		t.Errorf("ids %v", ids)
	}
	if len(a.got) != 1 || a.got[0].Path != "/usr/bin/dscl" || !slices.Equal(a.got[0].Args, []string{".", "-list", "/Groups", "PrimaryGroupID"}) {
		t.Errorf("command %+v", a.got)
	}
	if _, err := DarwinGroupIDs(&answer{out: Output{ExitCode: 40}}); err == nil {
		t.Error("a refused listing gave no error")
	}
	if _, err := DarwinGroupIDs(&answer{out: Output{TimedOut: true}}); err == nil {
		t.Error("a timed out listing gave no error")
	}
	if _, err := DarwinGroupIDs(&answer{err: errors.New("no dscl")}); err == nil {
		t.Error("a failed start gave no error")
	}
}
