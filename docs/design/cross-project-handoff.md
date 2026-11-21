# Team channels: agents of several projects work together

Status: design of 2026-11-21, approved by the owner on 2026-11-21 with the answers of section 9.
The server slice FEAT-TEAM-CHANNEL-SERVER turned it into the specification
([09-agents.md](../spec/09-agents.md) 3.9); where the two differ, the chapters win. Rulings:
[FEAT-TEAM-CHANNEL-SERVER](../decisions.d/FEAT-TEAM-CHANNEL-SERVER.md). Related:
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

SR4 gets an addendum: in a team channel, the final reply of a member that the server mirrors there
triggers every member it mentions by handle. Several mentions in one reply are allowed there (3.4
step 3 refuses them elsewhere, because there the members' runs share one thread; here each has its
own). The server copies the reply into the mentioned member's work thread as an input message
authored by the source member's agent (`origin = agent`) and starts a run there with the new
`trigger_kind = 'team'`, `triggering_message_id` set to that input, and `triggered_by` set to the
channel's sponsor: the active owner who switched team mode on. Create and every delivery check
`agent.invoke` for the work thread as that person (`actor.Recheck`, origin human), the same call
the wake fire makes (`runs/app/wakes.go`), so SR1 (I1, I2), SR2, SR3, project readiness and
archived or read-only checks apply as if the owner had written in the work thread. A member
mentioned while it runs gets the input as a follow-up (section 7 of 09). Outside team channels
every rule stays.

Why a copy in the work thread: the run context (09 4.6), the follow-ups (09 7) and the runner read
the inputs of a run from the run's own thread; a copy keeps all three unchanged, and the work
thread reads as the member's whole conversation.

### 3.4 Owner messages reach every member

A message of any active owner in the team channel goes to every member: the server copies it into
each work thread as an input authored by that owner (`origin = human`), a follow-up to a running
run, a new `team` run with the owner as `triggered_by` for an idle member. Mentions in it mark who
is addressed (`team.addressed` of the invocation); the others get it as context and may answer
briefly. In the runner's background mode it arrives with an owner code (18.5), so the session
treats it as the owner's own word: the runner keeps today's rule and decides from the input
message's origin and author, never from `triggered_by`. A relayed member reply is an agent's
message and never carries a code, and the runner labels it "from member @x".

### 3.5 Who may talk to whom

Later slice. Per channel a talk rule list (`team_talk_rules`: from handle, to handle, allow);
until it exists every member may mention every member. Edited in the app or with `/team allow @a @b` and
`/team deny @a @b`. A mention the rules forbid is shown greyed with the reason, it triggers nothing.

## 4. Safety brake (not a cost limit)

The brake exists to stop machines that run in circles, not to save money, and its defaults are
high (owner answer 3):

- **Turn budget per channel:** agent-triggered inputs since an owner's last message, at most
  `team.turn_budget`. At the budget the channel pauses agent triggers with a note; any owner
  message resets it.
- **Ping-pong detection:** two members that trigger each other alternately more than
  `team.pingpong_max` times without an owner message are paused with a note naming both; the
  whole channel pauses, because the pair may pull a third member in.
- **Stop all:** the stop control of the channel stops every active run in every work thread
  (09 8) and pauses agent triggers until an owner writes again.
- `team` runs and inputs do not count in `run_autonomy`; the brake above replaces the chain caps
  for them.

Every pause and stop posts a `system` note in the team channel; push notifications for it come
with the app slice.

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
- Audit: membership changes, every `team` trigger (with source member) and every refusal, every
  brake pause and the stop all.

## 6. Changes per part

- **Server (slice FEAT-TEAM-CHANNEL-SERVER):** `team_channels` (the team mode with sponsor and
  brake state), `team_members`, `team_inputs`, `team_mirrors`; operations `getChannelTeam`,
  `enableChannelTeam`, `disableChannelTeam`, `addTeamMember`, `removeTeamMember` and
  `stopChannelTeam` (actions `team.read`, `team.update`, `team.stop`); the trigger rule of 3.3 and
  the broadcast of 3.4 in the job `runs.team_input`; mirror of final replies into the team channel;
  mention parsing of handles; `trigger_kind = 'team'`; the reserved command name `/team`.
- **Run context (4.6):** unchanged for `team` runs: the input lies in the work thread.
- **Invocation:** a `team` block with channel, own member and handle, source, addressed flag and
  the member list (09 3.9.9).
- **Runner:** prompt section "Team channel" (members, how to mention, what the owner's broadcast
  means); accept `trigger_kind = team`; owner codes stay tied to the input message (origin
  `human`, author an owner); labels for member messages. Mentions need no command: the final
  reply's `@handle` is parsed by the server. Later a command `bothy-team say` to post mid-run.
- **Hermes adapter:** accept `trigger_kind = 'team'` and the team block, and the same prompt hint
  (built in FEAT-TEAM-CHANNEL-ADAPTER, 09 17.14).
- **Apps:** team mode switch, member dialog with handles, `/team add|remove|list|stop`, header
  with member states and stop all, relayed replies with handle and live box link, brake notes.

## 7. First slice and later steps

First slice: owner only; runner project members and Hermes members; work threads in the
members' home channels; agent mention trigger (every member may mention every member); owner
broadcast as follow-up or new run with owner codes; final reply relay; turn budget, ping-pong pause
and stop all; `/team add|remove|list|stop` and the app member dialog; header with states. The
server part is FEAT-TEAM-CHANNEL-SERVER; runner, Hermes adapter and apps follow.

Later: talk rules (`team_talk_rules`, `/team allow|deny`) and their matrix in the app;
`bothy-team say` mid-run; attachments in relayed inputs; push notifications for brake notes; all
runs inside the team channel (3.2 alternative); members who are people other than owners.

### 7.1 Open minors of the server slice

Found by the red team of FEAT-TEAM-CHANNEL-SERVER (round 1) without a shown harm; the next slice
that touches the team channels takes them along:

1. `followOnTeamRun` does not consult the brake: a follow-on run from inputs delivered before a
   pause or stop still starts (only when the stop ended the run otherwise than `user_cancelled`).
2. The check before a delivery tests `agent.invoke` for the authority but not whether the sponsor
   is still an active owner; 07 SR4 addendum and 09 3.9.5 should say the same.
3. `enableChannelTeam` does not refuse a channel that is already the home channel of a member.
4. `Recipients.Readers` names only persons; apps and agents with a read grant on a home channel
   see the owner's copied texts.
5. Removing a member deletes its open inputs (`team_inputs.member_id` cascades), so their
   follow-ups are then checked for the agent and refused; the "member removed" branch of
   `followOnTeamRun` is unused.
6. Deleting an owner's message or a member's reply leaves its copies in the work threads and the
   team channel.

Found by the red team in rounds 3 and 4 (outside the lines the fix of round 4 touched):

7. Done in red team round 7: the job no longer carries the run; it reads the reply's run from
   `run_answers`, which the tests write through a request with the run's invocation token.
8. Done in red team round 7: the job has no run argument any more.
9. The audit entry of a `team_source_not_owner` refusal names the sponsor as the actor, not the
   person who fed the run.

Found by the red team in round 6:

10. The context record refuses by default (D-TEAM-12): a system line or server note whose author
    is neither a person nor linked to a run (for example a wake note or a command reply without a
    run) refuses every later run of the work thread that reads it in its window. Safe, but its
    effect on the team flow is not checked for every system line code; the note could name the
    message that caused the refusal.

Found by the red team in round 8:

11. No test sends a real `Invocation-Token` header through the authenticator up to the link of
    `run_answers`; the team tests set the binding through the server's own acting-for source.

Found by the red team in round 9:

12. 03 `run_answers` and `run_feeders` do not say that an agent's message edited after it was
    written speaks for no person despite its link (09 3.9.4 and D-TEAM-12 do).
13. The example in the doc comment of `CopyMessage` omits `SourceVersion`, which is required.
14. When the Hermes adapter edits its own answer (`on_edit` with finalize) before the job runs,
    the owner's reply is not handed on (fail closed); the interplay has no test.
15. No test drives an edit between `TriggerMessageOf` and `CopyMessage` in the reply path; the
    version pin is held by the messaging test and the owner broadcast probe of the red team.

Found by the red team in round 10:

16. The ERD of the team channels draws `threads ||--o{ team_members`; `work_thread_id` is unique,
    so `||--o|` would be more precise.
17. The runs README lists the tables of `PersonalData` without `team_mirrors` and `run_answers`
    (neither holds personal data of its own; 20 section 2).

## 8. Alternatives rejected

- **Point to point handoff records** (the first version of this draft): one hop at a time with a
  return path; the owner's refinement wants a shared room with broadcast instead.
- **Runner-local routing between sessions:** bypasses the server, so no rights, audit,
  visibility or Hermes.
- **Lifting SR4 everywhere:** every agent message in every channel becomes a trigger; the team
  mode keeps that to channels the owner made for it.
- **The owner's personal token in the turn:** forges a human origin and puts a credential into a
  process that reads untrusted text.

## 9. Owner answers of 2026-11-21 (decisions)

1. An owner message without a mention starts idle members too: every member gets it, idle ones as
   a new run, running ones as a follow-up.
2. Mentions in an owner message do not narrow who gets it: every member gets it, the mentioned
   ones are marked as addressed.
3. The brake defaults are high values in named keys (`team.turn_budget`, `team.pingpong_max`, 09
   21), held by a test, so the brake only catches loops.
4. One work thread per member and team channel (the owner left it to the recommendation), so each
   member keeps its context; per topic later if channels grow.

Cost is no concern for the owner; finishing projects is.
