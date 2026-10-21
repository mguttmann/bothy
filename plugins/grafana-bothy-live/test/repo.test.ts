/** @jest-environment node */
// SPDX-License-Identifier: Apache-2.0
//
// Checks that tie the plugin to files outside src/: the seventh dashboard, the Grafana pin of
// the Compose stack, the logo colours and the license overrides of osv-scanner.toml.

import { readFileSync } from 'node:fs';
import path from 'node:path';

import { dark, light } from '../../../design/build/ts/tokens.gen';
import pluginJson from '../src/plugin.json';
import { QUERIES, VIEW_NAMES } from '../src/queries.gen';

const ROOT = path.resolve(__dirname, '..');
const REPO = path.resolve(ROOT, '..', '..');

interface Target {
  refId: string;
  expr: string;
  instant: boolean;
  range: boolean;
  legendFormat: string;
  datasource: { type: string; uid: string };
}
interface Panel {
  type: string;
  title: string;
  options: { view: string };
  datasource: { type: string; uid: string };
  targets: Target[];
}

describe('dashboard bothy-live.json', () => {
  const dashboard = JSON.parse(
    readFileSync(path.join(REPO, 'infra/observability/grafana/dashboards/bothy-live.json'), 'utf8')
  ) as { uid: string; title: string; panels: Panel[]; editable: boolean };

  it('is the dashboard bothy-live with one panel per view', () => {
    expect(dashboard.uid).toBe('bothy-live');
    expect(dashboard.title.endsWith(' / Live')).toBe(true);
    expect(dashboard.editable).toBe(false);
    expect(dashboard.panels.map((p) => p.type)).toEqual(VIEW_NAMES.map(() => 'bothy-live-panel'));
    expect(dashboard.panels.map((p) => p.options.view)).toEqual([...VIEW_NAMES]);
  });

  it('carries exactly the queries of queries.yaml per view', () => {
    for (const panel of dashboard.panels) {
      const view = panel.options.view as (typeof VIEW_NAMES)[number];
      expect(panel.title).toBe(QUERIES[view].title);
      expect(panel.datasource).toEqual({ type: 'prometheus', uid: 'prometheus' });
      expect(panel.targets).toEqual(
        QUERIES[view].queries.map((q) => ({
          refId: q.refId,
          datasource: { type: 'prometheus', uid: 'prometheus' },
          expr: q.expr,
          legendFormat: q.legend,
          instant: q.instant,
          range: !q.instant,
        }))
      );
    }
  });
});

describe('Grafana pin', () => {
  it('matches the Grafana version of the Compose stack', () => {
    const env = readFileSync(path.join(REPO, 'infra/compose/.env.example'), 'utf8');
    const pinned = /^BOTHY_GRAFANA_VERSION=(\S+)$/m.exec(env)?.[1];
    const compose = readFileSync(path.join(REPO, 'infra/compose/compose.yaml'), 'utf8');
    expect(compose).toContain(`grafana/grafana:\${BOTHY_GRAFANA_VERSION:-${pinned}}`);
    expect(pluginJson.dependencies.grafanaDependency).toBe(`>=${pinned}`);
    const pkg = JSON.parse(readFileSync(path.join(ROOT, 'package.json'), 'utf8')) as {
      dependencies: Record<string, string>;
    };
    expect(pkg.dependencies['@grafana/data']).toBe(pinned);
    expect(pkg.dependencies['@grafana/ui']).toBe(pinned);
  });
});

describe('component version', () => {
  it('is the VERSION file, also in package.json', () => {
    const version = readFileSync(path.join(ROOT, 'VERSION'), 'utf8').trim();
    const pkg = JSON.parse(readFileSync(path.join(ROOT, 'package.json'), 'utf8')) as { version: string };
    expect(pkg.version).toBe(version);
    expect(readFileSync(path.join(ROOT, 'CHANGELOG.md'), 'utf8')).toMatch(/^## \[Unreleased\]$/m);
  });
});

describe('logo', () => {
  it('uses only colours of the design tokens', () => {
    const svg = readFileSync(path.join(ROOT, 'src/img/logo.svg'), 'utf8');
    const used = svg.match(/#[0-9a-fA-F]{3,8}\b/g) ?? [];
    expect(used.length).toBeGreaterThan(0);
    const tokens = JSON.stringify([light, dark]).toLowerCase();
    for (const colour of used) {
      expect(tokens).toContain(`"${colour.toLowerCase()}"`);
    }
    expect(svg).not.toMatch(/rgba?\(|hsla?\(/);
  });
});

describe('osv-scanner.toml', () => {
  // The plugin bundles only its own code: webpack marks React, Emotion and @grafana/* external
  // (.config/bundler/externals.ts), Grafana provides them at runtime. A license or vulnerability
  // override may therefore name only a package that is a build or test tool (dev) or that is
  // reached only through those externals; a package that would enter dist/ never qualifies.
  const EXTERNALS = new Set(['react', 'react-dom', '@emotion/css', '@grafana/data', '@grafana/ui']);
  const lock = JSON.parse(readFileSync(path.join(ROOT, 'package-lock.json'), 'utf8')) as {
    packages: Record<string, { dependencies?: Record<string, string>; dev?: boolean }>;
  };

  /** Packages reachable from the production dependencies without passing an external. */
  function bundled(): Set<string> {
    const seen = new Set<string>();
    const root = lock.packages[''];
    const todo: Array<[string, string]> = Object.keys(root?.dependencies ?? {}).map((n) => [n, '']);
    while (todo.length) {
      const [name, from] = todo.pop()!;
      if (EXTERNALS.has(name) || seen.has(name)) {
        continue;
      }
      seen.add(name);
      const key = [from ? `${from}/node_modules/${name}` : '', `node_modules/${name}`].find(
        (k) => k && lock.packages[k]
      );
      const deps = key ? (lock.packages[key]?.dependencies ?? {}) : {};
      for (const dep of Object.keys(deps)) {
        todo.push([dep, key ?? '']);
      }
    }
    return seen;
  }

  it('overrides no package that enters the bundle', () => {
    const toml = readFileSync(path.join(ROOT, 'osv-scanner.toml'), 'utf8');
    const names = [...toml.matchAll(/^name = "([^"]+)"$/gm)].map((m) => m[1]!);
    expect(names.length).toBeGreaterThan(0);
    const inBundle = bundled();
    expect(inBundle.size).toBe(0);
    for (const name of names) {
      expect(inBundle.has(name)).toBe(false);
      const entries = Object.entries(lock.packages).filter(([k]) => k.endsWith(`node_modules/${name}`));
      expect(entries.length).toBeGreaterThan(0);
    }
  });

  it('gives every entry a reason', () => {
    const toml = readFileSync(path.join(ROOT, 'osv-scanner.toml'), 'utf8');
    const blocks = toml.split(/^\[\[/m).slice(1);
    for (const block of blocks) {
      expect(block).toMatch(/^reason = ".{20,}"$/m);
    }
  });
});
