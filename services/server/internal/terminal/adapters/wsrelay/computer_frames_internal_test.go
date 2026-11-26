// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/specschema"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// computerOps are the computer ops a host service sends (27 12.3).
var computerOps = []string{opComputerStarted, opComputerState, opComputerEnded}

// computerHostFrames are one frame per computer op a host service sends, with the JSON member
// names of 27 12.3 (the contract with CU-03; the host recording holds the frames of its code).
var computerHostFrames = []string{
	`{"op":"computer.started","d":{"session_id":"0192f4a0-7b1c-7c33-8e5f-6a7b8c9d0e1f","state":"active"}}`,
	`{"op":"computer.state","d":{"session_id":"0192f4a0-7b1c-7c33-8e5f-6a7b8c9d0e1f","state":"paused","reason":"locked","action_count":7,` +
		`"front_app":{"id":"com.example.editor","name":"Editor","signer":"ABCDE12345"}}}`,
	`{"op":"computer.ended","d":{"session_id":"0192f4a0-7b1c-7c33-8e5f-6a7b8c9d0e1f","close_reason":"stopped_by_user",` +
		`"close_detail":"local_escape","action_counts":{"left_click":3,"type":2},` +
		`"apps":[{"id":"com.example.editor","name":"Editor","signer":"ABCDE12345"}],"paused_ms":1500}}`,
}

// TestComputerHostFramesContract (CU-04 Design note 4): every computer op of the host service
// passes the contract of hostControl and the strict codec of the server with the member names of
// 27 12.3; a frame with an unknown member or another close reason is refused.
func TestComputerHostFramesContract(t *testing.T) {
	t.Parallel()
	c, err := specschema.Default()
	if err != nil {
		t.Fatal(err)
	}
	check, err := newSchemas()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, f := range computerHostFrames {
		if err := c.ValidateChannelFrame("hostControl", false, json.RawMessage(f)); err != nil {
			t.Errorf("frame %s: %v", f, err)
		}
		fr, err := decodeFrame([]byte(f), 32, check)
		if err != nil {
			t.Fatalf("frame %s: %v", f, err)
		}
		seen[fr.op] = true
	}
	for _, op := range computerOps {
		if !seen[op] {
			t.Errorf("no frame of %s", op)
		}
	}
	for _, bad := range []string{
		`{"op":"computer.started","d":{"session_id":"0192f4a0-7b1c-7c33-8e5f-6a7b8c9d0e1f","state":"active","image":"x"}}`,
		`{"op":"computer.ended","d":{"session_id":"0192f4a0-7b1c-7c33-8e5f-6a7b8c9d0e1f","close_reason":"run_ended"}}`,
		`{"op":"computer.state","d":{"session_id":"0192f4a0-7b1c-7c33-8e5f-6a7b8c9d0e1f","state":"closed","reason":"","action_count":1,` +
			`"front_app":{"id":"","name":"","signer":""}}}`,
	} {
		if _, err := decodeFrame([]byte(bad), 32, check); err == nil {
			t.Errorf("frame %s decoded", bad)
		}
	}
	ended, _ := decodeFrame([]byte(computerHostFrames[2]), 32, check)
	end := computerEndOf(ended.d.(*rest.HostComputerEnded))
	if end.Reason != domain.ComputerStoppedByUser || end.Detail != domain.ComputerLocalEscape || end.ActionCounts["left_click"] != 3 ||
		len(end.Apps) != 1 || end.Apps[0] != "com.example.editor:ABCDE12345" || end.PausedMS != 1500 {
		t.Fatalf("computer.ended decoded as %+v", end)
	}
}

// TestComputerServerFramesContract: the computer ops the server sends pass the contract of
// hostControl with the member names of 27 12.3 (computer.open carries every setting of 27 18).
func TestComputerServerFramesContract(t *testing.T) {
	t.Parallel()
	c, err := specschema.Default()
	if err != nil {
		t.Fatal(err)
	}
	id := ids.MustParse("0192f4a0-7b1c-7c33-8e5f-6a7b8c9d0e1f")
	frames := [][]byte{
		computerOpenFrame(app.ComputerOpenOp{SessionID: id, RunID: id, AgentID: id, UserID: id, AgentName: "helper", DataPath: app.ComputerDataPath(id),
			ExpiresAt: recordingTime, Settings: app.ComputerSettings{ImageMaxEdge: 1280, JPEGQuality: 80, KeyRepeatMax: 50, AuditAppsMax: 32,
				Settle: 500 * time.Millisecond, WaitMax: time.Minute, StatusInterval: 2 * time.Second, AutoScreenshot: true}}),
		computerCloseFrame(id, domain.ComputerRunEnded),
		computerDisableFrame(4),
	}
	for _, f := range frames {
		if err := c.ValidateChannelFrame("hostControl", true, f); err != nil {
			t.Errorf("server frame %s: %v", f, err)
		}
	}
	var open struct {
		D map[string]any `json:"d"`
	}
	if err := json.Unmarshal(frames[0], &open); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"session_id", "run_id", "agent_id", "agent_name", "user_id", "data_path", "expires_at", "settings"} {
		if _, ok := open.D[k]; !ok {
			t.Errorf("computer.open without %s", k)
		}
	}
}
