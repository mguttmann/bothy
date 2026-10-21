/** @jest-environment node */
// SPDX-License-Identifier: Apache-2.0
//
// The lint rule bothy/no-color-literal (eslint-rules/no-color-literal.cjs) keeps colours out of
// src/: they come only from design/build/ts/tokens.gen.ts (docs/spec/25-visualization.md 10).

import { spawnSync } from 'node:child_process';
import path from 'node:path';

import { Linter } from 'eslint';

const rule = require('../eslint-rules/no-color-literal.cjs');

const ROOT = path.resolve(__dirname, '..');

function lint(code: string): string[] {
  const linter = new Linter({ configType: 'flat' });
  const messages = linter.verify(
    code,
    [
      {
        files: ['**/*.js'],
        plugins: { bothy: { rules: { 'no-color-literal': rule } } },
        rules: { 'bothy/no-color-literal': 'error' },
      },
    ],
    'src/x.js'
  );
  return messages.map((m) => m.message);
}

describe('bothy/no-color-literal', () => {
  it.each([
    ['a hex colour', "const c = '#136f69';"],
    ['a short hex colour', "const c = '#fff';"],
    ['a hex colour inside CSS text', "const c = 'border: 1px solid #136F69';"],
    ['rgb()', "const c = 'rgb(0, 0, 0)';"],
    ['rgba()', "const c = 'rgba(0,0,0,0.5)';"],
    ['hsl()', "const c = 'hsl(120 50% 50%)';"],
    ['oklch()', "const c = 'oklch(0.7 0.1 200)';"],
    ['a named colour', "const s = { color: 'red' };"],
    ['a named colour in other case', "const s = { color: 'RebeccaPurple' };"],
    ['transparent', "const s = { background: 'transparent' };"],
    ['a named colour in a CSS declaration', "const c = 'color: navy';"],
    ['a colour in a template literal', 'const c = `1px solid ${w} #00ff00`;'],
    ['a named colour in a template literal declaration', 'const c = `background: ${x}; color: teal`;'],
    ['a named colour inside a border value', "const s = { border: '1px solid red' };"],
    ['a named colour inside a shadow value', "const s = { boxShadow: '0 0 2px black' };"],
    ['a named colour inside a quoted style key', "const s = { 'outline-color': 'thin dotted Blue' };"],
  ])('rejects %s', (_name, code) => {
    expect(lint(code)).toEqual([expect.stringContaining('design tokens')]);
  });

  it.each([
    ['a token reference', 'const s = { color: theme.colors.text.primary };'],
    ['text that only mentions a colour word', "const t = 'budget tight';"],
    ['an id-like hash that is no colour', "const t = '#backend';"],
    ['a word that contains a colour name', "const t = 'reduced';"],
    ['none and currentColor', "const a = 'none'; const b = 'currentColor';"],
    ['a colour word in a property that holds no colour', "const s = { label: 'red team' };"],
  ])('accepts %s', (_name, code) => {
    expect(lint(code)).toEqual([]);
  });

  it('is switched on for every source file of src/ by the project configuration', () => {
    const done = spawnSync(
      process.execPath,
      [path.join(ROOT, 'node_modules', 'eslint', 'bin', 'eslint.js'), '--print-config', 'src/module.ts'],
      { cwd: ROOT, encoding: 'utf8' }
    );
    expect(done.status).toBe(0);
    const config = JSON.parse(done.stdout) as { rules: Record<string, unknown> };
    expect(config.rules['bothy/no-color-literal']).toEqual([2]);
  });
});
