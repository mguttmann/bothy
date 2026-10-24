// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"strings"
	"testing"
)

// TestWriteCodesKeepsTheCauseOfAFailedStart: a server that refuses its start logs only
// "start failed" with error.type=start and prints the cause on stderr after its binary name and
// command, for example "bothy serve: ..." (cmd/bothy serve.go). Diagnose keeps that line, so a
// stack that does not become healthy names its cause; any other line without JSON is only counted.
func TestWriteCodesKeepsTheCauseOfAFailedStart(t *testing.T) {
	logs := strings.Join([]string{
		`api-1  | {"time":"x","level":"ERROR","msg":"start failed","error.type":"start","http.request.body":"secret"}`,
		`api-1  | bothy serve: config: terminal.idle_timeout: must be positive`,
		`api-1  | some other text with a value`,
		`bothy migrate: database: role missing`,
	}, "\n")
	var b bytes.Buffer
	WriteCodes(&b, []byte(logs))
	got := b.String()
	for _, want := range []string{
		"level=ERROR msg=start failed error.type=start",
		"bothy serve: config: terminal.idle_timeout: must be positive",
		"bothy migrate: database: role missing",
		"(1 lines without JSON not shown)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the diagnosis lacks %q:\n%s", want, got)
		}
	}
	for _, never := range []string{"secret", "some other text"} {
		if strings.Contains(got, never) {
			t.Errorf("the diagnosis shows %q:\n%s", never, got)
		}
	}
}
