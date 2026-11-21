# Team channels: agents of several projects work together

Status: design draft of 2026-11-21, not yet normative and not built. It proposes the decisions for
a later specification ticket; until that ticket changes them, the chapters win. Related:
[09-agents.md](../spec/09-agents.md) 3.2, 3.4, 3.6, 4.6, 5.8, 7, 8, 18.5, 18.16 and 18.19.8,
[07-permissions.md](../spec/07-permissions.md) SR1 to SR4,
[ADR 0077](../adr/0077-autonomous-runs-wake-ups-and-turn-handover.md),
[autonomous-runs.md](autonomous-runs.md).

## 1. Goal and done criterion

Owner request of 2026-11-21 (paraphrased): in one development space the owner runs runner threads
for several projects that depend on each other (for example a coding agent, a review agent, a
question and answer agent and a coordinator) and copies text between them by hand. He wants a
team channel: he adds project agents and Hermes agents as members, every member sees who else is
in it, agents hand work to each other by mentioning a member ("@review I built X, please take
over"), and whatever he writes there reaches every member. Agent to agent triggering is wanted in
such a channel; cost is not the concern, finishing the project is.

Done means: in a team channel a member's message that mentions another member starts or feeds
that member's run without any message of the owner; an owner message reaches every member,
running ones as a follow-up and idle ones as a new run; outside team channels nothing changes
(only humans trigger); the safety brake stops ping-pong and runaway chains with a visible note;
one stop control stops every run of the channel; no member gains a right the owner would not
give it with his own message.

## 2. User flow

1. The owner creates a channel "Team" in the space and switches it to team mode. He adds members
   in the app ("Add project", "Add agent") or in the channel (`/team add project coding`,
   `/team add @hermes-qa`, `/team list`, `/team remove`). Each member gets a handle
   (`@coding`, `@review`, `@qa`, `@coordinator`).
2. The channel header shows the members with their state (idle, running, paused) and a stop
   button. Each member works in its own work thread (section 3.2); its replies appear in the team
   channel under its handle, with a link to the live box.
3. The owner writes "@coding build feature X". Coding runs, and its final reply says "@review I
   built X on branch Y, please review". Review starts, finds issues, replies "@coding fix A and
   B". Coding continues in its existing session.
4. The owner writes "please wrap up now". Every running member gets it as a follow-up at once,
   every idle member gets a short run with it. Each member answers in the channel.

## 3. Key decisions

### 3.1 Members are agent and project pairs

One runner agent usually serves many project forums (09 18.16), so a member is a pair: an agent
and, for a `code_runner`, the project forum whose folder and binding it works in. A member has a
unique handle in the channel. Mentions name handles, not agents. New table `team_members`
(channel, agent, optional project forum, handle, home channel, state).

### 3.2 Work happens in the member's own work thread

Each member has one work thread per team channel: for a project member a thread of its project
forum, for any other agent a thread in its home channel (chosen when added). Runs, sessions, live
boxes and folders stay where the spec has them: one active run per thread (R-DM-14), one session
per thread (section 9), the project binding and readiness of 18.1, and SR3 all stay unchanged. The
server posts each final reply of a work thread run into the team channel as that agent, with the
member's handle and a link. Considered instead: all runs inside the team channel. Rejected for
now because it needs several active runs and sessions per thread and a binding per run, a change
to the core data model and to every adapter.

### 3.3 Trigger rule in team channels

SR4 gets an addendum: in a channel with `team_mode`, a message of origin `agent` posted for a
member (its relayed reply) triggers every member it mentions, if the channel's talk rules allow
that pair. Several mentions in one message are allowed there (3.4 step 3 refuses them elsewhere,
because there the members' runs share one thread; here each has its own). The run starts in the
mentioned member's work thread with the new `trigger_kind = 'team'`, `triggering_message_id` set
to the team message, and `triggered_by` set to the channel's sponsor: the active owner who switched
team mode on. Create and every delivery check `agent.invoke` for the work thread as that person
(`actor.Recheck`, origin human), the same call the wake fire makes today (`runs/app/wakes.go`), so
SR1 (I1, I2), SR2, SR3, project readiness and archived or read-only checks apply as if the owner
had written in the work thread. A member mentioned while it runs gets the message as a follow-up
(section 7). Outside team channels every rule stays.

### 3.4 Owner messages reach every member

A message of the sponsor (or of any active owner) in the team channel goes to every member:
a follow-up to a running run, a new `team` run for an idle member. Mentions in it mark who is
addressed; the others get it as context and may answer briefly. In the runner's background mode
it arrives with an owner code (18.5), so the session treats it as the owner's own word. Change
needed: today owner codes exist only when the run's triggering message is a person's
(`ownercode.is_owner_trigger`); for `team` runs the runner must take the owner from the run's
`triggered_by` instead. A relayed agent message never carries a code, and the runner labels it
"from member @x".

### 3.5 Who may talk to whom

Per channel a talk rule list (`team_talk_rules`: from handle, to handle, allow), default every
member may mention every member. Edited in the app or with `/team allow @a @b` and
`/team deny @a @b`. A mention the rules forbid is shown greyed with the reason, it triggers nothing.

## 4. Safety brake (not a cost limit)

The brake exists to stop machines that run in circles, not to save money, and its defaults are
high:

- **Turn budget per channel:** agent-triggered runs since the owner's last message, at most the
  channel's `team_turn_budget` (default from the named key `team.turn_budget_default`). At the
  budget the channel pauses agent triggers with a note; any owner message resets it.
- **Ping-pong detection:** two members that trigger each other alternately
  `team.pingpong_max` times while their runs report idle (no tool call except replying, the idle
  flag of 09 5.8.6) are paused as a pair with a note naming both.
- **Stop all:** the stop control of the channel stops every active run in every work thread
  (09 8), cancels queued `team` runs, and pauses agent triggers until the owner writes again.
- `team` runs are autonomous inputs and count in `run_autonomy` of their work thread, but the
  owner's broadcast resets that count like a follow-up of the person.

Every pause and stop posts a `system` note in the team channel and notifies the owner.

## 5. Security

- Rights: every run runs as the sponsor with the member agent's own tool class, binding and
  billing; agents gain no grant. The runner never sees another agent's token.
- People: in the first slice only owners may read the team channel and the home channels of its
  members; adding a member or a reader is refused when a reader of the team channel could not read
  a home channel or the other way round (`Authorizer.Readers`), so no text crosses to a person who
  could not read it at its source.
- Injection: one agent's text now makes another agent act, including agents with tool class
  `host`. That is the owner's wish inside his own trust domain; the runner prompt states that
  member messages are requests of a colleague, not the owner's word.
- Audit: membership changes, talk rule changes, every `team` trigger (with source member), every
  brake pause and the stop all.

## 6. Changes per part

- **Server:** `channels.team_mode` and sponsor, `team_members`, `team_talk_rules`, team counters;
  actions `team.member.add/remove/list`, `team.rule.update`, `team.stop_all`; the trigger rule of
  3.3 and the broadcast of 3.4 in the acceptance (`runs/app/accept.go` today refuses every
  non-human origin at line 117); relay of final replies into the team channel; mention parsing of
  handles; events `team.member_changed`, `team.paused`.
- **Run context (4.6):** a `team` block with channel, own handle, the member list (handle, kind,
  project, short description, state) and the talk rules.
- **Runner:** prompt section "Team channel" (members, how to mention, what the owner's broadcast
  means); owner codes from `triggered_by` for `team` runs; labels for member messages. Mentions
  need no command: the final reply's `@handle` is parsed by the server. Later a command
  `bothy-team say` to post mid-run.
- **Hermes adapter:** accept `trigger_kind = 'team'` and the team block (unchecked whether it
  refuses an unknown kind today), and the same prompt hint.
- **Apps:** team mode switch, member dialog with handles, header with member states and stop all,
  relayed replies with handle and live box link, greyed forbidden mentions, brake notes.

## 7. First slice and later steps

First slice: owner only; runner project members and Hermes members; work threads in the
members' home channels; agent mention trigger with talk rules (default all); owner broadcast as
follow-up or new run with owner codes; final reply relay; turn budget, ping-pong pause and stop
all; `/team add|remove|list` and the app member dialog; header with states.

Later: `bothy-team say` mid-run; editing talk rules as a matrix in the app; all runs inside the
team channel (3.2 alternative); members who are people other than owners.

## 8. Alternatives rejected

- **Point to point handoff records** (the first version of this draft): one hop at a time with a
  return path; the owner's refinement wants a shared room with broadcast instead.
- **Runner-local routing between sessions:** bypasses the server, so no rights, audit,
  visibility or Hermes.
- **Lifting SR4 everywhere:** every agent message in every channel becomes a trigger; the team
  mode keeps that to channels the owner made for it.
- **The owner's personal token in the turn:** forges a human origin and puts a credential into a
  process that reads untrusted text.

## 9. Open owner questions

1. Does an owner message without a mention start idle members too, or only running ones?
   Recommendation: all members, as requested; each idle one answers briefly unless addressed.
2. Do mentions inside an owner message narrow who gets it? Recommendation: no, everyone gets it,
   the mentioned members are marked as addressed.
3. Default of the turn budget and the ping-pong count? Recommendation: high values in the named
   keys, set in the specification ticket and held by a test, so the brake only catches loops.
4. Work threads: one per member and team channel for good, or a new one per topic thread of the
   team channel? Recommendation: one per member and team channel for the first slice, so each
   member keeps its context; per topic later if channels grow.
