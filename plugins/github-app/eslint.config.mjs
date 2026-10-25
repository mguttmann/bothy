// SPDX-License-Identifier: Apache-2.0
//
// ESLint with the type-checked recommended rules of typescript-eslint, as the TypeScript SDK
// runs it (sdk/typescript/eslint.config.mjs; docs/decisions.d/M2-49.md D-M2-49-1).

import js from "@eslint/js";
import tseslint from "typescript-eslint";

export default tseslint.config(
  // dist/ holds the output of `npm run build`, coverage/ that of `npm test`.
  {
    ignores: ["coverage/", "node_modules/", "dist/"],
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
