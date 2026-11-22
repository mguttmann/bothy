// SPDX-License-Identifier: Apache-2.0
//
// A run from a model command against the real server of this tree (docs/spec/09-agents.md 10.4,
// 4.5 point 6): the server delivers `agent.invocation` with `trigger_kind = command` and the
// command's `command_id`, and the token names that command. The adapter accepts the event,
// starts the run and the runtime executes. It needs the Go tool chain and a Docker-compatible
// runtime; without them it fails, it never skips (brief M2-30 Design note 12).

import { afterAll, beforeAll, describe, expect, inject, it } from "vitest";

import { CommandSpec } from "../../src/commands/index.js";
import { Adapter } from "../../src/runs/index.js";
import { RecordingHandler } from "../../src/testing/doubles.js";
import { eventually } from "../../src/testing/patience.js";
import { type RealServer, startRealServer } from "./sdkserver.js";

const ENDED = new Set(["completed", "failed", "cancelled"]);
// The real pause between two looks at the server.
const LOOK_MS = 100;

describe("a model command against the real server", () => {
  let real: RealServer;

  beforeAll(async () => {
    real = await startRealServer(inject("sdkserverBinary"));
  });

  afterAll(async () => {
    await real?.stop();
  });

  /** Waits until the run has ended and returns the server's view of it. */
  async function ended(runId: string) {
    await eventually(
      async () => ENDED.has((await real.runState(runId)).state),
      "the end of the run",
      { pauseMs: LOOK_MS },
    );
    return real.runState(runId);
  }

  it("starts and completes the run a person started with the command", async () => {
    const handler = new RecordingHandler();
    handler.clock = real.clock;
    const adapter = new Adapter(
      {
        baseUrl: real.baseUrl,
        instanceId: real.instanceId,
        agentId: real.agentId,
        credentials: real.credentials,
        consumer: "model-command",
        adapterName: "command-adapter",
        adapterVersion: "1.0.0",
        runtimeName: "command-runtime",
        settings: real.sdkSettings,
        // Not a control command, so an invocation creates a run (a model command, 09 10.4).
        commands: [new CommandSpec("recap", "Recap the thread")],
      },
      handler,
      { clock: real.clock },
    );
    const done = adapter.run();
    done.catch(() => undefined);
    try {
      // The adapter registers its command after `ready`; by then its consumer exists.
      let commandId = "";
      await eventually(
        async () => {
          const listed = await real.call("alex", "listChannelCommands", {
            path: { channel_id: real.channelId },
          });
          const items = (listed.body.items ?? []) as {
            id: string;
            name: string;
          }[];
          commandId = items.find((c) => c.name === "recap")?.id ?? "";
          return commandId !== "";
        },
        "the registered command",
        { pauseMs: LOOK_MS },
      );
      // A command has no message a thread could start from, so it runs in an existing thread.
      const first = await real.trigger("Hello @Lotse");
      expect((await ended(first.runId)).state).toBe("completed");

      const invoked = await real.call("alex", "invokeCommand", {
        path: { channel_id: real.channelId, command_id: commandId },
        body: { thread_id: first.threadId, options: [] },
      });
      expect(invoked.status).toBe(202);
      const runId = String(invoked.body.run_id);
      expect(runId).not.toBe(first.runId);

      const view = await ended(runId);
      expect([view.state, view.stateReason, view.starts]).toEqual([
        "completed",
        undefined,
        1,
      ]);
      expect(handler.count("execute", runId)).toBe(1);
      const inv = handler.invocations.find((i) => i.payload.run_id === runId);
      expect([
        inv?.type,
        inv?.payload.trigger_kind,
        inv?.payload.command_id,
        inv?.claims.triggerKind,
        inv?.claims.commandId,
      ]).toEqual([
        "agent.invocation",
        "command",
        commandId,
        "command",
        commandId,
      ]);
    } finally {
      await adapter.stop();
      await done.catch(() => undefined);
    }
  });
});
