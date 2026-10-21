// SPDX-License-Identifier: Apache-2.0
import { PanelPlugin } from '@grafana/data';

import pluginJson from './plugin.json';
import { plugin } from './module';
import { VIEW_NAMES } from './queries.gen';
import { DEFAULT_OPTIONS } from './types';

describe('module', () => {
  it('exports the panel plugin', () => {
    expect(plugin).toBeInstanceOf(PanelPlugin);
  });

  it('defaults to the live topology', () => {
    expect(DEFAULT_OPTIONS.view).toBe('topology');
    expect(VIEW_NAMES).toContain(DEFAULT_OPTIONS.view);
  });

  it('is the panel bothy-live-panel for the pinned Grafana', () => {
    expect(pluginJson.id).toBe('bothy-live-panel');
    expect(pluginJson.type).toBe('panel');
    expect(pluginJson.dependencies.grafanaDependency).toBe('>=13.2.3');
  });
});
