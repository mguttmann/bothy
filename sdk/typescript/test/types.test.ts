// SPDX-License-Identifier: Apache-2.0
/**
 * The hand layer declares no own type for an object the core generates (ADR 0026
 * "Confirmation", docs/spec/11-erweiterungen.md 11.1).
 *
 * Every exported interface, type alias, class and enum of a `.ts` file under `src/` outside
 * `src/generated/` is read with the TypeScript compiler API; none may carry the name of a schema
 * under `components.schemas`. The schema names come from the generated core, which gate G4 holds
 * equal to docs/api/openapi.yaml. The one allowed form is an alias that only names the
 * generated type of the same name (`type Event = components["schemas"]["Event"]`): it declares
 * nothing of its own.
 */

import { readFileSync, readdirSync } from "node:fs";
import { join, relative, sep } from "node:path";
import { fileURLToPath } from "node:url";

import ts from "typescript";
import { describe, expect, it } from "vitest";

const SRC = fileURLToPath(new URL("../src/", import.meta.url));
const GENERATED = join(SRC, "generated");

/** Every `.ts` file below `dir`. */
function tsFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true, recursive: true })
    .filter((entry) => entry.isFile() && entry.name.endsWith(".ts"))
    .map((entry) => join(entry.parentPath, entry.name));
}

function parse(path: string): ts.SourceFile {
  return ts.createSourceFile(
    path,
    readFileSync(path, "utf8"),
    ts.ScriptTarget.Latest,
    true,
  );
}

/** The member names of `components.schemas` in the generated core. */
function schemaNames(): Set<string> {
  const names = new Set<string>();
  for (const file of tsFiles(GENERATED).map(parse)) {
    for (const statement of file.statements) {
      if (
        !ts.isInterfaceDeclaration(statement) ||
        statement.name.text !== "components"
      )
        continue;
      for (const member of statement.members) {
        if (
          !ts.isPropertySignature(member) ||
          member.name.getText(file) !== "schemas" ||
          member.type === undefined ||
          !ts.isTypeLiteralNode(member.type)
        )
          continue;
        for (const schema of member.type.members)
          if (schema.name !== undefined)
            names.add(schema.name.getText(file).replace(/^["']|["']$/g, ""));
      }
    }
  }
  return names;
}

function isExported(node: ts.Node): boolean {
  return (
    ts.canHaveModifiers(node) &&
    (ts.getModifiers(node) ?? []).some(
      (m) => m.kind === ts.SyntaxKind.ExportKeyword,
    )
  );
}

/** Whether `node` is `components["schemas"]["<name>"]`, the generated type itself. */
function namesGenerated(node: ts.TypeNode, name: string): boolean {
  if (!ts.isIndexedAccessTypeNode(node)) return false;
  const outer = node.objectType;
  return (
    literal(node.indexType) === name &&
    ts.isIndexedAccessTypeNode(outer) &&
    literal(outer.indexType) === "schemas" &&
    ts.isTypeReferenceNode(outer.objectType) &&
    outer.objectType.typeName.getText() === "components"
  );
}

function literal(node: ts.TypeNode): string | undefined {
  return ts.isLiteralTypeNode(node) && ts.isStringLiteral(node.literal)
    ? node.literal.text
    : undefined;
}

/** An exported declaration of the hand layer: file, kind and name. */
interface Declared {
  readonly file: string;
  readonly kind: string;
  readonly name: string;
}

/** The exported type-like declarations of one file, without aliases of the generated type. */
function declarations(file: ts.SourceFile): Declared[] {
  const out: Declared[] = [];
  const where = relative(SRC, file.fileName);
  for (const statement of file.statements) {
    if (!isExported(statement)) continue;
    if (ts.isTypeAliasDeclaration(statement)) {
      const name = statement.name.text;
      if (!namesGenerated(statement.type, name))
        out.push({ file: where, kind: "type", name });
    } else if (ts.isInterfaceDeclaration(statement)) {
      out.push({ file: where, kind: "interface", name: statement.name.text });
    } else if (ts.isClassDeclaration(statement) && statement.name) {
      out.push({ file: where, kind: "class", name: statement.name.text });
    } else if (ts.isEnumDeclaration(statement)) {
      out.push({ file: where, kind: "enum", name: statement.name.text });
    }
  }
  return out;
}

const handFiles = tsFiles(SRC).filter(
  (path) => !path.startsWith(GENERATED + sep),
);
const schemas = schemaNames();

describe("no hand type repeats a generated schema", () => {
  it("finds the schemas of the generated core", () => {
    expect(schemas.has("RunStart")).toBe(true);
    expect(schemas.has("Problem")).toBe(true);
    expect(schemas.size).toBeGreaterThan(100);
  });

  it("reads the hand layer and skips the generated core", () => {
    const names = handFiles.map((path) => relative(SRC, path));
    expect(names).toContain("config.ts");
    expect(names.some((name) => name.startsWith("generated"))).toBe(false);
  });

  it("finds an own declaration with a schema name and allows the alias", () => {
    const sample = ts.createSourceFile(
      join(SRC, "sample.ts"),
      [
        'export type Event = components["schemas"]["Event"];',
        'export type EventPage = components["schemas"]["Event"];',
        "export interface RunStart { readonly x: string }",
        "export class Problem {}",
        "interface Heartbeat {}",
      ].join("\n"),
      ts.ScriptTarget.Latest,
      true,
    );
    expect(declarations(sample).map((d) => d.name)).toEqual([
      "EventPage",
      "RunStart",
      "Problem",
    ]);
  });

  it("declares no exported type with the name of a schema", () => {
    const clashes = handFiles
      .flatMap((path) => declarations(parse(path)))
      .filter((d) => schemas.has(d.name))
      .map((d) => `${d.file}: ${d.kind} ${d.name}`);
    expect(clashes).toEqual([]);
  });
});
