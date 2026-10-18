/** @jest-environment node */
// SPDX-License-Identifier: Apache-2.0
//
// The generator scripts/gen-queries.mts runs as the real command line, the way `make generate`
// and `make generate-check` call it, so the test holds the artifact and not a copy of its logic.

import { spawnSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';

const ROOT = path.resolve(__dirname, '..');
const SCRIPT = path.join(ROOT, 'scripts', 'gen-queries.mts');
const YAML = path.join(ROOT, 'queries.yaml');
const GENERATED = path.join(ROOT, 'src', 'queries.gen.ts');

function run(args: string[]) {
  const done = spawnSync(process.execPath, ['--no-warnings', SCRIPT, ...args], { cwd: ROOT, encoding: 'utf8' });
  return { code: done.status, out: done.stdout + done.stderr };
}

let dir = '';
beforeEach(() => {
  dir = mkdtempSync(path.join(tmpdir(), 'gen-queries-'));
});
afterEach(() => {
  rmSync(dir, { recursive: true, force: true });
});

describe('gen-queries', () => {
  it('finds the checked-in file current', () => {
    const r = run(['--check']);
    expect(r.out).toContain('current');
    expect(r.code).toBe(0);
  });

  it('writes exactly the checked-in file', () => {
    const out = path.join(dir, 'queries.gen.ts');
    expect(run(['--yaml', YAML, '--out', out]).code).toBe(0);
    expect(readFileSync(out, 'utf8')).toBe(readFileSync(GENERATED, 'utf8'));
  });

  it('fails the check when queries.yaml changed without regeneration', () => {
    const changed = path.join(dir, 'queries.yaml');
    writeFileSync(changed, readFileSync(YAML, 'utf8').replace('* 60', '* 120'));
    const r = run(['--check', '--yaml', changed, '--out', GENERATED]);
    expect(r.code).toBe(1);
    expect(r.out).toContain('stale');
  });

  it.each([
    ['an unknown view', (y: string) => y.replace('  timeline:', '  replay:')],
    ['a missing view', (y: string) => y.slice(0, y.indexOf('  timeline:'))],
    ['a duplicate ref_id', (y: string) => y.replace('ref_id: tool_steps', 'ref_id: active')],
    ['an empty expr', (y: string) => y.replace(/expr: max\(bothy_event_fanout_lag_seconds[^\n]*/, 'expr: ""')],
    ['an unknown unit', (y: string) => y.replace('unit: per_minute', 'unit: furlongs')],
    ['an unknown field', (y: string) => y.replace('instant: true', 'instant: true\n        datasource: loki')],
    ['an instant that is no boolean', (y: string) => y.replace('instant: true', 'instant: "yes"')],
  ])('refuses %s', (_name, edit) => {
    const bad = path.join(dir, 'queries.yaml');
    writeFileSync(bad, edit(readFileSync(YAML, 'utf8')));
    const r = run(['--yaml', bad, '--out', path.join(dir, 'out.ts')]);
    expect(r.code).toBe(2);
    expect(r.out).toMatch(/queries\.yaml/);
  });
});
