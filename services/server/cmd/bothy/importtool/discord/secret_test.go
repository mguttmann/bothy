// SPDX-License-Identifier: Apache-2.0

package discord

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
)

// secretForms are the token, its header form, its base64 forms and the signed query.
func secretForms() []string {
	return []string{
		testToken,
		"Bot " + testToken,
		base64.StdEncoding.EncodeToString([]byte(testToken)),
		base64.StdEncoding.EncodeToString([]byte("Bot " + testToken)),
		base64.RawURLEncoding.EncodeToString([]byte(testToken)),
		discordtest.SignedQuery,
		"hm=echoecho",
	}
}

// TestNoSecretInOutput (22 16 "No secret in output", REUSE 4.3): with echo faults on every
// route and every status, neither the debug log, nor an error text, nor the %+v and %#v forms
// of results and errors contain the token, its header or base64 forms or the signed query.
func TestNoSecretInOutput(t *testing.T) {
	statuses := []int{0, http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden,
		http.StatusNotFound, http.StatusTooManyRequests, http.StatusInternalServerError}
	for _, status := range statuses {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			h := newHarness(t, nil, func(c *Config) { c.MaxRetries = 1 })
			h.fake.Always(discordtest.AnyPath, discordtest.Echo(status))
			var out strings.Builder
			ctx := t.Context()
			record := func(v any, err error) {
				fmt.Fprintf(&out, "%v %+v %#v %s\n", v, v, v, mustMarshal(v))
				if err != nil {
					fmt.Fprintf(&out, "%v %+v %#v %s\n", err, err, err, err.Error())
				}
			}
			me, err := h.client.Me(ctx)
			record(me, err)
			g, err := h.client.Guild(ctx, guildID)
			record(g, err)
			chans, err := h.client.GuildChannels(ctx, guildID)
			record(chans, err)
			threads, err := h.client.ActiveThreads(ctx, guildID)
			record(threads, err)
			var pages []Channel
			err = h.client.ArchivedPublicThreads(ctx, generalID, func(p []Channel) error { pages = append(pages, p...); return nil })
			record(pages, err)
			err = h.client.ArchivedPrivateThreads(ctx, forumID, func(p []Channel) error { pages = append(pages, p...); return nil })
			record(pages, err)
			ch, err := h.client.Channel(ctx, threadID)
			record(ch, err)
			msgs, err := h.client.MessagesAfter(ctx, generalID, "1", 50)
			record(msgs, err)
			msgs, err = h.client.MessagesBefore(ctx, threadID, lastMsgID, 50)
			record(msgs, err)
			msgs, err = h.client.MessagesAround(ctx, generalID, middleMsgID, 5)
			record(msgs, err)
			record(h.client, nil)
			out.WriteString(h.logs.String())
			if h.logs.Len() == 0 {
				t.Fatal("no debug log captured")
			}
			for _, s := range secretForms() {
				if strings.Contains(out.String(), s) {
					i := strings.Index(out.String(), s)
					t.Fatalf("output contains a secret form %q near %q", s[:min(len(s), 8)], out.String()[max(0, i-300):i])
				}
			}
		})
	}
}

// mustMarshal returns the JSON form of v, or the marshal error text.
func mustMarshal(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

// TestTokenRedacted: every print and marshal form of a Token is [redacted].
func TestTokenRedacted(t *testing.T) {
	tok, err := NewToken(testToken)
	if err != nil {
		t.Fatal(err)
	}
	type wrap struct{ T Token }
	forms := []string{
		tok.String(), tok.GoString(), fmt.Sprint(tok), fmt.Sprintf("%v %+v %#v %s %q %x %d", tok, tok, tok, tok, tok, tok, tok),
		fmt.Sprintf("%+v %#v", wrap{tok}, wrap{tok}), mustMarshal(tok), mustMarshal(wrap{tok}), tok.LogValue().String(),
	}
	text, err := tok.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	forms = append(forms, string(text))
	for _, f := range forms {
		if strings.Contains(f, testToken) || !strings.Contains(f, "[redacted]") {
			t.Errorf("form %q", f)
		}
	}
	for _, bad := range []string{"", "   ", "a b", "a\nb", "a\tb", "a\x00b"} {
		if _, err := NewToken(bad); err == nil {
			t.Errorf("NewToken(%q) accepted", bad)
		}
	}
}
