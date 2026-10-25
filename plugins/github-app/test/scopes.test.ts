// SPDX-License-Identifier: Apache-2.0
// Scopes (brief M2-49 Design notes 13 and 16; PLAN row "the installation requires exactly the
// scopes from 10.1, a missing scope yields 403 scope_missing").

import { afterEach, describe, expect, it } from "vitest";

import { FakeChat } from "./fakes/chat.js";
import { fixture } from "./helpers/fixtures.js";
import { appFile, specScopes } from "./helpers/repo.js";
import { FORUM, startWorld, type World } from "./helpers/world.js";

let world: World | undefined;

afterEach(async () => {
  await world?.close();
  world = undefined;
});

describe("scopes", () => {
  it("requests exactly the scopes of 11 10.1", () => {
    const manifest = JSON.parse(appFile("manifest.json")) as {
      scopes: { required: string[]; optional: string[] };
    };
    const spec = specScopes();
    expect(spec.required).toEqual([
      "messages:write",
      "commands:write",
      "components:write",
    ]);
    expect(manifest.scopes.required).toEqual(spec.required);
    expect(manifest.scopes.optional).toEqual(spec.optional);
  });

  it("ends the job after one 403 scope_missing with one Warn line", async () => {
    world = await startWorld();
    world.chat.fail(
      "createForumThread",
      FakeChat.problem(403, "scope_missing"),
    );
    expect(
      await world.deliver(
        "issues",
        fixture("issues", "opened", { number: 14 }),
      ),
    ).toBe(202);
    await world.app.idle();
    expect(world.chat.callsOf("createForumThread")).toHaveLength(1);
    const lines = world.logs.filter((l) => l.reason === "scope_missing");
    expect(lines).toHaveLength(1);
    expect(lines[0]).toMatchObject({
      level: "warn",
      status: 403,
      reason: "scope_missing",
    });
    expect(world.chat.threadsOf(FORUM)).toHaveLength(0);
    expect(world.logs.filter((l) => l.msg === "worker.paused")).toHaveLength(0);
  });
});
