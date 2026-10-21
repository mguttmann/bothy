// SPDX-License-Identifier: Apache-2.0
//
// ESLint with the type-checked recommended rules of typescript-eslint (docs/DECISIONS.md
// M2-23/11). Generated code is outside style lint (M2-23/7); tsc in `make typecheck`
// and `generate-check` still checks it.

import js from "@eslint/js";
import tseslint from "typescript-eslint";

export default tseslint.config(
  // dist/ and build/ hold the outputs of `npm run build` and `npm run docs` (D-M2-30-2).
  {
    ignores: [
      "src/generated/",
      "coverage/",
      "node_modules/",
      "dist/",
      "build/",
    ],
  },
  js.configs.recommended,
  ...tseslint.configs.recommendedTypeChecked,
  {
    languageOptions: {
      parserOptions: {
        projectService: { allowDefaultProject: ["eslint.config.mjs"] },
        tsconfigRootDir: import.meta.dirname,
      },
    },
  },
  {
    files: ["**/*.mjs"],
    ...tseslint.configs.disableTypeChecked,
  },
);
