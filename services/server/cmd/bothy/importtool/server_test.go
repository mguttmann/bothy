// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/restclient"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
)

// TestRefusedBatchErrorNamesPlaceRuleAndItem: the error of a refused batch names the thread of the
// batch and, per field error of the server, the pointer, the rule and the origin key of the item
// the pointer names; a pointer outside the batch gets no key.
func TestRefusedBatchErrorNamesPlaceRuleAndItem(t *testing.T) {
	thread := "thread:13001001"
	b := restclient.ImportBatch{ThreadOriginKey: &thread, Items: []restclient.ImportObject{
		{OriginKey: "thread:13001001"}, {OriginKey: "message:13002002"}, {OriginKey: "pin:13002002"}}}
	body := `{"code":"validation_failed","status":400,"detail":"zq-detail","errors":[
		{"pointer":"/items/1/data/content_md","code":"empty","detail":"zq-field-detail"},
		{"pointer":"/items/2/data/message_origin_key","code":"limit_reached"},
		{"pointer":"/items/7/data/name","code":"too_long"},
		{"pointer":"/thread_origin_key","code":"invalid"},
		{"pointer":"","code":"invalid"}]}`
	err := batchError(&http.Response{StatusCode: http.StatusBadRequest}, []byte(body), b)
	want := "bothy: applyImportBatch: status 400 validation_failed, batch thread:13001001, " +
		"/items/1/data/content_md empty of message:13002002, /items/2/data/message_origin_key limit_reached of pin:13002002, " +
		"/items/7/data/name too_long, /thread_origin_key invalid, invalid"
	if err.Error() != want {
		t.Errorf("error text:\n got %s\nwant %s", err.Error(), want)
	}

	structure := restclient.ImportBatch{Items: []restclient.ImportObject{{OriginKey: "space:13000101"}, {OriginKey: "channel:13000103"}}}
	err = batchError(&http.Response{StatusCode: http.StatusBadRequest},
		[]byte(`{"code":"validation_failed","errors":[{"pointer":"/items/1/data/topic","code":"too_long"}]}`), structure)
	if want := "bothy: applyImportBatch: status 400 validation_failed, /items/1/data/topic too_long of channel:13000103"; err.Error() != want {
		t.Errorf("structure batch:\n got %s\nwant %s", err.Error(), want)
	}

	conflict := batchError(&http.Response{StatusCode: http.StatusConflict}, []byte(`{"code":"conflict","reason":"thread_not_imported"}`), b)
	if want := "bothy: applyImportBatch: status 409 conflict (thread_not_imported), batch thread:13001001"; conflict.Error() != want {
		t.Errorf("conflict:\n got %s\nwant %s", conflict.Error(), want)
	}
}

// TestAPIErrorCarriesNoContentOfTheBody: of a problem body the error text repeats only codes and
// pointers in their fixed forms; a pointer or a code of another form, the detail texts and every
// other member are dropped, and a long list of field errors ends in a count.
func TestAPIErrorCarriesNoContentOfTheBody(t *testing.T) {
	body := `{"code":"validation_failed","reason":"Zq Reason","detail":"zq-detail","zq_member":"zq-value","errors":[
		{"pointer":"/items/0/data/Zq Content Marker","code":"too_long","detail":"zq-field-detail"},
		{"pointer":"/items/0/data/topic","code":"Zq Code"},
		{"pointer":"zq-not-a-pointer","code":"invalid"},
		{"pointer":"/a","code":"invalid"},{"pointer":"/b","code":"invalid"},{"pointer":"/c","code":"invalid"},
		{"pointer":"/d","code":"invalid"},{"pointer":"/e","code":"invalid"}]}`
	err := apiError("applyImportBatch", &http.Response{StatusCode: http.StatusBadRequest}, []byte(body))
	want := "bothy: applyImportBatch: status 400 validation_failed, too_long, invalid, /a invalid, /b invalid, /c invalid, and 2 more"
	if err.Error() != want {
		t.Errorf("error text:\n got %s\nwant %s", err.Error(), want)
	}
	if strings.Contains(strings.ToLower(err.Error()), "zq") {
		t.Errorf("the error text carries a part of the body: %s", err.Error())
	}
	plain := apiError("getImport", &http.Response{StatusCode: http.StatusNotFound}, []byte(`not json`))
	if plain.Error() != "bothy: getImport: status 404" {
		t.Errorf("a body that is no problem document: %s", plain.Error())
	}
}

// TestItemPointer: only "/items/<i>" and "/items/<i>/<rest>" name an item.
func TestItemPointer(t *testing.T) {
	for pointer, want := range map[string]struct {
		n    int
		rest string
		ok   bool
	}{
		"/items/3/data/topic": {3, "/data/topic", true},
		"/items/0":            {0, "", true},
		"/items/12/item_kind": {12, "/item_kind", true},
		"/items":              {0, "", false},
		"/items/x/data":       {0, "", false},
		"/items/-1/data":      {0, "", false},
		"/thread_origin_key":  {0, "", false},
		"":                    {0, "", false},
	} {
		if n, rest, ok := itemPointer(pointer); n != want.n || rest != want.rest || ok != want.ok {
			t.Errorf("itemPointer(%q) = %d, %q, %v; want %d, %q, %v", pointer, n, rest, ok, want.n, want.rest, want.ok)
		}
	}
}

// TestMirrorNamesRefusedThread: a thread whose batch the server refuses stays behind, and the
// round says why: the note of its channel and a line on standard error name the thread, the place
// of the refusal, its rule and the item; the other threads are copied. Before, the reason went to
// a debug log the binary never opens, and the report only said that a thread failed.
func TestMirrorNamesRefusedThread(t *testing.T) {
	e := newTestEnv(t, discordtest.BasicFixture(t))
	src := e.startSource(fixtureMapping)
	e.bothy.RefuseBatch("thread:"+fxThread, 1, "/items/1/data/content_md", "invalid")
	if code := e.run("--guild", fxGuild, "--map", e.writeMap(fixtureMapping), "--apply", "--once", "--report", "json"); code != ExitOK {
		t.Fatalf("exit %d: %s", code, e.stderr.String())
	}
	reason := "bothy: applyImportBatch: status 400 validation_failed, batch thread:" + fxThread +
		", /items/1/data/content_md invalid of message:13002002"
	if want := "bothy import discord: thread " + fxThread + " of channel " + fxGeneral + " failed in this round: " + reason + "\n"; !strings.Contains(e.stderr.String(), want) {
		t.Errorf("standard error lacks the line %q:\n%s", want, e.stderr.String())
	}
	_, notes := reportCounts(t, e.stdout.String(), "#general")
	if want := noteThreadFailed + ": thread " + fxThread + ": " + reason; count(notes, want) != 1 {
		t.Errorf("the row of general lacks the note %q: %v", want, notes)
	}
	if got := e.bothy.ThreadMessages(src, "channel:"+fxGeneral); len(got) == 0 {
		t.Error("the main thread of general was not copied")
	}
	if got := e.bothy.ThreadMessages(src, "thread:"+fxThread); len(got) != 0 {
		t.Errorf("the refused thread has messages %v", got)
	}
}

// TestFullRunNamesRefusedBatch: a full run that meets a refused batch ends with exit 1 and the
// same text, so the owner sees the thread, the rule and the item instead of a bare status.
func TestFullRunNamesRefusedBatch(t *testing.T) {
	e := newTestEnv(t, discordtest.BasicFixture(t))
	e.startSource(fixtureMapping)
	e.bothy.RefuseBatch("thread:"+fxThread, 1, "/items/0/data/name", "bidi_control")
	if code := e.run("--guild", fxGuild, "--map", e.writeMap(fixtureMapping), "--apply"); code != ExitAborted {
		t.Fatalf("exit %d, want 1: %s", code, e.stderr.String())
	}
	want := "bothy import discord: bothy: applyImportBatch: status 400 validation_failed, batch thread:" + fxThread +
		", /items/0/data/name bidi_control of thread:" + fxThread + "\n"
	if !strings.Contains(e.stderr.String(), want) {
		t.Errorf("standard error lacks %q:\n%s", want, e.stderr.String())
	}
}

// TestSwitchFailedThreadIsNoLockProblem (22 13.3): when messages stay behind because their thread
// failed in the last round, switch ends with exit 1, names the failed thread with its reason and
// does not conclude that the Discord channel is not locked. Before, it printed the hint of a
// missing Discord lock for a thread the server refuses for good.
func TestSwitchFailedThreadIsNoLockProblem(t *testing.T) {
	e, _, mapFile := switchEnv(t, "")
	e.discord.AddMessage(fxThread, newMessage("13009001", e.clk.Now().Add(-time.Hour)))
	e.bothy.RefuseBatch("thread:"+fxThread, 1, "/items/1/data/content_md", "invalid")
	if code := e.run(switchArgs(mapFile, "--discord-locked")...); code != ExitAborted {
		t.Fatalf("exit %d, want 1: %s", code, e.stderr.String())
	}
	for _, want := range []string{"1 messages after the cursor were not copied", "1 of the channel's threads failed", "run switch again",
		"thread " + fxThread + " of channel " + fxGeneral + " failed in this round", "/items/1/data/content_md invalid of message:13009001"} {
		if !strings.Contains(e.stderr.String(), want) {
			t.Errorf("stderr lacks %q:\n%s", want, e.stderr.String())
		}
	}
	for _, not := range []string{"evidently not read-only", "Step 3"} {
		if strings.Contains(e.stderr.String(), not) {
			t.Errorf("stderr says %q for a thread that failed:\n%s", not, e.stderr.String())
		}
	}
}
