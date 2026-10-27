// SPDX-License-Identifier: Apache-2.0

package hostos

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
)

// DarwinGroupIDs returns the PrimaryGroupID of every group of the local directory node of macOS
// ("dscl . -list /Groups PrimaryGroupID"). dscl refuses to give a group an ID another group of
// the node has (eDSRecordAlreadyExists), so install picks the IDs of new accounts outside them.
func DarwinGroupIDs(r Runner) ([]int, error) {
	out, err := r.Run(context.Background(), Cmd{Path: "/usr/bin/dscl", Args: []string{".", "-list", "/Groups", "PrimaryGroupID"}, Timeout: probeTimeoutAll})
	if err != nil {
		return nil, err
	}
	if out.ExitCode != 0 || out.TimedOut {
		return nil, fmt.Errorf("dscl -list /Groups: exit code %d", out.ExitCode)
	}
	return ParseDsclIDs(out.Stdout), nil
}

// ParseDsclIDs parses the output of "dscl . -list <node> <ID attribute>": one record per line,
// its name and its numeric ID separated by spaces. Lines without a numeric last field are
// skipped.
func ParseDsclIDs(b []byte) []int {
	var ids []int
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 2 {
			continue
		}
		if n, err := strconv.Atoi(f[len(f)-1]); err == nil {
			ids = append(ids, n)
		}
	}
	return ids
}
