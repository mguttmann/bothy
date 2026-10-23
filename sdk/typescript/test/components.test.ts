// SPDX-License-Identifier: Apache-2.0
//
// Components of `component.v1` (docs/spec/09-agents.md 11.1, 11.5 and 11.6); port of
// `sdk/python/tests/test_components.py`. The builders return generated `ComponentCreate`
// objects and refuse, before anything is sent, what the server would refuse with
// `400 validation_failed`: styles and field types outside the vocabularies, the limits
// `component.max_per_message`, `component.custom_id_max_chars` and `component.form_max_fields`,
// and the rules of the definitions. Error texts name the field, never the text a caller passed.
// `respond` and `update` are proven against the FakeServer.

import { readFileSync } from "node:fs";

import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { createApiClient, type ApiClient } from "../src/client.js";
import {
  BUTTON_STYLES,
  FORM_FIELD_TYPES,
  approvalCard,
  button,
  cardMessage,
  checkComponents,
  form,
  questionCard,
  respond,
  select,
  update,
  type FormField,
  type SelectOption,
} from "../src/components/index.js";
import { DEFAULTS, resolveSettings } from "../src/config.js";
import { PreconditionFailedError } from "../src/errors.js";
import type { components } from "../src/generated/openapi.js";
import { FakeServer } from "../src/testing/fakeserver.js";

type ComponentCreate = components["schemas"]["ComponentCreate"];

const SMALL = resolveSettings({
  componentMaxPerMessage: 3,
  componentCustomIdMaxChars: 12,
  componentFormMaxFields: 2,
});

const CHAPTER = readFileSync(
  new URL("../../../docs/spec/09-agents.md", import.meta.url),
  "utf8",
);

/** The text of section `number` of chapter 09 up to the next heading. */
function section(number: string): string {
  const escaped = number.replace(/\./g, "\\.");
  const found = new RegExp(`^### ${escaped} [\\s\\S]*?(?=^#)`, "m").exec(
    CHAPTER,
  );
  if (found === null) throw new Error(`section ${number} not found`);
  return found[0];
}

/** `call` throws a RangeError naming `field` and never showing `value`. */
function refuses(field: string, call: () => unknown, value?: string): void {
  let caught: unknown;
  try {
    call();
  } catch (error) {
    caught = error;
  }
  expect(caught).toBeInstanceOf(RangeError);
  const message = (caught as Error).message;
  expect(message).toContain(field);
  if (value) expect(message).not.toContain(value);
}

function field(
  name: string,
  label: string,
  type: string,
  extra: Partial<FormField> = {},
): FormField {
  return { name, label, type, ...extra };
}

function option(
  label: string,
  value: string,
  description?: string,
): SelectOption {
  return description === undefined
    ? { label, value }
    : { label, value, description };
}

describe("builders", () => {
  it("button, select and form build generated objects", () => {
    const expires = Date.UTC(2026, 9, 21, 12, 0);
    const ok: ComponentCreate = button("ok", "Looks good", "yes", {
      style: "success",
      icon: "check",
      expiresAt: expires,
    });
    expect(ok).toEqual({
      custom_id: "ok",
      type: "button",
      definition: {
        label: "Looks good",
        style: "success",
        icon: "check",
        value: "yes",
      },
      expires_at: "2026-10-21T12:00:00.000Z",
    });
    expect(Object.keys(ok.definition)).toEqual([
      "label",
      "style",
      "icon",
      "value",
    ]);
    expect(button("b", "Label", "v").definition).toEqual({
      label: "Label",
      style: "secondary",
      value: "v",
    });
    expect(
      button("b", "Label", "v", { expiresAt: new Date(expires) }).expires_at,
    ).toBe("2026-10-21T12:00:00.000Z");

    const pick = select(
      "room",
      [option("Room A", "a", "Ground floor"), option("Room B", "b")],
      {
        placeholder: "Pick a room",
        minValues: 1,
        maxValues: 2,
        singleUse: true,
        restrictedTo: "run_starter",
      },
    );
    expect(pick).toEqual({
      custom_id: "room",
      type: "select",
      definition: {
        placeholder: "Pick a room",
        options: [
          { label: "Room A", value: "a", description: "Ground floor" },
          { label: "Room B", value: "b" },
        ],
        min_values: 1,
        max_values: 2,
      },
      single_use: true,
      restricted_to: "run_starter",
    });

    const survey = form(
      "survey",
      "Trip details",
      [
        field("destination", "Destination", "text", {
          required: true,
          maxLength: 80,
        }),
        field("class", "Class", "choice", { choices: ["economy", "business"] }),
      ],
      { submitLabel: "Send" },
    );
    expect(survey.definition).toEqual({
      title: "Trip details",
      submit_label: "Send",
      fields: [
        {
          name: "destination",
          label: "Destination",
          type: "text",
          required: true,
          max_length: 80,
        },
        {
          name: "class",
          label: "Class",
          type: "choice",
          required: false,
          choices: ["economy", "business"],
        },
      ],
    });
    expect(JSON.parse(JSON.stringify(survey))).toEqual(survey);
  });

  it("holds the vocabularies of 09 11.1", () => {
    const kinds = section("11.1");
    const buttonRow = /^\| `button` \|.*?`style` \((.*?)\)/m.exec(kinds);
    const formRow = /^\| `form` \|.*?`type` \S+ (.*?);/m.exec(kinds);
    expect(buttonRow).not.toBeNull();
    expect(formRow).not.toBeNull();
    const names = (text: string): string[] =>
      [...text.matchAll(/`([a-z]+)`/g)].map((m) => m[1] ?? "");
    expect(names(buttonRow?.[1] ?? "")).toEqual([...BUTTON_STYLES]);
    expect(names(formRow?.[1] ?? "")).toEqual([...FORM_FIELD_TYPES]);

    for (const style of BUTTON_STYLES)
      expect(button("b", "Label", "v", { style }).definition.style).toBe(style);
    refuses("style", () => button("b", "Label", "v", { style: "red" }), "red");
    refuses(
      "style",
      () => button("b", "Label", "v", { style: "#ff0000" }),
      "#ff0000",
    );
    refuses("fields[0].type", () => form("f", "T", [field("a", "A", "date")]));
    refuses("required_capability", () =>
      button("b", "Label", "v", { requiredCapability: "terminal" }),
    );
    refuses("required_capability", () =>
      button("b", "Label", "v", { requiredCapability: "superuser" }),
    );
    refuses("restricted_to", () =>
      button("b", "Label", "v", { restrictedTo: "admins" }),
    );
    refuses("expires_at", () =>
      button("b", "Label", "v", { expiresAt: Number.NaN }),
    );
    refuses("expires_at", () =>
      button("b", "Label", "v", { expiresAt: new Date("not a date") }),
    );
    refuses("single_use", () =>
      button("b", "Label", "v", { singleUse: "yes" as unknown as boolean }),
    );
  });

  it("takes the limits from the settings", () => {
    expect(
      button("x".repeat(12), "Label", "v", { settings: SMALL }).custom_id,
    ).toBe("x".repeat(12));
    refuses("custom_id", () =>
      button("x".repeat(13), "Label", "v", { settings: SMALL }),
    );
    refuses("custom_id", () => button("", "Label", "v"));
    const limit = DEFAULTS.componentCustomIdMaxChars;
    expect(button("y".repeat(limit), "Label", "v").custom_id).toBe(
      "y".repeat(limit),
    );
    refuses("custom_id", () => button("y".repeat(limit + 1), "Label", "v"));
    // The limit counts characters, not UTF-16 code units.
    expect(
      button("\u{1f44d}".repeat(12), "Label", "v", { settings: SMALL })
        .custom_id,
    ).toBe("\u{1f44d}".repeat(12));

    const two = [field("a", "A", "text"), field("b", "B", "number")];
    expect(
      form("f", "T", two, { settings: SMALL }).definition.fields,
    ).toHaveLength(2);
    refuses("fields", () =>
      form("f", "T", [...two, field("c", "C", "boolean")], { settings: SMALL }),
    );
    refuses("fields", () => form("f", "T", []));

    const three = [0, 1, 2].map((i) => button(`b${i}`, "Label", "v"));
    checkComponents(three, SMALL);
    refuses("components", () =>
      checkComponents([...three, button("b3", "Label", "v")], SMALL),
    );
    refuses("components", () =>
      checkComponents(
        [three[0] as ComponentCreate, three[0] as ComponentCreate],
        SMALL,
      ),
    );
    refuses("components[1].custom_id", () =>
      checkComponents(
        [
          three[0] as ComponentCreate,
          { ...(three[1] as ComponentCreate), custom_id: "z".repeat(13) },
        ],
        SMALL,
      ),
    );
    checkComponents(
      Array.from({ length: DEFAULTS.componentMaxPerMessage }, (_, i) =>
        button(`c${i}`, "L", "v"),
      ),
    );
  });

  it("holds the rules of the definitions", () => {
    const one = [option("A", "a")];
    refuses("options", () => select("s", []));
    refuses("options[1].value", () => select("s", [...one, option("B", "a")]));
    refuses("max_values", () => select("s", one, { maxValues: 2 }));
    refuses("max_values", () =>
      select("s", [...one, option("B", "b")], { maxValues: 0 }),
    );
    refuses("min_values", () => select("s", one, { minValues: -1 }));
    refuses("min_values", () =>
      select("s", [...one, option("B", "b")], { minValues: 2, maxValues: 1 }),
    );
    refuses("min_values", () =>
      select("s", one, { minValues: true as unknown as number }),
    );
    refuses("min_values", () => select("s", one, { minValues: 0.5 }));
    expect(select("s", one, { minValues: 0 }).definition.min_values).toBe(0);

    refuses("fields[0].name", () =>
      form("f", "T", [field("Bad Name", "A", "text")]),
    );
    refuses("fields[0].name", () =>
      form("f", "T", [field("1st", "A", "text")]),
    );
    refuses("fields[1].name", () =>
      form("f", "T", [field("a", "A", "text"), field("a", "B", "number")]),
    );
    refuses("fields[0].max_length", () =>
      form("f", "T", [field("a", "A", "boolean", { maxLength: 4 })]),
    );
    refuses("fields[0].max_length", () =>
      form("f", "T", [field("a", "A", "textarea", { maxLength: 0 })]),
    );
    refuses("fields[0].choices", () =>
      form("f", "T", [field("a", "A", "choice")]),
    );
    refuses("fields[0].choices", () =>
      form("f", "T", [field("a", "A", "text", { choices: ["x"] })]),
    );
    refuses("fields[0].choices", () =>
      form("f", "T", [field("a", "A", "choice", { choices: ["x", "x"] })]),
    );
    refuses("fields[0].required", () =>
      form("f", "T", [
        field("a", "A", "text", { required: 1 as unknown as boolean }),
      ]),
    );
    refuses("title", () => form("f", "", [field("a", "A", "text")]));
    refuses("label", () => button("b", "", "v"));
    refuses("value", () => button("b", "Label", ""));
  });

  it.each(["line\nbreak", "tab\there", "bidi‮evil", "nul\x00"])(
    "refuses control and bidi characters in %j",
    (text) => {
      refuses("label", () => button("b", text, "v"), text);
      refuses("value", () => button("b", "Label", text), text);
      refuses("custom_id", () => button(text, "Label", "v"), text);
      refuses(
        "options[0].description",
        () => select("s", [option("A", "a", text)]),
        text,
      );
      refuses(
        "fields[0].choices",
        () => form("f", "T", [field("a", "A", "choice", { choices: [text] })]),
        text,
      );
      // Emoji and accented letters are display text like any other.
      expect(button("b", "Café \u{1f44d}‍", "v").definition.label).toBeTruthy();
    },
  );
});

describe("cards", () => {
  it("builds the approval card of the table of 09 11.5", () => {
    const rows = [
      ...section("11.5").matchAll(
        /^\| [^|`]+ \| `([a-z]+)` \| (`[a-z_]+`|none) \| `([a-z_]+)` \|$/gm,
      ),
    ]
      .map((m) => [m[1] ?? "", m[2] ?? "", m[3] ?? ""] as const)
      .filter((r) => r[0] !== "value");
    expect(rows.map((r) => r[0])).toEqual([
      "once",
      "session",
      "always",
      "deny",
    ]);

    const card = approvalCard();
    expect(card.map((c) => c.custom_id)).toEqual(
      rows.map((r) => `approval.${r[0]}`),
    );
    card.forEach((component, i) => {
      const [value, capability, restricted] = rows[i] ?? ["", "", ""];
      expect(component.type).toBe("button");
      expect(component.definition.value).toBe(value);
      expect(component.definition.label).toBeTruthy();
      expect(component.required_capability).toBe(
        capability === "none" ? undefined : capability.replace(/`/g, ""),
      );
      expect(component.restricted_to).toBe(restricted);
      // One decision per card (09 11.3).
      expect(component.single_use).toBe(true);
    });
    expect(card[3]?.definition.style).toBe("danger");

    const without = approvalCard({ withAlways: false });
    expect(without.map((c) => c.custom_id)).toEqual([
      "approval.once",
      "approval.session",
      "approval.deny",
    ]);
    checkComponents(card);
  });

  it("builds the question of 09 11.6", () => {
    const answers = [option("Friday", "fri"), option("Monday", "mon")];

    const buttons = questionCard("when", answers);
    expect(buttons.map((c) => c.custom_id)).toEqual(["when.0", "when.1"]);
    expect(buttons.map((c) => c.definition.value)).toEqual(["fri", "mon"]);
    expect(buttons.map((c) => c.definition.label)).toEqual([
      "Friday",
      "Monday",
    ]);
    // The first answer wins and disables the rest (09 11.6); only the run's starter answers.
    expect(buttons.every((c) => c.single_use === true)).toBe(true);
    expect(buttons.every((c) => c.restricted_to === "run_starter")).toBe(true);

    const [pick, ...rest] = questionCard("when", answers, {
      asSelect: true,
      placeholder: "Pick a day",
    });
    expect(rest).toEqual([]);
    expect(pick?.type).toBe("select");
    expect(pick?.definition.options).toEqual([
      { label: "Friday", value: "fri" },
      { label: "Monday", value: "mon" },
    ]);
    expect([pick?.definition.min_values, pick?.definition.max_values]).toEqual([
      1, 1,
    ]);
    expect(pick?.single_use).toBe(true);

    const four = [0, 1, 2, 3].map((i) => option(String(i), String(i)));
    refuses("answers", () => questionCard("when", []));
    refuses("components", () => questionCard("q", four, { settings: SMALL }));
    refuses("custom_id", () =>
      questionCard("q".repeat(11), answers, { settings: SMALL }),
    );
    expect(
      questionCard("q", four, { asSelect: true, settings: SMALL }),
    ).toHaveLength(1);
  });

  it("wraps components in a MessageCreate body of kind card", () => {
    const runId = "01900000-0000-7000-8000-000000000101";
    const body = cardMessage("Allow the refund?", approvalCard(), { runId });
    expect(body.kind).toBe("card");
    expect(body.run_id).toBe(runId);
    expect(body.content_md).toBe("Allow the refund?");
    expect(body.components).toHaveLength(4);
    expect(body.client_msg_id).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/,
    );
    expect(body).not.toHaveProperty("reply_to_message_id");
    const many = Array.from({ length: 4 }, (_, i) => button(`b${i}`, "L", "v"));
    refuses("components", () => cardMessage("x", many, { settings: SMALL }));
    refuses("components", () => cardMessage("x", []));
  });
});

describe("respond and update against the FakeServer", () => {
  let fake: FakeServer;
  let api: ApiClient;

  beforeEach(async () => {
    fake = new FakeServer();
    await fake.start();
    api = createApiClient({
      baseUrl: fake.baseUrl,
      token: fake.credentials.apiToken,
      dpopKey: fake.credentials.dpop,
      clock: fake.clock,
      settings: { ...fake.sdkSettings, retryMaxAttempts: 1 },
    });
  });

  afterEach(async () => {
    await fake.stop();
  });

  it("answers an interaction and disables its component", async () => {
    const run = await fake.trigger();
    const interactionId = await fake.deliverInteraction(
      run.runId,
      "approval.once",
    );

    const answered = await respond(api, interactionId, "acknowledged", {
      text: "Refund approved",
    });
    expect(answered.id).toBe(interactionId);
    expect(answered.state).toBe("acknowledged");
    expect(answered.response).toEqual({ text: "Refund approved" });

    const disabled = await update(api, answered.component_id, 1, {
      state: "disabled",
    });
    expect(disabled.state).toBe("disabled");
    expect(disabled.version).toBe(2);
    // A stale version is the server's 412, typed.
    await expect(
      update(api, answered.component_id, 1, { state: "active" }),
    ).rejects.toBeInstanceOf(PreconditionFailedError);
  });

  it("checks the preconditions before sending", async () => {
    const before = fake.requests().length;
    await expect(
      respond(api, "01900000-0000-7000-8000-000000000001", "done" as "failed"),
    ).rejects.toThrow(/state/);
    await expect(
      update(api, "01900000-0000-7000-8000-000000000001", 0, {
        state: "disabled",
      }),
    ).rejects.toThrow(/version/);
    await expect(
      update(api, "01900000-0000-7000-8000-000000000001", 1, {
        state: "used" as "active",
      }),
    ).rejects.toThrow(/state/);
    await expect(
      update(api, "01900000-0000-7000-8000-000000000001", 1, {
        definition: { label: "x" },
        type: "button",
      } as never),
    ).rejects.toThrow(/type/);
    expect(fake.requests().length).toBe(before);
  });
});
