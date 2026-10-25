// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/filestest"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/transcription/transcriptiontest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// VoiceRunOptions start the fake transcription service of M3-09 and add to o the keys that send
// voice messages to it: voice.transcription_mode service, voice.transcription_url, and its
// loopback port in egress.allowed_hosts next to the entries o already has. The returned fake
// answers with the transcript the voice step programs.
func VoiceRunOptions(t testing.TB, o RunOptions) (RunOptions, *transcriptiontest.Server) {
	t.Helper()
	fake := transcriptiontest.New(t)
	cfg := map[string]string{}
	for k, v := range o.Config {
		cfg[k] = v
	}
	hosts := fake.Rule()
	if prev := cfg["egress.allowed_hosts"]; prev != "" {
		hosts = prev + "," + hosts
	}
	cfg["egress.allowed_hosts"] = hosts
	cfg["voice.transcription_mode"] = "service"
	cfg["voice.transcription_url"] = fake.URL()
	o.Config = cfg
	return o, fake
}

// voice drives the voice messages and transcripts of M3-09 (SEC-T10, 12 14): the Owner switches
// voice.enabled on, Alex uploads a generated .m4a and sends it as a voice message into the main
// thread of Support/Allgemein, the job files.transcribe sends it to the fake service, which
// answers with a canary transcript, Alex reads it with getTranscript and asks requestTranscription
// again, and Lotse sends submitTranscript with a second canary, which mode service refuses. The
// transcripts are content of Support/Allgemein: they may reach the database and authorized
// response bodies only, never a log, trace, metric, audit row, event payload or problem body.
func (sc *Scenario) voice(fake *transcriptiontest.Server) {
	sc.T.Helper()
	alex, lotse, owner := sc.W.People["Alex"], sc.W.Agents["Lotse"], sc.W.Owner
	ch := sc.W.Channels["Support/Allgemein"]
	text := sc.Reg.New(ClassContent, ch, "service transcript")
	answer, err := json.Marshal(map[string]string{"text": text, "language": "en"})
	if err != nil {
		sc.T.Fatal(err)
	}
	fake.Respond(http.StatusOK, string(answer))
	sc.put(owner, "/feature-flags/voice.enabled", map[string]any{"enabled": true, "reason": sc.Reg.New(ClassAdminReason, "", "voice flag reason")}, ok...)

	body := filestest.M4A(1000, 2500, filestest.AudioOptions{})
	sum := sha256.Sum256(body)
	file := str(obj(sc.post(alex, "/channels/"+ch+"/uploads", map[string]any{"name": sc.Reg.New(ClassContent, ch, "voice file name") + ".m4a",
		"size_bytes": len(body), "sha256": hex.EncodeToString(sum[:])}, created...)), "file_id")
	h := header("Content-Type", "application/octet-stream", "Content-Digest", "sha-256=:"+base64.StdEncoding.EncodeToString(sum[:])+":")
	etag := str(obj(sc.call(alex, req{method: http.MethodPut, path: "/uploads/" + file + "/parts/1", body: body, header: h}, ok...)), "etag")
	sc.post(alex, "/uploads/"+file+"/complete", map[string]any{"parts": []map[string]any{{"part_number": 1, "etag": etag}}}, ok...)
	testkit.Until(sc.T, "the voice file of the canary run complete", func() bool {
		return str(obj(sc.get(alex, "/files/"+file, ok...)), "upload_state") == "complete"
	})
	sc.post(alex, "/threads/"+sc.W.Threads["Support/Allgemein"]+"/messages", map[string]any{
		"client_msg_id": newID(), "kind": "voice", "content_md": "", "attachment_file_ids": []string{file}}, created...)
	testkit.Until(sc.T, "the server transcript of the canary run done", func() bool {
		return str(obj(sc.get(alex, "/files/"+file, ok...)), "transcript_status") == "done"
	})
	if tr := sc.get(alex, "/files/"+file+"/transcript", ok...); !strings.Contains(string(tr.Body), text) {
		sc.T.Errorf("getTranscript does not answer the transcript of the service")
	}
	sc.call(alex, req{method: http.MethodPost, path: "/files/" + file + "/transcribe", header: header("Idempotency-Key", newID())}, http.StatusAccepted)
	cur := sc.get(lotse, "/files/"+file, ok...)
	sc.call(lotse, req{method: http.MethodPut, path: "/files/" + file + "/transcript", header: header("If-Match", cur.Header.Get("ETag")),
		body: map[string]any{"text": sc.Reg.New(ClassContent, ch, "agent transcript"), "language": "en"}}, http.StatusConflict)
}
