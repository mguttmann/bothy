// SPDX-License-Identifier: Apache-2.0
/**
 * The build output `dist/` and the entry points of `package.json` (docs/DECISIONS.md D-M2-30-2,
 * docs/spec/11-erweiterungen.md 11.3).
 *
 * `npm run build` runs once before the checks. The three entries `.`, `./core` and `./testing`
 * load in a separate Node.js process through the package's own `exports` (self-reference), so
 * the test proves what an integration resolves, not what Vitest resolves. The module graph of
 * `dist/core.js` is followed through every static and dynamic import: the browser-safe entry
 * must reach no Node.js built-in module and none of the modules that hold secrets (`auth`,
 * `invocation`, `gateway`, `webhooks`, `outbox`).
 */

import { execFileSync, spawnSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import { builtinModules } from "node:module";
import { dirname, join, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

import { beforeAll, describe, expect, it } from "vitest";

const PACKAGE = fileURLToPath(new URL("..", import.meta.url));
const DIST = join(PACKAGE, "dist");
const SECRET_MODULES = ["auth", "invocation", "gateway", "webhooks", "outbox"];
const BUILTINS = new Set(builtinModules);
// Static `import ... from "x"`, `export ... from "x"`, bare `import "x"` and dynamic `import("x")`.
const SPECIFIER =
  /(?:\bfrom\s*|\bimport\s*\(\s*|^\s*import\s+)["']([^"']+)["']/gm;

interface PackageJson {
  readonly name: string;
  readonly private?: boolean;
  readonly type?: string;
  readonly sideEffects?: boolean;
  readonly files?: readonly string[];
  readonly exports?: Readonly<
    Record<string, { readonly types?: string; readonly import?: string }>
  >;
  readonly scripts?: Readonly<Record<string, string>>;
}

const manifest = JSON.parse(
  readFileSync(join(PACKAGE, "package.json"), "utf8"),
) as PackageJson;

/** The import specifiers of one emitted module. */
function specifiers(path: string): string[] {
  return [...readFileSync(path, "utf8").matchAll(SPECIFIER)].map(
    (match) => match[1] ?? "",
  );
}

/** Every module file `entry` reaches through relative imports, and every other specifier. */
function graph(entry: string): { files: string[]; external: string[] } {
  const files = new Set<string>();
  const external = new Set<string>();
  const queue = [entry];
  for (let path = queue.pop(); path !== undefined; path = queue.pop()) {
    if (files.has(path)) continue;
    files.add(path);
    for (const spec of specifiers(path)) {
      if (spec.startsWith(".")) queue.push(resolve(dirname(path), spec));
      else external.add(spec);
    }
  }
  return { files: [...files], external: [...external] };
}

/** Whether `spec` names a Node.js built-in module (with or without `node:`). */
function isBuiltin(spec: string): boolean {
  return spec.startsWith("node:") || BUILTINS.has(spec.split("/")[0] ?? spec);
}

beforeAll(() => {
  const build = spawnSync("npm", ["run", "--silent", "build"], {
    cwd: PACKAGE,
    encoding: "utf8",
  });
  // tsc reports its errors on stdout; they name files and types, never data.
  if (build.status !== 0)
    throw new Error(`npm run build failed:\n${build.stdout}${build.stderr}`);
}, 180_000);

describe("package.json", () => {
  it("stays private, ESM and free of side effects, and ships only dist", () => {
    expect(manifest.private).toBe(true);
    expect(manifest.type).toBe("module");
    expect(manifest.sideEffects).toBe(false);
    expect(manifest.files).toEqual(["dist"]);
  });

  it("maps the three entries to types and modules in dist", () => {
    expect(manifest.exports).toEqual({
      ".": { types: "./dist/index.d.ts", import: "./dist/index.js" },
      "./core": { types: "./dist/core.d.ts", import: "./dist/core.js" },
      "./testing": {
        types: "./dist/testing/index.d.ts",
        import: "./dist/testing/index.js",
      },
    });
    for (const entry of Object.values(manifest.exports ?? {})) {
      expect(existsSync(join(PACKAGE, entry.types ?? ""))).toBe(true);
      expect(existsSync(join(PACKAGE, entry.import ?? ""))).toBe(true);
    }
  });

  it("emits source maps and no test file", () => {
    expect(existsSync(join(DIST, "index.js.map"))).toBe(true);
    expect(existsSync(join(DIST, "test"))).toBe(false);
    expect(existsSync(join(DIST, "generated", "openapi.d.ts"))).toBe(true);
  });
});

describe("entry points", () => {
  it("load in Node.js through the package's exports", () => {
    const script = [
      `const names = ["${manifest.name}", "${manifest.name}/core", "${manifest.name}/testing"];`,
      "const out = {};",
      "for (const name of names) out[name] = Object.keys(await import(name)).length;",
      "console.log(JSON.stringify(out));",
    ].join("\n");
    const output = execFileSync(
      process.execPath,
      ["--input-type=module", "--eval", script],
      { cwd: PACKAGE, encoding: "utf8" },
    );
    const counts = JSON.parse(output) as Record<string, number>;
    expect(Object.keys(counts)).toHaveLength(3);
    for (const count of Object.values(counts)) expect(count).toBeGreaterThan(0);
  });
});

describe("dist/core.js", () => {
  const reach = () => graph(join(DIST, "core.js"));

  it("follows the module graph", () => {
    const { files } = reach();
    expect(files.length).toBeGreaterThan(1);
    expect(files.some((path) => path.endsWith(`${sep}tolerance.js`))).toBe(
      true,
    );
  });

  it("imports no Node.js built-in module", () => {
    expect(reach().external.filter(isBuiltin)).toEqual([]);
  });

  it("reaches none of the modules that hold secrets", () => {
    const reached = reach()
      .files.map((path) => relative(DIST, path).split(sep)[0] ?? "")
      .map((first) => first.replace(/\.js$/, ""));
    expect(reached.filter((first) => SECRET_MODULES.includes(first))).toEqual(
      [],
    );
  });

  it("detects a forbidden import in a sample graph", () => {
    const sample = 'import { a } from "node:crypto";\nimport "./x.js";\n';
    const found = [...sample.matchAll(SPECIFIER)].map((m) => m[1]);
    expect(found).toEqual(["node:crypto", "./x.js"]);
    expect(isBuiltin("crypto")).toBe(true);
    expect(isBuiltin("openapi-fetch")).toBe(false);
  });
});
