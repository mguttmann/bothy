// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"context"
	"fmt"
	"net/http"
)

// demoAdapterKind is the adapter kind of every demo agent: the Fake-Agent of the Python SDK
// (agents.adapter_kind is a free lower-case identifier, docs/DECISIONS.md M1-51).
const demoAdapterKind = "demo"

// grantReason and operatorReason are the reasons of the owner actions of the demo (SEC-AUD-5).
const (
	grantReason    = "Demo instance"
	operatorReason = "Demo instance"
)

// created is the member id of a creation answer.
type created struct {
	ID string `json:"id"`
}

// prices creates every price of the data set (createPrice, price.manage with step-up).
func (r *runner) prices(ctx context.Context) error {
	src := r.ds.PriceSource
	for _, p := range r.ds.Prices {
		body := map[string]any{
			"provider": p.Provider, "model": p.Model, "unit": p.Unit, "price_micros": p.PriceMicros, "currency": p.Currency,
			"valid_from": src.ValidFrom, "retrieved_at": src.ValidFrom, "source_url": src.URL,
		}
		if err := r.owner.ownerPost(ctx, "createPrice", "/prices", body, nil); err != nil {
			return err
		}
	}
	return nil
}

// people creates every person but the owner as a local account (createUser with local_account,
// 08 2.5) and signs each in on a device of its own with the code of the answer.
func (r *runner) people(ctx context.Context) error {
	for _, p := range r.ds.People {
		if p.Owner {
			continue
		}
		var out struct {
			User       created `json:"user"`
			Enrollment *struct {
				Code string `json:"code"`
			} `json:"enrollment"`
		}
		body := map[string]any{
			"handle": p.Handle, "display_name": p.DisplayName, "local_account": true,
			"email": p.Handle + "@example.com", "locale": p.Locale, "time_zone": p.TimeZone,
		}
		if err := r.owner.ownerPost(ctx, "createUser", "/users", body, &out); err != nil {
			return err
		}
		if out.Enrollment == nil {
			return fmt.Errorf("demo: createUser of %s returned no enrollment code", p.Handle)
		}
		s, err := newSession(r.tr)
		if err != nil {
			return err
		}
		if _, err := s.enroll(ctx, out.Enrollment.Code, p.DisplayName+" demo device"); err != nil {
			return err
		}
		r.sessions[p.Handle] = s
		r.res.People[p.Handle] = out.User.ID
	}
	return nil
}

// agents creates every agent of the data set (createAgent, agent.create with step-up).
func (r *runner) agents(ctx context.Context) error {
	for _, a := range r.ds.Agents {
		var out created
		body := map[string]any{"handle": a.Handle, "display_name": a.DisplayName, "description": a.Description, "adapter_kind": demoAdapterKind}
		if err := r.owner.ownerPost(ctx, "createAgent", "/agents", body, &out); err != nil {
			return err
		}
		r.res.Agents[a.Handle] = out.ID
	}
	return nil
}

// spaces creates the spaces, their channels and the grants of their members.
func (r *runner) spaces(ctx context.Context) error {
	for _, sp := range r.ds.Spaces {
		var space created
		body := namedBody{Name: sp.Name, Icon: sp.Icon, Color: sp.Color}
		if err := r.owner.ownerPost(ctx, "createSpace", "/spaces", body, &space); err != nil {
			return err
		}
		r.res.Spaces[sp.Name] = space.ID
		if err := r.channelsOf(ctx, sp, space.ID); err != nil {
			return err
		}
		for _, m := range sp.Members {
			g := grantBody{SubjectKind: "user", SubjectID: r.res.People[m.Person], TargetKind: "space", TargetID: space.ID,
				Level: m.Level, Capabilities: m.Capabilities, Reason: grantReason}
			if m.Agent != "" {
				g.SubjectKind, g.SubjectID = "agent", r.res.Agents[m.Agent]
			}
			if err := r.owner.ownerPost(ctx, "createGrant", "/grants", g, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// namedBody is the body of createSpace, createChannel and createHost: the members a request of
// them sends (the empty ones are left out).
type namedBody struct {
	Name   string `json:"name"`
	Icon   string `json:"icon,omitempty"`
	Color  string `json:"color,omitempty"`
	Kind   string `json:"kind,omitempty"`
	Topic  string `json:"topic,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// grantBody is the body of createGrant.
type grantBody struct {
	SubjectKind  string   `json:"subject_kind"`
	SubjectID    string   `json:"subject_id"`
	TargetKind   string   `json:"target_kind"`
	TargetID     string   `json:"target_id"`
	Level        string   `json:"level,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	Reason       string   `json:"reason"`
}

// channelsOf creates the channels of sp in the space spaceID.
func (r *runner) channelsOf(ctx context.Context, sp Space, spaceID string) error {
	for _, c := range sp.Channels {
		var ch struct {
			ID           string `json:"id"`
			MainThreadID string `json:"main_thread_id"`
		}
		body := namedBody{Name: c.Name, Kind: c.Kind, Topic: c.Topic}
		if err := r.owner.ownerPost(ctx, "createChannel", "/spaces/"+spaceID+"/channels", body, &ch); err != nil {
			return err
		}
		r.channels[channelKey(sp.Name, c.Name)] = channelRef{id: ch.ID, kind: c.Kind, mainThread: ch.MainThreadID}
		r.res.Channels[channelKey(sp.Name, c.Name)] = ch.ID
	}
	return nil
}

// threads writes the threads of the data set, each message in the session of its author. They are
// written before the operators are set (operators), so they open no run.
func (r *runner) threads(ctx context.Context) error {
	for _, sp := range r.ds.Spaces {
		for _, c := range sp.Channels {
			for _, t := range c.Threads {
				if err := r.thread(ctx, r.channels[channelKey(sp.Name, c.Name)], t); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// thread writes one thread of the data set: its first message opens it, the others follow.
func (r *runner) thread(ctx context.Context, channel channelRef, t Thread) error {
	id, err := r.newThread(ctx, channel, t.Messages[0].Author, t.Title, t.Messages[0].Text)
	if err != nil {
		return err
	}
	for _, m := range t.Messages[1:] {
		if err := r.post(ctx, m.Author, id, m.Text); err != nil {
			return err
		}
	}
	return nil
}

// messageBody is the body of createMessage and the first message of createThread.
type messageBody struct {
	ClientMsgID string `json:"client_msg_id"`
	ContentMD   string `json:"content_md"`
}

// newMessage returns a message body with a fresh client_msg_id.
func newMessage(text string) messageBody {
	return messageBody{ClientMsgID: newUUIDv7().String(), ContentMD: text}
}

// newThread opens a thread in channel as person with title and first message text: a forum post
// (createThread with the first message), or in a text channel a thread on a new message of the
// main thread that carries the title (03 "threads"). It returns the thread ID.
func (r *runner) newThread(ctx context.Context, channel channelRef, person, title, text string) (string, error) {
	s := r.sessions[person]
	body := struct {
		Title           string      `json:"title"`
		FirstMessage    messageBody `json:"first_message"`
		ParentMessageID string      `json:"parent_message_id,omitempty"`
	}{Title: title, FirstMessage: newMessage(text)}
	if channel.kind == "text" {
		var parent created
		if err := s.post(ctx, "createMessage", "/threads/"+channel.mainThread+"/messages", newMessage(title), &parent); err != nil {
			return "", err
		}
		body.ParentMessageID = parent.ID
	}
	var out struct {
		Thread created `json:"thread"`
	}
	if err := s.post(ctx, "createThread", "/channels/"+channel.id+"/threads", body, &out); err != nil {
		return "", err
	}
	return out.Thread.ID, nil
}

// post writes text into thread as person (createMessage).
func (r *runner) post(ctx context.Context, person, thread, text string) error {
	return r.sessions[person].post(ctx, "createMessage", "/threads/"+thread+"/messages", newMessage(text), nil)
}

// operators makes the operator of each channel of the data set its agent (setChannelOperator with
// the ETag of getChannel as If-Match).
func (r *runner) operators(ctx context.Context) error {
	for _, sp := range r.ds.Spaces {
		for _, c := range sp.Channels {
			if c.Operator == "" {
				continue
			}
			path := "/channels/" + r.channels[channelKey(sp.Name, c.Name)].id
			body := map[string]any{"mode": "agent", "agent_id": r.res.Agents[c.Operator], "reason": operatorReason}
			err := r.owner.withStepUp(ctx, func() error {
				var etag string
				if err := r.owner.do(ctx, call{op: "getChannel", method: http.MethodGet, path: path, want: http.StatusOK, etag: &etag}); err != nil {
					return err
				}
				return r.owner.do(ctx, call{op: "setChannelOperator", method: http.MethodPut, path: path + "/operator", body: body,
					idempotent: true, want: http.StatusOK, ifMatch: etag})
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}
