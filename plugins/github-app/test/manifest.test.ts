// SPDX-License-Identifier: Apache-2.0
// The manifest (brief M2-49 Design note 16; docs/spec/11-extensions.md 3.1).

import { describe, expect, it } from "vitest";

import { appFile, repoFile } from "./helpers/repo.js";

const manifest = JSON.parse(appFile("manifest.json")) as Record<
  string,
  unknown
>;

/** The fields of the table of 11 3.1. */
function fieldTable(): string[] {
  const chapter = repoFile("docs/spec/11-extensions.md");
  const section = chapter.slice(
    chapter.indexOf("### 3.1 Format"),
    chapter.indexOf("### 3.2 "),
  );
  const names: string[] = [];
  for (const line of section.split("\n")) {
    if (!line.startsWith("| `")) continue;
    const cell = line.split("|")[1] ?? "";
    for (const m of cell.matchAll(/`([a-z_.]+)`/g))
      names.push((m[1] ?? "").split(".")[0] ?? "");
  }
  return [...new Set(names)];
}

describe("manifest.json", () => {
  it("has the members of Design note 16", () => {
    expect(manifest).toMatchObject({
      schema_version: 1,
      slug: "github",
      name: "GitHub",
      publisher: "Reference integration",
      scopes: {
        required: ["messages:write", "commands:write", "components:write"],
        optional: ["messages:read", "events:subscribe"],
      },
      events: ["message.created"],
      components: ["form"],
      requested_grants: [
        { target_kind: "channel", level: "write", capabilities: [] },
      ],
    });
    expect(typeof manifest.description).toBe("string");
    expect(typeof manifest.data_egress).toBe("string");
    expect(manifest).not.toHaveProperty("homepage_url");
  });

  it("carries the version of VERSION and package.json", () => {
    const version = appFile("VERSION").trim();
    const pkg = JSON.parse(appFile("package.json")) as { version: string };
    expect(manifest.version).toBe(version);
    expect(pkg.version).toBe(version);
  });

  it("needs no newer API than docs/api/openapi.yaml", () => {
    const info = /^info:\n(?:\s+.*\n)*?\s+version:\s*(\S+)/m.exec(
      repoFile("docs/api/openapi.yaml"),
    );
    const [major = 0, minor = 0] = (info?.[1] ?? "").split(".").map(Number);
    const [wantMajor = 0, wantMinor = 0] = String(manifest.min_api_version)
      .split(".")
      .map(Number);
    expect(wantMajor * 1000 + wantMinor).toBeLessThanOrEqual(
      major * 1000 + minor,
    );
    expect(manifest.min_api_version).toBe(`${String(major)}.${String(minor)}`);
  });

  it("declares /issue with the option title and required_level trigger", () => {
    expect(manifest.commands).toEqual([
      {
        name: "issue",
        description: "Create a GitHub issue",
        options: [
          {
            name: "title",
            description: "Issue title",
            type: "string",
            required: true,
          },
        ],
        required_level: "trigger",
      },
    ]);
  });

  it("has both addresses on https and one host", () => {
    const delivery = manifest.delivery as Record<string, string>;
    expect(delivery.mode).toBe("webhook");
    const events = new URL(delivery.events_url ?? "");
    const interactions = new URL(delivery.interactions_url ?? "");
    expect(events.protocol).toBe("https:");
    expect(interactions.protocol).toBe("https:");
    expect(events.host).toBe(interactions.host);
    expect(events.pathname).toBe("/chat/events");
    expect(interactions.pathname).toBe("/chat/interactions");
  });

  it("has no member outside the field table of 11 3.1", () => {
    const allowed = fieldTable();
    expect(allowed).toContain("data_egress");
    for (const key of Object.keys(manifest)) expect(allowed).toContain(key);
  });
});
