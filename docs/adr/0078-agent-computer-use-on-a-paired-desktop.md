---
status: accepted
date: 2026-11-26
decision-makers: Owner, planning lead
consulted: Red team of the stage
informed: all teams
---

# Agent computer use on a paired desktop

Extends [ADR 0068](0068-terminal-through-a-host-service-with-noise-stream-and-invariant-i6.md)
(terminal through a host service with a Noise stream) to a second kind of host session, and
changes what the threat model held as T-TM-09 ("an agent never reaches a host") for one case: a
desktop of the owner that the owner enabled on that machine. Normative:
[27-computer-use.md](../spec/27-computer-use.md), [07-permissions.md](../spec/07-permissions.md)
2.15 and SR13, 03 `computer_sessions`; design note:
[computer-use.md](../design/computer-use.md); rulings:
[decisions.d/CU-01.md](../decisions.d/CU-01.md).

## Context and Problem Statement

The owner ruled on 2026-11-26 (OR-1, D-CU-01-1): an agent shall operate the owner's Mac or Windows
computer completely, terminal, browser, any app and the system settings included, and the owner
shall be able to start such work from any Bothy client, the phone included, without a confirmation
at the target machine. Until now the product excluded exactly this: 01 listed "Terminal for agents
or apps" as `never`, SPEC section 4 named it a non-goal, and the threat model rated "prompt
injection opens a terminal" (T-TM-09) as mitigated only because agents never reach a host. How
does an agent see and operate a desktop without giving the server the screen content, without a
new host mode on Windows, and with what remains of protection when no capability may be removed?

## Decision Drivers

- OR-1: full operation, start from anywhere, no confirmation per session, macOS and Windows in
  the first stage.
- The server reads nothing (R-TM-02, SEC-TERM-1): screenshots and input stay end-to-end
  encrypted between the agent's tool and the host.
- Protection that removes no capability: owner only, visible while it runs, stoppable from every
  client and at the machine, bound to the run that the owner started.
- One mechanism for both platforms, without the owner account mode of ADR 0073 (which does not
  exist on Windows).
- No fixed limits; every bound is a named key.

## Considered Options

1. Tool first: a `stdio` MCP server `bothy-computer-mcp` next to the runner, Noise `NK` to the
   host through the existing relay, a new session kind in `bothy-host`, and a separate signed
   helper in the owner's desktop session that captures and injects.
2. Host first: the host service captures and injects itself, the server brokers the stream to the
   runner (the Noise end moves into the server).
3. Claude Code's built-in computer use on the runner's machine.
4. Keep the non-goal.

## Decision Outcome

Chosen: option 1. The owner's own message in a thread starts a run; in that run, and only there,
the agent may open one computer session on a desktop of the owner that the owner enabled once on
that machine (SR13). The runner asks the server on the agent's behalf; the server checks SR13,
issues a single-use attach token bound to run, agent, host and the MCP server's ephemeral key, and
relays only ciphertext. The host service checks the token and the local enablement and drives the
helper over a local channel that admits only the console user's installed, signed helper. The
helper shows an indicator with a stop button, stops on a real Esc, pauses while the screen is
locked and releases every key on every end. The session ends with its run at the latest. No
action, key, app or setting is refused by Bothy; what the operating system refuses is reported
(`os_refused`, `secure_desktop`).

### Consequences

- Good, because the owner gets the full control asked for, from any client, on Mac and Windows,
  while the server still sees no screen content.
- Good, because the existing pairing, keys, relay, revocation path and audit of the terminal carry
  the new session kind; nothing about terminal sessions changes.
- Bad, because a hijacked agent in an owner's run acts with all the owner's rights on that machine
  (T-CU-02). Data marking, the system prompt, the run binding, the indicator, stop and audit make
  it visible and stoppable; nothing prevents it. The threat model records this residual risk as
  high and the owner accepted it with OR-1.
- Bad, because a stolen owner session can start such work from afar without a step-up (T-CU-08);
  device revocation, the indicator and the audit are the answer.
- Bad, because screenshots reach the LLM provider and, until the run ends, the runner host's disk.
- Bad, because two native helpers with their own signing paths are new code on both platforms, and
  hosted CI runners may not be able to prove capture and input (an open gap if the probe fails).
- Neutral, because the Hermes adapter, a live view, more input kinds, a background mode and other
  persons are later tickets (CU-08 to CU-12).

### Confirmation

- Tests CU-T01 to CU-T24 of 27 section 15: guards of `computer.open` and `computer.attach`
  (not an owner run, a wake or team run, a disabled or foreign desktop, a revoked owner, an ended
  run, a replayed token), frames and directions, coordinate mapping, chunking at the Noise limit,
  pause and resume, every stop path with no key left pressed, the sentinel test that finds no image
  byte on the server, and the transcript removal against the pinned CLI's layout.
- An end-to-end run in Compose with the Linux test backend (Xvfb and XTest) from runner to helper.
- Native jobs on `macos-15` and `windows-2025` after the probe of CU-02.
- The stage red team with attacks on T-CU-01 to T-CU-08 before the stage merges.

## Pros and Cons of the Options

### Tool first

- Good, because the Noise end stays outside the server, the `stdio` rule (R-BA-05) puts the tool
  under `host` and SR2, and Windows needs no new host mode.
- Bad, because images must be split into Noise messages of at most 65535 bytes and a helper per
  platform must be built and signed.

### Host first

- Good, because there is one process less on the host.
- Bad, because the server would see every screenshot (it breaks R-TM-02 and SEC-TERM-1), and the
  host service runs as a service user outside the desktop session without cgo, so it cannot
  capture there anyway.

### Claude Code's built-in computer use

- Good, because nothing needs building.
- Bad, because it acts on the runner's machine, not on the owner's desktop, and needs an
  interactive session.

### Keep the non-goal

- Good, because T-TM-09 stays as it was.
- Bad, because it contradicts the owner's ruling.

## More Information

The design note keeps the evidence, the comparison of both approaches and the facts of the first
draft that the source contradicted (D-CU-01-6, D-CU-01-17, D-CU-01-18, D-CU-01-21, D-CU-01-24).
