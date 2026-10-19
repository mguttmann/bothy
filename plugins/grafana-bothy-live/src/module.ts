// SPDX-License-Identifier: Apache-2.0
/**
 * Entry point of the panel plugin bothy-live-panel: Grafana loads `plugin` from module.js.
 * The options select one of the five views of docs/spec/25-visualisierung.md section 8.2; the
 * dashboard bothy-live.json sets one panel per view with the targets of queries.yaml.
 */

import { PanelPlugin } from '@grafana/data';

import { LivePanel } from './components/LivePanel';
import { QUERIES, VIEW_NAMES } from './queries.gen';
import { DEFAULT_OPTIONS, type LiveOptions } from './types';

/** The panel plugin with its options. */
export const plugin = new PanelPlugin<LiveOptions>(LivePanel).setPanelOptions((builder) =>
  builder
    .addRadio({
      path: 'view',
      name: 'View',
      description: 'Which live view this panel shows; the targets must be the ones of queries.yaml for that view.',
      defaultValue: DEFAULT_OPTIONS.view,
      settings: { options: VIEW_NAMES.map((v) => ({ value: v, label: QUERIES[v].title })) },
    })
    .addNumberInput({
      path: 'warnBudgetRatio',
      name: 'Warn below remaining budget',
      description: 'Below this remaining error budget ratio the traffic light shows "budget tight".',
      defaultValue: DEFAULT_OPTIONS.warnBudgetRatio,
      settings: { min: 0, max: 1, step: 0.05 },
      showIf: (o) => o.view === 'executive',
    })
    .addBooleanSwitch({
      path: 'showCosts',
      name: 'Show costs',
      defaultValue: DEFAULT_OPTIONS.showCosts,
      showIf: (o) => o.view === 'topology' || o.view === 'backend',
    })
);
