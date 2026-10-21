// SPDX-License-Identifier: Apache-2.0

package notify_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	notifyjobs "github.com/OWNER/bothy/services/server/internal/notify/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// readModel is the read right of the random world, computed from the grants and group
// memberships the test wrote, independent of authz: person -> channel -> readable.
type readModel map[ids.ID]map[ids.ID]bool

// can reports whether person reads channel in the model.
func (m readModel) can(person, channel ids.ID) bool { return m[person][channel] }

// set marks channel readable for person.
func (m readModel) set(person, channel ids.ID) {
	if m[person] == nil {
		m[person] = map[ids.ID]bool{}
	}
	m[person][channel] = true
}

// TestAZP5_PushOnlyReadableChannels (PLAN M3 exit criterion 2, AZ-P5 for push): in seeded random
// worlds of grants, group memberships and revocations on top of the world of 07 11.1, with
// random messages of the Owner that mention random people and every person at the personal level
// all, no notify.push_send, window or flush job exists for a recipient who cannot read the thread
// at decision time, every recipient of a message hint is in Recipients.Readers and allowed by
// authorize (push.deliver), and every mentioned person who can read gets exactly one mention hint.
// The random world generator of AZ-P8 (M1-49) lives in test files of internal/authz/app over the
// fakes of authz, so it cannot be reused here; this smaller generator writes its worlds through the
// API and keeps the read model on its own.
func TestAZP5_PushOnlyReadableChannels(t *testing.T) {
	t.Parallel()
	for _, seed := range []uint64{1, 2, 3} {
		t.Run(fmt.Sprintf("seed_%d", seed), func(t *testing.T) {
			t.Parallel()
			azp5World(t, seed)
		})
	}
}

// azp5World runs one random world of TestAZP5_PushOnlyReadableChannels.
func azp5World(t *testing.T, seed uint64) {
	w := newWorld(t)
	rnd := rand.New(rand.NewPCG(seed, seed^0x5eed))
	channels := []ids.ID{w.allgemein, w.eskalation, w.kampagnen, w.werkzeuge, w.notizen}
	spaceOf := map[ids.ID]ids.ID{w.allgemein: w.support, w.eskalation: w.support, w.kampagnen: w.marketing, w.werkzeuge: w.engineering,
		w.notizen: w.research}
	spaces := []ids.ID{w.support, w.marketing, w.engineering, w.research}
	people := []struct {
		name string
		id   ids.ID
	}{{"alex", w.alex}, {"sam", w.sam}, {"robin", w.robin}}
	model := readModel{}
	for _, ch := range channels {
		model.set(w.owner, ch)
	}
	// The base world: G-1 (Alex, Support), G-2 (Sam, Marketing), G-4 (Leser with Sam, Allgemein);
	// G-3 of Robin is on Projekt-A, a forum outside the channels of this test.
	alexSupport, samLeser := rnd.IntN(10) >= 3, rnd.IntN(10) >= 3
	if alexSupport {
		model.set(w.alex, w.allgemein)
		model.set(w.alex, w.eskalation)
	} else {
		w.revoke(w.g1)
	}
	model.set(w.sam, w.kampagnen)
	if samLeser {
		model.set(w.sam, w.allgemein)
	} else {
		w.removeMember(w.leser, w.sam)
	}
	// Random grants of read on channels and spaces to the people.
	for _, p := range people {
		for _, ch := range channels {
			if rnd.IntN(4) == 0 {
				w.grant(`"subject_kind":"user","subject_id":"` + p.id.String() + `","target_kind":"channel","target_id":"` + ch.String() +
					`","level":"read","reason":"Random reader"`)
				model.set(p.id, ch)
			}
		}
		for _, sp := range spaces {
			if rnd.IntN(7) == 0 {
				w.grant(`"subject_kind":"user","subject_id":"` + p.id.String() + `","target_kind":"space","target_id":"` + sp.String() +
					`","level":"read","reason":"Random space reader"`)
				for ch, s := range spaceOf {
					if s == sp {
						model.set(p.id, ch)
					}
				}
			}
		}
	}
	// A random group with random members and a read grant on a random channel.
	g := w.want(w.do("owner", http.MethodPost, "/api/v1/groups", `{"name":"Zufall"}`, key()...), http.StatusCreated, "")
	group := ids.MustParse(g["id"].(string))
	target := channels[rnd.IntN(len(channels))]
	w.grant(`"subject_kind":"group","subject_id":"` + group.String() + `","target_kind":"channel","target_id":"` + target.String() +
		`","level":"read","reason":"Random group"`)
	for _, p := range people {
		if rnd.IntN(2) == 0 {
			w.want(w.do("owner", http.MethodPost, "/api/v1/groups/"+group.String()+"/members", `{"user_id":"`+p.id.String()+`"}`, key()...),
				http.StatusCreated, "")
			model.set(p.id, target)
		}
	}
	// Robin gets a phone too, so every person has one push registration.
	w.phone(w.robin, "android", "fcm")
	for _, name := range []string{"owner", "alex", "sam", "robin"} {
		w.setMySetting(name, "notify.default_level", `"all"`)
	}
	// Random messages of the Owner in the main threads, each mentioning a random set of people.
	type sentMsg struct {
		ID, Channel ids.ID
		Mentioned   []ids.ID
	}
	threadChannel := map[ids.ID]ids.ID{}
	var msgs []sentMsg
	for i := range 12 {
		ch := channels[rnd.IntN(len(channels))]
		th := w.mainThread(ch)
		threadChannel[th] = ch
		var mentions []string
		var mentioned []ids.ID
		for _, p := range people {
			if rnd.IntN(3) == 0 {
				mentions = append(mentions, "@"+p.name)
				mentioned = append(mentioned, p.id)
			}
		}
		w.clk.Advance(time.Duration(rnd.IntN(20)) * time.Second)
		m := w.post("owner", th, strings.TrimSpace(strings.Join(mentions, " ")+fmt.Sprintf(" note %d", i)))
		msgs = append(msgs, sentMsg{ID: m.ID, Channel: ch, Mentioned: mentioned})
		w.consume()
	}
	w.advance(2 * time.Minute) // past notify.coalesce_window: every window ends
	w.flushDue()
	w.consume()
	ctx := context.Background()
	jobs := w.pushJobs()
	if len(jobs) == 0 {
		t.Fatal("the random world produced no push job")
	}
	for _, j := range jobs {
		ch, ok := threadChannel[j.Args.ThreadID]
		if !ok {
			t.Fatalf("a push job outside the threads of the test: %+v", j.Args)
		}
		if !model.can(j.Args.RecipientID, ch) {
			t.Fatalf("seed %d: a push job for %s, who cannot read the channel %s: %+v", seed, j.Args.RecipientID, ch, j.Args)
		}
		if j.Kind() != domain.KindMessage {
			continue
		}
		readers, err := w.az.Readers(ctx, ch)
		must(t, err)
		if !slices.Contains(readers, j.Args.RecipientID) {
			t.Fatalf("seed %d: the recipient %s of a message hint is not in Recipients.Readers %v", seed, j.Args.RecipientID, readers)
		}
		d, err := w.az.Authorize(authorize.Filter(ctx), actor.Recheck(j.Args.RecipientID), authorize.ActionPushDeliver,
			authorize.Resource{Kind: authorize.ResourceThread, ID: j.Args.ThreadID, ChannelID: ch, SpaceID: spaceOf[ch]})
		must(t, err)
		if !d.Allowed() {
			t.Fatalf("seed %d: authorize denies push.deliver to the recipient %s of a message hint", seed, j.Args.RecipientID)
		}
	}
	for _, kind := range []string{notifyjobs.KindCoalesceWindow, notifyjobs.KindCoalesceFlush} {
		for _, j := range w.jobsOf(kind) {
			var a struct {
				UserID   ids.ID `json:"user_id"`
				ThreadID ids.ID `json:"thread_id"`
			}
			must(t, json.Unmarshal(j.Args, &a))
			if !model.can(a.UserID, threadChannel[a.ThreadID]) {
				t.Fatalf("seed %d: a %s job for %s, who cannot read the thread", seed, kind, a.UserID)
			}
		}
	}
	allowedMentions, deniedMentions := 0, 0
	for _, m := range msgs {
		for _, p := range m.Mentioned {
			if model.can(p, m.Channel) {
				allowedMentions++
			} else {
				deniedMentions++
			}
			var got []pushJob
			for _, j := range jobs {
				if j.Args.MessageID == m.ID && j.Args.RecipientID == p && j.Kind() == domain.KindMention {
					got = append(got, j)
				}
			}
			want := 0
			if model.can(p, m.Channel) {
				want = 1
			}
			if len(got) != want {
				t.Fatalf("seed %d: message %s mentions %s (reads: %v): %d mention jobs, want %d", seed, m.ID, p, model.can(p, m.Channel), len(got), want)
			}
		}
	}
	t.Logf("seed %d: %d push jobs, %d mentions of readers, %d mentions of people without read right", seed, len(jobs), allowedMentions, deniedMentions)
}
