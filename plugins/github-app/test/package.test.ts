// SPDX-License-Identifier: Apache-2.0
// The package (brief M2-49 Design note 2).

import { describe, expect, it } from "vitest";

import { APP_VERSION } from "../src/github/client.js";
import { appFile, repoFile } from "./helpers/repo.js";

const pkg = JSON.parse(appFile("package.json")) as {
  private: boolean;
  type: string;
  license: string;
  version: string;
  dependencies: Record<string, string>;
  devDependencies: Record<string, string>;
};
const sdk = JSON.parse(repoFile("sdk/typescript/package.json")) as {
  devDependencies: Record<string, string>;
};

describe("package.json", () => {
  it("is private, ESM, Apache-2.0 and at the version of VERSION", () => {
    expect(pkg.private).toBe(true);
    expect(pkg.type).toBe("module");
    expect(pkg.license).toBe("Apache-2.0");
    expect(pkg.version).toBe(appFile("VERSION").trim());
    expect(APP_VERSION).toBe(pkg.version);
  });

  it("depends on the SDK of the repository only", () => {
    expect(pkg.dependencies).toEqual({
      "bothy-sdk": "file:../../sdk/typescript",
    });
  });

  it("pins every dev dependency exactly, at the SDK's versions where shared", () => {
    for (const [name, version] of Object.entries(pkg.devDependencies)) {
      expect(version).toMatch(/^\d+\.\d+\.\d+$/);
      const shared = sdk.devDependencies[name];
      if (shared !== undefined) expect(version).toBe(shared);
    }
  });
});
