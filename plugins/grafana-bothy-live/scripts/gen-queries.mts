// SPDX-License-Identifier: Apache-2.0
/**
 * Generator of `src/queries.gen.ts` from `queries.yaml` (docs/spec/25-visualization.md 8.2).
 *
 * The YAML file is the only source of the panel's queries; the TypeScript file is derived and
 * never edited by hand. Node runs this file directly (type stripping, Node >= 22.18).
 *
 * Usage:
 *   node scripts/gen-queries.mts [--check] [--yaml <file>] [--out <file>]
 *
 * Without `--check` the script writes the output file. With `--check` it compares a fresh
 * rendering with the output file and changes nothing.
 *
 * Exit codes: 0 written or current; 1 `--check` found the output stale; 2 the YAML file is
 * invalid (the message names the file and the field).
 *
 * Example: `node scripts/gen-queries.mts --check` in the plugin directory.
 */

import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { parse } from 'yaml';

/** The five views of 25 section 8.2, in display order; the YAML file must name exactly these. */
const VIEWS = ['topology', 'backend', 'delivery', 'executive', 'timeline'] as const;
/** Units a view knows how to format. */
const UNITS = ['count', 'per_minute', 'seconds', 'ratio', 'currency'] as const;
/** Fields of one query; any other field is an error, so a typo cannot pass silently. */
const QUERY_FIELDS = ['ref_id', 'expr', 'instant', 'legend', 'unit'] as const;
const REF_ID = /^[a-z][a-z0-9_]*$/;

/** One validated query. */
interface Query {
  refId: string;
  expr: string;
  instant: boolean;
  legend: string;
  unit: (typeof UNITS)[number];
}

/** A YAML file that does not have the shape of the module docstring. */
class InvalidQueries extends Error {}

function fail(file: string, where: string, message: string): never {
  throw new InvalidQueries(`${file}: ${where}: ${message}`);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/**
 * Parses and validates the YAML text.
 *
 * @param text - content of queries.yaml
 * @param file - name used in error messages
 * @returns title and queries per view, in the order of VIEWS
 * @throws InvalidQueries when a view, field or value does not match the documented shape
 */
function readQueries(text: string, file: string): Array<{ view: string; title: string; queries: Query[] }> {
  const doc: unknown = parse(text);
  if (!isRecord(doc) || !isRecord(doc.views)) {
    fail(file, 'top level', 'needs a mapping "views"');
  }
  const views = doc.views as Record<string, unknown>;
  const names = Object.keys(views);
  for (const name of names) {
    if (!(VIEWS as readonly string[]).includes(name)) {
      fail(file, `views.${name}`, `unknown view; allowed: ${VIEWS.join(', ')}`);
    }
  }
  return VIEWS.map((view) => {
    const entry = views[view];
    if (!isRecord(entry)) {
      fail(file, `views.${view}`, 'missing');
    }
    if (typeof entry.title !== 'string' || entry.title === '') {
      fail(file, `views.${view}.title`, 'needs a non-empty string');
    }
    if (!Array.isArray(entry.queries) || entry.queries.length === 0) {
      fail(file, `views.${view}.queries`, 'needs a non-empty list');
    }
    const seen = new Set<string>();
    const queries = (entry.queries as unknown[]).map((raw, i) => {
      const where = `views.${view}.queries[${i}]`;
      if (!isRecord(raw)) {
        fail(file, where, 'needs a mapping');
      }
      for (const key of Object.keys(raw)) {
        if (!(QUERY_FIELDS as readonly string[]).includes(key)) {
          fail(file, `${where}.${key}`, `unknown field; allowed: ${QUERY_FIELDS.join(', ')}`);
        }
      }
      const { ref_id: refId, expr, instant, legend, unit } = raw;
      if (typeof refId !== 'string' || !REF_ID.test(refId)) {
        fail(file, `${where}.ref_id`, 'needs a lower-case identifier');
      }
      if (seen.has(refId)) {
        fail(file, `${where}.ref_id`, `duplicate ref_id ${refId} in view ${view}`);
      }
      seen.add(refId);
      if (typeof expr !== 'string' || expr.trim() === '') {
        fail(file, `${where}.expr`, 'needs a non-empty PromQL expression');
      }
      if (typeof instant !== 'boolean') {
        fail(file, `${where}.instant`, 'needs true or false');
      }
      if (typeof legend !== 'string') {
        fail(file, `${where}.legend`, 'needs a string');
      }
      if (typeof unit !== 'string' || !(UNITS as readonly string[]).includes(unit)) {
        fail(file, `${where}.unit`, `needs one of ${UNITS.join(', ')}`);
      }
      return { refId, expr: expr.trim(), instant, legend, unit: unit as Query['unit'] };
    });
    return { view, title: entry.title, queries };
  });
}

/**
 * Renders the TypeScript module.
 *
 * @param text - content of queries.yaml
 * @param file - name used in error messages
 * @returns the complete content of src/queries.gen.ts
 * @throws InvalidQueries when the YAML file is invalid
 */
export function renderQueries(text: string, file: string): string {
  const views = readQueries(text, file);
  const lines = [
    '// SPDX-License-Identifier: Apache-2.0',
    '// Generated by scripts/gen-queries.mts from queries.yaml. Do not edit; change queries.yaml and run `make generate`.',
    '',
    '/** Names of the five views of docs/spec/25-visualization.md section 8.2, in display order. */',
    `export const VIEW_NAMES = ${JSON.stringify(VIEWS)} as const;`,
    '',
    '/** One of the five views. */',
    'export type ViewName = (typeof VIEW_NAMES)[number];',
    '',
    '/** How a view formats the value of a query. */',
    `export type QueryUnit = ${UNITS.map((u) => JSON.stringify(u)).join(' | ')};`,
    '',
    '/** One Grafana target of a view: the view reads the frames of `refId`. */',
    'export interface LiveQuery {',
    '  readonly refId: string;',
    '  readonly expr: string;',
    '  readonly instant: boolean;',
    '  readonly legend: string;',
    '  readonly unit: QueryUnit;',
    '}',
    '',
    '/** Title and queries of one view. */',
    'export interface ViewQueries {',
    '  readonly title: string;',
    '  readonly queries: readonly LiveQuery[];',
    '}',
    '',
    '/** The queries of every view, from queries.yaml. */',
    'export const QUERIES: Readonly<Record<ViewName, ViewQueries>> = {',
  ];
  for (const { view, title, queries } of views) {
    lines.push(`  ${view}: {`, `    title: ${JSON.stringify(title)},`, '    queries: [');
    for (const q of queries) {
      lines.push(
        '      {',
        `        refId: ${JSON.stringify(q.refId)},`,
        `        expr: ${JSON.stringify(q.expr)},`,
        `        instant: ${q.instant},`,
        `        legend: ${JSON.stringify(q.legend)},`,
        `        unit: ${JSON.stringify(q.unit)},`,
        '      },'
      );
    }
    lines.push('    ],', '  },');
  }
  lines.push('};', '');
  return lines.join('\n');
}

function main(argv: string[]): number {
  const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
  let check = false;
  let yamlFile = path.join(root, 'queries.yaml');
  let outFile = path.join(root, 'src', 'queries.gen.ts');
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === '--check') {
      check = true;
    } else if (arg === '--yaml' && argv[i + 1] !== undefined) {
      yamlFile = path.resolve(argv[++i] ?? '');
    } else if (arg === '--out' && argv[i + 1] !== undefined) {
      outFile = path.resolve(argv[++i] ?? '');
    } else {
      process.stderr.write(
        `gen-queries: unknown argument ${arg}\nusage: gen-queries.mts [--check] [--yaml <file>] [--out <file>]\n`
      );
      return 2;
    }
  }
  let rendered: string;
  try {
    rendered = renderQueries(readFileSync(yamlFile, 'utf8'), path.relative(root, yamlFile) || yamlFile);
  } catch (err) {
    if (err instanceof InvalidQueries) {
      process.stderr.write(`gen-queries: ${err.message}\n`);
      return 2;
    }
    throw err;
  }
  const rel = path.relative(root, outFile) || outFile;
  if (check) {
    let current = '';
    try {
      current = readFileSync(outFile, 'utf8');
    } catch {
      current = '';
    }
    if (current !== rendered) {
      process.stderr.write(
        `gen-queries: ${rel} is stale against queries.yaml; run \`make generate\` and commit the result\n`
      );
      return 1;
    }
    process.stdout.write(`gen-queries: ${rel} is current\n`);
    return 0;
  }
  writeFileSync(outFile, rendered);
  process.stdout.write(`gen-queries: wrote ${rel}\n`);
  return 0;
}

process.exitCode = main(process.argv.slice(2));
