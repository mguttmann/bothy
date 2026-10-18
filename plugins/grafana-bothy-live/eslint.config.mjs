// SPDX-License-Identifier: Apache-2.0
//
// Extends the ESLint configuration of the scaffold (.config/eslint.config.mjs, kept as
// generated). Additions: the rule bothy/no-color-literal for the source code (colours only from
// the design tokens, docs/DECISIONS.md D-M7-25-3), a doc comment on every exported symbol of
// src/ (brief M7-25, definition of done), and typed linting of the tests and scripts.

import { defineConfig } from 'eslint/config';
import jsdoc from 'eslint-plugin-jsdoc';

import baseConfig from './.config/eslint.config.mjs';
import noColorLiteral from './eslint-rules/no-color-literal.cjs';

export default defineConfig([
  {
    ignores: ['**/node_modules/', '**/coverage/', '**/dist/', '**/.eslintcache'],
  },
  ...baseConfig,
  {
    files: ['src/**/*.{ts,tsx}'],
    plugins: { bothy: { rules: { 'no-color-literal': noColorLiteral } } },
    rules: { 'bothy/no-color-literal': 'error' },
  },
  {
    files: ['src/**/*.{ts,tsx}'],
    ignores: ['src/**/*.test.{ts,tsx}'],
    plugins: { jsdoc },
    rules: {
      'jsdoc/require-jsdoc': [
        'error',
        {
          publicOnly: true,
          require: { FunctionDeclaration: true, ArrowFunctionExpression: true, ClassDeclaration: true },
          contexts: [
            'TSInterfaceDeclaration',
            'TSTypeAliasDeclaration',
            'ExportNamedDeclaration > VariableDeclaration',
          ],
        },
      ],
    },
  },
  {
    files: ['test/**/*.{ts,tsx}', 'scripts/**/*.mts'],
    languageOptions: {
      parserOptions: {
        project: './tsconfig.json',
      },
    },
  },
]);
