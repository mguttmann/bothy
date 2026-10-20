// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// fileScope is the scope of the file name canaries: no observer may read it, because before
// attachment only the uploader reads a file (05 file.read, M3-03), also where the observer reads
// the scope channel (Sam reads Support/Allgemein).
const fileScope = "file before attachment"

// files drives the upload sessions with a canary file name (SEC-T10 path "file name", M3-03):
// Alex creates an upload whose name carries the canary, sends its part, completes it and reads
// it with getFile and getUpload, aborts a second upload with the same name, and provokes a 413
// and a 400 with it. The name may reach Alex's own answers only.
func (sc *Scenario) files() {
	sc.T.Helper()
	alex := sc.W.People["Alex"]
	ch := sc.W.Channels["Support/Allgemein"]
	name := sc.Reg.New(ClassContent, fileScope, "file name") + ".txt"
	body := []byte("canary part")
	sum := sha256.Sum256(body)
	upload := map[string]any{"name": name, "size_bytes": len(body), "sha256": hex.EncodeToString(sum[:])}

	file := str(obj(sc.post(alex, "/channels/"+ch+"/uploads", upload, created...)), "file_id")
	h := header("Content-Type", "application/octet-stream", "Content-Digest", "sha-256=:"+base64.StdEncoding.EncodeToString(sum[:])+":")
	etag := str(obj(sc.call(alex, req{method: http.MethodPut, path: "/uploads/" + file + "/parts/1", body: body, header: h}, ok...)), "etag")
	sc.get(alex, "/uploads/"+file, ok...)
	sc.post(alex, "/uploads/"+file+"/complete", map[string]any{"parts": []map[string]any{{"part_number": 1, "etag": etag}}}, ok...)
	sc.get(alex, "/files/"+file, ok...)
	sc.downloads(alex, file)

	second := str(obj(sc.post(alex, "/channels/"+ch+"/uploads", upload, created...)), "file_id")
	sc.del(alex, "/uploads/"+second, none...)

	limit, err := strconv.ParseInt(sc.Server.ConfigValue("file.max_upload_bytes"), 10, 64)
	if err != nil {
		sc.T.Errorf("file.max_upload_bytes: %v", err)
	}
	sc.post(alex, "/channels/"+ch+"/uploads", map[string]any{"name": name, "size_bytes": limit + 1, "sha256": hex.EncodeToString(sum[:])},
		http.StatusRequestEntityTooLarge)
	sc.post(alex, "/channels/"+ch+"/uploads", map[string]any{"name": name, "size_bytes": len(body), "sha256": "not hex"}, http.StatusBadRequest)
}

// downloads drives the read half of 12 9 on the file of files (M3-05): it waits until the scan
// pass made it deliverable (ScenarioConfig switches the scan off), downloads it with a range,
// asks for a thumbnail it does not have and for its transcript, and creates a download ticket.
// The ticket is a bearer secret (12 9.2 point 4, ADR 0048 "Confirmation"): it is registered as a
// secret and fetched once without credentials, so no sink may carry it.
func (sc *Scenario) downloads(alex *harness.Client, file string) {
	sc.T.Helper()
	deadline := time.Now().Add(time.Minute)
	for str(obj(sc.get(alex, "/files/"+file, ok...)), "scan_status") == "pending" {
		if time.Now().After(deadline) {
			sc.T.Errorf("file of the canary run still pending")
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	sc.call(alex, req{method: http.MethodGet, path: "/files/" + file + "/content", header: header("Range", "bytes=0-3")}, http.StatusPartialContent)
	sc.get(alex, "/files/"+file+"/thumbnails/small", http.StatusNotFound)
	sc.get(alex, "/files/"+file+"/transcript", ok...)
	url := str(obj(sc.post(alex, "/files/"+file+"/tickets", map[string]any{"variant": "original"}, created...)), "url")
	_, ticket, found := strings.Cut(url, "?ticket=")
	if !found {
		sc.T.Errorf("createFileTicket answered no ticket URL")
		return
	}
	sc.Reg.Register(ticket, ClassSecret, "", "download ticket")
	sc.anon(alex, req{method: http.MethodGet, path: strings.TrimPrefix(url, "/api/v1")}, ok...)
}
