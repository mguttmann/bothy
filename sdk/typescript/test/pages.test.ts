// SPDX-License-Identifier: Apache-2.0
//
// Cursor pagination (docs/spec/06-api.md 14 point 1): pages follow `next_cursor` until it is
// absent.

import { describe, expect, it } from "vitest";

import { pages, pageItems } from "../src/pages.js";

interface UserPageBody {
  items: string[];
  next_cursor?: string;
}

const PAGES: Record<string, UserPageBody> = {
  start: { items: ["Alex", "Sam"], next_cursor: "c1" },
  c1: { items: [], next_cursor: "c2" },
  c2: { items: ["Robin"] },
};

describe("pages", () => {
  it("follows next_cursor until it is absent", async () => {
    const cursors: (string | undefined)[] = [];
    const seen: UserPageBody[] = [];
    for await (const page of pages((cursor) => {
      cursors.push(cursor);
      return Promise.resolve(PAGES[cursor ?? "start"] as UserPageBody);
    }))
      seen.push(page);
    expect(cursors).toEqual([undefined, "c1", "c2"]);
    expect(seen.map((p) => p.items)).toEqual([["Alex", "Sam"], [], ["Robin"]]);
  });

  it("stops early when the caller breaks and starts at a given cursor", async () => {
    const cursors: (string | undefined)[] = [];
    for await (const page of pages(
      (cursor) => {
        cursors.push(cursor);
        return Promise.resolve(PAGES[cursor ?? "start"] as UserPageBody);
      },
      { cursor: "c1" },
    )) {
      expect(page.items).toEqual([]);
      break;
    }
    expect(cursors).toEqual(["c1"]);
  });

  it("passes a failed page to the caller", async () => {
    const failing = pages<UserPageBody>(() =>
      Promise.reject(new Error("boom")),
    );
    await expect(failing.next()).rejects.toThrow("boom");
  });

  it("pageItems yields the items of every page", async () => {
    const items: string[] = [];
    for await (const item of pageItems((cursor) =>
      Promise.resolve(PAGES[cursor ?? "start"] as UserPageBody),
    ))
      items.push(item);
    expect(items).toEqual(["Alex", "Sam", "Robin"]);
  });

  it("treats a null next_cursor as the last page", async () => {
    let calls = 0;
    const seen: number[][] = [];
    for await (const page of pages(() => {
      calls++;
      return Promise.resolve({ items: [1], next_cursor: null });
    }))
      seen.push(page.items);
    expect(calls).toBe(1);
    expect(seen).toEqual([[1]]);
  });
});
